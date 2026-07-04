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

// Specialization keeps the percentage split but applies an efficiency modifier:
// the specialized unit gets a bonus, every other unit a penalty. Here half the
// points go to troopers (penalized) and half to the Tanks specialty (bonused).
func TestSpecializedBonusAndPenalty(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("me", "Mine")
	e.Regions = RegionMix{Industrial: 100}
	e.Land = e.Regions.Total()
	e.ProdTroopers, e.ProdJets, e.ProdTurrets = 50, 0, 0
	e.ProdBombers, e.ProdTanks, e.ProdCarriers = 0, 50, 0
	e.Specialized = "Tanks"

	w.manufacture(e)

	pts := 100 * IndustryPointsPerRegion
	wantTroopers := (pts * 50 / 100) / CostTrooper * (100 - SpecialtyPenaltyPct) / 100
	wantTanks := (pts * 50 / 100) / CostTank * (100 + SpecialtyBonusPct) / 100
	if e.MadeTroopers != wantTroopers {
		t.Errorf("MadeTroopers (penalized) = %d, want %d", e.MadeTroopers, wantTroopers)
	}
	if e.MadeTanks != wantTanks {
		t.Errorf("MadeTanks (bonused) = %d, want %d", e.MadeTanks, wantTanks)
	}
}

// With no specialization the split is honored exactly (no modifier).
func TestUnspecializedHonorsSplit(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("me", "Mine")
	e.Regions = RegionMix{Industrial: 100}
	e.Land = e.Regions.Total()
	e.ProdTroopers, e.ProdJets, e.ProdTurrets = 100, 0, 0
	e.ProdBombers, e.ProdTanks, e.ProdCarriers = 0, 0, 0

	w.manufacture(e)

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
