package menu

import (
	"strings"
	"sync"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestConcurrentBuyIsRaceFree proves buyUnit's world lock. It models the real
// concurrency this project creates: separate sessions sharing ONE *game.World.
// One goroutine repeatedly buys (Recruit writes p.Troopers/p.Gold); another
// repeatedly renders the scoreboard (printScores snapshots every empire's
// Troopers via NetWorth). buyUnit wraps its Recruit in w.With and printScores
// snapshots under w.With, so the write and the read share the world mutex —
// no data race. Remove buyUnit's w.With and the unlocked Recruit write races the
// scoreboard's Troopers read, which -race reports as a DATA RACE (test fails).
// That is what gives this test teeth: it fails without the lock, passes with it.
func TestConcurrentBuyIsRaceFree(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.AICount = 1
	w := game.NewWorldSeed(cfg, 1)
	p := w.AddHuman("alice", "Alice")
	startTroopers := p.Troopers

	const iterations = 400
	// Match the price buyUnit's gather uses to the price Recruit actually charges
	// (the per-turn fluctuating TrooperPrice; constant here since no turn is
	// played), so gold accounting reconciles exactly.
	unitPrice := w.TrooperPrice(p)
	initialGold := int64(unitPrice * iterations) // enough to fund every buy
	p.Gold = initialGold

	// Two SESSIONS over one world: each goroutine drives its own ctx (the active
	// empire is per-session cached state), the realistic shape of this project's
	// concurrency. Both resolve the same shared empire p through w.
	cW := &ctx{World: w, handle: p.Owner}
	cR := &ctx{World: w, handle: p.Owner}
	price := func(_ *ctx) int { return unitPrice }
	apply := func(gw *game.World, e *game.Empire, n int) error { return gw.Recruit(e, n) }
	action := buyUnit("Troopers", false, price, apply)

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
	if spent := initialGold - p.Gold; spent != int64(bought*unitPrice) {
		t.Fatalf("gold/troopers mismatch: spent %d gold but bought %d troopers at %d each",
			spent, bought, unitPrice)
	}
}

// TestBuyRefusesInsufficientGold checks buyUnit's error path: when the empire
// cannot afford the requested amount, Recruit's own gold precondition refuses
// it and buyUnit surfaces the failure. This is a plain single-goroutine check of
// Recruit's precondition — it does NOT exercise the world lock (see
// TestConcurrentBuyIsRaceFree for that).
func TestBuyRefusesInsufficientGold(t *testing.T) {
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	p := w.AddHuman("alice", "Alice")
	p.Gold = 100
	startTroopers := p.Troopers
	c := &ctx{World: w, handle: p.Owner}

	price := func(_ *ctx) int { return 10 }
	action := buyUnit("Troopers", false, price, func(gw *game.World, e *game.Empire, n int) error {
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

// TestConcurrentGroupAttackGatherIsRaceFree proves createGroupAttack's world
// lock (#206), in the same shape as the buy test above: one goroutine imports
// remote boards the way an inbound packet does, another opens Create Group
// Attack and backs out at the planet prompt. The gather runs before that
// prompt, so backing out still exercises it.
//
// ImportBoard both assigns into RemoteBoards and appends to it, so an unlocked
// walk of that slice is a torn read of its header rather than a stale-but-whole
// value. With the gather inside w.Read the two share the world mutex; take the
// Read away and -race reports a DATA RACE, which is what gives this teeth.
func TestConcurrentGroupAttackGatherIsRaceFree(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.AICount = 1
	cfg.IBBS = true
	w := game.NewWorldSeed(cfg, 1)
	p := w.AddHuman("alice", "Alice")
	p.Protection = 0 // a new realm cannot attack, and would return before the gather

	const iterations = 300
	cR := &ctx{World: w, handle: p.Owner}
	cW := &ctx{World: w, handle: p.Owner}

	var wg sync.WaitGroup
	wg.Add(2)

	// Writer: import boards, as ApplyPacket does when scores arrive. Assigning
	// over an existing entry and appending a new one are both exercised.
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			board := game.RemoteBoard{
				BoardID: "Planet " + string(rune('A'+i%8)),
				Scores:  []game.RemoteScore{{Empire: "Baron", Land: i, NetWorth: i}},
			}
			cW.With(func() { cW.ImportBoard(board) })
		}
	}()

	// Reader: open Create Group Attack and quit at the planet prompt.
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			f := &fakeSession{keys: []rune("0\r")}
			createGroupAttack(f, cR)
		}
	}()

	wg.Wait()
	var boards int
	w.Read(func() { boards = len(w.RemoteBoards) })
	if boards == 0 {
		t.Fatal("no boards were imported, so the reader never had a slice to walk")
	}
	// The reader must have REACHED the gather, not returned above it: a new
	// realm is blocked from attacking before any of this runs, and a test that
	// stopped there would pass with the lock removed (AGENTS.md).
	f := &fakeSession{keys: []rune("0\r")}
	createGroupAttack(f, cR)
	if out := f.out.String(); !strings.Contains(out, "Target which planet?") {
		t.Fatalf("the reader never reached the planet gather:\n%s", out)
	}
}
