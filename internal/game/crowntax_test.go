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
// payment was. BRE-verified formula and multiplier.
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

	w.PayCrownTax(e, req) // paid in full: no penalty
	if e.Support != 100 {
		t.Errorf("full payment should not cost support, got %d", e.Support)
	}

	w, e = newE()
	w.PayCrownTax(e, 0) // paid nothing: the largest penalty
	// BRE's (paid+1)/(required+1) means even paying zero never costs the full
	// CrownTaxSupportPenalty — it approaches it from below.
	if want := 100 - req*CrownTaxSupportPenalty/(req+1); e.Support != want {
		t.Errorf("paying nothing: support %d, want %d", e.Support, want)
	}
	if e.Support <= 100-CrownTaxSupportPenalty {
		t.Errorf("penalty should stay under the %d cap, support %d", CrownTaxSupportPenalty, e.Support)
	}

	w, e = newE()
	w.PayCrownTax(e, req/2) // half: a partial penalty, and never negative
	if e.Support >= 100 || e.Support <= 100-CrownTaxSupportPenalty {
		t.Errorf("half payment: support %d should sit between %d and 100",
			e.Support, 100-CrownTaxSupportPenalty)
	}
}
