package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// Gold is the last row and starts at nothing, so an existing empire produces
// exactly what it did before the row existed.
func TestGoldIsTheLastIndustryRowAndStartsAtZero(t *testing.T) {
	rows := prodRows()
	if got := rows[len(rows)-1].name; got != "Gold" {
		t.Errorf("last row = %q, want Gold", got)
	}
	if game.DefaultProdGoldPct != 0 {
		t.Errorf("default gold share = %d%%, want 0", game.DefaultProdGoldPct)
	}
	var e game.Empire
	if p := rows[len(rows)-1].field(&e); p != &e.ProdGold {
		t.Error("the Gold row is not wired to Empire.ProdGold")
	}
}

// Gold is a production row, not a unit, so it must not appear as a
// specialization — the efficiency modifier has no units to apply to.
func TestGoldCannotBeSpecialized(t *testing.T) {
	w := newWorld()
	f := &fakeSession{keys: []rune("7\r")} // the position Gold occupies on Set Industries
	specializeIndustry(f, w)

	out := stripANSI(f.out.String())
	if strings.Contains(out, "7) Gold") {
		t.Error("Gold is offered as a specialization")
	}
	if got := w.Player().Specialized; got != "" {
		t.Errorf("Specialized = %q, want the choice refused", got)
	}
}

// The specialized row carries the marker; nothing else does, and the sentence
// that used to explain it is gone.
func TestSpecializedRowIsMarkedWithAsterisks(t *testing.T) {
	w := newWorld()
	if err := w.mutatePlayer(func(p *game.Empire) error {
		p.Specialized = "Tanks"
		p.Regions.Industrial = 100
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f := &fakeSession{keys: []rune("n\r")} // decline "Change Production?"
	setIndustries(f, w)
	out := f.out.String()

	for _, line := range strings.Split(out, "\n") {
		plain := stripANSI(line)
		marked := strings.Contains(plain, "* * *")
		if strings.HasPrefix(plain, "Tanks") != marked {
			t.Errorf("marker on the wrong row: %q", plain)
		}
	}
	if strings.Contains(stripANSI(out), "Specialized in") {
		t.Error("the explanatory sentence should be gone")
	}
}

// Specialize Industry is drawn as a menu, not a bare list: the original gives it
// a red-accent [Specialization] frame with a Quit item, and IB's own convention
// is a numbered list ending in "0) Quit" read by ChoiceQuit — never a one-off
// "0 to cancel" prompt. Asserts the frame AND the state effect, so a flow change
// upstream cannot leave this green while never reaching the screen.
func TestSpecializeIsAStandardMenu(t *testing.T) {
	w := newWorld()
	f := &fakeSession{keys: []rune("5 ")} // 5 = Tanks, then dismiss the confirmation
	specializeIndustry(f, w)

	out := stripANSI(f.out.String())
	for _, want := range []string{"[Specialization]", "5) Tanks", "0) Quit"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen is missing %q:\n%s", want, out)
		}
	}
	if got := w.Player().Specialized; got != "Tanks" {
		t.Errorf("Specialized = %q, want Tanks — the choice never took effect", got)
	}
}

// Declining says so rather than returning in silence, as the original does.
func TestSpecializeDeclineIsReported(t *testing.T) {
	w := newWorld()
	f := &fakeSession{keys: []rune("0 ")}
	specializeIndustry(f, w)

	if got := w.Player().Specialized; got != "" {
		t.Errorf("Specialized = %q, want nothing set", got)
	}
	if out := stripANSI(f.out.String()); !strings.Contains(out, "left unspecialized") {
		t.Errorf("declining should say so:\n%s", out)
	}
}

// The Paused bar has to close its own line. A menu opens with its title rule and
// no newline of its own, so a bar left un-terminated ran straight into it:
// "─»>Paused<«──────────[System]──────────" on one row.
func TestPausedBarDoesNotRunIntoTheNextScreen(t *testing.T) {
	w := newWorld()
	w.With(func() { w.Player().Regions.Industrial = 10 })
	// Specialize in Tanks (5), dismiss the Paused bar, then draw a menu.
	f := &fakeSession{keys: []rune("5y \r")}
	specializeIndustry(f, w)
	draw(f, w, BuildMenus().System)

	out := f.out.String()
	if !strings.Contains(out, "Paused") {
		t.Fatalf("the pause never happened, so the test proves nothing:\n%s", out)
	}
	for _, line := range strings.Split(stripANSI(out), "\n") {
		if strings.Contains(line, "Paused") && strings.Contains(line, "[System]") {
			t.Errorf("the Paused bar and the menu rule share a line:\n%q", line)
		}
	}
}

// The Specialization box is 14 columns, closed by a 14-column rule under a bare
// 16-column [Specialization] title that overhangs it — the one box in BRE whose
// title is wider than its content (docs/dev/bre-screens.md). Golden literals off
// the capture: 14 dashes and an unfilled title line, not a rebuild from
// specializationRuleWidth.
func TestSpecializationBoxIsFourteenColumnsWithAnOverhangingTitle(t *testing.T) {
	w := newWorld()
	f := &fakeSession{keys: []rune("0\r \r")} // Quit the list, then the ok() pause
	specializeIndustry(f, w)
	out := f.out.String()

	// Reached the screen, and Quit took effect — a script that ran dry would end
	// the session cleanly and leave this green having drawn nothing.
	if !strings.Contains(stripANSI(out), "1) Troopers") {
		t.Fatalf("script never reached the Specialization list:\n%s", stripANSI(out))
	}
	if got := w.Player().Specialized; got != "" {
		t.Errorf("Specialized = %q, want Quit to leave it unset", got)
	}
	// No fill either side of the title, and the brackets and title keep the
	// colours a filled title line uses: dim accent, bright brackets, bright white.
	const title = "\x1b[31m\x1b[91m[\x1b[97mSpecialization\x1b[91m]\x1b[31m\x1b[0m"
	if !strings.Contains(out, title) {
		t.Errorf("the title line should overhang the box unfilled, got:\n%q", out)
	}
	const rule = "\x1b[31m──────────────\x1b[0m"
	if !strings.Contains(out, rule) {
		t.Errorf("expected the 14-column dim-red closing rule, got:\n%q", out)
	}
}

// Once the walk has allocated 100%, every later type can only be 0 — asking for
// it is a dead question, so the walk stops and says so.
func TestSetIndustriesStopsAskingOnceFullyAllocated(t *testing.T) {
	w := newWorld()
	if err := w.mutatePlayer(func(p *game.Empire) error {
		p.Regions.Industrial = 100
		p.ProdJets = 25
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Accept "Change Production?", then spend the whole budget on the first row.
	f := &fakeSession{keys: []rune("y100\r")}
	setIndustries(f, w)
	out := stripANSI(f.out.String())

	// The first row was reached; the second never asked.
	if !strings.Contains(out, "Troopers") {
		t.Fatalf("the production walk never started:\n%s", out)
	}
	if strings.Count(out, "Jets") > 1 { // once in the report table, never as a prompt
		t.Errorf("Jets was still prompted for with 0%% left:\n%s", out)
	}
	if !strings.Contains(out, "remaining types are set to 0%") {
		t.Errorf("no notice that the rest were zeroed:\n%s", out)
	}
	if p := w.Player(); p.ProdTroopers != 100 || p.ProdJets != 0 {
		t.Errorf("ProdTroopers = %d, ProdJets = %d; want 100 and 0", p.ProdTroopers, p.ProdJets)
	}
}
