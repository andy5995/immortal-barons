package game

import (
	"fmt"
	"testing"
)

// The AI keeps a working reserve back for food and maintenance, and BOTH of its
// spenders have to respect it. Buying forces out of a share of all gold — which
// is what it used to do — meant the reserve was only ever spent on soldiers, so
// a realm could never save past it, and aiExpandLand, which refuses to touch it,
// stopped buying land for good.
func TestTheAIDoesNotBuyForcesOutOfItsWorkingReserve(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("ai", "Reserved")
	e.Regions = RegionMix{Agricultural: 500}
	e.syncLand()
	e.Protection = 0
	e.Food = 1_000_000
	e.Gold = w.aiReserve(e) // exactly the reserve and not a coin more

	before := e.Gold
	w.aiBuildForces(e)

	if e.Gold != before {
		t.Errorf("the AI spent %d of its %d reserve on forces; only the surplus above it is spendable",
			before-e.Gold, before)
	}
}

// And the effect that matters: a realm left alone goes on expanding instead of
// freezing at whatever size it had reached when its upkeep caught up with its
// income. Run over several seeds — one trajectory would not have caught the
// freeze, which struck two realms in some runs and none in others.
func TestAnUnthreatenedAIKeepsExpanding(t *testing.T) {
	for seed := int64(1); seed <= 4; seed++ {
		cfg := DefaultConfig()
		cfg.AICount = 1 // alone on the planet, so nothing can take its land
		w := NewWorldSeed(cfg, seed)

		day := func(n int) { w.DailyMaintenance(fmt.Sprintf("2026-09-%02d", n)) }
		for n := 1; n <= 15; n++ {
			day(n)
		}
		ai := w.AIEmpires()[0]
		mid := ai.Land
		for n := 16; n <= 30; n++ {
			day(n)
		}

		if ai.Land <= mid {
			t.Errorf("seed %d: land went %d -> %d over the second fortnight; an unopposed realm should still be growing",
				seed, mid, ai.Land)
		}
	}
}
