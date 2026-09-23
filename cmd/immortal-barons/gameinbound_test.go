package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// fileDropRoster names its boards by addresses no FTN parser reads: a league
// whose packets are moved by something other than FTN has no reason to carry
// one. Alpha hosts Bravo and Bravo hosts Charlie, so the league routes.
var fileDropRoster = []game.LeagueNode{
	{Number: 1, Name: "Alpha BBS", Address: "alpha.example.net", City: "Here", State: "ST", Country: "USA", Hosts: []int{2}},
	{Number: 2, Name: "Bravo BBS", Address: "bravo.example.net", City: "There", State: "ST", Country: "USA", Hosts: []int{3}},
	{Number: 3, Name: "Charlie BBS", Address: "charlie.example.net", City: "Yonder", State: "ST", Country: "USA"},
}

// fileDropBoard is Bravo BBS with no FTN settings at all: its bbs.cfg names the
// game's own directories and nothing else, as a board whose mailer delivers
// straight into the game's inbound is set up.
func fileDropBoard(t *testing.T) game.Config {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"in", "out"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	bbs := "BoardID Bravo BBS\nLeagueNumber 900\nGameInbound in\nGameOutbound out\n"
	if err := os.WriteFile(filepath.Join(dir, store.BoardConfigFile), []byte(bbs), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteNodeList(filepath.Join(dir, store.NodeListFile), fileDropRoster); err != nil {
		t.Fatal(err)
	}
	cfg, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg.IBBS = true
	w := store.NewGame(cfg)
	w.LeagueNodes = fileDropRoster
	if err := store.Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// alphaWrites has Alpha BBS send one IP message to each board named, and
// returns the packets with the bytes Alpha wrote for them.
func alphaWrites(t *testing.T, roster []game.LeagueNode, to ...string) ([]game.Packet, [][]byte) {
	t.Helper()
	cfg := game.DefaultConfig()
	cfg.BoardID = "Alpha BBS"
	cfg.LeagueNumber = 900
	cfg.IBBS = true
	w := game.NewWorldSeed(cfg, 1)
	w.LeagueNodes = roster
	from := w.AddHuman("alpha", "Alpha Realm")
	staging := t.TempDir()
	// One outbox apiece: two messages for one board in one outbox travel as a
	// single packet.
	for i, board := range to {
		w.SendIPMessage(from, []string{board}, false, fmt.Sprintf("Message %d.", i))
		w.StampOutbox()
		if _, err := store.WriteOutbox(w, staging, false); err != nil {
			t.Fatal(err)
		}
	}
	var packets []game.Packet
	var raws [][]byte
	for _, name := range dirEntries(t, staging) {
		data, err := os.ReadFile(filepath.Join(staging, name))
		if err != nil {
			t.Fatal(err)
		}
		var p game.Packet
		if err := json.Unmarshal(data, &p); err != nil {
			t.Fatal(err)
		}
		if p.ToBoard == "" {
			continue // a broadcast, not one of the messages
		}
		packets, raws = append(packets, p), append(raws, data)
	}
	if len(packets) != len(to) {
		t.Fatalf("Alpha wrote %d addressed packets, want %d", len(packets), len(to))
	}
	return packets, raws
}

// transportBundle wraps packets the way the FTN transport does when node 1
// hands them on: a ZIP holding a manifest and each packet's bytes unchanged. It
// is written out here rather than borrowed from the transport, so the test pins
// the format a board of another build sends.
func transportBundle(t *testing.T, packets []game.Packet, raws [][]byte) []byte {
	t.Helper()
	type entry struct {
		Route []int `json:"route"`
	}
	manifest := struct {
		Format   int     `json:"format"`
		Delivery string  `json:"delivery"`
		Entries  []entry `json:"entries"`
	}{Format: 1, Delivery: "attach"}
	for range packets {
		manifest.Entries = append(manifest.Entries, entry{Route: []int{1}})
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	write := func(name string, data []byte) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	write("manifest.json", body)
	for i, p := range packets {
		write(fmt.Sprintf("packets/%06d/%s", i, store.PacketFilename(p, raws[i])), raws[i])
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

var planetaryModes = map[string]func(game.Config) error{
	"-planetary": func(cfg game.Config) error { return runPlanetary(cfg, false) },
	"-maint":     func(cfg game.Config) error { return runMaint(cfg, "2026-09-22") },
	"-full": func(cfg game.Config) error {
		if err := fullInbound(cfg, false); err != nil {
			return err
		}
		return fullOutbound(cfg, false)
	},
}

// #230: a bundle that the mailer delivered straight into the game's inbound is
// unwrapped and applied in the same run, on a board with FTN settings and on
// one with none, and a plain packet beside it is applied as it always was.
func TestABundleInTheGameInboundIsApplied(t *testing.T) {
	boards := map[string]struct {
		setup  func(*testing.T) game.Config
		roster []game.LeagueNode
	}{
		"FTN board":       {func(t *testing.T) game.Config { return ftnBoard(t, "") }, ftnRoster},
		"file-drop board": {fileDropBoard, fileDropRoster},
	}
	for boardName, board := range boards {
		for mode, run := range planetaryModes {
			t.Run(boardName+" "+mode, func(t *testing.T) {
				cfg := board.setup(t)
				packets, raws := alphaWrites(t, board.roster, "Bravo BBS", "Bravo BBS")
				bundled := filepath.Join(cfg.Inbound(), "00640001.BRP")
				if err := os.WriteFile(bundled, transportBundle(t, packets[:1], raws[:1]), 0o644); err != nil {
					t.Fatal(err)
				}
				plain := filepath.Join(cfg.Inbound(), store.PacketFilename(packets[1], raws[1]))
				if err := os.WriteFile(plain, raws[1], 0o644); err != nil {
					t.Fatal(err)
				}
				if err := run(cfg); err != nil {
					t.Fatalf("%s: %v", mode, err)
				}
				if !seen(t, cfg, packets[0]) {
					t.Error("the bundled packet was not applied")
				}
				if !seen(t, cfg, packets[1]) {
					t.Error("the plain packet beside the bundle was not applied")
				}
				if left := dirEntries(t, cfg.Inbound()); len(left) != 0 {
					t.Errorf("left in the game's inbound: %v", left)
				}
				if bad := filepath.Join(cfg.DataDir, store.BadDir); len(dirOrNone(t, bad)) != 0 {
					t.Errorf("set aside as unreadable: %v", dirOrNone(t, bad))
				}
			})
		}
	}
}

// A bundle entry addressed onward, on a board that cannot forward over FTN, is
// treated as the same packet arriving unbundled would be: the planetary step
// passes it on towards its board through the game's outbound.
func TestABundledPacketInTransitIsPassedOnByThePlanetaryStep(t *testing.T) {
	cfg := fileDropBoard(t)
	packets, raws := alphaWrites(t, fileDropRoster, "Charlie BBS")
	if err := os.WriteFile(filepath.Join(cfg.Inbound(), "00640001.BRP"), transportBundle(t, packets, raws), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runPlanetary(cfg, false); err != nil {
		t.Fatal(err)
	}
	if left := dirEntries(t, cfg.Inbound()); len(left) != 0 {
		t.Errorf("left in the game's inbound: %v", left)
	}
	forwarded := false
	for _, name := range dirEntries(t, cfg.Outbound()) {
		data, err := os.ReadFile(filepath.Join(cfg.Outbound(), name))
		if err != nil {
			t.Fatal(err)
		}
		var p game.Packet
		if err := json.Unmarshal(data, &p); err != nil {
			t.Fatalf("%s in the outbound is not a plain packet: %v", name, err)
		}
		if p.FromBoard == "Alpha BBS" && p.ToBoard == "Charlie BBS" && p.Seq == packets[0].Seq {
			forwarded = true
		}
	}
	if !forwarded {
		t.Errorf("the packet for Charlie was not passed on; outbound holds %v", dirEntries(t, cfg.Outbound()))
	}
}

// A file that opens like a bundle but is not one is set aside once it is too old
// to be a write in progress, so it is neither applied nor met again on every
// later run. A young one is left alone, and the planetary step does not set it
// aside as a corrupt packet in the meantime.
func TestAnUnreadableBundleInTheGameInboundIsSetAside(t *testing.T) {
	cfg := fileDropBoard(t)
	broken := filepath.Join(cfg.Inbound(), "00640002.BRP")
	if err := os.WriteFile(broken, []byte("PK\x03\x04 cut off in transfer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runPlanetary(cfg, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(broken); err != nil {
		t.Fatalf("a bundle young enough to be arriving was moved: %v", err)
	}
	if bad := dirOrNone(t, filepath.Join(cfg.DataDir, store.BadDir)); len(bad) != 0 {
		t.Errorf("the planetary step set the bundle aside as a corrupt packet: %v", bad)
	}

	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(broken, old, old); err != nil {
		t.Fatal(err)
	}
	for run := 1; run <= 2; run++ {
		if err := runPlanetary(cfg, false); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
	}
	if left := dirEntries(t, cfg.Inbound()); len(left) != 0 {
		t.Errorf("left in the game's inbound: %v", left)
	}
	if aside := dirOrNone(t, filepath.Join(cfg.DataDir, "ftn-spool", "bad")); len(aside) != 1 {
		t.Errorf("set aside by the transport: %v, want the one bundle", aside)
	}
}

// -full never waits on the transport lock for the game's inbound either, and a
// bundle it skipped stays where it is, for the run holding the lock.
func TestFullDoesNotWaitToUnwrapTheGameInbound(t *testing.T) {
	cfg := fileDropBoard(t)
	packets, raws := alphaWrites(t, fileDropRoster, "Bravo BBS")
	bundled := filepath.Join(cfg.Inbound(), "00640001.BRP")
	if err := os.WriteFile(bundled, transportBundle(t, packets, raws), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(bundled, old, old); err != nil {
		t.Fatal(err)
	}
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
	if _, err := os.Stat(bundled); err != nil {
		t.Errorf("the skipped bundle did not stay in the game's inbound: %v", err)
	}
	if seen(t, cfg, packets[0]) {
		t.Error("a bundle -full skipped was applied anyway")
	}
}

// dirOrNone lists dir, treating a directory that was never made as empty.
func dirOrNone(t *testing.T, dir string) []string {
	t.Helper()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	return dirEntries(t, dir)
}
