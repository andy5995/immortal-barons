package game

import "testing"

// TestSupportDriftsWithTax checks that a high tax rate erodes Support over
// several turns, while the stable tax rate (15%) holds it near 100.
func TestSupportDriftsWithTax(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("hightax", "High Tax Realm")
	e.Tax = 39
	for i := 0; i < 10; i++ {
		w.GrowFood(e) // keep the realm fed so Support moves from tax, not starvation
		w.PlayTurn(e, "2026-07-03")
	}
	if e.Support >= 100 {
		t.Errorf("high tax should erode Support, got %d", e.Support)
	}

	w2 := NewWorldSeed(DefaultConfig(), 1)
	stable := w2.AddHuman("stable", "Stable Realm")
	stable.Tax = SupportTaxNeutral - SupportTaxDivisor // one point of free recovery a turn
	stable.Support = 50                                // start low to confirm it drifts back up
	for i := 0; i < 60; i++ {
		w2.GrowFood(stable) // keep it fed so Support returns from tax stability, not starving
		w2.PlayTurn(stable, "2026-07-03")
	}
	if stable.Support < 90 {
		t.Errorf("a modest tax rate should return Support near 100, got %d", stable.Support)
	}
}

// The per-turn drift pivots on SupportTaxNeutral: below it a realm recovers
// support for free, above it the realm bleeds support even without a riot.
// Binary-verified — Support -= (Tax - 30) / 10 every turn.
func TestSupportDriftPivotsOnNeutralTax(t *testing.T) {
	run := func(tax, turns int) int {
		w := NewWorldSeed(DefaultConfig(), 1)
		e := w.AddHuman("e", "E")
		e.Tax, e.Support = tax, 50
		for range turns {
			w.GrowFood(e)
			w.PlayTurn(e, "2026-07-03")
		}
		return e.Support
	}
	// At the neutral rate the drift is zero, and the rate is low enough that
	// riots are the only thing that can move support.
	if got := run(SupportTaxNeutral, 1); got != 50 {
		t.Errorf("at the neutral rate support should not drift, got %d", got)
	}
	if got := run(SupportTaxNeutral-2*SupportTaxDivisor, 1); got != 52 {
		t.Errorf("two divisors below neutral should gain 2 points, got %d", got)
	}
	if got := run(SupportTaxNeutral+2*SupportTaxDivisor, 1); got > 48 {
		t.Errorf("two divisors above neutral should lose at least 2 points, got %d", got)
	}
}

// TestLowSupportCutsCoastalIncome checks that Coastal income scales down
// with Support, isolating the effect by giving two otherwise-identical
// empires only Coastal regions.
func TestLowSupportCutsCoastalIncome(t *testing.T) {
	cfg := DefaultConfig()

	wHigh := NewWorldSeed(cfg, 1)
	high := wHigh.AddHuman("high", "High Support")
	high.Regions = RegionMix{Coastal: 100}
	high.syncLand()
	high.Support = 100
	high.Tax = 0
	highStart := high.Gold

	wLow := NewWorldSeed(cfg, 1)
	low := wLow.AddHuman("low", "Low Support")
	low.Regions = RegionMix{Coastal: 100}
	low.syncLand()
	low.Support = 20
	low.Tax = 0
	lowStart := low.Gold

	wHigh.CollectIncome(high)
	wLow.CollectIncome(low)

	highGain := high.Gold - highStart
	lowGain := low.Gold - lowStart
	if lowGain >= highGain {
		t.Errorf("low-support empire should earn less gold: high=%d low=%d", highGain, lowGain)
	}
}

// TestHighTaxCanRiot checks that a sustained high tax rate can trigger
// riots, while the stable tax rate never does, across the same seeded runs.
func TestHighTaxCanRiot(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 42)
	e := w.AddHuman("rioter", "Riot Realm")
	e.Tax = 39

	// A turn runs population growth AND (maybe) a riot, so a riot turn can
	// still net a population gain when logistic growth outweighs the riot's
	// People/15 loss. We can't isolate the riot's People effect from a full
	// PlayTurn, so here we only assert a riot never *raises* Support.
	sawRiot := false
	for i := 0; i < 200; i++ {
		supportBefore := e.Support
		w.PlayTurn(e, "2026-07-03")
		if e.LastRiot {
			sawRiot = true
			if e.Support > supportBefore {
				t.Errorf("riot should not raise Support: before=%d after=%d", supportBefore, e.Support)
			}
		}
	}
	if !sawRiot {
		t.Error("expected at least one riot at Tax=39 across 200 turns")
	}

	// At/below the riot tax floor (BRE: riots need tax > 10), no riot can ever
	// fire — tax 15 CAN riot (~2.25%/turn per tax^2/10000), so the floor is the
	// real no-riot guarantee.
	w2 := NewWorldSeed(DefaultConfig(), 42)
	stable := w2.AddHuman("stable", "Stable Realm")
	stable.Tax = RiotTaxFloor
	for i := 0; i < 200; i++ {
		w2.PlayTurn(stable, "2026-07-03")
		if stable.LastRiot {
			t.Error("no riot should fire at or below the riot tax floor")
		}
	}
}
