package play

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestSessionOnboardsAndSaves(t *testing.T) {
	cfg := cfgIn(t.TempDir()) // reuse the helper from play_test.go
	w := game.NewWorldSeed(cfg, 1)
	saved := false
	f := &fakeSession{keys: []rune(" \rKhanate\r0")} // splash, language (English), realm name, quit
	if _, err := Session(f, Identity{Handle: "Khan"}, w, cfg, "", game.MaintReport{}, func() error { saved = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if w.FindByOwner("khan") == nil {
		t.Fatal("empire should have been onboarded into the shared world")
	}
	if !saved {
		t.Fatal("Session should call save at end of session")
	}
}
