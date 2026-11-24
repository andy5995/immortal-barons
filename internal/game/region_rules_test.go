package game

import (
	"strings"
	"testing"
)

// A changed region row reaches the engine: the figures are read from the rules,
// not from the constants they came from.
func TestModdedRegionRowsReachTheEngine(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Rules = BaseRules()
	e := w.AddHuman("a", "Alpha")
	e.Regions = RegionMix{}
	e.Regions.Desert = 1000

	if got := w.FoodGrown(e); got != 0 {
		t.Fatalf("a Desert-only realm grows %d food under the base rules, want 0", got)
	}
	rr := w.Rules.Regions[Desert]
	rr.FoodBase, rr.FoodRate = 300, 0
	w.Rules.Regions[Desert] = rr
	if err := w.Rules.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := w.FoodGrown(e); got != 300*1000 {
		t.Errorf("1000 Deserts at food_base 300 grow %d food, want 300000", got)
	}
	rr.FoodShare = 50
	w.Rules.Regions[Desert] = rr
	if got := w.FoodGrown(e); got != 150*1000 {
		t.Errorf("at food_share 50 they grow %d food, want 150000", got)
	}

	e.Regions = RegionMix{}
	e.Regions.Urban = 10
	if got := w.IncomeThisTurn(e).GoldFrom(Urban); got != 0 {
		t.Fatalf("Urban pays %d gold under the base rules, want 0", got)
	}
	ur := w.Rules.Regions[Urban]
	ur.GoldBase, ur.GoldRate = 1000, 0
	w.Rules.Regions[Urban] = ur
	// Region gold is raised by the gold technology factor; a realm with none
	// researched has the unit factor, so it is exactly count x base.
	if got := w.IncomeThisTurn(e).GoldFrom(Urban); got != 10*1000 {
		t.Errorf("10 Urban regions at gold_base 1000 pay %d gold, want 10000", got)
	}

	e.Regions = RegionMix{}
	e.Regions.Agricultural = 10
	if got := w.RegionUpkeep(e); got != 10*RegionUpkeepPerLand {
		t.Fatalf("upkeep %d under the base rules, want %d", got, 10*RegionUpkeepPerLand)
	}
	ar := w.Rules.Regions[Agricultural]
	ar.Upkeep = 100
	w.Rules.Regions[Agricultural] = ar
	if got := w.RegionUpkeep(e); got != 1000 {
		t.Errorf("10 Agricultural regions at upkeep 100 cost %d, want 1000", got)
	}
}

// The three figures ruined land may keep are marked on their fields, so an
// exemption cannot drift away from the figure it belongs to.
func TestWasteExemptionsLiveOnTheFields(t *testing.T) {
	allowed := map[string]bool{"upkeep": true, "gold_share": true, "food_share": true}
	for _, f := range RegionFields {
		if f.WasteMay != allowed[f.Key] {
			t.Errorf("%s: WasteMay = %v, want %v", f.Key, f.WasteMay, allowed[f.Key])
		}
	}
	rr := baseRegionRules()[Waste]
	rr.GoldBase, rr.Upkeep = 1, ModRegionUpkeepMax
	errs := rr.Validate(Waste)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "waste: gold_base must be 0: ruined land produces nothing") {
		t.Errorf("Validate = %v, want only gold_base refused", errs)
	}
}

// A research weight above 1 cannot lift the research count past the realm's
// land, so a modded realm researches no faster than an all-Technology one.
func TestResearchCountIsHeldToTheLand(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Rules = BaseRules()
	tr := w.Rules.Regions[Technology]
	tr.Research = ModRegionResearchMax
	w.Rules.Regions[Technology] = tr
	e := w.AddHuman("a", "Alpha")
	e.Regions = RegionMix{Technology: 1_000}
	if got := w.researchRegions(e); got != 1_000 {
		t.Errorf("research count %d, want the land, 1000", got)
	}
	e.Regions = RegionMix{Technology: 10, Desert: 990}
	if got := w.researchRegions(e); got != 1_000 {
		t.Errorf("research count %d, want 1000 (10 x 100)", got)
	}
}
