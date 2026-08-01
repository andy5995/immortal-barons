package game

import (
	"testing"
	"time"
)

func TestPlayTurnAffectsOnlyActingEmpire(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 2
	w := NewWorldSeed(cfg, 1)
	other := w.Empires[1]
	otherGold := other.Gold
	me := w.AddHuman("me", "Mine")
	turns := me.TurnsLeft
	prot := me.Protection

	w.CollectIncome(me) // income is now credited at turn start, not inside PlayTurn
	w.PlayTurn(me, "2026-07-03")

	if me.TurnsLeft != turns-1 {
		t.Errorf("TurnsLeft: want %d, got %d", turns-1, me.TurnsLeft)
	}
	if me.Protection != prot-1 {
		t.Errorf("Protection: want %d, got %d", prot-1, me.Protection)
	}
	if me.LastPlayed != "2026-07-03" {
		t.Errorf("LastPlayed: got %q", me.LastPlayed)
	}
	if other.Gold != otherGold {
		t.Error("PlayTurn/CollectIncome must not touch other empires")
	}
	if me.Gold <= 10000 {
		t.Errorf("acting empire should collect income, got %d", me.Gold)
	}
}

func TestScoreAccumulatesFlatPerTurn(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	// Desert-only, no military, no tax: no food spoilage, no riots — so each
	// turn awards exactly the flat ScorePerTurn, regardless of realm size.
	e.Regions = RegionMix{Desert: 100}
	e.syncLand()
	e.Troopers, e.Carriers, e.Tax, e.Food = 0, 0, 0, 0
	if e.Score != 0 {
		t.Fatalf("want Score 0 at start, got %d", e.Score)
	}
	for i := 0; i < 3; i++ {
		w.PlayTurn(e, "2026-07-03")
	}
	if e.Score != 3*ScorePerTurn {
		t.Errorf("Score after 3 turns: want %d (3 x %d), got %d", 3*ScorePerTurn, ScorePerTurn, e.Score)
	}
}

func TestScoreUnaffectedBySpoilage(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Tax = 0          // no riot
	e.Food = 1_000_000 // way over buffer -> spoilage
	w.PlayTurn(e, "2026-07-03")
	// Score is the cumulative earned metric — spoilage does not ding it.
	if e.Score != ScorePerTurn {
		t.Errorf("spoilage should not affect Score: want %d, got %d", ScorePerTurn, e.Score)
	}
	if e.LastSpoiled <= 0 {
		t.Errorf("expected food to spoil, LastSpoiled=%d", e.LastSpoiled)
	}
}

// Rivers pay gold AND food every turn — IB's divergence from BRE, where a river
// does one or the other (see RiverFishShare). Both halves must show up on every
// single turn, or a player who committed to rivers is back to guessing.
func TestRiversPayGoldAndFoodEveryTurn(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Regions = RegionMix{River: 24}
	e.syncLand()
	wantFood := 24 * RiverFishFood * RiverFishShare / 100
	for d := 0; d < 40; d++ {
		w.GameDay = d
		b := w.IncomeThisTurn(e)
		if b.Rivers <= 0 {
			t.Fatalf("day %d: rivers should always pay hydropower gold, got %d", d, b.Rivers)
		}
		if b.RiverFood != wantFood {
			t.Fatalf("day %d: river food want %d, got %d", d, wantFood, b.RiverFood)
		}
	}
}

// Both halves are scaled by RiverFishShare: the gold lands inside the
// share-scaled yield band, and the food is exactly the share of a full haul.
// (This checks the scaling, not the expectation-preservation claim behind the
// choice of share — that rests on the measured 30% fishing rate, not on code.)
func TestRiverYieldSplitByShare(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Regions = RegionMix{River: 1}
	e.syncLand()
	gold := w.riverGold(e)
	if lo, hi := RiverBase/2*(100-RiverFishShare)/100, (RiverBase+RiverRate)*(100-RiverFishShare)/100; gold < lo || gold > hi {
		t.Errorf("river gold %d outside the share-scaled yield band [%d, %d]", gold, lo, hi)
	}
	if got, want := w.riverFood(e), RiverFishFood*RiverFishShare/100; got != want {
		t.Errorf("river food want %d, got %d", want, got)
	}
}

