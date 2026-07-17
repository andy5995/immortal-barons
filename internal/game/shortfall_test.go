package game

import "testing"

func TestFoodShortfallPenaltyScales(t *testing.T) {
	run := func(foodFrac int) (supportDrop, leftPct int) {
		w := NewWorldSeed(DefaultConfig(), 1)
		e := w.AddHuman("me", "Mine")
		e.Support, e.Tax = 100, 12
		e.Regions, e.Land = RegionMix{}, 0
		e.People = 1_000_000
		need := e.FoodUpkeep()
		e.Food = need * foodFrac / 100 // feed foodFrac% of the need
		before := e.People
		w.PlayTurn(e, "2026-07-03")
		return 100 - e.Support, e.LastStarved * 100 / before
	}
	// ~27% fed => ~73% short: matches the live BRE point (~50 support drop).
	d73, l73 := run(27)
	t.Logf("73%% short: support drop=%d, people-left=%d%%", d73, l73)
	// Fully unfed => the max penalty.
	d100, l100 := run(0)
	t.Logf("100%% short: support drop=%d, people-left=%d%%", d100, l100)
	if d73 <= 0 || d100 <= d73 {
		t.Errorf("support drop should scale with shortfall: 73%%=%d 100%%=%d", d73, d100)
	}
	if l73 <= 0 || l100 < l73 {
		t.Errorf("emigration should scale with shortfall: 73%%=%d%% 100%%=%d%%", l73, l100)
	}
}
