package game

import "testing"

// A 32-bit build (a Win32 BBS door) has 32-bit ints, and several of the game's
// products pass 2^31 at ordinary empire sizes. These run on any build — the
// arithmetic they pin is the same width everywhere once the intermediates are
// int64 — but they are the cases that were actually wrong on 32-bit, so keep
// them and run the suite with GOARCH=386 when touching this math.

// The reported case: a 710-region realm on a 32-bit door was billed -210,763
// gold in region maintenance, because Land*913*10000 wrapped inside techLower.
func TestRegionUpkeepStaysPositiveAtDoorScale(t *testing.T) {
	for _, land := range []int{235, 710, 6883, 20000} {
		e := &Empire{Land: land, Regions: RegionMix{Coastal: land}}
		if got := (&World{}).RegionUpkeep(e); got <= 0 {
			t.Errorf("land=%d: region upkeep came out %d, want a positive charge", land, got)
		}
		if want := int64(land * RegionUpkeepPerLand); (&World{}).RegionUpkeep(e) != want {
			t.Errorf("land=%d: region upkeep %d, want %d with no technology", land, (&World{}).RegionUpkeep(e), want)
		}
	}
}

func TestTechScalingSurvivesLargeValues(t *testing.T) {
	// Two hundred million, well past what wraps at 32 bits once multiplied by
	// TechFactorUnit.
	const v = 200_000_000
	if got := techLower(v, TechFactorUnit); got != v {
		t.Errorf("techLower with a neutral factor = %d, want %d", got, v)
	}
	if got := techRaise(v, TechFactorUnit); got != v {
		t.Errorf("techRaise with a neutral factor = %d, want %d", got, v)
	}
}

func TestPctOfSurvivesMoneyScale(t *testing.T) {
	if got := pctOf(50_000_000, 80); got != 40_000_000 {
		t.Errorf("pctOf(50m, 80) = %d, want 40,000,000", got)
	}
	if got := pctOf(2_000_000_000, 100); got != 2_000_000_000 {
		t.Errorf("pctOf at the money cap = %d, want it unchanged", got)
	}
}

// A rich AI must still spend: its budget is a percentage of gold, which wrapped
// negative on 32-bit and left it buying nothing at all.
func TestRichAIStillBuysForces(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.AIProfile, e.AISkill = AIProfileAggressor, AISkillSharp
	e.Regions = RegionMix{Coastal: 50}
	e.syncLand()
	e.Gold = 500_000_000
	w.aiBuildForces(e)
	if e.Troopers+e.Turrets+e.Tanks+e.Jets == 0 {
		t.Error("an AI holding 500m gold bought no forces at all")
	}
	if e.Gold < 0 {
		t.Errorf("spending drove gold negative: %d", e.Gold)
	}
}

// The SDI program multiplies its funding total by a percentage and by a
// part-payment, both of which pass 2^31 at ordinary funding levels. A 32-bit
// build reported a funding total of -36,870 after a half-paid upkeep bill on a
// million-gold program.
func TestSDIMathSurvivesFundingScale(t *testing.T) {
	w := NewWorld(DefaultConfig())
	e := &Empire{}
	for _, funding := range []int64{1_000_000, 75_000_000, 500_000_000} {
		e.SDIFunding = funding
		e.TurnProgress.SDIFunded = 0
		if got, want := w.SDIMaintenance(e), funding/25; got != want {
			t.Errorf("funding=%d: upkeep %d, want %d", funding, got, want)
		}
		if got, want := w.SDISpendAllowance(e), max(SDIMinSpend, funding/5); got != want {
			t.Errorf("funding=%d: allowance %d, want %d", funding, got, want)
		}
	}

	// Paying half the upkeep halves the program, and never goes negative.
	e.SDIFunding = 1_000_000
	e.Gold = 100_000
	w.PaySDI(e, w.SDIMaintenance(e)/2)
	if e.SDIFunding != 500_000 {
		t.Errorf("funding after a half payment = %d, want 500,000", e.SDIFunding)
	}
}

// A mod may set a region figure far above the base game's (region_rules.go's
// limits), which brings the region-count products into int32's reach at a few
// thousand regions: 30,000 Deserts at food_base 100,000 grow 3e9 food. On a
// 32-bit build the totals saturate rather than wrap negative.
func TestModdedRegionTotalsSaturate(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Rules = BaseRules()
	e := w.AddHuman("a", "Alpha")
	e.Regions = RegionMix{Desert: 30_000}
	e.Land = e.Regions.Total()
	rr := w.Rules.Regions[Desert]
	rr.FoodBase, rr.FoodRate, rr.PopCap, rr.GoldBase, rr.GoldRate = ModRegionFoodMax, 0, ModRegionPopCapMax, ModRegionGoldMax, 0
	w.Rules.Regions[Desert] = rr
	if err := w.Rules.Validate(); err != nil {
		t.Fatal(err)
	}
	want := fitInt(3_000_000_000)
	if got := w.FoodGrown(e); got != want {
		t.Errorf("FoodGrown = %d, want %d", got, want)
	}
	if got := w.AvgFood(e, Desert); got != want {
		t.Errorf("AvgFood = %d, want %d", got, want)
	}
	if got := w.popCapacity(e); got <= 0 {
		t.Errorf("popCapacity = %d, want a positive capacity", got)
	}
	// Income is money, held in int64 all the way to creditGold's cap: 30,000
	// Deserts at gold_base 1,000,000 pay 3e10 before taxes and trade.
	if got := w.IncomeThisTurn(e).Gold(); got < 30_000_000_000 {
		t.Errorf("income = %d, want at least 30,000,000,000", got)
	}
}