func TestIndustryGoldIsUnallocatedCapacity(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Regions = RegionMix{Industrial: 114}
	e.syncLand()
	// Each of 6 types at 15% = 90% to units, 10% unallocated -> gold.
	e.ProdTroopers, e.ProdJets, e.ProdTurrets = 15, 15, 15
	e.ProdBombers, e.ProdTanks, e.ProdCarriers = 15, 15, 15
	// 10%% unallocated of a 114-region pool, at whatever yield this game-day drew.
	perRegion := w.regionDraw(e, 5, IndustryGoldRate) + IndustryGoldBase
	if got, want := w.industrialGold(e), perRegion*114/100*10; got != want {
		t.Errorf("industrial gold at 90%% allocation: want %d, got %d", want, got)
	}
	p := w.ProjectedProduction(e)
	if want := 114 * UnitPointsPerRegion * 15 / 100 / CostTrooper; p[0] != want {
		t.Errorf("troopers made: want %d, got %d", want, p[0])
	}
	if p[5] >= p[0] { // carriers cost far more -> far fewer than troopers
		t.Errorf("carriers (%d) should be far fewer than troopers (%d)", p[5], p[0])
	}
	// Allocate 100% -> no unallocated capacity -> no industrial gold.
	e.ProdTroopers, e.ProdJets, e.ProdTurrets = 40, 30, 30
	e.ProdBombers, e.ProdTanks, e.ProdCarriers = 0, 0, 0
	if g := w.industrialGold(e); g != 0 {
		t.Errorf("100%% allocation should yield 0 industrial gold, got %d", g)
	}
}

func TestIndustrySpecializationBonusPenalty(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Regions = RegionMix{Industrial: 114}
	e.syncLand()
	e.ProdTroopers, e.ProdJets, e.ProdTurrets = 15, 15, 15
	e.ProdBombers, e.ProdTanks, e.ProdCarriers = 15, 15, 15
	base := w.ProjectedProduction(e)
	e.Specialized = "Tanks"
	spec := w.ProjectedProduction(e)
	if want := base[4] * (100 + SpecialtyBonusPct) / 100; spec[4] != want { // tanks +25%
		t.Errorf("specialized tanks: want %d, got %d", want, spec[4])
	}
	if want := base[0] * (100 - SpecialtyPenaltyPct) / 100; spec[0] != want { // troopers -15%
		t.Errorf("penalized troopers: want %d, got %d", want, spec[0])
	}
}

func TestCollectIncomeCoversStartingMaintenance(t *testing.T) {
	// Regression for the auto-deposit bug: with income credited at turn start,
	// a starting empire whose gold was swept to the bank can still pay its
	// maintenance from the income the turn earns.
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("h", "Realm")
	e.Gold = 0 // as if auto-deposit banked everything at the end of last turn
	w.CollectIncome(e)
	if due := e.ForcesUpkeep() + e.RegionUpkeep(); e.Gold < due {
		t.Errorf("collected income %d does not cover maintenance %d", e.Gold, due)
	}
}

// TestAgriFoodBandMatchesLiveBRE: BRE's Agricultural output is one Base+draw
// roll per turn shared by every Agricultural region, so the printed total always
// divides exactly by the region count and the per-region figure spans exactly
// 300..304. Both properties are captured facts, not IB choices.
func TestAgriFoodBandMatchesLiveBRE(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Regions = RegionMix{Agricultural: 144}
	e.syncLand()
	seen := map[int]bool{}
	for d := 0; d < 400; d++ {
		w.GameDay = d
		total := w.FoodProduced(e)
		if total%144 != 0 {
			t.Fatalf("day %d: total %d does not divide by the region count — the draw is not shared", d, total)
		}
		per := total / 144
		if per < FoodAgriBase || per >= FoodAgriBase+FoodAgriRate {
			t.Fatalf("day %d: per-region %d outside the captured band [%d, %d]", d, per, FoodAgriBase, FoodAgriBase+FoodAgriRate-1)
		}
		seen[per] = true
	}
	if len(seen) != FoodAgriRate {
		t.Errorf("draw reached %d of %d values in the band: %v", len(seen), FoodAgriRate, seen)
	}
}

