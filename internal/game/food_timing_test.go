package game

import "testing"

// Food growth is credited at TURN START now (matching BRE), so a player can sell
// or spend this turn's growth the same turn instead of losing it to end-of-turn
// spoilage. GrowFood adds it; processEconomy no longer does.
func TestGrowFoodCreditsGrowthAtTurnStart(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("a", "A")
	e.Food = 1000
	grown := w.FoodGrown(e)
	if grown <= 0 {
		t.Fatalf("test needs a positive food yield, got %d", grown)
	}
	w.GrowFood(e)
	if e.Food != 1000+grown {
		t.Errorf("GrowFood should credit this turn's growth: Food = %d, want %d", e.Food, 1000+grown)
	}
}

// processEconomy no longer re-adds growth — it only consumes and spoils. With the
// growth already credited at turn start, running it must NOT add another helping
// of FoodGrown (that would double-count and re-introduce the always-spoils bug).
func TestProcessEconomyConsumesAndSpoilsButDoesNotGrow(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("a", "A")
	e.Food = 100_000 // stock already includes this turn's growth
	consume := e.FoodUpkeep()

	w.processEconomy(e)

	afterConsume := 100_000 - consume
	wantSpoil := techLower(afterConsume*FoodSpoilPct/100, e.TechDecayFactor())
	want := afterConsume - wantSpoil
	if e.Food != want {
		t.Errorf("processEconomy should consume+spoil only, no growth re-add: Food = %d, want %d", e.Food, want)
	}
}

// The point of the timing fix: selling this turn's surplus down to consumption
// (what the sell default suggests) drains the granary to zero after feeding, so
// nothing spoils — matching BRE's "sell excess -> no decay". Simulated at the
// game layer: grow at turn start, sell everything above consumption, then the
// end-of-turn economy consumes it to 0 with 0 spoilage.
func TestSellToConsumptionYieldsNoSpoilage(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("a", "A")
	w.GrowFood(e)           // turn-start growth, now sellable
	e.Food = e.FoodUpkeep() // player sold all above consumption
	w.processEconomy(e)     // consume -> 0, spoil 5% of 0 = 0
	if e.LastSpoiled != 0 {
		t.Errorf("selling down to consumption should yield zero spoilage, got %d", e.LastSpoiled)
	}
}
