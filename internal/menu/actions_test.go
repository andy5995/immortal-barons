package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestWriteMacrosBackspaceEdits: recording a macro, Backspace deletes the last
// key and Enter is recorded — mirroring BRE's editor.
func TestWriteMacrosBackspaceEdits(t *testing.T) {
	w := newWorld()
	// Edit Ctrl-D: 6, X, Backspace (removes X), C, 5, Enter, then Ctrl-D to end.
	f := &fakeSession{keys: []rune{'D', '6', 'X', '\b', 'C', '5', '\r', 4}}
	writeMacros(f, w)
	if got := w.Player().Macros["D"]; got != "6C5\r" {
		t.Errorf("macro D: want %q, got %q", "6C5\r", got)
	}
}

// TestWriteMacrosIgnoresControlKeys: stray control keys are not recorded.
func TestWriteMacrosIgnoresControlKeys(t *testing.T) {
	w := newWorld()
	// Edit Ctrl-D: 6, a stray Ctrl-G (bell, rune 7, ignored), C, then Ctrl-D.
	f := &fakeSession{keys: []rune{'D', '6', 7, 'C', 4}}
	writeMacros(f, w)
	if got := w.Player().Macros["D"]; got != "6C" {
		t.Errorf("control key should be ignored: want %q, got %q", "6C", got)
	}
}

// TestAdvisorsWarnLowSupport checks the Civilian advisor surfaces the
// low-support nudge when Support drops below the threshold.
func TestAdvisorsWarnLowSupport(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Support = 20

	f := &fakeSession{}
	renderAdvisor(f, w, advisorCivilian)
	out := f.out.String()
	if !strings.Contains(out, "The people grow restless") {
		t.Errorf("expected low-support advice; output:\n%s", out)
	}
}

// TestAdvisorsWarnFoodDeficit checks the Civilian advisor surfaces the
// food-shortfall nudge when stores won't cover next turn's consumption.
func TestAdvisorsWarnFoodDeficit(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Food = 0

	f := &fakeSession{}
	renderAdvisor(f, w, advisorCivilian)
	out := f.out.String()
	if !strings.Contains(out, "food will not last the turn") {
		t.Errorf("expected food-deficit advice; output:\n%s", out)
	}
}

// TestAdvisorsWarnGrowthOutrunsFood checks the Civilian advisor warns while a
// realm is still fed now but its support-driven population is growing toward a
// capacity whose food need outruns production (issue #35).
func TestAdvisorsWarnGrowthOutrunsFood(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.People = 100                              // tiny now → current consumption is trivial
	p.Support = 100                             // high support → large population capacity
	p.Land = 200                                // capacity scales with land + support
	p.Regions = game.RegionMix{Agricultural: 1} // barely any food production
	p.Troopers, p.Jets, p.Tanks = 0, 0, 0       // isolate the population food need
	p.Food = 10_000_000                         // stores comfortably cover this turn

	f := &fakeSession{}
	renderAdvisor(f, w, advisorCivilian)
	out := f.out.String()
	if !strings.Contains(out, "still growing") {
		t.Errorf("expected the grow-past-food warning; output:\n%s", out)
	}
}

// TestAdvisorsWarnLowMorale checks the Military advisor surfaces the
// desertion-risk nudge when Morale drops below the threshold.
func TestAdvisorsWarnLowMorale(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Morale = 20

	f := &fakeSession{}
	renderAdvisor(f, w, advisorMilitary)
	out := f.out.String()
	if !strings.Contains(out, "Desertion is a real risk") {
		t.Errorf("expected low-morale advice; output:\n%s", out)
	}
}

// TestAdvisorsMenuSelectsAdvisor drives BRE's four-advisor submenu: it lists the
// four advisors, and picking one shows that advisor's greeting and its domain's
// advice before returning to the list.
func TestAdvisorsMenuSelectsAdvisor(t *testing.T) {
	w := newWorld()
	w.Player().Morale = 20 // triggers the desertion nudge under the Military advisor

	f := &fakeSession{keys: []rune("3 0")} // 3=Military, space clears its pause, 0=quit
	visitAdvisors(f, w)
	out := f.out.String()
	for _, want := range []string{
		"Civilian", "Economic", "Military", "Technology", // the submenu list
		"I am your Military advisor", // first-person greeting
		"Desertion is a real risk",   // the Military advisor's advice
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in advisor output; got:\n%s", want, out)
		}
	}
}