func TestProcessEconomyTracksConsumed(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	wantFood := e.FoodUpkeep()

	w.PlayTurn(e, "2026-07-03")

	if e.LastFoodConsumed != wantFood {
		t.Errorf("LastFoodConsumed: want %d, got %d", wantFood, e.LastFoodConsumed)
	}
}

func TestForcesUpkeepFormula(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	// newEmpire has Troopers=100 and no other units, no Technology regions => no
	// TechFactor reduction. Rates are tenths of a gold, truncated on the total.
	// 100 troopers -> 40 gold is the live BRE figure, not a derived one.
	want := (100*MaintTrooperTenths + 0*MaintJetTenths + 0*MaintTurretTenths +
		0*MaintBomberTenths + 0*MaintTankTenths + 0*MaintCarrierTenths) / MaintTenthsPerGold
	if want != 40 {
		t.Fatalf("a 100-trooper realm should owe the captured 40 gold, not %d", want)
	}
	if got := e.ForcesUpkeep(); got != want {
		t.Errorf("ForcesUpkeep: want %d, got %d", want, got)
	}
}

// TestUpkeepMatchesLiveBRE pins both upkeep rates to figures captured from the
// original at Maintenance Costs "Medium". These are the fidelity contract: a
// failure here means IB has stopped charging what BRE charged, not that the
// test needs updating.
func TestUpkeepMatchesLiveBRE(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)

	forces := []struct{ turrets, want int }{
		{49691, 44721}, {99382, 89443}, {159207, 143286}, {219032, 197128},
	}
	for _, c := range forces {
		e := w.AddHuman("f"+string(rune(c.turrets%26+'a')), "Fortress")
		e.Troopers, e.Turrets = 0, c.turrets
		if got := e.ForcesUpkeep(); got != c.want {
			t.Errorf("ForcesUpkeep(%d turrets) = %d, want %d", c.turrets, got, c.want)
		}
	}

	// Flat per region, whatever the type mix or empire size.
	regions := []struct{ land, want int }{
		{15, 13695}, {5917, 5402221}, {6397, 5840461}, {6837, 6242181},
	}
	for _, c := range regions {
		e := w.AddHuman("r"+string(rune(c.land%26+'a')), "Realm")
		e.Regions = RegionMix{Coastal: c.land}
		e.Land = c.land
		if got := e.RegionUpkeep(); got != c.want {
			t.Errorf("RegionUpkeep(%d regions) = %d, want %d", c.land, got, c.want)
		}
	}
}

func TestDailyMaintenanceInitialisesDate(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.DailyMaintenance("2026-07-03")
	if w.LastMaintDate != "2026-07-03" {
		t.Errorf("first maintenance should just set the date, got %q", w.LastMaintDate)
	}
	if w.GameDay != 0 {
		t.Errorf("first maintenance should not advance the day, got %d", w.GameDay)
	}
}

// Maintenance never advances more than ONE game day, however long the game sat
// idle: missed days are lost, not banked. Catching them all up at once meant a
// returning player met several days of raids, riots and AI turns in one login.
func TestDailyMaintenanceAdvancesOneDayOnly(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-07-01"
	me := w.AddHuman("me", "Mine")
	me.TurnsLeft = 0

	// Four days with nobody playing, then one run.
	if r := w.DailyMaintenance("2026-07-05"); r.Days != 1 {
		t.Errorf("a four-day gap should still advance one day, reported %d", r.Days)
	}
	if w.GameDay != 1 {
		t.Errorf("GameDay should advance by exactly 1, got %d", w.GameDay)
	}
	if w.LastMaintDate != "2026-07-02" {
		t.Errorf("the game clock should step one day, got %q", w.LastMaintDate)
	}
	if me.TurnsLeft != cfg.TurnsPerDay {
		t.Errorf("turns should be refilled to %d, got %d", cfg.TurnsPerDay, me.TurnsLeft)
	}

	// A second login the same real day must not advance another game day, even
	// though the game clock is still behind.
	me.TurnsLeft = 0
	if r := w.DailyMaintenance("2026-07-05"); r.Days != 0 {
		t.Errorf("a second run the same day should do nothing, reported %d", r.Days)
	}
	if w.GameDay != 1 {
		t.Errorf("GameDay should stay at 1 on a same-day rerun, got %d", w.GameDay)
	}

	// The next real day advances one more.
	if r := w.DailyMaintenance("2026-07-06"); r.Days != 1 {
		t.Errorf("the next real day should advance one day, reported %d", r.Days)
	}
	if w.GameDay != 2 {
		t.Errorf("GameDay should be 2 after a second day, got %d", w.GameDay)
	}
}

