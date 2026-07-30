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

	pts := w.BoostSupport(e, e.SupportBoostMax()) // overpay the request by half
	if e.Support != 100 {
		t.Errorf("support should cap at 100, got %d", e.Support)
	}
	if pts != 5 {
		t.Errorf("boost from 95 should report the 5 points actually gained, got %d", pts)
	}
}

// Paying the full request restores the whole deficit; paying half restores about
// half. Both the cost and the payment-to-points ratio are binary-verified.
func TestSupportBoostCostAndAward(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Support, e.People = 97, 23874
	if want := 3 * (3*23874 + 500); e.SupportBoostCost() != want {
		t.Errorf("cost want %d, got %d", want, e.SupportBoostCost())
	}
	full := *e
	full.Gold = 1_000_000
	if pts := w.BoostSupport(&full, full.SupportBoostCost()); pts != 3 || full.Support != 100 {
		t.Errorf("full payment should buy all 3 points, got %d (support %d)", pts, full.Support)
	}
	half := *e
	half.Gold = 1_000_000
	if pts := w.BoostSupport(&half, half.SupportBoostCost()/2); pts != 1 {
		t.Errorf("half payment should buy 1 of 3 points, got %d", pts)
	}
	// A collapsed realm cannot buy its way back to 100 in one turn: the crown only
	// charges for MaxSupportBoostPerTurn points at a time.
	low := *e
	low.Support, low.Gold = 20, 1_000_000_000
	if pts := w.BoostSupport(&low, low.SupportBoostCost()); pts != MaxSupportBoostPerTurn {
		t.Errorf("full payment should buy the %d-point cap, got %d", MaxSupportBoostPerTurn, pts)
	}
	// Overpaying scales the award past that cap — the ratio, not the cap, is what
	// the payment multiplies. Half again the request buys half again the points.
	over := *e
	over.Support, over.Gold = 20, 1_000_000_000
	want := MaxSupportBoostPerTurn * SupportBoostMaxNum / SupportBoostMaxDen
	if pts := w.BoostSupport(&over, over.SupportBoostMax()); pts != want {
		t.Errorf("overpaying by half should buy %d points, got %d", want, pts)
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
