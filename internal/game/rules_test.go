package game

import (
	"math"
	"testing"
)

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
	if err := modded.Rules.Validate(); err != nil {
		t.Fatal(err)
	}
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

// Net worth is held at the 32-bit ceiling instead of wrapping when a mod
// weights a unit heavily, so a 32-bit door ranks a large realm as the largest.
func TestNetWorthHoldsAtTheCeiling(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Rules = BaseRules()
	u := w.Rules.Units[Trooper]
	u.NetWorth = ModUnitNetWorthMax
	w.Rules.Units[Trooper] = u
	e := w.AddHuman("h", "Heavy")
	e.Troopers = 1_000_000 // 10 billion of net worth at 10,000 apiece
	if got := w.NetWorth(e); got != math.MaxInt32 {
		t.Errorf("NetWorth = %d, want it held at %d", got, math.MaxInt32)
	}
}

// The planet's total net worth is held at the same ceiling, so two realms at it
// do not wrap the daily bulletin on a 32-bit door.
func TestPlanetNetWorthHoldsAtTheCeiling(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Rules = BaseRules()
	u := w.Rules.Units[Trooper]
	u.NetWorth = ModUnitNetWorthMax
	w.Rules.Units[Trooper] = u
	for _, name := range []string{"a", "b"} {
		w.AddHuman(name, "Heavy "+name).Troopers = 1_000_000
	}
	if got := planetTotals(w).NetWorth; got != math.MaxInt32 {
		t.Errorf("planet net worth = %d, want it held at %d", got, math.MaxInt32)
	}
}

// A mod's offense and defense reach the same-planet battle's strength sums.
// Golden figures: 1,000 jets at 3.25 attack for 3,250 (2,000 at the base 2) and
// defend for 1,500 at 1.5 (none at the base 0); 100 tanks at 7 with a finished
// HeadQuarters keep the HQ curve, 7 x 4.5/3.5 = 9 apiece, so 900.
func TestModdedStrengthReachesTheBattle(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("h", "Hosts")
	e.Troopers, e.Turrets, e.Bombers, e.Tanks = 0, 0, 0, 0
	e.Jets, e.Carriers = 1000, 10 // every jet carried, so all of them fly
	if got := w.Offense(e); got != 2000 {
		t.Fatalf("base offense %d, want 2000 (jet 2)", got)
	}

	w.Rules = BaseRules()
	j := w.Rules.Units[Jet]
	j.Offense, j.Defense = 325, 150
	w.Rules.Units[Jet] = j
	k := w.Rules.Units[Tank]
	k.Offense = 700
	w.Rules.Units[Tank] = k
	if err := w.Rules.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := w.Offense(e); got != 3250 {
		t.Errorf("offense with jets at 3.25 = %d, want 3250", got)
	}
	if got := w.Defense(e); got != 1500 {
		t.Errorf("defense with jets at 1.5 = %d, want 1500", got)
	}

	e.Jets, e.Tanks, e.HQ = 0, 100, 100
	if got := w.Offense(e); got != 900 {
		t.Errorf("100 tanks at 7 with a finished HQ = %d, want 900", got)
	}
	if got := w.Defense(e); got != 450 {
		t.Errorf("tank defense left at the base = %d, want 450", got)
	}
}