func TestDailyMaintenanceIdempotent(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-07-03"
	w.GameDay = 5
	w.DailyMaintenance("2026-07-03")
	if w.GameDay != 5 {
		t.Errorf("same-day maintenance should be a no-op, got %d", w.GameDay)
	}
}

func TestDailyMaintenanceCullsDead(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-07-02"
	dead := w.AddHuman("gone", "Gone")
	dead.Land = 0
	w.DailyMaintenance("2026-07-03")
	// The empire dies as the day rolls, and because its death then lies in the
	// past (GameDay advanced past DiedDay) the husk is swept the same maintenance
	// pass, freeing the owner to rebuild on a later login.
	if w.FindByOwner("gone") != nil {
		t.Error("dead empire's husk should be culled once its death is in the past")
	}
}

func TestDailyMaintenanceSweepsStaleHuskSameDay(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-07-03" // already current: no day rolls over
	w.GameDay = 5
	dead := w.AddHuman("gone", "Gone")
	dead.Alive = false
	dead.DiedDay = 3 // died in the past (DiedDay < GameDay)
	w.DailyMaintenance("2026-07-03")
	if w.GameDay != 5 {
		t.Errorf("no day should roll over, GameDay got %d", w.GameDay)
	}
	if w.FindByOwner("gone") != nil {
		t.Error("stale husk should be swept by the same-day maintenance sweep")
	}
}

func TestDailyMaintenanceKeepsHuskThatDiedToday(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-07-03"
	w.GameDay = 5
	dead := w.AddHuman("fresh", "Fresh")
	dead.Alive = false
	dead.DiedDay = 5 // died today (DiedDay == GameDay)
	w.DailyMaintenance("2026-07-03")
	if w.FindByOwner("fresh") == nil {
		t.Error("a husk that died today should be kept, not swept")
	}
}

func TestDailyMaintenanceHandlesMalformedDate(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-07-0" // malformed, lexicographically < today
	done := make(chan struct{})
	go func() { w.DailyMaintenance("2026-07-03"); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("DailyMaintenance hung on a malformed LastMaintDate")
	}
	if w.LastMaintDate != "2026-07-03" {
		t.Errorf("LastMaintDate = %q, want snapped to today", w.LastMaintDate)
	}
}

// BRE-verified (2026-07-16): 5% of the ENTIRE food stock spoils each turn, with
// NO floor below which nothing spoils.
func TestFoodSpoils5PctNoFloor(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.People, e.Troopers, e.Jets, e.Tanks = 0, 0, 0, 0 // no consumption
	e.Regions, e.Land = RegionMix{}, 0                 // no food grown
	e.Food = 200                                       // below the old 1000 "floor" — BRE spoils it anyway

	w.PlayTurn(e, "2026-07-03")

	if e.LastSpoiled != 200*FoodSpoilPct/100 { // 5% of 200 = 10, no floor
		t.Errorf("food should spoil %d%% with no floor: want %d, got %d", FoodSpoilPct, 200*FoodSpoilPct/100, e.LastSpoiled)
	}
	if e.Food != 200-200*FoodSpoilPct/100 {
		t.Errorf("food after spoilage: want %d, got %d", 200-200*FoodSpoilPct/100, e.Food)
	}
}

