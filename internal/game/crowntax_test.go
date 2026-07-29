package game

import "testing"

// The crown tax is a flat share of the turn's gross gold income and a pure sink.
// Verified against BRE, where the same formula reproduces 28 of 28 live charges.
func TestCrownTaxIsShareOfIncome(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Regions = RegionMix{Coastal: 50, Desert: 50, Industrial: 20}
	e.syncLand()

	b := w.IncomeThisTurn(e)
	if want := b.CrownTaxBase() * w.Config.PlanetaryTaxRate / 100; w.CrownTax(e) != want {
		t.Errorf("CrownTax = %d, want %d", w.CrownTax(e), want)
	}
	// Trading proceeds are outside the base — BRE accumulates it at the six
	// region/tax income sites only.
	if b.CrownTaxBase() != b.Gold()-b.Trade {
		t.Errorf("base %d should be Gold %d minus Trade %d", b.CrownTaxBase(), b.Gold(), b.Trade)
	}
	// Rate 0 means no tax at all; the sysop can switch it off.
	w.Config.PlanetaryTaxRate = 0
	if got := w.CrownTax(e); got != 0 {
		t.Errorf("rate 0 should levy nothing, got %d", got)
	}
}

// Paying the tax removes the gold from the game — no recipient gains it.
func TestCrownTaxIsASink(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Gold = 10000
	total := func() int {
		n := 0
		for _, x := range w.Empires {
			n += x.Gold
		}
		return n
	}
	before := total()
	w.PayCrownTax(e, 250)
	if e.Gold != 9750 {
		t.Errorf("payer gold = %d, want 9750", e.Gold)
	}
	if after := total(); after != before-250 {
		t.Errorf("world gold %d -> %d: the tax must leave the economy", before, after)
	}
}

// Underpaying the crown tax costs popular support, scaled by how short the
// payment was. BRE-verified formula and multiplier — and, as in BRE, the hit is
// deferred to turn rollover rather than applied on the spot.
func TestCrownTaxShortfallCostsSupport(t *testing.T) {
	newE := func() (*World, *Empire) {
		w := NewWorldSeed(DefaultConfig(), 1)
		e := w.AddHuman("t", "T")
		e.Regions = RegionMix{Coastal: 50, Desert: 50}
		e.syncLand()
		e.Gold = 1_000_000
		e.Support = 100
		return w, e
	}
	w, e := newE()
	req := w.CrownTax(e)
	if req <= 0 {
		t.Fatal("test realm should owe a crown tax")
	}

	w.PayCrownTax(e, req) // paid in full: nothing owed
	if e.PendingSupportPenalty != 0 || e.Support != 100 {
		t.Errorf("full payment should cost nothing, pending=%d support=%d",
			e.PendingSupportPenalty, e.Support)
	}

	w, e = newE()
	w.PayCrownTax(e, 0)
	// Not applied yet — that is the point of deferring it.
	if e.Support != 100 {
		t.Errorf("penalty must not land until rollover, support already %d", e.Support)
	}
	want := req * CrownTaxSupportPenalty / (req + 1)
	if e.PendingSupportPenalty != want {
		t.Errorf("pending penalty %d, want %d", e.PendingSupportPenalty, want)
	}
	// BRE's (paid+1)/(required+1) means even paying zero never costs the full
	// CrownTaxSupportPenalty — it approaches it from below.
	if want >= CrownTaxSupportPenalty {
		t.Errorf("penalty %d should stay under the %d cap", want, CrownTaxSupportPenalty)
	}

	w.PlayTurn(e, "2026-07-29") // rollover applies and clears it
	if e.Support != 100-want {
		t.Errorf("after rollover support %d, want %d", e.Support, 100-want)
	}
	if e.PendingSupportPenalty != 0 {
		t.Errorf("rollover should clear the pending penalty, got %d", e.PendingSupportPenalty)
	}

	w, e = newE()
	w.PayCrownTax(e, req/2) // half: a partial penalty
	if p := e.PendingSupportPenalty; p <= 0 || p >= want {
		t.Errorf("half payment pending %d should sit between 0 and %d", p, want)
	}
}
