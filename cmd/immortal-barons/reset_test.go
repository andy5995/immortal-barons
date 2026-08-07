package main

import (
	"os"
	"path/filepath"
	"testing"

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
