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
// 2026-08-15, which lost both the label and the colour.
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
	// It is the LAST thing the advisor says, as it is in the capture.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 || !strings.Contains(lines[len(lines)-1], ansi.FgCyan) {
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
