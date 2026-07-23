package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
)

// TestShowInstructions pages through the whole linear manual: the overview leads
// and the assembled help topics follow (BRE's Instructions item).
func TestShowInstructions(t *testing.T) {
	w := newWorld()
	f := &fakeSession{keys: []rune(strings.Repeat(" ", 200))} // continue past every page
	showInstructions(f, w)
	out := f.out.String()
	if !strings.Contains(out, "How to Play") {
		t.Errorf("overview should lead the instructions; got:\n%.200s", out)
	}
	if !strings.Contains(out, "Troopers") {
		t.Error("instructions should stitch in help topics like Troopers")
	}
}

// TestShowInstructionsQuitsEarly: Q on the first page stops before later topics.
func TestShowInstructionsQuitsEarly(t *testing.T) {
	w := newWorld()
	f := &fakeSession{keys: []rune("Q")}
	showInstructions(f, w)
	out := f.out.String()
	if !strings.Contains(out, "How to Play") {
		t.Error("the overview should print before an early quit")
	}
	if strings.Contains(out, "Troopers") {
		t.Error("Q on the first page should stop before the topic sections")
	}
}

// hiNums highlights numeric runs (BRE makes figures pop) and leaves words alone;
// a grouping comma between digits stays inside one highlighted run.
func TestHiNumsHighlightsFigures(t *testing.T) {
	out := hiNums("You broke the Sharks and recovered 848 troopers, 9,999 gold.")
	if !strings.Contains(out, ansi.FgBrightYellow+"848"+ansi.Reset) {
		t.Errorf("848 should be a highlighted run; got %q", out)
	}
	if !strings.Contains(out, ansi.FgBrightYellow+"9,999"+ansi.Reset) {
		t.Errorf("grouped 9,999 should be one highlighted run; got %q", out)
	}
	if strings.Contains(out, ansi.FgBrightYellow+"Sharks") {
		t.Errorf("words must not be highlighted; got %q", out)
	}
}

// allocateCaptured lets the winner split the captured regions across types
// (#58): assign 6 as River then 4 as Mountain; the loop ends when all 10 are
// placed and the empire gains exactly that split.
func TestAllocateCapturedSplitsByType(t *testing.T) {
	w := newWorld()
	p := w.Player()
	beforeLand, beforeRiver, beforeMtn := p.Land, p.Regions.River, p.Regions.Mountain
	f := &fakeSession{keys: []rune("R6\rM4\r")} // River 6, Mountain 4 -> 10 placed
	allocateCaptured(f, w, 10)
	p = w.Player()
	if p.Regions.River != beforeRiver+6 {
		t.Errorf("River = %d, want %d", p.Regions.River, beforeRiver+6)
	}
	if p.Regions.Mountain != beforeMtn+4 {
		t.Errorf("Mountain = %d, want %d", p.Regions.Mountain, beforeMtn+4)
	}
	if p.Land != beforeLand+10 {
		t.Errorf("Land = %d, want %d", p.Land, beforeLand+10)
	}
}

// Quitting the allocator early (0) must not lose captured land: the unassigned
// remainder defaults to Coastal so Land still rises by the full captured count.
func TestAllocateCapturedRemainderToCoastal(t *testing.T) {
	w := newWorld()
	p := w.Player()
	beforeLand, beforeCoastal, beforeRiver := p.Land, p.Regions.Coastal, p.Regions.River
	f := &fakeSession{keys: []rune("R3\r0")} // River 3, then quit: 7 remain
	allocateCaptured(f, w, 10)
	p = w.Player()
	if p.Regions.River != beforeRiver+3 {
		t.Errorf("River = %d, want %d", p.Regions.River, beforeRiver+3)
	}
	if p.Regions.Coastal != beforeCoastal+7 {
		t.Errorf("Coastal (remainder) = %d, want %d", p.Regions.Coastal, beforeCoastal+7)
	}
	if p.Land != beforeLand+10 {
		t.Errorf("Land = %d, want %d (no captured land lost)", p.Land, beforeLand+10)
	}
}

// The macro-slot list is framed top and bottom by BRE's inset rule — a
// double-line (═) segment set into a single-line rule (from a live capture).
func TestWriteMacrosFramesListWithInsetRule(t *testing.T) {
	w := newWorld()
	f := &fakeSession{keys: []rune{'0'}} // '0' isn't a macro key: draw the list, then exit
	writeMacros(f, w)
	out := f.out.String()
	if n := strings.Count(out, insetRule); n != 2 {
		t.Errorf("macro list should be framed by the inset rule top and bottom (want 2), got %d:\n%s", n, out)
	}
	// The list must sit between the two rules, not outside them.
	first := strings.Index(out, insetRule)
	last := strings.LastIndex(out, insetRule)
	if ctrl := strings.Index(out, "Ctrl-D:"); !(first < ctrl && ctrl < last) {
		t.Errorf("Ctrl-D slot should fall between the two rules (first=%d ctrl=%d last=%d)", first, ctrl, last)
	}
}

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
// food-shortfall nudge when the realm ends the turn with negative food:
// production trails consumption and stores can't bridge the gap.
func TestAdvisorsWarnFoodDeficit(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Regions = game.RegionMix{Agricultural: 1} // barely any production
	p.Troopers = 1000                           // a hungry army outruns it
	p.Food = 0

	f := &fakeSession{}
	renderAdvisor(f, w, advisorCivilian)
	out := f.out.String()
	if !strings.Contains(out, "food will not last the turn") {
		t.Errorf("expected food-deficit advice; output:\n%s", out)
	}
}

// TestAdvisorsNoFalseFoodWarning guards against issue where the Civilian
// advisor cried "food will not last" whenever stored food was below one turn's
// consumption, ignoring production. A realm that grows more than it eats never
// starves however empty its stores are.
func TestAdvisorsNoFalseFoodWarning(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Food = 0          // empty pre-growth stores, but a fresh realm runs a food surplus
	w.World.GrowFood(p) // this turn's growth is credited at turn start, as a real turn does

	f := &fakeSession{}
	renderAdvisor(f, w, advisorCivilian)
	out := f.out.String()
	if strings.Contains(out, "will not last") {
		t.Errorf("surplus realm must not warn of starvation; output:\n%s", out)
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
		"Krane, your war advisor",  // the named military advisor's greeting
		"Desertion is a real risk", // the Military advisor's advice
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

	f := &fakeSession{keys: []rune("A\r\r\r\r ")} // target A, full-force defaults, then a key to clear the report pause
	if r := regularAttack(f, w); r != Back {
		t.Errorf("completed attack should advance the turn (Back), got %v", r)
	}
}
