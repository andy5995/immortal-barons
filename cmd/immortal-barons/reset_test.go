package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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

	league := &leagueSetup{BoardID: "BravoBBS", Inbound: in, Outbound: out}
	if err := runReset(cfg, false, league, charsetUTF8, true); err != nil {
		t.Fatalf("runReset: %v", err)
	}

	got, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IBBS {
		t.Error("a league reset should leave inter-BBS play on")
	}
	if got.BoardID != "BravoBBS" {
		t.Errorf("BoardID = %q, want BravoBBS", got.BoardID)
	}
	if got.Inbound() != in || got.Outbound() != out {
		t.Errorf("packet dirs = %q / %q, want %q / %q", got.Inbound(), got.Outbound(), in, out)
	}
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
	league := &leagueSetup{BoardID: "Alpha BBS", Outbound: out}
	if err := runReset(cfg, false, league, charsetUTF8, true); err != nil {
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
	if err := runReset(cfg, false, &leagueSetup{ImportPath: path}, charsetUTF8, true); err != nil {
		t.Fatalf("runReset: %v", err)
	}

	got, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.BoardID != "Avalon" {
		t.Errorf("BoardID = %q, want Avalon", got.BoardID)
	}
	if got.LeagueNumber != 900 {
		t.Errorf("LeagueNumber = %d, want 900", got.LeagueNumber)
	}
	if got.Inbound() != filepath.Join(dir, "fd-files") {
		t.Errorf("Inbound = %q, want the file's incoming-files directory", got.Inbound())
	}
	// BRE's netmail directory holds .MSG files; IB's outbound holds the packets
	// themselves. Reading one as the other would point the board at a directory
	// its mailer treats as something else entirely.
	if got.Outbound() == filepath.Join(dir, "fd-netmail") {
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

	league := &leagueSetup{BoardID: "New Name", ImportPath: path}
	if err := runReset(cfg, false, league, charsetUTF8, true); err != nil {
		t.Fatalf("runReset: %v", err)
	}
	got, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.BoardID != "New Name" {
		t.Errorf("BoardID = %q, want the flag to win", got.BoardID)
	}
	if got.LeagueNumber != 900 {
		t.Errorf("LeagueNumber = %d, want the imported 900", got.LeagueNumber)
	}
}
