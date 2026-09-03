package menu

import (
	"strings"
	"testing"
)

// The carrier screen is where a grounded air force can be fixed, so it says how
// many carriers that would take before asking how many to buy.
func TestBuyCarriersReportsTheShortfall(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold, p.Prices.Carrier = 5_000_000, 5000
	p.Jets, p.Carriers = 40_250, 250 // 403 needed, 153 short
	f := &fakeSession{keys: []rune("150\r")}

	buyCarriers(f, w)

	out := stripANSI(f.out.String())
	if want := "Your 40,250 Jets need 153 more Carriers"; !strings.Contains(out, want) {
		t.Errorf("shortfall note missing %q:\n%s", want, out)
	}
	if p.Carriers != 400 {
		t.Errorf("Carriers = %d, want 400 (the note must not change the purchase)", p.Carriers)
	}
}

// A realm whose lift already covers its jets is told nothing.
func TestBuyCarriersSaysNothingWhenLiftIsEnough(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold, p.Prices.Carrier = 5_000_000, 5000
	p.Jets, p.Carriers = 40_000, 400
	f := &fakeSession{keys: []rune("0\r")}

	buyCarriers(f, w)

	if strings.Contains(stripANSI(f.out.String()), "more Carriers before") {
		t.Errorf("shortfall note shown with enough carriers:\n%s", f.out.String())
	}
}

// A partial flight still needs a whole carrier.
func TestCarrierShortfallRoundsUp(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold, p.Prices.Carrier = 5_000_000, 5000
	p.Jets, p.Carriers = 101, 1
	f := &fakeSession{keys: []rune("0\r")}

	buyCarriers(f, w)

	if !strings.Contains(stripANSI(f.out.String()), "need 1 more Carriers") {
		t.Errorf("101 jets over 1 carrier is 1 short:\n%s", f.out.String())
	}
}
