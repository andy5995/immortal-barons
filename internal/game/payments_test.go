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
	base := int64(e.Land * RegionUpkeepPerLand)
	if want, got := techLower(base, e.TechMaintFactor()), e.RegionUpkeep(); got != want {
		t.Errorf("RegionUpkeep with tech: want %d, got %d", want, got)
	}
	if e.RegionUpkeep() >= base {
		t.Errorf("technology should lower region upkeep: %d not < %d", e.RegionUpkeep(), base)
	}
}

// TestTechnologyRaisesFoodProduced locks #20: food-region output is raised by
// the Technology factor ("increased region output"). BRE raises the 300 base
// and leaves the per-turn draw alone, so the factor lands inside agriFood.
func TestTechnologyRaisesFoodProduced(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Regions = RegionMix{Agricultural: 80, Technology: 20}
	e.Land = e.Regions.Total()
	draw := w.regionDraw(e, 6, FoodAgriRate)
	plain := w.agriFood(e)
	if want := 300 + draw; plain != want { // 300 is BRE's, not a tunable
		t.Errorf("unteched yield per region: want %d, got %d", want, plain)
	}
	if got, want := w.FoodProduced(e), 80*plain; got != want {
		t.Errorf("FoodProduced: want %d, got %d", want, got)
	}
	e.TechSlots[TechSlotGold] = 40
	if want, got := techRaise(300, e.TechFoodFactor())+draw, w.agriFood(e); got != want {
		t.Errorf("teched yield per region: want %d, got %d", want, got)
	}
	if w.agriFood(e) <= plain {
		t.Errorf("technology should raise food output: %d not > %d", w.agriFood(e), plain)
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
	// The penalty is filed, not applied on the spot — BRE accumulates the whole
	// payment stage and spends it at rollover — and it lands on MORALE alone.
	// Golden literal from BRE.OVR 0x2F077: trunc((1 - 1/(due+1)) x 40) = 39 for
	// any due above 39.
	if e.PendingMoralePenalty != 39 {
		t.Errorf("PendingMoralePenalty = %d, want 39", e.PendingMoralePenalty)
	}
	if e.PendingSupportPenalty != 0 {
		t.Errorf("the forces shortfall must not touch support, got %d", e.PendingSupportPenalty)
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
	// BRE's live prompt read 23,874 MILLION people and asked 216,366 gold, so the
	// same realm is 23874 x PopBREUnitScale in IB's count and must be charged the
	// same gold. Asserted as the golden literal, not as the formula.
	e.Support, e.People = 97, 23874*PopBREUnitScale
	if want := int64(216_366); e.SupportBoostCost() != want {
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
	want := MaxSupportBoostPerTurn * SupportBoostMaxPct / 100
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

// The Maintenance Costs ladder is the original's own, not the generic preset
// one: a QUARTER at Low and FOUR TIMES at High, so the same army costs sixteen
// times as much to keep between the two ends. Golden literals — IB applied half
// and double until the routine was read (#56).
func TestMaintenanceCostsFollowTheOriginalsLadder(t *testing.T) {
	due := func(l Level) (int64, int64) {
		cfg := DefaultConfig()
		cfg.MaintCosts = l
		w := NewWorldSeed(cfg, 1)
		e := w.AddHuman("h", "Realm")
		e.Troopers, e.Jets, e.Tanks = 10_000, 1_000, 500
		e.Regions.Urban, e.Regions.Agricultural = 100, 100
		e.syncLand()
		return w.ForcesDue(e), w.RegionsDue(e)
	}
	mf, mr := due(Medium)
	if mf <= 0 || mr <= 0 {
		t.Fatalf("the Medium baseline is empty (forces %d, regions %d)", mf, mr)
	}
	for _, c := range []struct {
		level       Level
		wf, wr      int64
		whatItMeans string
	}{
		{None, 0, 0, "free"},
		{Low, mf / 4, mr / 4, "a quarter"},
		{High, mf * 4, mr * 4, "four times"},
	} {
		gf, gr := due(c.level)
		if gf != c.wf {
			t.Errorf("%v forces upkeep = %d, want %s of Medium (%d)", c.level, gf, c.whatItMeans, c.wf)
		}
		if gr != c.wr {
			t.Errorf("%v region upkeep = %d, want %s of Medium (%d)", c.level, gr, c.whatItMeans, c.wr)
		}
	}
}
