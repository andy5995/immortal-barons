package game

import "testing"

// Every figure a mod can set reaches the code that uses it. A read site left on
// a constant would play the base value under a mod and pass every other test.
func TestModdedRulesReachTheEngine(t *testing.T) {
	base := NewWorldSeed(DefaultConfig(), 1)
	modded := NewWorldSeed(DefaultConfig(), 1)
	modded.Rules = BaseRules()
	u := modded.Rules.Units[Tank]
	u.Cost = 250        // half: twice as many tanks built
	u.NetWorth = 2500   // double
	u.UpkeepTenths = 60 // ten times
	u.PriceLo, u.PriceHi, u.PriceStep = 5000, 6000, 10
	modded.Rules.Units[Tank] = u
	modded.Reset() // prices the fresh market under the rules

	if got := *Tank.Stored(&modded.Prices); got != 5500 {
		t.Errorf("starting tank price %d, want the modded band's middle, 5500", got)
	}

	eb, em := base.AddHuman("b", "Base"), modded.AddHuman("m", "Modded")
	for _, e := range []*Empire{eb, em} {
		e.Troopers, e.Tanks = 0, 1000
	}
	if got, want := modded.NetWorth(em)-base.NetWorth(eb), 1250; got != want {
		t.Errorf("1000 tanks at 2.5 instead of 1.25 add %d net worth, want %d", got, want)
	}
	if got, want := modded.ForcesUpkeep(em), 10*base.ForcesUpkeep(eb); got != want {
		t.Errorf("tank upkeep %d, want ten times the base %d", got, want)
	}

	idx := -1
	for i, g := range MilitaryGoods {
		if g == Tank {
			idx = i
		}
	}
	for _, e := range []*Empire{eb, em} {
		e.Regions.Industrial = 100
		e.Specialized = ""
		*Tank.Prod(e) = 100
	}
	pb, pm := base.ProjectedProduction(eb)[idx], modded.ProjectedProduction(em)[idx]
	if pb == 0 || pm < 2*pb-1 || pm > 2*pb+1 {
		t.Errorf("tanks built %d at half the cost, want about twice the base %d", pm, pb)
	}

	*Tank.Stored(&em.Prices) = 5500
	for range 50 {
		modded.stepPrices(em)
		if p := *Tank.Stored(&em.Prices); p < 5000 || p > 6000 {
			t.Fatalf("tank price walked to %d, outside the modded band", p)
		}
	}
}
