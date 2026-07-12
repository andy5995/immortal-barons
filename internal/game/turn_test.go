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

func TestScoreSpoilageAndRiotPenalize(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Tax = 0          // no riot
	e.Food = 1_000_000 // way over buffer -> spoilage
	w.PlayTurn(e, "2026-07-03")
	// base +ScorePerTurn, minus a spoilage ding of ScorePerTurn/ScoreSpoilPenaltyDiv.
	if want := ScorePerTurn - ScorePerTurn/ScoreSpoilPenaltyDiv; e.Score != want {
		t.Errorf("spoilage: Score want %d, got %d", want, e.Score)
	}
	if e.LastSpoiled <= 0 {
		t.Errorf("expected food to spoil, LastSpoiled=%d", e.LastSpoiled)
	}
}

func TestRiversFishOrHydropowerNotBoth(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Regions = RegionMix{River: 24}
	e.syncLand()
	sawFish, sawGold := false, false
	for d := 0; d < 40; d++ {
		w.GameDay = d
		b := w.IncomeThisTurn(e)
		fish, gold := b.RiverFood > 0, b.Rivers > 0
		if fish == gold {
			t.Fatalf("day %d: rivers should do exactly one of fish/hydropower (fish=%v gold=%v)", d, fish, gold)
		}
		if fish {
			sawFish = true
			if b.RiverFood != 24*RiverFishFood {
				t.Errorf("day %d: river food want %d, got %d", d, 24*RiverFishFood, b.RiverFood)
			}
		} else {
			sawGold = true
		}
	}
	if !sawFish || !sawGold {
		t.Errorf("over 40 days expected both fishing and hydropower turns (fish=%v gold=%v)", sawFish, sawGold)
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
	if got, want := w.industrialGold(e), IndustryPointsPerRegion*10/100; got != want {
		t.Errorf("industrial gold/region at 90%% allocation: want %d, got %d", want, got)
	}
	p := w.ProjectedProduction(e)
	if want := 114 * IndustryPointsPerRegion * 15 / 100 / CostTrooper; p[0] != want {
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
	// TechFactor reduction.
	want := 100*6 + 0*12 + 0*9 + 0*13 + 0*6 + 0*1
	if got := e.ForcesUpkeep(); got != want {
		t.Errorf("ForcesUpkeep: want %d, got %d", want, got)
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

func TestDailyMaintenanceCatchesUpAndRefills(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-07-01"
	me := w.AddHuman("me", "Mine")
	me.TurnsLeft = 0
	w.DailyMaintenance("2026-07-03") // two days missed
	if w.LastMaintDate != "2026-07-03" {
		t.Errorf("should catch up to today, got %q", w.LastMaintDate)
	}
	if w.GameDay != 2 {
		t.Errorf("two days should advance GameDay to 2, got %d", w.GameDay)
	}
	if me.TurnsLeft != cfg.TurnsPerDay {
		t.Errorf("turns should be refilled to %d, got %d", cfg.TurnsPerDay, me.TurnsLeft)
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

func TestHQAdvancesEachTurn(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Gold = HQCost
	if err := w.StartHQ(e); err != nil {
		t.Fatalf("StartHQ: %v", err)
	}
	if e.HQ != 5 {
		t.Fatalf("HQ after start: want 5, got %d", e.HQ)
	}

	want := []int{10, 15, 20}
	for _, w2 := range want {
		w.PlayTurn(e, "2026-07-03")
		if e.HQ != w2 {
			t.Errorf("HQ after turn: want %d, got %d", w2, e.HQ)
		}
	}

	e.HQ = 100
	w.PlayTurn(e, "2026-07-03")
	if e.HQ != 100 {
		t.Errorf("HQ should cap at 100, got %d", e.HQ)
	}
}

func TestFoodSpoilageAboveBuffer(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.People = 0
	e.Troopers = 0
	e.Jets = 0
	e.Tanks = 0
	e.Regions = RegionMix{}
	e.Land = 0
	e.Food = 100000

	w.PlayTurn(e, "2026-07-03")

	if e.LastSpoiled <= 0 {
		t.Errorf("expect food spoilage above buffer, got LastSpoiled=%d", e.LastSpoiled)
	}
}

func TestFoodNoSpoilageBelowBuffer(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.People = 100
	e.Troopers = 10
	e.Jets = 0
	e.Tanks = 0
	e.Regions = RegionMix{}
	e.Land = 0
	e.Food = 150 // consumption (110) leaves 40, well below the buffer (220)

	w.PlayTurn(e, "2026-07-03")

	if e.LastSpoiled != 0 {
		t.Errorf("expect no spoilage below buffer, got LastSpoiled=%d", e.LastSpoiled)
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
