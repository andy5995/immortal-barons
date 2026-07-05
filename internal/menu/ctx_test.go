package menu

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestCtxActiveIsPerSession(t *testing.T) {
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	a := w.AddHuman("alice", "Alice")
	b := w.AddHuman("bob", "Bob")

	ca := &ctx{World: w, active: a}
	cb := &ctx{World: w, active: b}

	if ca.Player() != a {
		t.Fatalf("ca.Player() = %v, want alice", ca.Player())
	}
	if cb.Player() != b {
		t.Fatalf("cb.Player() = %v, want bob", cb.Player())
	}
	// They share the same world data (liveness) but not the active pointer.
	if ca.World != cb.World {
		t.Fatal("both sessions must share one World")
	}
}
