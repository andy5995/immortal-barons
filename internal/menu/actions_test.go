package menu

import (
	"strings"
	"testing"
)

// TestEmpireStatusShowsTechnologyBonus checks that the Empire Status screen
// surfaces the Technology regions' efficiency bonus (TechFactor), and that
// the line is omitted when the empire has no Technology regions.
func TestEmpireStatusShowsTechnologyBonus(t *testing.T) {
	w := newWorld()
	p := w.Player()

	// Give the empire a Technology share large enough to produce a bonus
	// without hitting TechFactorCap, so the rendered percent is exact.
	p.Land = 100
	p.Regions.Technology = 20
	// TechFactor = Regions.Technology * 100 / Land = 20%.

	f := &fakeSession{}
	renderEmpireStatus(f, w)
	out := f.out.String()
	if !strings.Contains(out, "Technology Bonus") {
		t.Errorf("expected Technology Bonus line; output:\n%s", out)
	}
	// The percent is colorized (ANSI codes split "+" from "20%"), so check
	// for the line and the number rather than the literal "+20%" substring.
	if !strings.Contains(out, "20%") {
		t.Errorf("expected 20%% technology bonus; output:\n%s", out)
	}
}

func TestEmpireStatusNoTechnologyBonusWhenZero(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Land = 100
	p.Regions.Technology = 0

	f := &fakeSession{}
	renderEmpireStatus(f, w)
	out := f.out.String()
	if strings.Contains(out, "Technology Bonus") {
		t.Errorf("did not expect Technology Bonus line with 0 Technology regions; output:\n%s", out)
	}
}
