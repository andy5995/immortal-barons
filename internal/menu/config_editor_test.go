package menu

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/mod"
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
	// the Inter-BBS toggle, replaced by the -ibbs-reset command. 45 (Pirates), 46
	// (allied market trading) and 47 (required version) are IB's own, numbered
	// past the last of the original's own fields. 48 was a league name, removed
	// once nothing displayed it; its number is retired rather than reused, so an
	// old note naming 48 cannot land on a different setting. 21 was the AI-baron
	// count, retired with the barons themselves in v0.2.0, and its number is held
	// back the same way. 49 (local attacks per day) and 50 (the S3-Sabre Constant
	// Dial), 51 (Full Bank Start) and 52 (Starting Regions) are IB's own. 53
	// (Mods) is on a stand-alone board's Mods page, which configPages(true)
	// does not build; TestConfigEditorModsPage covers it.
	const fields = 49
	for n := 1; n <= 52; n++ {
		if n == 21 || n == 22 || n == 48 {
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
	if !strings.Contains(out, "page 3 of 5") {
		t.Errorf("two N presses should reach page 3, got:\n%s", out)
	}
}

// P wraps backwards off the first page, so a sysop who overshoots gets back.
func TestConfigEditorPreviousPageWraps(t *testing.T) {
	w := newWorld()
	w.Config.DataDir = t.TempDir()
	f := &fakeSession{keys: []rune("p\rq\r ")}
	ConfigEditor(f, w.World)
	if out := f.out.String(); !strings.Contains(out, "page 5 of 5") {
		t.Errorf("P on the first page should wrap to the last, got:\n%s", out)
	}
}

// The packet directories are set in the editor, not by hand in config.json, so
// both must be reachable and a cleared answer must keep the current one.
func TestConfigEditorLeavesTheBoardsOwnSettingsToBBSCfg(t *testing.T) {
	w := newWorld()
	w.Config.DataDir = t.TempDir()
	w.Config.IBBS = true // the packet directories are only shown to a league board
	w.Config.InboundDir = "ftn/in"
	w.Config.BoardID = "Alpha BBS"

	// 39 is GameInbound. It answers with where the setting lives instead of a
	// prompt: this editor saves config.json, and the four settings that name the
	// board are read from bbs.cfg, which the game never writes (#152).
	f := &fakeSession{keys: []rune("39\r q\r ")}
	ConfigEditor(f, w.World)

	if w.Config.InboundDir != "ftn/in" || w.Config.BoardID != "Alpha BBS" {
		t.Errorf("the editor changed a bbs.cfg setting: %q / %q", w.Config.InboundDir, w.Config.BoardID)
	}
	if !strings.Contains(f.out.String(), "bbs.cfg") {
		t.Error("the editor did not say where the setting is edited")
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
		"Terrorism costs", "Lost forces return", "Gooie Kablooie", "This planet",
		"League number", "League Coordinator",
		"Local attacks", "Local attack scoring", "Attack costs", "S3-Sabre",
	} {
		if strings.Contains(off, label) {
			t.Errorf("Game Setup shows %q on a stand-alone board", label)
		}
		if !strings.Contains(on, label) {
			t.Errorf("Game Setup omits %q on a league board", label)
		}
	}
}

// Both editors hold the two bank rates to BRE's own ranges — "(50; 200)" and
// "(35; 100)" on its config-help screens. They used to accept 0, which is a bank
// that pays nothing, and the two editors have to agree or a sysop's setting
// depends on which one they opened.
func TestBothEditorsHoldTheBankRatesToBRERanges(t *testing.T) {
	dir := t.TempDir()
	w := newWorld()
	w.Config.DataDir = dir

	// Field 6 (Interest Rate) then 7 (Standard Investment Rate), each set to 1,
	// then save and exit.
	f := &fakeSession{keys: []rune("6\r1\r7\r1\rs\r ")}
	ConfigEditor(f, w.World)

	if w.Config.InterestRate != game.MinBankInterest {
		t.Errorf("Interest Rate = %d, want the %d floor", w.Config.InterestRate, game.MinBankInterest)
	}
	if w.Config.StdInvestRate != game.MinStdInvestRate {
		t.Errorf("Standard Investment Rate = %d, want the %d floor", w.Config.StdInvestRate, game.MinStdInvestRate)
	}

	// The tview editor clamps through clampRange, which is what its own field
	// handler and save binder both call — so the two editors cannot drift apart.
	if got := clampRange(1, game.MinBankInterest, game.MaxBankInterest); got != game.MinBankInterest {
		t.Errorf("tview clamp gave %d, want the %d floor", got, game.MinBankInterest)
	}
	if got := clampRange(1, game.MinStdInvestRate, game.MaxStdInvestRate); got != game.MinStdInvestRate {
		t.Errorf("tview clamp gave %d, want the %d floor", got, game.MinStdInvestRate)
	}
	if got := clampRange(9999, game.MinBankInterest, game.MaxBankInterest); got != game.MaxBankInterest {
		t.Errorf("tview clamp gave %d, want the %d ceiling", got, game.MaxBankInterest)
	}
}

// TestKnobsCycleThroughEveryValue pins the order each sysop knob walks, and
// that every value is reachable and the walk returns to where it started. The
// four cyclers were four separate switches whose default arm carried the wrap;
// nextIn has to reproduce each one exactly, including where an unset value
// lands.
func TestKnobsCycleThroughEveryValue(t *testing.T) {
	t.Run("cost", func(t *testing.T) {
		want := []game.Level{game.Medium, game.Low, game.None, game.High}
		got := game.High
		for i, w := range want {
			if got = cycleCost(got); got != w {
				t.Fatalf("step %d: got %v, want %v", i+1, got, w)
			}
		}
	})
	t.Run("damage and reward skip None", func(t *testing.T) {
		want := []game.Level{game.Medium, game.Low, game.High}
		got := game.High
		for i, w := range want {
			if got = cycleHML(got); got != w {
				t.Fatalf("step %d: got %v, want %v", i+1, got, w)
			}
		}
		// None is not on this ladder, so it steps onto the first value.
		if got := cycleHML(game.None); got != game.High {
			t.Errorf("None stepped to %v, want High", got)
		}
	})
	t.Run("buy military", func(t *testing.T) {
		want := []game.BuyMode{game.BuyNo, game.BuyLimited, game.BuyYes}
		got := game.BuyYes
		for i, w := range want {
			if got = cycleBuy(got); got != w {
				t.Fatalf("step %d: got %v, want %v", i+1, got, w)
			}
		}
	})
	t.Run("sabre", func(t *testing.T) {
		want := []game.SabreMode{
			game.SabreNone, game.SabreRandom,
			game.SabreConstant, game.SabreUserSelect,
		}
		got := game.SabreUserSelect
		for i, w := range want {
			if got = cycleSabre(got); got != w {
				t.Fatalf("step %d: got %v, want %v", i+1, got, w)
			}
		}
	})
}

// writeTestMod creates <dataDir>/mods/<name> with a manifest and values.toml.
func writeTestMod(t *testing.T, dataDir, name, values string) {
	t.Helper()
	dir := filepath.Join(dataDir, mod.Dir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		mod.ManifestFile: "name = \"" + name + "\"\nversion = \"1.0\"\nmod_api = 1\nlicense = \"CC0-1.0\"\n",
		mod.ValuesFile:   values,
	}
	for f, body := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// writeTestOptionMod adds two options to a mod: a bool that switches a section
// and a number that sets a figure. Both editors must offer them.
func writeTestOptionMod(t *testing.T, dataDir, name string) {
	t.Helper()
	dir := filepath.Join(dataDir, mod.Dir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "name = \"" + name + "\"\nversion = \"1.0\"\nmod_api = 1\nlicense = \"CC0-1.0\"\n" +
		"[options.fast_start]\ntype = \"bool\"\nlabel = \"Fast start\"\ndescription = \"Cheap jets\"\ndefault = 0\n" +
		"[options.tank_cost]\ntype = \"number\"\nlabel = \"Tank cost\"\ndescription = \"What one tank costs\"\ndefault = 500\nmin = 1\nmax = 1000\n"
	values := "[units.tank]\nbuild_cost = { option = \"tank_cost\" }\n[when.fast_start.units.jet]\nbuild_cost = 500\n"
	for f, body := range map[string]string{mod.ManifestFile: manifest, mod.ValuesFile: values} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// The line editor's Mods page offers an options row, and a number choice that
// is outside the mod's range is refused with the range named.
func TestConfigEditorModOptions(t *testing.T) {
	dir := t.TempDir()
	writeTestOptionMod(t, dir, "opt")
	w := newWorld()
	w.Config.DataDir = dir
	w.Config.Mods = []string{"opt"}

	f := &fakeSession{keys: []rune("53\r22\r700\r2\r7000\r800\r0\r0s\r")}
	ConfigEditor(f, w.World)

	out := f.out.String()
	if !strings.Contains(out, "Options for the mods in play") || !strings.Contains(out, "Tank cost: 800") {
		t.Fatalf("the option screen or its new value never appeared:\n%s", out)
	}
	if !strings.Contains(out, "Enter a whole number from 1 to 1000") {
		t.Errorf("the out-of-range refusal did not name the range:\n%s", out)
	}
	if got := w.Config.ModOptions["opt"]["tank_cost"]; got != 800 {
		t.Errorf("tank_cost = %d, want 800", got)
	}
}

// The tview Mods tab holds one field per option, writes its choice into
// ModOptions, and refuses a number outside the bounds when the field ends.
func TestConfigTUIModOptions(t *testing.T) {
	dir := t.TempDir()
	writeTestOptionMod(t, dir, "opt")
	w := newWorld()
	w.Config.DataDir = dir
	w.Config.Mods = []string{"opt"}
	tui := newConfigTUI(w.World)
	form := tui.forms[len(tui.forms)-1]

	field := func(label string) *tview.InputField {
		for i := 0; i < form.GetFormItemCount(); i++ {
			item := form.GetFormItem(i)
			if sf, ok := item.(*styledField); ok {
				item = sf.FormItem
			}
			if f, ok := item.(*tview.InputField); ok && strings.HasPrefix(f.GetLabel(), label) {
				return f
			}
		}
		t.Fatalf("no option field %q on the Mods tab", label)
		return nil
	}
	cost := field(noStar + "opt: Tank cost")
	cost.SetText("800")
	if got := tui.collect().ModOptions["opt"]["tank_cost"]; got != 800 {
		t.Errorf("collected tank_cost = %d, want 800", got)
	}
	cost.SetText("9999")
	cost.InputHandler()(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone), func(tview.Primitive) {})
	if !tui.modalUp || !strings.Contains(cost.GetText(), "500") {
		t.Errorf("an out-of-range option was not refused (modal %v, text %q)", tui.modalUp, cost.GetText())
	}
	if got := tui.collect().ModOptions["opt"]["tank_cost"]; got != 500 {
		t.Errorf("collected tank_cost = %d, want the default after refusal", got)
	}
}

// The Mods page turns a mod on, and refuses one that will not load rather than
// leaving it for the reset to reject after the sysop has saved.
func TestConfigEditorModsPage(t *testing.T) {
	dir := t.TempDir()
	writeTestMod(t, dir, "a-cheap", "[units.tank]\nbuild_cost = 400\n")
	writeTestMod(t, dir, "b-broken", "[units.tank]\nbuild_cost = 0\n")
	w := newWorld()
	w.Config.DataDir = dir

	// Page 5 is Mods. 53 opens the list; 1 turns a-cheap on, 2 tries b-broken
	// (refused, with a pause), 0 leaves the list, S saves. The list reads one
	// key per choice.
	f := &fakeSession{keys: []rune("53\r12 0s\r ")}
	ConfigEditor(f, w.World)

	out := f.out.String()
	if !strings.Contains(out, "[x] a-cheap 1.0") {
		t.Fatalf("the list never showed a-cheap turned on:\n%s", out)
	}
	if !strings.Contains(out, "Configuration saved.") || !strings.Contains(out, "build_cost") {
		t.Errorf("the refusal or the save never happened:\n%s", out)
	}
	if got := strings.Join(w.Config.Mods, ","); got != "a-cheap" {
		t.Errorf("Mods = %q, want a-cheap alone", got)
	}
}

// The tview Mods tab saves the checked mods, and a league board has no Mods tab.
func TestConfigTUIModsTab(t *testing.T) {
	dir := t.TempDir()
	writeTestMod(t, dir, "a-cheap", "[units.tank]\nbuild_cost = 400\n")
	writeTestMod(t, dir, "z-cheap", "[units.jet]\nbuild_cost = 400\n")
	w := newWorld()
	w.Config.DataDir = dir
	w.Config.Mods = []string{"z-cheap"}

	tui := newConfigTUI(w.World)
	if got := tui.titles[len(tui.titles)-1]; got != modsTitle {
		t.Fatalf("last tab is %q, want %q", got, modsTitle)
	}
	form := tui.forms[len(tui.forms)-1]
	// Item 0 is the note, then the enabled z-cheap, then a-cheap. Toggling as a
	// sysop would (Space) keeps the order mods were turned on in: z-cheap off
	// and on again lands after a-cheap, as it does in the line editor.
	toggle := func(i int) {
		b := form.GetFormItem(i).(*styledField).FormItem.(*tview.Checkbox)
		b.InputHandler()(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone), func(tview.Primitive) {})
	}
	toggle(2)
	if got := strings.Join(tui.collect().Mods, ","); got != "z-cheap,a-cheap" {
		t.Errorf("Mods = %q, want z-cheap,a-cheap", got)
	}
	toggle(1)
	toggle(1)
	if got := strings.Join(tui.collect().Mods, ","); got != "a-cheap,z-cheap" {
		t.Errorf("Mods = %q, want a-cheap,z-cheap", got)
	}

	w.Config.IBBS = true
	if got := newConfigTUI(w.World).titles; slices.Contains(got, modsTitle) {
		t.Errorf("a league board got a Mods tab: %v", got)
	}
}

// Neither editor saves a mod list that will not load: the reset after a save
// would refuse it with config.json already rewritten.
func TestEditorsRefuseToSaveABrokenModList(t *testing.T) {
	dir := t.TempDir()
	writeTestMod(t, dir, "b-broken", "[units.tank]\nbuild_cost = 0\n")
	w := newWorld()
	w.Config.DataDir = dir
	w.Config.Mods = []string{"b-broken"}

	f := &fakeSession{keys: []rune("s\r q\r ")}
	if ConfigEditor(f, w.World) {
		t.Error("the line editor reported a save")
	}
	if !strings.Contains(f.out.String(), "not saved") {
		t.Errorf("no refusal shown:\n%s", f.out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err == nil {
		t.Error("the line editor wrote config.json")
	}

	tui := newConfigTUI(w.World)
	tui.save()
	if tui.saved {
		t.Error("the tview editor saved")
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err == nil {
		t.Error("the tview editor wrote config.json")
	}
}
