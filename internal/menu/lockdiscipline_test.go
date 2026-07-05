package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestBuyRevalidatesUnderLock(t *testing.T) {
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	p := w.AddHuman("alice", "Alice")
	p.Gold = 100
	startTroopers := p.Troopers
	c := &ctx{World: w, active: p}

	price := func(_ *ctx) int { return 10 }
	apply := func(gw *game.World, e *game.Empire, n int) error { return gw.Recruit(e, n) }
	action := buy2("Troopers", false, price, func(gw *game.World, e *game.Empire, n int) error {
		// Simulate another session draining gold between the prompt and the apply.
		e.Gold = 5
		return apply(gw, e, n)
	})

	// Ask to buy 10 (affordable at prompt time: 100/10). Under lock, gold is 5.
	f := &fakeSession{keys: []rune("10\r")}
	action(f, c)

	if p.Troopers != startTroopers {
		t.Fatalf("stale purchase applied: Troopers = %d, want unchanged %d", p.Troopers, startTroopers)
	}
	if !strings.Contains(strings.ToLower(f.out.String()), "gold") {
		t.Fatalf("expected an insufficient-gold refusal, got: %q", f.out.String())
	}
}
