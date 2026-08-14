package game

import "testing"

// Golden literals from BRE's food-allocation routine (BRE.OVR 0x38104 for the
// people's penalty, 0x381E5 for the civil war, 0x382E9 for the army's). The
// ratio carries a +1 on each side, so a realm that meets its need exactly pays
// nothing and one that meets none of it pays 40 x need/(need+1).
func TestShortfallPenaltyIsBinaryVerified(t *testing.T) {
	for _, c := range []struct{ need, given, want int64 }{
		{100, 100, 0}, // fed: no penalty
		{100, 0, 39},  // trunc(100 x 40 / 101)
		{100, 50, 19}, // trunc(50 x 40 / 101)
		{999, 0, 39},  // the +1 keeps a total shortfall just under the scale
		{0, 0, 0},     // nothing needed, nothing owed
		{100, 200, 0}, // overfed
	} {
		if got := shortfallPenalty(c.need, c.given, StarvationPenaltyScale); got != int(c.want) {
			t.Errorf("shortfallPenalty(%d, %d, 40) = %d, want %d", c.need, c.given, got, c.want)
		}
	}
}

// The civil war lights only below 65% of the people's food need, and its
// severity is ROUNDED where the penalties above truncate.
func TestCivilWarSeverityThreshold(t *testing.T) {
	for _, c := range []struct{ need, given, want int64 }{
		{100, 70, 0},  // comfortably above the threshold
		{100, 65, 0},  // r = 66/101 = 0.653, still above 0.65
		{100, 64, 11}, // r = 0.6436 -> round(0.3564 x 30) = 11
		{100, 0, 30},  // round(100 x 30 / 101) = 30
		{100, 50, 15}, // round(50 x 30 / 101) = 15
	} {
		if got := civilWarSeverity(c.need, c.given, FoodCivilWarThresholdPct); got != int(c.want) {
			t.Errorf("civilWarSeverity(%d, %d, 65) = %d, want %d", c.need, c.given, got, c.want)
		}
	}
}

// A shortfall in the PEOPLE's food costs popular support; a shortfall in the
// ARMY's costs military morale. The two are billed separately, and BRE feeds the
// people first — so a realm with just enough for its people starves only its
// troops.
func TestStarvationSplitsSupportAndMorale(t *testing.T) {
	mk := func(food int) *Empire {
		w := NewWorldSeed(DefaultConfig(), 1)
		e := w.AddHuman("me", "Mine")
		e.Regions, e.Land = RegionMix{}, 0
		e.People, e.Troopers = 100_000, 200_000
		e.Food = food
		w.feed(e)
		return e
	}
	full := mk(1_000_000)
	if full.PendingSupportPenalty != 0 || full.PendingMoralePenalty != 0 || full.CivilWarSeverity != 0 {
		t.Errorf("a fed realm owes nothing, got support=%d morale=%d war=%d",
			full.PendingSupportPenalty, full.PendingMoralePenalty, full.CivilWarSeverity)
	}
	peopleOnly := mk(mk(0).PeopleFoodUpkeep())
	if peopleOnly.PendingSupportPenalty != 0 {
		t.Errorf("the people were fed first, so support owes nothing, got %d", peopleOnly.PendingSupportPenalty)
	}
	if peopleOnly.PendingMoralePenalty <= 0 {
		t.Errorf("the army went unfed, so morale should owe: got %d", peopleOnly.PendingMoralePenalty)
	}
	if peopleOnly.CivilWarSeverity != 0 {
		t.Errorf("an army shortfall never lights a civil war, got %d", peopleOnly.CivilWarSeverity)
	}
	none := mk(0)
	if none.PendingSupportPenalty <= 0 || none.CivilWarSeverity <= 0 {
		t.Errorf("a wholly unfed realm owes support and faces civil war, got support=%d war=%d",
			none.PendingSupportPenalty, none.CivilWarSeverity)
	}
	// BRE has no starvation emigration: nobody leaves for want of food.
	if none.People != 100_000 {
		t.Errorf("starvation must not move population: %d", none.People)
	}
}

