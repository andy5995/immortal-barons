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
	goldBefore := e.Gold

	w.Manufacture(e)

	// Take the expectation from ProjectedProduction rather than re-deriving it:
	// a second hand-rolled formula here truncated where production rounds, and
	// disagreed by one unit whenever a percentage left a fraction.
	want := w.ProjectedProduction(e)
	wantTroopers, wantJets, wantTurrets := want[0], want[1], want[2]
	wantTanks, wantCarriers := want[4], want[5]

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

	// Industrial gold is credited via IncomeThisTurn now; Manufacture only sets
	// e.IndustryGold (for the report) and must NOT credit gold itself.
	wantGold := w.industrialGold(e)
	if e.IndustryGold != wantGold {
		t.Errorf("IndustryGold = %d, want %d", e.IndustryGold, wantGold)
	}
	if e.Gold != goldBefore {
		t.Errorf("Manufacture must not credit gold: before=%d after=%d", goldBefore, e.Gold)
	}
}

func TestManufactureZeroWithoutIndustrial(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("me", "Mine")
	e.Regions = RegionMix{Coastal: 100}
	e.Land = e.Regions.Total()

	w.Manufacture(e)

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

	w.Manufacture(e)

	// Compare against the same realm with no specialty rather than re-deriving the
	// arithmetic: the exact counts are pinned against live BRE figures in
	// TestProjectedProductionMatchesBRE, and a second hand-rolled formula here only
	// duplicates it — with a different rounding rule, which is how this test used to
	// disagree by one unit on an exact half.
	plain := *e
	plain.Specialized = ""
	base := w.ProjectedProduction(&plain)

	if e.MadeTroopers >= base[0] {
		t.Errorf("MadeTroopers (penalized) = %d, want fewer than the unspecialized %d", e.MadeTroopers, base[0])
	}
	if e.MadeTanks <= base[4] {
		t.Errorf("MadeTanks (bonused) = %d, want more than the unspecialized %d", e.MadeTanks, base[4])
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

	w.Manufacture(e)

	pts := 100 * UnitPointsPerRegion
	if want := pts / CostTrooper; e.MadeTroopers != want {
		t.Errorf("MadeTroopers = %d, want %d", e.MadeTroopers, want)
	}
}

func TestProdMigration(t *testing.T) {
	e := &Empire{}
	e.EnsureProduction()

	if e.ProdTroopers != DefaultProdTroopersPct || e.ProdJets != DefaultProdJetsPct || e.ProdTurrets != DefaultProdTurretsPct ||
		e.ProdBombers != DefaultProdBombersPct || e.ProdTanks != DefaultProdTanksPct || e.ProdCarriers != DefaultProdCarriersPct {
		t.Errorf("EnsureProduction gave unexpected defaults: %+v", e)
	}
}

// TestDefaultProdLiftsItsJets: the default split must build carriers at exactly
// the rate its jets need lifting — output is pct/cost, so jets/carriers must
// come to JetsPerCarrier. A change to either constant that misses this ratio
// wastes points on the most expensive unit in the table.
func TestDefaultProdLiftsItsJets(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	// Enough industry that the round-to-nearest on each unit type is noise; the
	// ratio itself does not depend on the count.
	e.Regions = RegionMix{Industrial: 10000}
	e.Land = e.Regions.Total()

	made := w.ProjectedProduction(e)
	jets, carriers := made[1], made[5]
	if carriers == 0 || jets/carriers != JetsPerCarrier {
		t.Errorf("default production made %d jets and %d carriers; want %d jets per carrier",
			jets, carriers, JetsPerCarrier)
	}
	total := DefaultProdTroopersPct + DefaultProdJetsPct + DefaultProdTurretsPct +
		DefaultProdBombersPct + DefaultProdTanksPct + DefaultProdCarriersPct
	if total != 100 {
		t.Errorf("default percentages total %d%%, want the whole pool spent on units", total)
	}
}

// TestProdKeepsIntentionalZero: an already-initialized empire whose player set
// every percentage to 0 (all output → gold) must NOT be reset by the
// load-time EnsureProduction repair.
func TestProdKeepsIntentionalZero(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine") // created initialized, at the default split
	e.ProdTroopers, e.ProdJets, e.ProdTurrets = 0, 0, 0
	e.ProdBombers, e.ProdTanks, e.ProdCarriers = 0, 0, 0

	e.EnsureProduction() // runs on every reload — must leave the zeros alone

	if sum := e.ProdTroopers + e.ProdJets + e.ProdTurrets + e.ProdBombers + e.ProdTanks + e.ProdCarriers; sum != 0 {
		t.Errorf("intentional all-zero production must persist across reload, got sum %d", sum)
	}
}
