package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// A member board is set up in one command: naming the board skips the settings
// editor, which a test has no terminal for anyway. The league ruleset is not
// this board's to choose — it arrives in the Coordinator's next broadcast.
func TestIBBSResetWithBoardIDSkipsTheEditor(t *testing.T) {
	dir := t.TempDir()
	cfg, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	in, out := filepath.Join(dir, "ftn-in"), filepath.Join(dir, "filebox")

	out2 := captureReset(t, cfg, &leagueSetup{BoardID: "BravoBBS", Inbound: in, Outbound: out})

	got, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IBBS {
		t.Error("a league reset should leave inter-BBS play on")
	}
	// The settings that name the board are handed back as bbs.cfg lines: the
	// game reads that file and never writes it (#152).
	wantsLine(t, out2, "BoardID BravoBBS")
	wantsLine(t, out2, "Inbound "+in)
	wantsLine(t, out2, "Outbound "+out)
	// The reset creates them, so the first -planetary run has somewhere to read.
	for _, d := range []string{in, out} {
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			t.Errorf("packet directory %s was not created (%v)", d, err)
		}
	}
}

// The Coordinator's broadcast must be signed on its way out. -league-config
// writes the outbox directly rather than through RunPlanetary, so it has to do
// the stamping itself; an unsigned ruleset is refused by every board that
// receives it, and the sysop sees a key problem that isn't one.
func TestLeagueConfigBroadcastIsSigned(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	if _, err := store.GenerateCoordKey(dir); err != nil {
		t.Fatal(err)
	}
	cfg, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	captureReset(t, cfg, &leagueSetup{BoardID: "Alpha BBS", Outbound: out})
	// The sysop's half of the setup: the reset prints these, they type them.
	if err := os.WriteFile(filepath.Join(dir, store.BoardConfigFile),
		[]byte("BoardID Alpha BBS\nOutbound "+out+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteNodeList(filepath.Join(dir, store.NodeListFile),
		[]game.LeagueNode{
			{Number: 1, Name: "Alpha BBS", Address: "1:1/1", City: "Local", State: "XX", Country: "USA"},
			{Number: 2, Name: "Bravo BBS", Address: "1:1/2", City: "Local", State: "XX", Country: "USA"},
		}); err != nil {
		t.Fatal(err)
	}
	if err := runLeagueConfig(cfg); err != nil {
		t.Fatalf("runLeagueConfig: %v", err)
	}

	// The outbound directory is fresh, so whatever is in it is the broadcast.
	files, err := filepath.Glob(filepath.Join(out, "*"))
	if err != nil || len(files) != 1 {
		t.Fatalf("expected one packet in %s, got %v (%v)", out, files, err)
	}
	raw, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var p game.Packet
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Signature) == 0 {
		t.Error("the broadcast ruleset went out unsigned; every board will refuse it")
	}
	if p.Seq == 0 {
		t.Error("the broadcast has no sequence number, so it is exempt from replay detection")
	}
}

// A sysop converting a league they already run points -ibbs-reset at their
// existing BRE BBS.CFG rather than retyping it. Its layout is BRE's own, from
// docs/BBS.SAM in the original distribution.
func TestIBBSResetImportsABREBoardConfig(t *testing.T) {
	dir := t.TempDir()
	cfg, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "BBS.CFG.orig")
	body := "John Smith\nAvalon\n363/277\n" + filepath.Join(dir, "fd-files") +
		"\n" + filepath.Join(dir, "fd-netmail") + "\n900\nFRONTDOOR\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	// No -board-id: the imported name is what skips the editor, which a test
	// has no terminal for.
	out := captureReset(t, cfg, &leagueSetup{ImportPath: path})

	wantsLine(t, out, "BoardID Avalon")
	wantsLine(t, out, "LeagueNumber 900")
	wantsLine(t, out, "Inbound "+filepath.Join(dir, "fd-files"))
	// BRE's netmail directory holds .MSG files; IB's outbound holds the packets
	// themselves. Reading one as the other would point the board at a directory
	// its mailer treats as something else entirely.
	if strings.Contains(out, filepath.Join(dir, "fd-netmail")) {
		t.Error("the netmail directory was read as the outbound directory")
	}
}

// An explicit flag beats the file, so a sysop can import the rest and still
// rename the board.
func TestBoardIDFlagBeatsTheImportedName(t *testing.T) {
	dir := t.TempDir()
	cfg, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "BBS.CFG.orig")
	if err := os.WriteFile(path, []byte("Sysop\nOld Name\n1:1/1\nin\nnet\n900\nBINKLEY\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureReset(t, cfg, &leagueSetup{BoardID: "New Name", ImportPath: path})

	wantsLine(t, out, "BoardID New Name")
	wantsLine(t, out, "LeagueNumber 900")
}

// captureReset runs a reset with stdout redirected, because the per-board
// settings are now printed for the sysop to paste into bbs.cfg rather than
// written (#152) — the printout is the only place they land.
func captureReset(t *testing.T, cfg game.Config, league *leagueSetup) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	runErr := runReset(cfg, false, league, charsetUTF8, true)
	os.Stdout = saved
	w.Close()
	out, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		t.Fatalf("runReset: %v (output: %s)", runErr, out)
	}
	return string(out)
}

// wantsLine fails unless the printed bbs.cfg carries this exact setting line.
func wantsLine(t *testing.T, out, line string) {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimRight(l, "\r") == line {
			return
		}
	}
	t.Errorf("the printed %s has no %q line:\n%s", store.BoardConfigFile, line, out)
}

// The reset that started #152: a board with its identity already set had all
// four settings returned to defaults. Nothing about a rules reset may touch
// that file.
func TestIBBSResetLeavesAnExistingBoardConfigAlone(t *testing.T) {
	dir := t.TempDir()
	own := "BoardID Alpha BBS\nLeagueNumber 900\nInbound ftn/in\nOutbound ftn/out\n"
	path := filepath.Join(dir, store.BoardConfigFile)
	if err := os.WriteFile(path, []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	captureReset(t, cfg, &leagueSetup{BoardID: "Alpha BBS"})

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != own {
		t.Errorf("%s was rewritten by the reset:\n%s", store.BoardConfigFile, got)
	}
	after, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if after.LeagueNumber != 900 || after.InboundDir != "ftn/in" {
		t.Errorf("the board lost its own settings: %+v", after)
	}
}
