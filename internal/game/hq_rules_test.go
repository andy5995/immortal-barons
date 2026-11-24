package game

import (
	"slices"
	"strings"
	"testing"
)

// A modded HeadQuarters figure reaches the engine: a read site left on a
// constant would play the base value under a mod.
func TestModdedHQReachesTheEngine(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Rules = BaseRules()
	w.Rules.HQ.PriceBase = 2000
	w.Rules.HQ.PricePerTurn = 0
	w.Rules.HQ.PriceJitter = 0
	w.Rules.HQ.BuildStart = 10
	w.Rules.HQ.BuildPerTurn = 10
	if err := w.Rules.Validate(); err != nil {
		t.Fatal(err)
	}

	e := w.AddHuman("tester", "Testland")
	if got := w.HQPrice(e); got != 2000 {
		t.Fatalf("HQ price %d, want the modded flat 2000", got)
	}
	e.Gold = 2000
	if err := w.StartHQ(e); err != nil {
		t.Fatal(err)
	}
	if e.Gold != 0 || e.HQ != 10 {
		t.Errorf("after buying: gold %d HQ %d, want 0 and 10", e.Gold, e.HQ)
	}
	w.PlayTurn(e, "2026-07-03")
	if e.HQ != 20 {
		t.Errorf("HQ after one turn %d, want 20 (start 10, plus 10)", e.HQ)
	}
}

// The tank bonus is the per-HQ-point term of the local tank curve.
func TestModdedTankBonusScalesTheDefense(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Rules = BaseRules()
	e := w.AddHuman("tester", "Testland")
	e.Troopers, e.Jets, e.Turrets, e.Bombers, e.Carriers = 0, 0, 0, 0, 0
	e.Tanks, e.HQ = 100, 100
	// Base: 3.5 -> 4.5 troopers a tank at a finished HeadQuarters.
	before := w.Defense(e)
	tank := w.Rules.Units[Tank]
	tank.HQBonus = 2
	w.Rules.Units[Tank] = tank
	after := w.Defense(e)
	if before >= after {
		t.Errorf("defense %d then %d, want a bigger bonus to raise it", before, after)
	}
	// (350 + 2*100) / 350 of a tank's 350 hundredths, 100 tanks: 550 troopers
	// before technology; with no Technology regions the factor is exactly 1.
	if got := w.unitStrength(Tank, 100, TankStrengthPctBase, 100); got != 550 {
		t.Errorf("100 tanks at HQ 100 with bonus 0.02 = %d troopers, want 550", got)
	}
}

func TestHQRulesAreRefusedOutOfRange(t *testing.T) {
	cases := []struct {
		name string
		set  func(h *HQRules)
		want string
	}{
		{"start", func(h *HQRules) { h.BuildStart = 0 }, "build_start 0 is outside 1 to 100"},
		{"per turn", func(h *HQRules) { h.BuildPerTurn = 101 }, "build_per_turn 101 is outside 1 to 100"},
		{"loss jitter zero", func(h *HQRules) { h.BattleLossJitter = 0 }, "battle_loss_jitter 0 is outside 1 to 100"},
		{"price base", func(h *HQRules) { h.PriceBase = 0 }, "price_base 0 is outside 1 to 1000000000"},
		{"cap below base", func(h *HQRules) { h.PriceCap = 4999 }, "price_cap 4999 must be at least price_base (5000)"},
		{"cap jitter", func(h *HQRules) { h.PriceCapJitter = h.PriceCap }, "price_cap_jitter 100000 is outside 0 to one below price_cap"},
		{"per turn", func(h *HQRules) { h.PricePerTurn = 2_000_000 }, "price_per_turn 2000000 is outside 0 to 1000000"},
	}
	for _, c := range cases {
		r := BaseRules()
		c.set(&r.HQ)
		err := r.Validate()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to say %q", c.name, err, c.want)
		}
	}
}

// The largest price the limits allow still computes without wrapping an int.
func TestHQPriceAtTheLimitsDoesNotWrap(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Rules = BaseRules()
	w.Rules.HQ.PriceBase = ModHQPriceMax
	w.Rules.HQ.PricePerTurn = ModHQPricePerTurnMax
	w.Rules.HQ.PriceCap = ModHQPriceMax
	if err := w.Rules.Validate(); err != nil {
		t.Fatal(err)
	}
	e := w.AddHuman("tester", "Testland")
	e.TurnsPlayed = 2_000_000_000
	if got := w.HQPrice(e); got <= 0 || got > ModHQPriceMax {
		t.Errorf("HQ price %d, want a positive price within %d", got, ModHQPriceMax)
	}
}

// A HeadQuarters strengthens whichever units carry an HQ bonus: troopers given
// the tank's 0.01 gain the tank's proportion, and the tank keeps its own.
func TestHQBonusOnOtherUnits(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Rules = BaseRules()
	// 350 x 100 troopers' worth of pct at HQ 100: (350 + 100) / 350 of 1.00 each.
	if got := w.unitStrength(Trooper, 350, 100, 100); got != 350 {
		t.Fatalf("troopers with no bonus = %d, want 350", got)
	}
	tr := w.Rules.Units[Trooper]
	tr.HQBonus = 1
	w.Rules.Units[Trooper] = tr
	if err := w.Rules.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := w.unitStrength(Trooper, 350, 100, 100); got != 450 {
		t.Errorf("350 troopers with bonus 0.01 at HQ 100 = %d, want 450", got)
	}
	if got, want := w.UnitsWithHQBonus(), []*Good{Trooper, Tank}; !slices.Equal(got, want) {
		t.Errorf("UnitsWithHQBonus = %v, want troopers and tanks", got)
	}
	tr.HQBonus = ModUnitHQBonusMax + 1
	w.Rules.Units[Trooper] = tr
	if err := w.Rules.Validate(); err == nil || !strings.Contains(err.Error(), "trooper: hq_bonus 0.11 is outside 0 to 0.1") {
		t.Errorf("err %v, want the bonus refused", err)
	}
}