// A civil war halves popular support and destroys its severity percent of the
// realm's regions and of every unit type, escrowed listings included.
func TestCivilWarDestroysAShareOfEverything(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Regions = RegionMix{Coastal: 100, Agricultural: 100}
	e.syncLand()
	e.Support = 80
	e.Troopers, e.Jets, e.Turrets, e.Bombers, e.Tanks, e.Carriers = 1000, 1000, 1000, 1000, 1000, 1000
	e.Gold = 1_000_000
	if err := w.SetMarketListing(e, "Tank", 500, 10); err != nil {
		t.Fatalf("listing tanks: %v", err)
	}
	e.CivilWarSeverity = 30

	w.resolveCivilWar(e)

	if e.Support != 40 {
		t.Errorf("support = %d, want 40 (80 halved)", e.Support)
	}
	if e.LastCivilWar != 30 {
		t.Errorf("LastCivilWar = %d, want 30", e.LastCivilWar)
	}
	if e.Land != 140 {
		t.Errorf("land = %d, want 140 (200 less 30%%)", e.Land)
	}
	for _, c := range []struct {
		name string
		got  int
	}{{"troopers", e.Troopers}, {"jets", e.Jets}, {"turrets", e.Turrets},
		{"bombers", e.Bombers}, {"tanks", e.Tanks}, {"carriers", e.Carriers}} {
		want := 700
		if c.name == "tanks" {
			want = 350 // 500 of the 1000 are escrowed on the market
		}
		if c.got != want {
			t.Errorf("%s = %d, want %d", c.name, c.got, want)
		}
	}
	if got := w.MarketForSale(e.Name, "Tank"); got != 350 {
		t.Errorf("escrowed tanks = %d, want 350 — a listing is no shelter", got)
	}
	if e.CivilWarSeverity != 0 {
		t.Errorf("severity should be spent, got %d", e.CivilWarSeverity)
	}
}

// Region maintenance lights a civil war on a far easier trigger than famine:
// anything under 90% of what is due (BRE.OVR 0x2F23C), against famine's 65%.
func TestUnderpaidRegionsFileACivilWar(t *testing.T) {
	pay := func(pct int64) *Empire {
		w := NewWorldSeed(DefaultConfig(), 1)
		e := w.AddHuman("me", "Mine")
		due := w.RegionsDue(e)
		if due <= 0 {
			t.Skip("no region upkeep to underpay")
		}
		e.Gold = due
		w.PayRegions(e, due*pct/100)
		return e
	}
	if e := pay(100); e.CivilWarSeverity != 0 || e.PendingSupportPenalty != 0 {
		t.Errorf("paying in full costs nothing, got war=%d support=%d", e.CivilWarSeverity, e.PendingSupportPenalty)
	}
	if e := pay(95); e.CivilWarSeverity != 0 {
		t.Errorf("95%% paid is above the threshold, got war=%d", e.CivilWarSeverity)
	}
	e := pay(50)
	if e.CivilWarSeverity <= 0 {
		t.Errorf("half-paid land upkeep should file a civil war, got %d", e.CivilWarSeverity)
	}
	if e.PendingSupportPenalty <= 0 {
		t.Errorf("and cost support, got %d", e.PendingSupportPenalty)
	}
}

// A fed realm at a low tax rate recovers popular support each turn from the tax
// drift, while a starving one loses it faster than the drift returns it.
func TestFedRealmRecoversSupport(t *testing.T) {
	mk := func(fed bool) *Empire {
		w := NewWorldSeed(DefaultConfig(), 1)
		e := w.AddHuman("me", "Mine")
		e.Support, e.Tax = 50, 0
		e.Regions, e.Land = RegionMix{Coastal: 20}, 0 // land for capacity, no food production
		e.syncLand()
		e.People = 1000
		if fed {
			e.Food = 1_000_000
		} else {
			e.Food = 0 // will go short of consumption -> starves
		}
		w.PlayTurn(e, "2026-07-03")
		return e
	}
	fed := mk(true)
	if fed.Support <= 50 {
		t.Errorf("a fed realm at a low tax rate should gain support from 50, got %d", fed.Support)
	}
	if s := mk(false).Support; s >= fed.Support {
		t.Errorf("starving realm should lose support instead: got %d vs %d", s, fed.Support)
	}
}