// TestAdvisorsHealthyEmpire checks that a healthy empire's advisors report their
// figures and raise none of the warning lines.
func TestAdvisorsHealthyEmpire(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.HQ = 100
	p.Land = 100
	p.Regions.Technology = 20
	p.TechLevel = 200 // 20% bonus, ramped → "Technology stands at 20%"
	p.Agents = 5
	p.Support = 100
	p.Tax = 10
	p.Morale = 100
	p.Gold = 100_000
	p.Food = 1_000_000
	p.Jets = 0
	p.Debt = 0

	f := &fakeSession{}
	for _, d := range []advisorDomain{advisorCivilian, advisorEconomic, advisorMilitary, advisorTechnology} {
		renderAdvisor(f, w, d)
	}
	out := f.out.String()
	// The informational report is always present.
	for _, want := range []string{
		"Our people number", "We earn about", "Our forces:", "Technology stands at",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected the report line %q; output:\n%s", want, out)
		}
	}
	// No warning line fires for a healthy empire.
	for _, warn := range []string{
		"no HeadQuarters", "more jets than our carriers", "Desertion",
		"will not last", "grow restless", "risk riots", "treasury is empty",
		"no covert agents",
	} {
		if strings.Contains(out, warn) {
			t.Errorf("did not expect the warning %q for a healthy empire; output:\n%s", warn, out)
		}
	}
}

// TestEndProtectionEarly: confirming ends new-realm protection (one-way);
// declining leaves it; a realm not under protection is a no-op.
func TestEndProtectionEarly(t *testing.T) {
	w := newWorld()

	w.Player().Protection = 0
	endProtection(&fakeSession{}, w)
	if got := w.Player().Protection; got != 0 {
		t.Errorf("no protection: should stay 0, got %d", got)
	}

	w.Player().Protection = 5
	endProtection(&fakeSession{keys: []rune{'n'}}, w) // decline
	if got := w.Player().Protection; got != 5 {
		t.Errorf("declining should keep protection, got %d", got)
	}

	// Confirming drops protection to its last turn (still protected now); the
	// turn-end tick in PlayTurn clears it.
	endProtection(&fakeSession{keys: []rune{'y'}}, w) // confirm
	if got := w.Player().Protection; got != 1 {
		t.Errorf("confirming should leave one turn of protection, got %d", got)
	}
}

// TestAttackGatedByProtection: a protected realm is refused (Stay + message) on
// a regular attack AND a pirate raid — the gate covers pirates too.
func TestAttackGatedByProtection(t *testing.T) {
	w := newWorld() // a new realm starts under protection
	if w.Player().Protection == 0 {
		t.Fatal("expected a protected new realm")
	}
	f := &fakeSession{}
	if r := regularAttack(f, w); r != Stay {
		t.Errorf("regular attack under protection: want Stay, got %v", r)
	}
	if !strings.Contains(f.out.String(), "New Realm Protection") {
		t.Errorf("expected the protection message; got:\n%s", f.out.String())
	}
	f2 := &fakeSession{}
	if r := attackPirates(f2, w); r != Stay {
		t.Errorf("pirate raid under protection: want Stay, got %v", r)
	}
	if !strings.Contains(f2.out.String(), "New Realm Protection") {
		t.Errorf("pirate raid should be gated by protection; got:\n%s", f2.out.String())
	}
}

// TestRegularAttackAdvancesTurn: a completed War-menu attack returns Back so the
// turn pipeline moves forward (one attack per turn).
func TestRegularAttackAdvancesTurn(t *testing.T) {
	w := newWorld()
	w.Player().Protection = 0
	v := w.AddHuman("victim", "Victimville")
	v.Protection = 0

	f := &fakeSession{keys: []rune("1\r ")} // target 1, then a key to clear the report pause
	if r := regularAttack(f, w); r != Back {
		t.Errorf("completed attack should advance the turn (Back), got %v", r)
	}
}
