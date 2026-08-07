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
