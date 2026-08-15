package menu

import (
	"reflect"
	"strings"
	"testing"

	"github.com/rivo/tview"

	"github.com/andy5995/immortal-barons/internal/game"
)

// distinctConfig gives every editable field a distinct, in-range value so a
// round-trip through the form catches a binder that reads the wrong widget or
// writes the wrong Config field (the copy-paste hazard in the repetitive
// addInt/addDropDown wiring).
func distinctConfig() game.Config {
	c := game.DefaultConfig()
	c.TurnsPerDay = 2
	c.ProtectionTurns = 3
	c.GameLength = 4
	c.GameStartDate = "2026-01-02"
	c.JoinDate = "2026-03-04"
	c.AICount = 5
	c.InitialMarketLand = 6
	c.LandPerDay = 7
	// The two bank rates carry BRE's own floors, so a round-trip fixture has to
	// use in-range values: 8 and 9 would clamp and read back as the floors.
	c.InterestRate = 80
	c.StdInvestRate = 90
	c.SteadyInvest = true
	c.MaxTaxRate = 11
	c.FoodUnlimited = true
	c.BuyMilitary = game.BuyLimited
	c.MaintCosts = game.High
	c.TradeCosts = game.Low
	c.RegionCosts = game.None
	c.AttackDamage = game.High
	c.AttackRewards = game.Low
	c.SlappenheimerHandling = game.SlappenheimerRandom
	c.MaxIndividualAttacks = 12
	c.MaxRegions = 13
	c.MaxPlayers = 14
	c.IBBS = true
	c.BoardID = "TESTBBS"
	c.InboundDir = "/srv/in"
	c.OutboundDir = "/srv/out"
	c.IdleTimeoutSecs = 15
	c.MaxIdleWarnings = 16
	return c
}

func TestConfigTUIRoundTrip(t *testing.T) {
	w := newWorld()
	w.Config = distinctConfig()
	tui := newConfigTUI(w.World)

	got := tui.collect()
	if !reflect.DeepEqual(got, w.Config) {
		t.Errorf("collect() did not round-trip the config\n got: %+v\nwant: %+v", got, w.Config)
	}
}

func TestConfigTUIClampsAndValidates(t *testing.T) {
	w := newWorld()
	w.Config = distinctConfig()
	tui := newConfigTUI(w.World)

	// An over-max integer clamps to the field's ceiling.
	setField(t, tui, 0, noStar+"Turns per day", func(f *tview.InputField) { f.SetText("99999") })
	// A malformed date is ignored, keeping the value the editor opened with.
	setField(t, tui, 0, noStar+"Game Start Date (YYYY-MM-DD)", func(f *tview.InputField) { f.SetText("not-a-date") })

	got := tui.collect()
	if got.TurnsPerDay != game.MaxTurnsPerDay {
		t.Errorf("turns per day should clamp to %d, got %d", game.MaxTurnsPerDay, got.TurnsPerDay)
	}
	if got.GameStartDate != "2026-01-02" {
		t.Errorf("malformed date should keep the opening value, got %q", got.GameStartDate)
	}
}

// setField reaches into a built form to drive a widget the way the user would,
// so the test exercises the same read-back path collect() uses. It matches on a
// label prefix since numeric labels now carry a " [lo-hi]" range suffix.
func setField(t *testing.T, tui *configTUI, tab int, labelPrefix string, edit func(*tview.InputField)) {
	t.Helper()
	form := tui.forms[tab]
	for i := 0; i < form.GetFormItemCount(); i++ {
		item := form.GetFormItem(i)
		if !strings.HasPrefix(item.GetLabel(), labelPrefix) {
			continue
		}
		// Fields are wrapped in *styledField (zebra/yellow styling); unwrap it.
		if sf, ok := item.(*styledField); ok {
			item = sf.FormItem
		}
		f, ok := item.(*tview.InputField)
		if !ok {
			t.Fatalf("field %q is not an input field", labelPrefix)
		}
		edit(f)
		return
	}
	t.Fatalf("no form field with label prefix %q on tab %d", labelPrefix, tab)
}

