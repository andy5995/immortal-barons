package game

import "testing"

// TestTechnologyLowersRegionUpkeep locks #20: region maintenance is reduced by
// the Technology factor, the same way military upkeep is.
func TestTechnologyLowersRegionUpkeep(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Regions = RegionMix{Agricultural: 80, Technology: 20}
	e.Land = e.Regions.Total() // 100
	e.TechSlots[TechSlotMaint] = 40
	base := e.Land * RegionUpkeepPerLand
	if want, got := techLower(base, e.TechMaintFactor()), e.RegionUpkeep(); got != want {
		t.Errorf("RegionUpkeep with tech: want %d, got %d", want, got)
	}
	if e.RegionUpkeep() >= base {
		t.Errorf("technology should lower region upkeep: %d not < %d", e.RegionUpkeep(), base)
	}
}

// TestTechnologyRaisesFoodProduced locks #20: food-region output is raised by
// the Technology factor ("increased region output").
func TestTechnologyRaisesFoodProduced(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Regions = RegionMix{Agricultural: 80, Technology: 20}
	e.Land = e.Regions.Total()
	e.TechSlots[TechSlotGold] = 40
	base := e.Regions.foodProduced()
	if want, got := techRaise(base, e.TechFoodFactor()), e.FoodProduced(); got != want {
		t.Errorf("FoodProduced with tech: want %d, got %d", want, got)
	}
	if e.FoodProduced() <= base {
		t.Errorf("technology should raise food output: %d not > %d", e.FoodProduced(), base)
	}
}

func TestPayForcesFullNoDesertion(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Gold = 100000
	before := e.Troopers
	req := e.ForcesUpkeep()

	if lost := w.PayForces(e, req); lost != 0 {
		t.Errorf("paying in full should cause no desertion, lost %d", lost)
	}
	if e.Troopers != before {
		t.Errorf("troopers should be unchanged, %d -> %d", before, e.Troopers)
	}
	if e.LastGoldPaid != req {
		t.Errorf("LastGoldPaid: want %d, got %d", req, e.LastGoldPaid)
	}
}

func TestPayForcesShortfallDeserts(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Troopers, e.Jets, e.Turrets, e.Tanks = 1000, 100, 100, 100
	e.Gold = 1_000_000
	e.Support = 100

	lost := w.PayForces(e, 0) // pay nothing => full shortfall
	if lost <= 0 {
		t.Fatal("paying nothing should cause desertion")
	}
	// At full non-payment fracPct=100, desertPct = ArmyDesertRate.
	wantTroopers := 1000 - 1000*ArmyDesertRate/100
	if e.Troopers != wantTroopers {
		t.Errorf("troopers: want %d, got %d", wantTroopers, e.Troopers)
	}
	if e.Support >= 100 {
		t.Errorf("support should drop on non-payment, got %d", e.Support)
	}
}

func TestPayRegionsShortfallRevolts(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	before := e.Land
	e.Gold = 0

	lost := w.PayRegions(e, 0) // can afford nothing => full shortfall
	if lost <= 0 {
		t.Fatal("unpaid regions should revolt")
	}
	if e.Land != before-lost {
		t.Errorf("land: want %d, got %d", before-lost, e.Land)
	}
	if e.Land != e.Regions.Total() {
		t.Errorf("Land/Regions invariant broken: Land=%d Total=%d", e.Land, e.Regions.Total())
	}
}

func TestBoostSupportCapsAt100(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Support = 95
	e.Gold = 1_000_000

	pts := w.BoostSupport(e, 100*SupportPerBoostGold) // enough for 100 points
	if e.Support != 100 {
		t.Errorf("support should cap at 100, got %d", e.Support)
	}
	if pts <= 0 {
		t.Error("expected support points gained")
	}
}

func TestClampGiveLimitsToGold(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Gold = 50
	e.Troopers = 0 // ForcesUpkeep small; give more than gold
	e.Carriers = 0

	w.PayForces(e, 1_000_000) // asks to give a fortune
	if e.Gold != 0 {
		t.Errorf("cannot pay more than gold on hand, gold=%d", e.Gold)
	}
	if e.LastGoldPaid != 50 {
		t.Errorf("LastGoldPaid should equal what was affordable (50), got %d", e.LastGoldPaid)
	}
}
