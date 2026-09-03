package menu

import (
	"strings"
	"testing"
)

// setJetPrices pins the two prices the carrier bundle is arithmetic on, so the
// expected counts below are golden literals rather than a restatement of the
// formula under test.
func setJetPrices(w *ctx) {
	p := w.Player()
	p.Prices.Jet, p.Prices.Carrier = 1500, 5000
	p.Gold = 2_000_000
	p.Jets, p.Carriers = 0, 0
}

// A 1000-jet order at 1500 gold is a 1,500,000 budget. A flight costs
// 100x1500 + 5000 = 155,000, so the budget covers 9 of them; paying for those 9
// carriers leaves 1,455,000, which buys 970 jets. The whole budget is spent and
// not a gold more.
func TestBuyJetsWithCarriersSpendsTheSameGold(t *testing.T) {
	w := newWorld()
	setJetPrices(w)
	f := &fakeSession{keys: []rune("1000\ry")}

	buyJets(f, w)

	p := w.Player()
	if p.Jets != 970 || p.Carriers != 9 {
		t.Errorf("Jets/Carriers = %d/%d, want 970/9", p.Jets, p.Carriers)
	}
	if p.Gold != 500_000 {
		t.Errorf("Gold = %d, want 500000 (the plain 1000-jet price, no more)", p.Gold)
	}
	if !strings.Contains(f.out.String(), "970 Jets and 9 Carriers purchased.") {
		t.Errorf("purchase not reported:\n%s", f.out.String())
	}
}

// The offer states both counts, so the trade is a decision rather than a guess.
func TestBuyJetsOfferQuotesTheExactCounts(t *testing.T) {
	w := newWorld()
	setJetPrices(w)
	f := &fakeSession{keys: []rune("1000\rn")}

	buyJets(f, w)

	// The line wraps, so match either side of the break rather than across it.
	for _, want := range []string{"970 Jets and 9 Carriers to lift them", "1,000 Jets on their own"} {
		if !strings.Contains(f.out.String(), want) {
			t.Errorf("offer did not quote %q:\n%s", want, f.out.String())
		}
	}
}

func TestBuyJetsDeclinedBuysJetsAlone(t *testing.T) {
	w := newWorld()
	setJetPrices(w)
	f := &fakeSession{keys: []rune("1000\rn")}

	buyJets(f, w)

	p := w.Player()
	if p.Jets != 1000 || p.Carriers != 0 {
		t.Errorf("Jets/Carriers = %d/%d, want 1000/0", p.Jets, p.Carriers)
	}
	if p.Gold != 500_000 {
		t.Errorf("Gold = %d, want 500000", p.Gold)
	}
}

// Enter at the offer takes the (y/N) default, which is the plain buy.
func TestBuyJetsOfferDefaultsToNo(t *testing.T) {
	w := newWorld()
	setJetPrices(w)
	f := &fakeSession{keys: []rune("1000\r\r")}

	buyJets(f, w)

	if p := w.Player(); p.Jets != 1000 || p.Carriers != 0 {
		t.Errorf("Jets/Carriers = %d/%d, want 1000/0", p.Jets, p.Carriers)
	}
}

// An order no bigger than one carrier's load is never asked about.
func TestBuyJetsSmallOrderIsNotOffered(t *testing.T) {
	w := newWorld()
	setJetPrices(w)
	f := &fakeSession{keys: []rune("100\r")}

	buyJets(f, w)

	p := w.Player()
	if p.Jets != 100 || p.Carriers != 0 {
		t.Errorf("Jets/Carriers = %d/%d, want 100/0", p.Jets, p.Carriers)
	}
	if strings.Contains(f.out.String(), "Include carriers") {
		t.Errorf("small order was offered carriers:\n%s", f.out.String())
	}
}
