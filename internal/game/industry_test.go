package game

import "testing"

func TestManufactureSplitsByPercent(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("me", "Mine")
	e.Regions = RegionMix{Industrial: 100}
	e.Land = e.Regions.Total()

	beforeTroopers, beforeJets, beforeTurrets := e.Troopers, e.Jets, e.Turrets
	beforeTanks, beforeCarriers := e.Tanks, e.Carriers

	w.manufacture(e)

	pts := 100 * IndustryPointsPerRegion
	wantTroopers := (pts * e.ProdTroopers / 100) / CostTrooper
	wantJets := (pts * e.ProdJets / 100) / CostJet
	wantTurrets := (pts * e.ProdTurrets / 100) / CostTurret
	wantTanks := (pts * e.ProdTanks / 100) / CostTank
	wantCarriers := (pts * e.ProdCarriers / 100) / CostCarrier

	if e.MadeTroopers != wantTroopers {
		t.Errorf("MadeTroopers = %d, want %d", e.MadeTroopers, wantTroopers)
	}
	if e.MadeJets != wantJets {
		t.Errorf("MadeJets = %d, want %d", e.MadeJets, wantJets)
	}
	if e.MadeTurrets != wantTurrets {
		t.Errorf("MadeTurrets = %d, want %d", e.MadeTurrets, wantTurrets)
	}
	if e.MadeTanks != wantTanks {
		t.Errorf("MadeTanks = %d, want %d", e.MadeTanks, wantTanks)
	}
	if e.MadeCarriers != wantCarriers {
		t.Errorf("MadeCarriers = %d, want %d", e.MadeCarriers, wantCarriers)
	}
	if e.Troopers != beforeTroopers+wantTroopers {
		t.Errorf("Troopers stock = %d, want %d", e.Troopers, beforeTroopers+wantTroopers)
	}
	if e.Jets != beforeJets+wantJets {
		t.Errorf("Jets stock = %d, want %d", e.Jets, beforeJets+wantJets)
	}
	if e.Turrets != beforeTurrets+wantTurrets {
		t.Errorf("Turrets stock = %d, want %d", e.Turrets, beforeTurrets+wantTurrets)
	}
	if e.Tanks != beforeTanks+wantTanks {
		t.Errorf("Tanks stock = %d, want %d", e.Tanks, beforeTanks+wantTanks)
	}
	if e.Carriers != beforeCarriers+wantCarriers {
		t.Errorf("Carriers stock = %d, want %d", e.Carriers, beforeCarriers+wantCarriers)
	}
	if wantTroopers <= wantTanks {
		t.Errorf("expected more troopers made than tanks (cheaper unit), got troopers=%d tanks=%d", wantTroopers, wantTanks)
	}

	wantGold := e.Regions.Industrial * IndustryGoldPerRegion
	if e.IndustryGold != wantGold {
		t.Errorf("IndustryGold = %d, want %d", e.IndustryGold, wantGold)
	}
}

func TestManufactureZeroWithoutIndustrial(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("me", "Mine")
	e.Regions = RegionMix{Coastal: 100}
	e.Land = e.Regions.Total()

	w.manufacture(e)

	if e.MadeTroopers != 0 || e.MadeJets != 0 || e.MadeTurrets != 0 ||
		e.MadeBombers != 0 || e.MadeTanks != 0 || e.MadeCarriers != 0 {
		t.Errorf("expected all Made* to be 0, got %+v", e)
	}
	if e.IndustryGold != 0 {
		t.Errorf("IndustryGold = %d, want 0", e.IndustryGold)
	}
}

// Specialization does not replace the percentage split; it absorbs the points
// the percentages leave unspent. Here 40% goes to troopers and the remaining
// 60% falls to the Tanks specialty.
func TestSpecializedAbsorbsSurplus(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("me", "Mine")
	e.Regions = RegionMix{Industrial: 100}
	e.Land = e.Regions.Total()
	e.ProdTroopers, e.ProdJets, e.ProdTurrets = 40, 0, 0
	e.ProdBombers, e.ProdTanks, e.ProdCarriers = 0, 0, 0
	e.Specialized = "Tanks"

	w.manufacture(e)

	pts := 100 * IndustryPointsPerRegion
	if want := (pts * 40 / 100) / CostTrooper; e.MadeTroopers != want {
		t.Errorf("MadeTroopers = %d, want %d", e.MadeTroopers, want)
	}
	if want := (pts * 60 / 100) / CostTank; e.MadeTanks != want {
		t.Errorf("MadeTanks (surplus to specialty) = %d, want %d", e.MadeTanks, want)
	}
}

// When the percentages already sum to 100, the specialty gets no surplus and
// the split is honored exactly.
func TestSpecializedNoSurplusHonorsSplit(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("me", "Mine")
	e.Regions = RegionMix{Industrial: 100}
	e.Land = e.Regions.Total()
	e.ProdTroopers, e.ProdJets, e.ProdTurrets = 100, 0, 0
	e.ProdBombers, e.ProdTanks, e.ProdCarriers = 0, 0, 0
	e.Specialized = "Tanks"

	w.manufacture(e)

	if e.MadeTanks != 0 {
		t.Errorf("MadeTanks = %d, want 0 (no surplus)", e.MadeTanks)
	}
	pts := 100 * IndustryPointsPerRegion
	if want := pts / CostTrooper; e.MadeTroopers != want {
		t.Errorf("MadeTroopers = %d, want %d", e.MadeTroopers, want)
	}
}

func TestProdMigration(t *testing.T) {
	e := &Empire{}
	e.EnsureProduction()

	if e.ProdTroopers != 30 || e.ProdJets != 20 || e.ProdTurrets != 15 ||
		e.ProdBombers != 5 || e.ProdTanks != 20 || e.ProdCarriers != 10 {
		t.Errorf("EnsureProduction gave unexpected defaults: %+v", e)
	}
}
