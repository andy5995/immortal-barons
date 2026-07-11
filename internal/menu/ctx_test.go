package menu

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestCtxActiveIsPerSession(t *testing.T) {
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	a := w.AddHuman("alice", "Alice")
	b := w.AddHuman("bob", "Bob")

	ca := &ctx{World: w, handle: "alice"}
	cb := &ctx{World: w, handle: "bob"}

	if ca.Player() != a {
		t.Fatalf("ca.Player() = %v, want alice", ca.Player())
	}
	if cb.Player() != b {
		t.Fatalf("cb.Player() = %v, want bob", cb.Player())
	}
	// They share the same world data (liveness) but resolve different empires.
	if ca.World != cb.World {
		t.Fatal("both sessions must share one World")
	}
}

// TestCtxPlayerSurvivesReload proves the handle indirection: after the world's
// empire slice is replaced (as the door's FileStore does on every reload), the
// ctx still returns the CURRENT empire for its handle — a cached *Empire would
// have gone stale and pointed at the discarded slice.
func TestCtxPlayerSurvivesReload(t *testing.T) {
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	old := w.AddHuman("alice", "Alice")
	c := &ctx{World: w, handle: "alice"}
	if c.Player() != old {
		t.Fatalf("Player() = %v, want the original empire", c.Player())
	}

	// Simulate a FileStore reload: replace the empire set with equivalents
	// (discarding the old pointers) and bump the reload generation, the way the
	// door's per-action reload does.
	w.Empires = nil
	fresh := w.AddHuman("alice", "Alice")
	w.MarkReloaded()

	if c.Player() != fresh {
		t.Fatalf("Player() after reload = %v, want the reloaded empire", c.Player())
	}
	if c.Player() == old {
		t.Fatal("Player() returned the stale pre-reload pointer")
	}
}