func TestTechCutsSpoilage(t *testing.T) {
	setup := func(regions RegionMix) (*World, *Empire) {
		w := NewWorldSeed(DefaultConfig(), 1)
		e := w.AddHuman("me", "Mine")
		e.People, e.Troopers, e.Jets, e.Tanks = 0, 0, 0, 0
		e.Regions = regions
		e.syncLand()
		e.Food = 100000
		return w, e
	}

	wBase, base := setup(RegionMix{Coastal: 100})
	wTech, tech := setup(RegionMix{Coastal: 60, Technology: 40})
	tech.TechSlots[TechSlotDecay] = 200 // well-researched spoilage reduction

	wBase.PlayTurn(base, "2026-07-03")
	wTech.PlayTurn(tech, "2026-07-03")

	if base.LastSpoiled <= 0 {
		t.Fatalf("expect spoilage in base case, got %d", base.LastSpoiled)
	}
	if tech.LastSpoiled >= base.LastSpoiled {
		t.Errorf("Technology regions should reduce spoilage: base=%d tech=%d", base.LastSpoiled, tech.LastSpoiled)
	}
}

func TestIncomeReportMatchesCredit(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("h", "Realm")
	e.Gold = 0
	sum := w.IncomeThisTurn(e).Gold()
	w.CollectIncome(e)
	if e.Gold != sum {
		t.Errorf("credited %d but the itemized breakdown sums to %d", e.Gold, sum)
	}
}

// TestAIBuildsDefensiveForceMix checks the AI (#36) no longer buys only
// troopers: when food-healthy it builds a mix that includes defensive turrets
// and tanks, giving its realm an actual defensive posture.
func TestAIBuildsDefensiveForceMix(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("ai", "AI")
	e.Regions = RegionMix{Agricultural: 50} // ~15k food/turn produced
	e.syncLand()
	e.People = 1000  // tiny upkeep so the realm is food-healthy
	e.Protection = 0 // past protection: the AI builds a defensive force (under protection it expands land)
	e.Troopers, e.Jets, e.Turrets, e.Tanks, e.Carriers = 0, 0, 0, 0, 0
	e.Food = 1_000_000
	e.Gold = 500_000
	e.Investments = nil

	w.aiManageEconomy(e)

	if e.Troopers == 0 {
		t.Error("AI should still buy troopers, got 0")
	}
	if e.Turrets == 0 {
		t.Error("AI should buy turrets for defense, got 0")
	}
	if e.Tanks == 0 {
		t.Error("AI should buy tanks, got 0")
	}
	if e.Defense() == 0 {
		t.Error("AI should have a defensive posture, got 0 Defense")
	}
}

// TestAIInvestsIdleGold checks the AI parks its clear surplus in investments
// (#36) instead of hoarding gold that just sits.
func TestAIInvestsIdleGold(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("ai", "AI")
	e.Regions = RegionMix{Agricultural: 50}
	e.syncLand()
	e.People = 1000
	e.Troopers, e.Jets, e.Turrets, e.Tanks, e.Carriers = 0, 0, 0, 0, 0
	e.Food = 1_000_000
	e.Gold = 2_000_000
	e.Investments = nil

	w.aiManageEconomy(e)

	if len(e.Investments) == 0 {
		t.Error("AI with a large surplus should invest, got no investments")
	}
	if e.Gold >= 2_000_000 {
		t.Errorf("AI should have spent/invested gold, still holds %d", e.Gold)
	}
}

// TestAIStartsHQ checks the AI builds a HeadQuarters (#36) once it fields tanks,
// so its tanks get the HQ offense/defense multiplier instead of being unamplified.
func TestAIStartsHQ(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("ai", "AI")
	e.Regions = RegionMix{Agricultural: 50}
	e.syncLand()
	e.People = 1000
	e.Troopers, e.Jets, e.Turrets, e.Tanks, e.Carriers = 0, 0, 0, 0, 0
	e.Food = 1_000_000
	e.Gold = 500_000
	e.HQ = 0
	e.Protection = 0 // past protection: the AI buys tanks (then an HQ); under protection it expands land
	e.Investments = nil

	w.aiManageEconomy(e)

	if e.Tanks == 0 {
		t.Fatal("precondition: AI should have bought tanks")
	}
	if e.HQ == 0 {
		t.Error("AI with tanks and ample gold should start a HeadQuarters, HQ still 0")
	}
}
