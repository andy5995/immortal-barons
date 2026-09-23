package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// ftnBoard is Bravo BBS, a league member whose mailer delivers into mailer-in
// and whose netmail is written to netmail. extra is appended to its bbs.cfg.
func ftnBoard(t *testing.T, extra string) game.Config {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"in", "out", "mailer-in", "netmail"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	bbs := "BoardID Bravo BBS\nLeagueNumber 900\nGameInbound in\nGameOutbound out\n" +
		"IncomingFileDir mailer-in\nNetmailDir netmail\nSubjectPath Basename\n" + extra
	if err := os.WriteFile(filepath.Join(dir, store.BoardConfigFile), []byte(bbs), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteNodeList(filepath.Join(dir, store.NodeListFile), ftnRoster); err != nil {
		t.Fatal(err)
	}
	cfg, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg.IBBS = true
	w := store.NewGame(cfg)
	w.LeagueNodes = ftnRoster
	if err := store.Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// Every field filled: the roster parser skips an entry with an empty one.
var ftnRoster = []game.LeagueNode{
	{Number: 1, Name: "Alpha BBS", Address: "1:1/1", City: "Here", State: "ST", Country: "USA"},
	{Number: 2, Name: "Bravo BBS", Address: "1:1/2", City: "There", State: "ST", Country: "USA"},
}

// alphaPacket is a packet Alpha BBS wrote for Bravo, written into dir.
func alphaPacket(t *testing.T, dir string) game.Packet {
	t.Helper()
	cfg := game.DefaultConfig()
	cfg.BoardID = "Alpha BBS"
	cfg.LeagueNumber = 900
	cfg.IBBS = true
	w := game.NewWorldSeed(cfg, 1)
	w.LeagueNodes = ftnRoster
	w.SendIPMessage(w.AddHuman("alpha", "Alpha Realm"), []string{"Bravo BBS"}, false, "Hello, Bravo.")
	w.StampOutbox()
	staging := t.TempDir()
	if _, err := store.WriteOutbox(w, staging, false); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(staging)
	if err != nil || len(entries) == 0 {
		t.Fatalf("Alpha wrote no packet: %v", err)
	}
	var packet game.Packet
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(staging, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &packet); err != nil {
			t.Fatal(err)
		}
		if packet.ToBoard != "Bravo BBS" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
		return packet
	}
	t.Fatal("Alpha wrote nothing for Bravo")
	return packet
}

// seen reports whether Bravo's saved world has applied p.
func seen(t *testing.T, cfg game.Config, p game.Packet) bool {
	t.Helper()
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return w.SeenPacket(p)
}

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// The transport runs on either side of the planetary step in every mode that
// runs one: what the mailer delivered is unwrapped BEFORE the step, so the step
// applies it in the same run, and what the step wrote is handed off AFTER the
// save, so nothing is left in the game's outbound.
func TestPlanetaryModesRunTheTransportAroundTheStep(t *testing.T) {
	for mode, run := range map[string]func(game.Config) error{
		"-planetary": func(cfg game.Config) error { return runPlanetary(cfg, false) },
		"-maint":     func(cfg game.Config) error { return runMaint(cfg, "2026-09-22") },
		"-full": func(cfg game.Config) error {
			if err := fullInbound(cfg, false); err != nil {
				return err
			}
			return fullOutbound(cfg, false)
		},
	} {
		t.Run(mode, func(t *testing.T) {
			cfg := ftnBoard(t, "")
			packet := alphaPacket(t, filepath.Join(cfg.DataDir, "mailer-in"))
			if err := run(cfg); err != nil {
				t.Fatalf("%s: %v", mode, err)
			}
			if !seen(t, cfg, packet) {
				t.Error("the packet the mailer delivered was not applied in the same run")
			}
			if left := dirEntries(t, filepath.Join(cfg.DataDir, "mailer-in")); len(left) != 0 {
				t.Errorf("left in the mailer's inbound: %v", left)
			}
			if left := dirEntries(t, cfg.Outbound()); len(left) != 0 {
				t.Errorf("left in the game's outbound after the handoff: %v", left)
			}
			if msgs := dirEntries(t, filepath.Join(cfg.DataDir, "netmail")); len(msgs) == 0 {
				t.Error("no netmail was written for Alpha")
			}
		})
	}
}

// A failed unwrap costs the run nothing it already has: the packets waiting in
// the game's own inbound are applied, and the run succeeds.
func TestAFailedUnwrapStillAppliesTheGameInbound(t *testing.T) {
	cfg := ftnBoard(t, "")
	// A file where the mailer's directory should be: reading it fails.
	incoming := filepath.Join(cfg.DataDir, "mailer-in")
	if err := os.Remove(incoming); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(incoming, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	packet := alphaPacket(t, cfg.Inbound())
	if err := runPlanetary(cfg, false); err != nil {
		t.Fatalf("-planetary after a failed unwrap: %v", err)
	}
	if !seen(t, cfg, packet) {
		t.Error("the packet already in the game's inbound was not applied")
	}
}

// A failed handoff comes after the save, so the run's work is kept, and it
// raises the alarm: a non-zero exit and the OnFault hook. -full raises the hook
// and never the exit code, since a caller is waiting behind it.
func TestAFailedHandoffKeepsTheWorkAndRaisesTheAlarm(t *testing.T) {
	for _, mode := range []string{"-planetary", "-full"} {
		t.Run(mode, func(t *testing.T) {
			cfg := ftnBoard(t, "")
			hookOut := filepath.Join(cfg.DataDir, "hook.out")
			cfg.OnFault = `printf '%s' "$IB_FAULTS" > "` + hookOut + `"`
			packet := alphaPacket(t, cfg.Inbound())
			// The netmail directory is gone, so no attach can be written.
			if err := os.Remove(filepath.Join(cfg.DataDir, "netmail")); err != nil {
				t.Fatal(err)
			}
			var err error
			if mode == "-planetary" {
				err = runPlanetary(cfg, false)
				if !errors.Is(err, errFaults) {
					t.Errorf("a failed handoff returned %v, want errFaults", err)
				}
			} else {
				if err = fullInbound(cfg, false); err == nil {
					err = fullOutbound(cfg, false)
				}
				if err != nil {
					t.Errorf("-full ended with %v; a caller is waiting behind it", err)
				}
			}
			if !seen(t, cfg, packet) {
				t.Error("the planetary work was lost to the failed handoff")
			}
			if runtime.GOOS != "windows" {
				got, err := os.ReadFile(hookOut)
				if err != nil {
					t.Fatalf("the OnFault hook did not run: %v", err)
				}
				if !strings.Contains(string(got), "FTN handoff failed") {
					t.Errorf("the hook was told %q", got)
				}
			}
		})
	}
}

// A caller's session never waits on the transport lock: another run holding it
// is doing the same work, so -full skips its half and goes on.
func TestFullSkipsTheTransportWhenItsLockIsHeld(t *testing.T) {
	cfg := ftnBoard(t, "")
	alphaPacket(t, filepath.Join(cfg.DataDir, "mailer-in"))
	held, err := store.LockPath(filepath.Join(cfg.DataDir, "barons-ftn.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	done := make(chan error, 1)
	go func() {
		err := fullInbound(cfg, false)
		if err == nil {
			err = fullOutbound(cfg, false)
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("-full waited on the transport lock")
	}
	if left := dirEntries(t, filepath.Join(cfg.DataDir, "mailer-in")); len(left) != 1 {
		t.Errorf("the mailer's inbound holds %v; the skipped unwrap should have left it alone", left)
	}
}

// A board off the league runs no transport under -full, even with transport
// lines set: runFull checks those settings only for a league board, so it must
// not act on them for any other.
func TestFullRunsNoTransportOffTheLeague(t *testing.T) {
	cfg := ftnBoard(t, "")
	alphaPacket(t, filepath.Join(cfg.DataDir, "mailer-in"))
	cfg.IBBS = false
	if err := fullInbound(cfg, false); err != nil {
		t.Fatal(err)
	}
	if left := dirEntries(t, filepath.Join(cfg.DataDir, "mailer-in")); len(left) != 1 {
		t.Errorf("the mailer's inbound holds %v; a board off the league unwrapped it", left)
	}
}

// A board with no transport lines runs exactly as before: nothing is unwrapped
// or handed off, and nothing complains.
func TestNoTransportLinesLeavesTheModesAlone(t *testing.T) {
	cfg := leagueBoard(t)
	if err := runPlanetary(cfg, false); err != nil {
		t.Fatalf("-planetary on a file-drop board: %v", err)
	}
}

// The modes that move packets refuse settings an older version wrote, with the
// lines to paste; a door session is not among them.
func TestPacketModesRefuseOldSettings(t *testing.T) {
	for name, body := range map[string]string{
		store.BoardConfigFile: "BoardID Bravo BBS\nLeagueNumber 900\nInbound in\n",
		"ftn.cfg":             "InboundDir mailer-in\n",
	} {
		cfg := leagueBoard(t)
		if err := os.WriteFile(filepath.Join(cfg.DataDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		for mode, run := range map[string]func() error{
			"-maint":      func() error { return runMaint(cfg, "2026-09-22") },
			"-planetary":  func() error { return runPlanetary(cfg, false) },
			"-full":       func() error { return runFull(cfg, "tester", "2026-09-22", 0, true, false) },
			"-ftn-status": func() error { return runFTNStatus(cfg) },
		} {
			err := run()
			if err == nil || !strings.Contains(err.Error(), "  ") {
				t.Errorf("%s with an old %s returned %v, want a refusal with lines to paste", mode, name, err)
			}
		}
	}
}
