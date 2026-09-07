package game

import "testing"

// The gold handout pays every living realm purse/50, charging the purse once
// per realm. Golden literals, not the constant (AGENTS.md).
func TestCrownGoldHandoutPaysEveryRealmTheSameShare(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Empires = nil
	for _, n := range []string{"A", "B", "C"} {
		e := w.AddHuman(n, n)
		e.Gold = 0
	}
	w.RefundPool = 10_000_000
	w.crownGoldHandout()

	const wantShare = 200_000 // 10,000,000 / 50
	for _, e := range w.Empires {
		if e.Gold != wantShare {
			t.Errorf("%s got %d gold, want %d", e.Name, e.Gold, wantShare)
		}
		if len(e.Events) != 1 {
			t.Errorf("%s got %d recap entries, want 1", e.Name, len(e.Events))
		}
	}
	// Charged once per realm, not once.
	if want := int64(10_000_000 - 3*wantShare); w.RefundPool != want {
		t.Errorf("purse = %d, want %d (charged per realm)", w.RefundPool, want)
	}
}

// The trooper handout divides by the realm count first, so a crowded planet
// gets fewer each -- the opposite of the gold handout.
func TestCrownTrooperHandoutDividesByRealmCount(t *testing.T) {
	for _, tc := range []struct{ realms, want int }{{2, 5_000}, {5, 2_000}} {
		w := NewWorldSeed(DefaultConfig(), 1)
		w.Empires = nil
		for i := 0; i < tc.realms; i++ {
			e := w.AddHuman(string(rune('A'+i)), string(rune('A'+i)))
			e.Troopers = 0
		}
		w.RefundPool = 10_000_000 // /1000 = 10,000 troopers to share out
		w.crownTrooperHandout()
		for _, e := range w.Empires {
			if e.Troopers != tc.want {
				t.Errorf("%d realms: %s got %d troopers, want %d", tc.realms, e.Name, e.Troopers, tc.want)
			}
		}
		if want := int64(10_000_000 - tc.realms*tc.want*1000); w.RefundPool != want {
			t.Errorf("%d realms: purse = %d, want %d", tc.realms, w.RefundPool, want)
		}
	}
}

// Capped at 25,000 each however deep the purse is.
func TestCrownTrooperHandoutIsCapped(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Empires = nil
	e := w.AddHuman("A", "A")
	e.Troopers = 0
	w.RefundPool = 10_000_000_000
	w.crownTrooperHandout()
	if e.Troopers != 25_000 {
		t.Errorf("got %d troopers, want the 25,000 cap", e.Troopers)
	}
}

// A purse too thin to buy one trooper each buys none, rather than rounding
// somebody up -- and pays out no gold either.
func TestCrownHandoutsDoNothingOnAnEmptyPurse(t *testing.T) {
	for _, pay := range []func(*World){(*World).crownGoldHandout, (*World).crownTrooperHandout} {
		w := NewWorldSeed(DefaultConfig(), 1)
		w.Empires = nil
		e := w.AddHuman("A", "A")
		e.Gold, e.Troopers = 0, 0
		w.RefundPool = 999 // under one trooper's price, and /50 = 19 gold
		before := w.RefundPool
		pay(w)
		if e.Troopers != 0 {
			t.Errorf("troopers moved on a thin purse: %d", e.Troopers)
		}
		_ = before
	}
	// And nothing at all on an empty one.
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Empires = nil
	e := w.AddHuman("A", "A")
	e.Gold, e.Troopers = 0, 0
	w.RefundPool = 0
	w.crownGoldHandout()
	w.crownTrooperHandout()
	if e.Gold != 0 || e.Troopers != 0 || len(e.Events) != 0 {
		t.Errorf("an empty purse paid out: gold=%d troopers=%d events=%v", e.Gold, e.Troopers, e.Events)
	}
}
