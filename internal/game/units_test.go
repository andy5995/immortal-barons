package game

import "testing"

// The table is what several screens and both naming forms now resolve through,
// so a row missing an accessor would be a nil dereference on whichever screen
// reaches it first rather than a compile error (#134).
func TestGoodRowsAreComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range AllGoods {
		if g.Singular == "" || g.Plural == "" {
			t.Errorf("%+v: both naming forms are stored in save files and must be set", g)
		}
		for _, name := range []string{g.Singular, g.Plural} {
			if seen[name] && g.Singular != g.Plural {
				t.Errorf("%q names two goods; GoodByName cannot tell them apart", name)
			}
			seen[name] = true
		}
		if g.Count == nil || g.Basket == nil || g.Price == nil {
			t.Errorf("%s: every tradeable good has a count, a basket slot and a shop price", g.Plural)
		}
		if g.Military && (g.Prod == nil || g.Made == nil || g.Cost <= 0 || g.NetWorth <= 0) {
			t.Errorf("%s: a unit Industrial builds needs its production fields, cost and net worth", g.Plural)
		}
		if !g.Military && (g.Prod != nil || g.Made != nil) {
			t.Errorf("%s: Industrial regions do not build it, so it has no production fields", g.Plural)
		}
	}
	if len(MilitaryGoods) != 6 {
		t.Errorf("MilitaryGoods = %d units, want BRE's 6", len(MilitaryGoods))
	}
	for _, g := range MarketGoods {
		if GoodByName(g.Singular) != g {
			t.Errorf("%s is on the market but does not resolve by name", g.Singular)
		}
	}
}

// Every good's accessors must point at that good's own fields. A copy-paste
// slip in the table would otherwise show up as one screen quietly reporting
// another unit's count.
func TestGoodAccessorsPointAtTheirOwnFields(t *testing.T) {
	var e Empire
	var b TradeBasket
	for i, g := range AllGoods {
		*g.Count(&e) = 100 + i
		*g.Basket(&b) = 200 + i
		if g.Prod != nil {
			*g.Prod(&e) = 10 + i
			*g.Made(&e) = 20 + i
		}
	}
	for i, g := range AllGoods {
		if got := *g.Count(&e); got != 100+i {
			t.Errorf("%s: Count reads %d, want %d — two rows share a field", g.Plural, got, 100+i)
		}
		if got := *g.Basket(&b); got != 200+i {
			t.Errorf("%s: Basket reads %d, want %d — two rows share a field", g.Plural, got, 200+i)
		}
		if g.Prod == nil {
			continue
		}
		if got := *g.Prod(&e); got != 10+i {
			t.Errorf("%s: Prod reads %d, want %d", g.Plural, got, 10+i)
		}
		if got := *g.Made(&e); got != 20+i {
			t.Errorf("%s: Made reads %d, want %d", g.Plural, got, 20+i)
		}
	}
}

// The pirate enum is persisted as a number, and its labels used to live in a
// separate list in another package, in an order nothing checked.
func TestPirateSpoilsResolveThroughTheTable(t *testing.T) {
	for _, c := range []struct {
		spoil PirateSpoil
		want  string
	}{
		{SpoilTroopers, "Troopers"}, {SpoilJets, "Jets"}, {SpoilTurrets, "Turrets"},
		{SpoilTanks, "Tanks"}, {SpoilGold, "Gold"}, {SpoilAgents, "Agents"},
	} {
		if got := c.spoil.Label(); got != c.want {
			t.Errorf("spoil %d labels %q, want %q", c.spoil, got, c.want)
		}
	}
	if SpoilGold.Good() != nil {
		t.Error("gold is not a market good and has no table row")
	}
	if got := SpoilTroopers.marketGood(); got != "Trooper" {
		t.Errorf("a raid on the market looks for %q, want the singular Trooper", got)
	}
}
