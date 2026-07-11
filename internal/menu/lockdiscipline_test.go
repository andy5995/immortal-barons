package menu

import (
	"strings"
	"sync"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestConcurrentBuyIsRaceFree proves buy2's world lock. It models the real
// concurrency this project creates: separate sessions sharing ONE *game.World.
// One goroutine repeatedly buys (Recruit writes p.Troopers/p.Gold); another
// repeatedly renders the scoreboard (printScores snapshots every empire's
// Troopers via NetWorth). buy2 wraps its Recruit in w.With and printScores
// snapshots under w.With, so the write and the read share the world mutex —
// no data race. Remove buy2's w.With and the unlocked Recruit write races the
// scoreboard's Troopers read, which -race reports as a DATA RACE (test fails).
// That is what gives this test teeth: it fails without the lock, passes with it.
func TestConcurrentBuyIsRaceFree(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.AICount = 1
	w := game.NewWorldSeed(cfg, 1)
	p := w.AddHuman("alice", "Alice")
	startTroopers := p.Troopers

	const iterations = 400
	// Match the price buy2's gather uses to the price Recruit actually charges
	// (w.Prices.Trooper), so gold accounting reconciles exactly.
	unitPrice := w.Prices.Trooper
	initialGold := unitPrice * iterations // enough to fund every buy
	p.Gold = initialGold

	// Two SESSIONS over one world: each goroutine drives its own ctx (the active
	// empire is per-session cached state), the realistic shape of this project's
	// concurrency. Both resolve the same shared empire p through w.
	cW := &ctx{World: w, handle: p.Owner}
	cR := &ctx{World: w, handle: p.Owner}
	price := func(_ *ctx) int { return unitPrice }
	apply := func(gw *game.World, e *game.Empire, n int) error { return gw.Recruit(e, n) }
	action := buy2("Troopers", false, price, apply)

	var wg sync.WaitGroup
	wg.Add(2)

	// Writer: buy one trooper per iteration (Recruit mutates p under w.With).
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			f := &fakeSession{keys: []rune("1\r")}
			action(f, cW)
		}
	}()

	// Reader: render the scoreboard, which reads every empire's Troopers via
	// NetWorth under w.With (a snapshot).
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			printScores(&fakeSession{}, cR)
		}
	}()

	wg.Wait()

	if p.Gold < 0 {
		t.Fatalf("gold went negative: %d", p.Gold)
	}
	// Every gold piece spent must be accounted for by a trooper gained.
	bought := p.Troopers - startTroopers
	if spent := initialGold - p.Gold; spent != bought*unitPrice {
		t.Fatalf("gold/troopers mismatch: spent %d gold but bought %d troopers at %d each",
			spent, bought, unitPrice)
	}
}

// TestBuyRefusesInsufficientGold checks buy2's error path: when the empire
// cannot afford the requested amount, Recruit's own gold precondition refuses
// it and buy2 surfaces the failure. This is a plain single-goroutine check of
// Recruit's precondition — it does NOT exercise the world lock (see
// TestConcurrentBuyIsRaceFree for that).
func TestBuyRefusesInsufficientGold(t *testing.T) {
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	p := w.AddHuman("alice", "Alice")
	p.Gold = 100
	startTroopers := p.Troopers
	c := &ctx{World: w, handle: p.Owner}

	price := func(_ *ctx) int { return 10 }
	action := buy2("Troopers", false, price, func(gw *game.World, e *game.Empire, n int) error {
		// Simulate gold being drained between the prompt and the apply.
		e.Gold = 5
		return gw.Recruit(e, n)
	})

	f := &fakeSession{keys: []rune("10\r")}
	action(f, c)

	if p.Troopers != startTroopers {
		t.Fatalf("stale purchase applied: Troopers = %d, want unchanged %d", p.Troopers, startTroopers)
	}
	if !strings.Contains(strings.ToLower(f.out.String()), "afford") {
		t.Fatalf("expected an insufficient-gold refusal, got: %q", f.out.String())
	}
}
