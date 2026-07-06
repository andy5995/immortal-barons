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

// TestAdvisorsWarnLowSupport checks the Advisors screen surfaces the
// low-support nudge when Support drops below the threshold.
func TestAdvisorsWarnLowSupport(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Support = 20

	f := &fakeSession{}
	renderAdvisors(f, w)
	out := f.out.String()
	if !strings.Contains(out, "The people grow restless") {
		t.Errorf("expected low-support advice; output:\n%s", out)
	}
}

// TestAdvisorsWarnFoodDeficit checks the food-shortfall nudge appears when
// stores won't cover next turn's consumption.
func TestAdvisorsWarnFoodDeficit(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Food = 0

	f := &fakeSession{}
	renderAdvisors(f, w)
	out := f.out.String()
	if !strings.Contains(out, "food will not last the turn") {
		t.Errorf("expected food-deficit advice; output:\n%s", out)
	}
}

// TestAdvisorsWarnLowMorale checks the desertion-risk nudge appears when
// Military Morale drops below the threshold.
func TestAdvisorsWarnLowMorale(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Morale = 20

	f := &fakeSession{}
	renderAdvisors(f, w)
	out := f.out.String()
	if !strings.Contains(out, "Desertion is a real risk") {
		t.Errorf("expected low-morale advice; output:\n%s", out)
	}
}

// TestAdvisorsHealthyEmpire checks that an empire with no outstanding issues
// gets the neutral "in good order" message and none of the warning lines.
func TestAdvisorsHealthyEmpire(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.HQ = 100
	p.Land = 100
	p.Regions.Technology = 20 // gives a non-zero TechFactor
	p.Agents = 5

	f := &fakeSession{}
	renderAdvisors(f, w)
	out := f.out.String()
	if !strings.Contains(out, "in good order") {
		t.Errorf("expected the neutral healthy-empire message; output:\n%s", out)
	}
	for _, warn := range []string{
		"HeadQuarters", "carriers", "Desertion", "food will not last",
		"grow restless", "riots", "Technology infrastructure", "treasury is empty",
		"debt", "covert agents",
	} {
		if strings.Contains(out, warn) {
			t.Errorf("did not expect warning containing %q for a healthy empire; output:\n%s", warn, out)
		}
	}
}
