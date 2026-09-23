package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
)

// The Technology advisor closes with BRE's set-apart NOTE, not another body
// line: a bright-cyan "NOTE:" and a cyan body hanging under it
// (docs/dev/bre-screens.md). IB folded it into ordinary white prose until
// 2026-08-15, which lost both the label and the color.
func TestTechnologyAdvisorClosesWithBREsNoteBlock(t *testing.T) {
	f := &fakeSession{}
	w := newWorld()
	w.With(func() {
		p := w.Player()
		p.Regions.Technology = 40
		for i := range p.TechSlots {
			p.TechSlots[i] = 4000
		}
	})
	renderAdvisor(f, w, advisorTechnology)
	out := f.out.String()

	if !strings.Contains(out, ansi.FgBrightCyan+"NOTE:") {
		t.Errorf("the NOTE label is not bright cyan:\n%s", out)
	}
	if !strings.Contains(out, ansi.FgCyan+"Technology levels are relative") {
		t.Errorf("the NOTE body is not cyan:\n%s", out)
	}
	// It is the LAST thing the advisor says, as it is in the capture: only IB's
	// closing rule follows it.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 3 || !strings.Contains(lines[len(lines)-2], ansi.FgCyan) {
		t.Errorf("the NOTE does not close the report:\n%s", out)
	}
	// A percentage line still reads BRE's way: white aspect name, yellow figure.
	if !strings.Contains(out, ansi.FgBrightWhite+"military forces") {
		t.Errorf("the aspect name lost its bright-white emphasis:\n%s", out)
	}
}

// The Military advisor quotes the Mountain industry boost, which is a share of
// the realm rather than a count — the dilution is the part a player cannot see
// anywhere else.
func TestMilitaryAdvisorReportsTheMountainIndustryBoost(t *testing.T) {
	f := &fakeSession{}
	w := newWorld()
	w.With(func() {
		p := w.Player()
		p.Regions = game.RegionMix{Mountain: 10, Industrial: 90} // 10% mountains
	})
	renderAdvisor(f, w, advisorMilitary)
	if out := f.out.String(); !strings.Contains(out, ansi.FgBrightWhite+"130") {
		t.Errorf("no mountain boost figure:\n%s", out)
	}

	f = &fakeSession{}
	w.With(func() {
		w.Player().Regions = game.RegionMix{Mountain: 50, Industrial: 50}
	})
	renderAdvisor(f, w, advisorMilitary)
	if out := f.out.String(); !strings.Contains(out, "at their limit, "+ansi.FgBrightWhite+"150") {
		t.Errorf("the capped boost is not reported as capped:\n%s", out)
	}
}

// The Military advisor warns of a carrier shortage only once the carriers cannot
// lift 80% of the jets — BRE's threshold (#239) — and not merely when a few jets
// would be left on deck.
func TestMilitaryAdvisorCarrierWarningThreshold(t *testing.T) {
	for _, tc := range []struct {
		carriers int
		warn     bool
	}{{8, false}, {7, true}} {
		w := newWorld()
		w.With(func() { p := w.Player(); p.Jets, p.Carriers = 1000, tc.carriers })
		f := &fakeSession{}
		renderAdvisor(f, w, advisorMilitary)
		got := strings.Contains(stripANSI(f.out.String()), "more jets than our carriers")
		if got != tc.warn {
			t.Errorf("1000 jets, %d carriers: carrier warning shown = %v, want %v", tc.carriers, got, tc.warn)
		}
	}
}

// Desertion starts below 40 morale, so the advisor warns of it there and not
// above.
func TestMilitaryAdvisorDesertionWarningThreshold(t *testing.T) {
	for _, tc := range []struct {
		morale int
		warn   bool
	}{{40, false}, {39, true}} {
		w := newWorld()
		w.With(func() { w.Player().Morale = tc.morale })
		f := &fakeSession{}
		renderAdvisor(f, w, advisorMilitary)
		got := strings.Contains(stripANSI(f.out.String()), "may desert")
		if got != tc.warn {
			t.Errorf("morale %d: desertion warning shown = %v, want %v", tc.morale, got, tc.warn)
		}
	}
}

// The force table shortens its counts to k and m and counts forces away on a
// group attack, both as the original's report does (#239).
func TestMilitaryAdvisorForceTableShortensAndCountsAwayForces(t *testing.T) {
	w := newWorld()
	w.With(func() {
		p := w.Player()
		p.Troopers, p.Tanks = 24310, 9000
		w.GroupAttacks = append(w.GroupAttacks, game.GroupAttack{
			Contributors: []game.Contribution{{Owner: p.Owner, AttackForce: game.AttackForce{Tanks: 1500}}},
		})
	})
	f := &fakeSession{}
	renderAdvisor(f, w, advisorMilitary)
	out := stripANSI(f.out.String())
	for _, want := range []string{"[24k Troopers]", "[10k Tanks]", "1,500 units are away on attacks"} {
		if !strings.Contains(out, want) {
			t.Errorf("force table missing %q:\n%s", want, out)
		}
	}
}
