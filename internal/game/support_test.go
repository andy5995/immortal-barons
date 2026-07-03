package game

import "testing"

// TestSupportDriftsWithTax checks that a high tax rate erodes Support over
// several turns, while the stable tax rate (15%) holds it near 100.
func TestSupportDriftsWithTax(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("hightax", "High Tax Realm")
	e.Tax = 39
	for i := 0; i < 10; i++ {
		w.PlayTurn(e, "2026-07-03")
	}
	if e.Support >= 100 {
		t.Errorf("high tax should erode Support, got %d", e.Support)
	}

	w2 := NewWorldSeed(DefaultConfig(), 1)
	stable := w2.AddHuman("stable", "Stable Realm")
	stable.Tax = SupportStableTax
	stable.Support = 50 // start low to confirm it drifts back up
	for i := 0; i < 20; i++ {
		w2.PlayTurn(stable, "2026-07-03")
	}
	if stable.Support < 90 {
		t.Errorf("stable tax rate should hold/return Support near 100, got %d", stable.Support)
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

	wHigh.PlayTurn(high, "2026-07-03")
	wLow.PlayTurn(low, "2026-07-03")

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

	sawRiot := false
	for i := 0; i < 200; i++ {
		peopleBefore := e.People
		supportBefore := e.Support
		w.PlayTurn(e, "2026-07-03")
		if e.LastRiot {
			sawRiot = true
			if e.Support > supportBefore {
				t.Errorf("riot should not raise Support: before=%d after=%d", supportBefore, e.Support)
			}
			if e.People > peopleBefore {
				t.Errorf("riot should not raise People: before=%d after=%d", peopleBefore, e.People)
			}
		}
	}
	if !sawRiot {
		t.Error("expected at least one riot at Tax=39 across 200 turns")
	}

	w2 := NewWorldSeed(DefaultConfig(), 42)
	stable := w2.AddHuman("stable", "Stable Realm")
	stable.Tax = SupportStableTax
	for i := 0; i < 200; i++ {
		w2.PlayTurn(stable, "2026-07-03")
		if stable.LastRiot {
			t.Error("no riot should fire at the stable tax rate")
		}
	}
}