// Clearing a packet-directory field keeps the current path: -planetary has
// nowhere to read or write with an empty one.
func TestConfigTUIKeepsPacketDirWhenCleared(t *testing.T) {
	w := newWorld()
	w.Config = distinctConfig()
	tui := newConfigTUI(w.World)

	setField(t, tui, 3, star+"Inbound Dir", func(f *tview.InputField) { f.SetText("  ") })

	if got := tui.collect(); got.InboundDir != "/srv/in" {
		t.Errorf("InboundDir = %q, want the opening /srv/in", got.InboundDir)
	}
}

// tuiLabels is every field label the tview editor builds for a config, with the
// ★/spacer prefix and any " [lo-hi]" hint stripped, so it can be matched against
// the line editor's labels.
func tuiLabels(c game.Config) []string {
	w := newWorld()
	w.Config = c
	tui := newConfigTUI(w.World)
	var out []string
	for _, form := range tui.forms {
		for i := 0; i < form.GetFormItemCount(); i++ {
			label := form.GetFormItem(i).GetLabel()
			label = strings.TrimPrefix(strings.TrimPrefix(label, star), noStar)
			if j := strings.Index(label, " ["); j >= 0 {
				label = label[:j]
			}
			out = append(out, strings.TrimSpace(label))
		}
	}
	return out
}

// The two editors must hide the same settings off a league. They keep separate
// field lists — the line editor filters by number, the tview one by branch — so
// without this a setting added to one is shown by the other to a board that was
// never asked it.
func TestBothEditorsHideTheSameLeagueFields(t *testing.T) {
	c := distinctConfig()
	c.IBBS = true
	withLeague := tuiLabels(c)
	c.IBBS = false
	standalone := map[string]bool{}
	for _, l := range tuiLabels(c) {
		standalone[l] = true
	}

	var hidden []string
	for _, l := range withLeague {
		if !standalone[l] {
			hidden = append(hidden, l)
		}
	}

	// The line editor's own hidden set, by label.
	want := map[string]bool{}
	for _, page := range configPages(true) {
		for _, f := range page.fields {
			if ibbsOnlyFields[f.n] {
				want[f.label] = true
			}
		}
	}

	if len(hidden) != len(want) {
		t.Errorf("the tview editor hides %d fields off a league, the line editor %d:\n tview: %v\n line:  %v",
			len(hidden), len(want), hidden, want)
	}
	for _, l := range hidden {
		// The tview labels carry the "(0=unlimited)" hints the line editor puts in
		// its prompt instead, so match on the shared prefix.
		matched := false
		for w := range want {
			if strings.HasPrefix(l, w) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("the tview editor hides %q off a league; the line editor still offers it", l)
		}
	}
}

// The star means "inter-BBS option", as it does in the original — not "part of
// the league ruleset", which marked 25 of 32 fields and told a stand-alone sysop
// the opposite of what BRE's star tells them. Asserts the exact set on BOTH
// editors: the tview labels carry hints the line editor does not, and matching
// them by equality silently starred only half the set while every test passed.
func TestOnlyInterBBSFieldsAreStarred(t *testing.T) {
	c := distinctConfig()
	c.IBBS = true
	w := newWorld()
	w.Config = c
	tui := newConfigTUI(w.World)

	got := map[string]bool{}
	for _, form := range tui.forms {
		for i := 0; i < form.GetFormItemCount(); i++ {
			if l := form.GetFormItem(i).GetLabel(); strings.HasPrefix(l, star) {
				got[strings.TrimSpace(strings.TrimPrefix(l, star))] = true
			}
		}
	}

	want := ibbsOnlyLabels()
	if len(got) != len(want) {
		t.Errorf("tview editor stars %d fields, want the %d inter-BBS ones:\n got:  %v\n want: %v",
			len(got), len(want), got, want)
	}
	for g := range got {
		matched := false
		for wl := range want {
			if strings.HasPrefix(g, wl) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("%q is starred but is not an inter-BBS option", g)
		}
	}
}

// The star's legend only belongs on screen where a starred field can appear. A
// stand-alone board is never shown an inter-BBS field, so both the line
// editor's footer and the tview status bar leave the legend off there.
func TestStarLegendOnlyOnALeagueBoard(t *testing.T) {
	for _, ibbs := range []bool{true, false} {
		got := footerKeys(ibbs)
		if has := strings.Contains(got, "inter-BBS option"); has != ibbs {
			t.Errorf("IBBS=%v: legend present=%v, want %v (footer: %q)", ibbs, has, ibbs, got)
		}
	}
}
