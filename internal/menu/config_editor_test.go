package menu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigEditorEditsAndSaves(t *testing.T) {
	dir := t.TempDir()
	w := newWorld()
	w.Config.DataDir = dir
	w.Config.TurnsPerDay = 10

	// Edit item 1 (turns per day) to 20, then S to save and exit.
	f := &fakeSession{keys: []rune("1\r20\rs\r ")}
	ConfigEditor(f, w.World)

	if w.Config.TurnsPerDay != 20 {
		t.Errorf("expected turns per day 20, got %d", w.Config.TurnsPerDay)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Errorf("config.json should have been saved: %v", err)
	}
}

func TestConfigEditorTurnsFloorAtOne(t *testing.T) {
	dir := t.TempDir()
	w := newWorld()
	w.Config.DataDir = dir

	// Try to set turns per day to 0; it must floor at 1 (a day needs a turn).
	// Value 0, then S to save and exit.
	f := &fakeSession{keys: []rune("1\r0\rs\r ")}
	ConfigEditor(f, w.World)

	if w.Config.TurnsPerDay != 1 {
		t.Errorf("turns per day should floor at 1, got %d", w.Config.TurnsPerDay)
	}
}

// Every field survives the split into pages, and each page fits an 80x24 screen
// — the whole point of paging it (issue #100).
func TestConfigPagesCoverEveryFieldAndFit(t *testing.T) {
	pages := configPages(true)
	seen := map[int]string{}
	for _, g := range pages {
		// Title rule, page heading, closing rule, legend, and the prompt take five
		// rows on top of the fields.
		const chrome = 5
		if rows := len(g.fields) + chrome; rows > 24 {
			t.Errorf("page %q needs %d rows, more than an 80x24 screen", g.title, rows)
		}
		for _, f := range g.fields {
			if where, dup := seen[f.n]; dup {
				t.Errorf("field %d appears on both %q and %q", f.n, where, g.title)
			}
			seen[f.n] = g.title
			if f.value == nil || f.edit == nil {
				t.Errorf("field %d (%s) is missing a value or edit function", f.n, f.label)
			}
		}
	}
	// The identifiers ran 1..38 before the pages existed and must still, so no
	// sysop's habit or note breaks. Later fields are appended above that. 22 was
	// the Inter-BBS toggle, replaced by the -ibbs-reset command; 45 is Pirates,
	// IB's own switch, numbered past the last of the original's own fields.
	const fields = 44
	for n := 1; n <= 45; n++ {
		if n == 22 {
			continue
		}
		if _, ok := seen[n]; !ok {
			t.Errorf("field %d is on no page", n)
		}
	}
	if len(seen) != fields {
		t.Errorf("got %d fields, want %d", len(seen), fields)
	}
}

// A field number reaches its field from any page: the numbers identify fields,
// not rows on the current screen.
func TestConfigEditorEditsAcrossPages(t *testing.T) {
	dir := t.TempDir()
	w := newWorld()
	w.Config.DataDir = dir
	w.Config.MaxTaxRate = 30

	// N to the Economy page, N again past it, then edit 9 (Max Tax Rate, back on
	// Economy) from where we stand, and save.
	f := &fakeSession{keys: []rune("n\rn\r9\r25\rs\r ")}
	ConfigEditor(f, w.World)

	if w.Config.MaxTaxRate != 25 {
		t.Errorf("expected max tax rate 25, got %d", w.Config.MaxTaxRate)
	}
	out := f.out.String()
	if !strings.Contains(out, "page 3 of 4") {
		t.Errorf("two N presses should reach page 3, got:\n%s", out)
	}
}

// P wraps backwards off the first page, so a sysop who overshoots gets back.
func TestConfigEditorPreviousPageWraps(t *testing.T) {
	w := newWorld()
	w.Config.DataDir = t.TempDir()
	f := &fakeSession{keys: []rune("p\rq\r ")}
	ConfigEditor(f, w.World)
	if out := f.out.String(); !strings.Contains(out, "page 4 of 4") {
		t.Errorf("P on the first page should wrap to the last, got:\n%s", out)
	}
}

// The packet directories are set in the editor, not by hand in config.json, so
// both must be reachable and a cleared answer must keep the current one.
func TestConfigEditorSetsPacketDirs(t *testing.T) {
	w := newWorld()
	w.Config.DataDir = t.TempDir()
	w.Config.IBBS = true // the packet directories are only asked of a league board
	w.Config.InboundDir = "./data/inbound"
	w.Config.OutboundDir = "./data/outbound"

	f := &fakeSession{keys: []rune("39\r/srv/in\r40\r/srv/out\rs\r ")}
	ConfigEditor(f, w.World)

	if w.Config.InboundDir != "/srv/in" {
		t.Errorf("InboundDir = %q, want /srv/in", w.Config.InboundDir)
	}
	if w.Config.OutboundDir != "/srv/out" {
		t.Errorf("OutboundDir = %q, want /srv/out", w.Config.OutboundDir)
	}

	f = &fakeSession{keys: []rune("39\r\rs\r ")}
	ConfigEditor(f, w.World)
	if w.Config.InboundDir != "/srv/in" {
		t.Errorf("an empty answer changed InboundDir to %q", w.Config.InboundDir)
	}
}

// A stand-alone reset never asks the league settings; -ibbs-reset does. The two
// sets must be the same ones Game Setup shows and hides.
func TestConfigPagesHideLeagueFieldsOffLeague(t *testing.T) {
	has := func(ibbs bool, n int) bool { return findConfigField(configPages(ibbs), n) != nil }
	for n := range ibbsOnlyFields {
		if has(false, n) {
			t.Errorf("field %d is offered on a stand-alone board", n)
		}
		if !has(true, n) {
			t.Errorf("field %d is missing from an -ibbs-reset", n)
		}
	}
	// A setting that is not league-only stays on both.
	if !has(false, 1) || !has(true, 1) {
		t.Error("Turns per day should be offered either way")
	}
}

// Game Setup is the third place that knows which rules are league-only, and it
// is what a player reads. A rule the editor only asks a league board must not
// be reported to a stand-alone one, where the figure is whatever the default
// happened to be.
func TestGameSetupHidesLeagueRulesOffLeague(t *testing.T) {
	show := func(ibbs bool) string {
		w := newWorld()
		w.Config.IBBS = ibbs
		f := &fakeSession{keys: []rune(" ")}
		gameSetup(f, w)
		return f.out.String()
	}
	off, on := show(false), show(true)

	for _, label := range []string{
		"Group attacks per day", "Terrorist ops per day", "Bombing ops per day",
		"Terrorism costs", "Lost forces return after", "Clingy Annihilator", "This planet",
		"Local attacks", "Local attack scoring", "Dupe checking",
	} {
		if strings.Contains(off, label) {
			t.Errorf("Game Setup shows %q on a stand-alone board", label)
		}
		if !strings.Contains(on, label) {
			t.Errorf("Game Setup omits %q on a league board", label)
		}
	}
}
