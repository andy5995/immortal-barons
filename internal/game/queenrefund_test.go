package game

import "testing"

// The refund is a share of the Queen's purse, and the purse loses exactly what
// it paid. The two figures below are BRE's own: a fresh game's first refund is
// 2,000 and its second is 1,960, which is what pinned the 100,000 seed and the
// 2% rate.
func TestQueenRefundIsAShareOfThePool(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Protection = 0

	if w.RefundPool != 100_000 {
		t.Fatalf("fresh purse = %d, want 100,000", w.RefundPool)
	}
	if got := w.QueenRefund(e); got != 2000 {
		t.Errorf("first refund = %d, want 2000", got)
	}
	if w.RefundPool != 98_000 {
		t.Errorf("purse after paying 2,000 = %d, want 98,000", w.RefundPool)
	}
	if got := w.QueenRefund(e); got != 1960 {
		t.Errorf("second refund = %d, want 1960", got)
	}
}

// Past 100,000,000 the Queen gives back 7% instead of 2%. 14,000,000 out of a
// 200,000,000 purse is the largest refund seen in the captures.
func TestQueenRefundRateRisesWithAFullPurse(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Protection = 0 // the cap would otherwise mask the rate

	w.RefundPool = 100_000_000 // at the threshold, not past it
	if got := w.QueenRefund(e); got != 2_000_000 {
		t.Errorf("at the threshold: refund = %d, want 2,000,000 (the 2%% rate)", got)
	}
	w.RefundPool = 200_000_000
	if got := w.QueenRefund(e); got != 14_000_000 {
		t.Errorf("past the threshold: refund = %d, want 14,000,000 (the 7%% rate)", got)
	}
}

// The 1,000,000 ceiling applies only while the realm is still under New Realm
// Protection, so a newcomer to a mature planet cannot open with a windfall.
func TestQueenRefundIsCappedOnlyWhileProtected(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")

	w.RefundPool = 200_000_000
	e.Protection = 1
	if got := w.QueenRefund(e); got != 1_000_000 {
		t.Errorf("protected realm: refund = %d, want the 1,000,000 cap", got)
	}
	if want := 200_000_000 - 1_000_000; w.RefundPool != want {
		t.Errorf("purse = %d, want %d — it loses exactly what it paid", w.RefundPool, want)
	}
	e.Protection = 0
	if got := w.QueenRefund(e); got <= 1_000_000 {
		t.Errorf("established realm: refund = %d, the cap must not apply", got)
	}
}

// An empty purse pays nothing at all rather than announcing a refund of zero.
// A world saved before the refund existed loads with an empty one.
func TestQueenRefundOnAnEmptyPurse(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	before := e.Gold

	w.RefundPool = 0
	if got := w.QueenRefund(e); got != 0 {
		t.Errorf("empty purse paid %d", got)
	}
	// Too little to divide: 2% of 49 truncates to nothing.
	w.RefundPool = 49
	if got := w.QueenRefund(e); got != 0 {
		t.Errorf("a purse of 49 paid %d", got)
	}
	if e.Gold != before || w.RefundPool != 49 {
		t.Errorf("nothing should have moved: gold %d, purse %d", e.Gold, w.RefundPool)
	}
}

// The crown tax is what fills the purse, and it banks the gold actually handed
// over rather than the sum demanded.
func TestCrownTaxFillsTheRefundPool(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Regions = RegionMix{Coastal: 50, Desert: 50, Industrial: 20}
	e.syncLand()
	e.Gold = 100_000
	w.RefundPool = 0

	req := w.CrownTax(e)
	if req <= 1 {
		t.Fatalf("this realm owes %d — the test needs a real tax bill", req)
	}
	w.PayCrownTax(e, req-1) // one gold short
	if w.RefundPool != req-1 {
		t.Errorf("purse = %d, want %d — what was paid, not what was owed", w.RefundPool, req-1)
	}
}
