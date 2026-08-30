package menu

import (
	"fmt"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
)

func TestAskYesNo(t *testing.T) {
	cases := []struct {
		keys   string
		defYes bool
		want   bool
	}{
		{"y", true, true},
		{"\r", true, true},
		{"n", true, false},
		{"N", true, false},
		{"y", false, true},
		{"\r", false, false},
		{"n", false, false},
		{"N", false, false},
	}
	for _, c := range cases {
		f := &fakeSession{keys: []rune(c.keys)}
		got := AskYesNo(f, "Continue?", c.defYes)
		if got != c.want {
			t.Errorf("AskYesNo(%q, defYes=%v) = %v, want %v", c.keys, c.defYes, got, c.want)
		}
	}
}

func TestAskYesNoHint(t *testing.T) {
	f := &fakeSession{keys: []rune("\r")}
	AskYesNo(f, "Continue?", true)
	if !strings.Contains(f.out.String(), "Y/n") { // colored parens surround the letters
		t.Errorf("expected Y/n hint, got %q", f.out.String())
	}

	f2 := &fakeSession{keys: []rune("\r")}
	AskYesNo(f2, "Continue?", false)
	if !strings.Contains(f2.out.String(), "y/N") {
		t.Errorf("expected y/N hint, got %q", f2.out.String())
	}
}

func TestIncomeReportWritesNonEmpty(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")} // pause keypress
	w := newWorld()
	incomeReport(f, w)
	if f.out.Len() == 0 {
		t.Error("expected incomeReport to write output")
	}
	if !strings.Contains(f.out.String(), "gold was earned in taxes.") {
		t.Error("expected the income report's first line")
	}
}

// BRE opens the income lines under its 75-column blue inset rule and gives them
// no heading (docs/dev/bre-screens.md). IB drew a blue-backed "Income Report"
// bar of its own instead until 2026-08-25 — the only place in the game that
// used a filled background for a heading. Golden literal off the capture, so a
// retune has to bring new evidence.
func TestIncomeReportOpensUnderTheCapturedRule(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")}
	w := newWorld()
	incomeReport(f, w)
	out := f.out.String()

	const rule = "\x1b[34m─────═══════════════───────────────────────────────────────────────────────\x1b[0m"
	if !strings.HasPrefix(out, "\n"+rule+"\n") {
		t.Errorf("the income lines should open under the 75-column blue rule, got %q", out)
	}
	if strings.Contains(out, "Income Report") {
		t.Error("BRE gives the income lines no heading")
	}
	if strings.Contains(out, "\x1b[44m") {
		t.Error("no screen fills a heading background; that style is gone")
	}
}

func TestEndOfTurnStatsWritesNonEmpty(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")} // pause keypress
	w := newWorld()
	endOfTurnStats(f, w)
	if f.out.Len() == 0 {
		t.Error("expected endOfTurnStats to write output")
	}
	if !strings.Contains(f.out.String(), "End of Turn Statistics") {
		t.Error("expected the End of Turn Statistics heading")
	}
}

// A turn with no migration still reports the figure: BRE prints "gained 0"
// (cap/kd3-01.cap, twice), and there is no third string in the binary —
// process_end_of_turn references only "Your dominion gained "/"lost ". IB used
// to print nothing at all, which left a starving realm (growth is forced to
// zero with an empty granary) with no line to read.
func TestEndOfTurnStatsReportsZeroPopulationGrowth(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")}
	w := newWorld()
	w.Player().LastPopGrowth = 0
	endOfTurnStats(f, w)
	if !strings.Contains(f.out.String(), "gained \x1b[96m0\x1b[0m people.") {
		t.Errorf("want the gained-0 line, got %q", f.out.String())
	}
}

// The screen is bracketed by BRE's 75-column blue inset rule — 5 single, 15
// double, 55 single — one under the heading and one closing the block. Golden
// literal off the capture rather than a rebuild from rule75Width, so a retune
// has to bring new evidence.
func TestEndOfTurnStatsIsBracketedByTheCapturedRule(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")}
	w := newWorld()
	w.Player().LastSpoiled = 57 // a body line, so the two rules cannot be adjacent
	endOfTurnStats(f, w)
	out := f.out.String()

	const rule = "\x1b[34m─────═══════════════───────────────────────────────────────────────────────\x1b[0m"
	if n := strings.Count(out, rule); n != 2 {
		t.Fatalf("want the 75-column blue inset rule twice, got %d:\n%q", n, out)
	}
	if !strings.Contains(out, "End of Turn Statistics:\x1b[0m\n"+rule) {
		t.Error("the rule should sit immediately under the heading")
	}
	if !strings.HasSuffix(out, rule+"\n") {
		t.Error("the rule should close the block")
	}
	if !strings.Contains(out, "57") {
		t.Fatalf("the food-spoilage line never rendered, so the block was empty:\n%q", out)
	}
}

// TestIncomeReportShowsIndustryProduction checks #71: production (and its
// report lines) moved from end-of-turn to turn start, alongside income.
func TestIncomeReportShowsIndustryProduction(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")} // pause keypress
	w := newWorld()
	p := w.Player()
	p.Regions.Industrial = 10
	// Allocate only 30% to troopers so the other 70% pays out as industrial gold
	// (BRE's trade-off: units and gold share one capacity pool).
	p.ProdTroopers, p.ProdJets, p.ProdTurrets = 30, 0, 0
	p.ProdBombers, p.ProdTanks, p.ProdCarriers = 0, 0, 0
	w.World.Manufacture(p) // turn-start step, done by runTurn's loop before incomeReport

	incomeReport(f, w)
	out := f.out.String()
	for _, want := range []string{
		"gold was earned from Industrial Zones", // industrial gold, now itemized in the breakdown
		"Your Industrial Zones built",
		"Troopers",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected incomeReport to contain %q, got:\n%s", want, out)
		}
	}
}

// TestEndOfTurnStatsNoLongerShowsIndustryProduction is the flip side of
// TestIncomeReportShowsIndustryProduction: the industry lines no longer
// belong in endOfTurnStats (#71).
func TestEndOfTurnStatsNoLongerShowsIndustryProduction(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")} // pause keypress
	w := newWorld()
	p := w.Player()
	p.Regions.Industrial = 10
	w.World.Manufacture(p)

	endOfTurnStats(f, w)
	out := f.out.String()
	for _, unwanted := range []string{
		"gold was produced by your Industry",
		"Your Industrial Zones built",
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("expected endOfTurnStats to NOT contain %q, got:\n%s", unwanted, out)
		}
	}
}

func TestRunTurnNoTurnsLeftReturnsImmediately(t *testing.T) {
	f := &fakeSession{keys: []rune("  ")} // pause keypress inside ok(), then seeScores' pause
	w := newWorld()
	w.Player().TurnsLeft = 0
	runTurn(f, w)
	if !strings.Contains(f.out.String(), "used all of your turns today") {
		t.Error("expected the no-turns-left message")
	}
}

func TestIncomeReportSurfacesAndClearsPirateRaids(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")} // one key for the pause
	w := newWorld()
	p := w.Player()
	// Slot 3 is the fourth band, painted plain red by BRE's palette. The colour
	// is keyed on the slot, not the name, so a world with other names still
	// gets it.
	p.PirateHits = []game.PirateHit{{Faction: "Sharks", Slot: 3, Spoil: game.SpoilTanks, Amount: 7777}}

	incomeReport(f, w)

	// The line names the faction, the figure and the one thing taken, as BRE's
	// does; the figure is comma-grouped, which is IB's recorded divergence.
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Sharks have captured 7,777 Tanks") {
		t.Errorf("income report should show BRE's raid line, got:\n%s", out)
	}
	if !strings.Contains(f.out.String(), ansi.FgRed+"Sharks") {
		t.Error("the faction name should carry its own color")
	}
	if len(p.PirateHits) != 0 {
		t.Error("income report should clear the raid notice after showing it")
	}
}

func TestPaymentStageAutoPays(t *testing.T) {
	f := &fakeSession{}
	w := newWorld()
	w.Player().Prefs.AutoPayMaint = true
	p := w.Player()
	p.Gold = 100000
	before := p.Gold
	want := p.ForcesUpkeep() + p.RegionUpkeep() + w.World.CrownTax(p)

	paymentStage(f, w, BuildMenus().Bank)

	if p.Gold != before-want {
		t.Errorf("auto-pay should deduct %d, gold %d -> %d", want, before, p.Gold)
	}
	// BRE's auto-pay summary: one comma-grouped total, then "Gold paid."
	if !strings.Contains(f.out.String(), comma(want)+"\x1b[0m Gold paid.") {
		t.Errorf("expected the auto-pay total line for %s gold, got:\n%s", comma(want), f.out.String())
	}
}

// TestPaymentStageBypassesAutoPayOnLowSupport is BRE-verified (BRE.OVR
// `allocate_turn_budget`, 0x02eebb): the silent Auto-Pay total only fires
// when popular support and military morale both sit at their 100 cap and the
// realm holds no Waste regions. Below-full support bypasses Auto-Pay for
// this turn only, even with the preference on and gold to spare.
func TestPaymentStageBypassesAutoPayOnLowSupport(t *testing.T) {
	f := &fakeSession{keys: []rune("n\r\r\r\r")} // decline bank, accept forces/regions/crown tax, accept the support-boost prompt
	w := newWorld()
	w.Player().Prefs.AutoPayMaint = true
	p := w.Player()
	p.Gold = 1_000_000
	p.Support = 50

	paymentStage(f, w, BuildMenus().Bank)

	out := f.out.String()
	if !strings.Contains(out, "Maintenance:") {
		t.Errorf("expected the manual maintenance sequence to run with support below 100, got:\n%s", out)
	}
	if strings.Contains(out, "Gold paid.") {
		t.Errorf("auto-pay should bypass its silent total while support is below 100:\n%s", out)
	}
}

// TestPaymentStageBypassesAutoPayOnLowMorale is the morale twin of
// TestPaymentStageBypassesAutoPayOnLowSupport.
func TestPaymentStageBypassesAutoPayOnLowMorale(t *testing.T) {
	f := &fakeSession{keys: []rune("n\r\r\r\r")} // decline bank, accept forces/regions/crown tax, accept the morale-boost prompt
	w := newWorld()
	w.Player().Prefs.AutoPayMaint = true
	p := w.Player()
	p.Gold = 1_000_000
	p.Morale = 50

	paymentStage(f, w, BuildMenus().Bank)

	out := f.out.String()
	if !strings.Contains(out, "Maintenance:") {
		t.Errorf("expected the manual maintenance sequence to run with morale below 100, got:\n%s", out)
	}
	if strings.Contains(out, "Gold paid.") {
		t.Errorf("auto-pay should bypass its silent total while morale is below 100:\n%s", out)
	}
}

// TestPaymentStageBypassesAutoPayOnWaste is the Waste-region twin: any Waste
// on the books bypasses Auto-Pay's silent total, the same as low
// support/morale, so the decontamination offer in the manual flow still
// reaches the player.
func TestPaymentStageBypassesAutoPayOnWaste(t *testing.T) {
	f := &fakeSession{keys: []rune("n\r\r\r\r")} // decline bank, accept forces/regions/crown tax, accept the decontamination offer
	w := newWorld()
	w.Player().Prefs.AutoPayMaint = true
	p := w.Player()
	p.Gold = 1_000_000
	p.Regions.Waste = 50

	paymentStage(f, w, BuildMenus().Bank)

	out := f.out.String()
	if !strings.Contains(out, "Maintenance:") {
		t.Errorf("expected the manual maintenance sequence to run with Waste regions on hand, got:\n%s", out)
	}
	if strings.Contains(out, "Gold paid.") {
		t.Errorf("auto-pay should bypass its silent total while Waste regions remain:\n%s", out)
	}
}

func TestPaymentStageManualFullPayNoDesertion(t *testing.T) {
	f := &fakeSession{keys: []rune("n\r\r\r")} // decline bank, accept suggested (full) for forces, regions, crown tax
	w := newWorld()
	w.Player().Prefs.AutoPayMaint = false
	p := w.Player()
	p.Gold = 100000
	p.Support = 100 // skips the optional support-boost prompt
	beforeTroopers := p.Troopers
	want := p.ForcesUpkeep() + p.RegionUpkeep() + w.World.CrownTax(p)
	before := p.Gold

	paymentStage(f, w, BuildMenus().Bank)

	if p.Troopers != beforeTroopers {
		t.Errorf("full pay should not desert troopers, %d -> %d", beforeTroopers, p.Troopers)
	}
	if p.Gold != before-want {
		t.Errorf("manual full pay should deduct %d, gold %d -> %d", want, before, p.Gold)
	}
}

func TestPaymentStageManualUnderpayDeserts(t *testing.T) {
	f := &fakeSession{keys: []rune("n0\r0\r0\r")} // decline bank, give 0 to forces, regions and crown tax
	w := newWorld()
	w.Player().Prefs.AutoPayMaint = false
	p := w.Player()
	p.Gold = 100000
	p.Support = 100
	beforeTroopers := p.Troopers

	paymentStage(f, w, BuildMenus().Bank)

	if p.Troopers >= beforeTroopers {
		t.Errorf("underpaying forces should desert troopers, %d -> %d", beforeTroopers, p.Troopers)
	}
	if !strings.Contains(f.out.String(), "deserted") {
		t.Error("expected a desertion notice")
	}
}

// TestRunTurnConsumesATurn scripts a full pass through the pipeline for one
// turn. Key map, derived by driving the current flow (the old script and its
// comment described the pre-#70 pre-turn Diplomacy stop): four pauses (the
// Queen's refund, income, status, maintenance-paid), Quit Spending, Quit
// Attack — the Covert/Trading/Message stops are Preferences-gated and off by
// default — then decline "Continue to your next turn?" to stop after one turn.
func TestRunTurnConsumesATurn(t *testing.T) {
	keys := "    00n"
	f := &fakeSession{keys: []rune(keys)}
	w := newWorld()
	w.Player().Prefs.AutoPayMaint = true // pay maintenance silently; this test is about the turn loop
	left := w.Player().TurnsLeft
	runTurn(f, w)
	if w.Player().TurnsLeft != left-1 {
		t.Errorf("expected TurnsLeft %d, got %d", left-1, w.Player().TurnsLeft)
	}
	// The script must have ended by DECLINING the continue prompt, not by
	// running dry mid-turn — running dry also leaves TurnsLeft at left-1 and
	// hides a re-mapped key.
	if !strings.Contains(f.out.String(), "Continue to your next turn?") {
		t.Errorf("script never reached the continue prompt:\n%s", f.out.String())
	}
}

// TestRunTurnCovertGatedBeforeSpending checks the Covert stage's placement and
// gate: with an agent (and VisitCovert on) it runs right before Spending; with
// no agent it is skipped entirely — a fresh realm starts with zero agents.
func TestRunTurnCovertGatedBeforeSpending(t *testing.T) {
	// With an agent, Covert Operations appears before the Spending Menu.
	f := &fakeSession{keys: []rune("0\r   0000nn")}
	w := newWorld()
	w.Player().Prefs.AutoPayMaint = true
	w.Player().Prefs.VisitCovert, w.Player().Prefs.VisitTrading, w.Player().Prefs.VisitMessage = true, true, true // this test walks every optional menu
	w.Player().Agents = 1
	runTurn(f, w)
	out := f.out.String()
	cov, spend := strings.Index(stripANSI(out), "Covert Operations"), strings.Index(stripANSI(out), "[Spending]")
	if cov == -1 || spend == -1 {
		t.Fatalf("expected both the Covert and Spending menus; got:\n%s", out)
	}
	if cov > spend {
		t.Errorf("Covert should come before Spending (offsets cov=%d spend=%d)", cov, spend)
	}

	// With no agent, the Covert stage is skipped (one fewer menu to quit).
	f2 := &fakeSession{keys: []rune("0\r   000nn")}
	w2 := newWorld()
	w2.Player().Prefs.AutoPayMaint = true
	w2.Player().Prefs.VisitCovert, w2.Player().Prefs.VisitTrading, w2.Player().Prefs.VisitMessage = true, true, true
	w2.Player().Agents = 0
	runTurn(f2, w2)
	out2 := f2.out.String()
	if strings.Contains(out2, "Covert Operations") {
		t.Errorf("Covert should be skipped with no agents; got:\n%s", out2)
	}
	if !strings.Contains(stripANSI(out2), "[Spending]") {
		t.Error("Spending should still run with no agents")
	}
}

// TestRunTurnShowsPreTurnStopsInOrder checks BRE's pre-turn sequence: the
// event log ("Since your last play..."), then Diplomacy, then Change
// Production (Set Industries), all before the ordinary turn pipeline (#63).
// TestRunTurnHasNoPreTurnDiplomacyOrProduction checks the Play flow shows the
// event log then goes straight into the turn — Diplomacy and Change Production
// are no longer pre-turn stops (they live on the System menu).
func TestRunTurnHasNoPreTurnDiplomacyOrProduction(t *testing.T) {
	keys := " 0\r   0000nn" // leading pause for the non-empty event log
	f := &fakeSession{keys: []rune(keys)}
	w := newWorld()
	w.Player().Prefs.AutoPayMaint = true
	w.Player().Prefs.VisitCovert, w.Player().Prefs.VisitTrading, w.Player().Prefs.VisitMessage = true, true, true // this test walks every optional menu
	w.Player().Agents = 1                                                                                         // hold an agent so the Covert stage runs (it now gates on that)
	w.Player().Events = []game.Event{{Text: "A dragon attacked your regions."}}

	runTurn(f, w)
	out := f.out.String()

	eventsAt := strings.Index(out, "Since your last play, this has happened:")
	incomeAt := strings.Index(out, "gold was earned in taxes.")
	if eventsAt == -1 || incomeAt == -1 {
		t.Fatalf("expected the event log and the income report, got:\n%s", out)
	}
	if eventsAt > incomeAt {
		t.Errorf("event log should precede the income report, got offsets %d, %d", eventsAt, incomeAt)
	}
	if strings.Contains(out, ansi.FgBrightWhite+"Diplomacy") {
		t.Errorf("Diplomacy should not appear in the Play flow (System menu only):\n%s", out)
	}
	if strings.Contains(out, "Change Production?") {
		t.Errorf("Change Production should not appear in the Play flow (System menu only):\n%s", out)
	}
}

// TestRunTurnPreTurnStopsOnceAcrossTwoTurns checks #70: Diplomacy and Change
// Production are pre-turn stops for the whole Play session, not per turn —
// they must appear exactly once even when the player continues into a
// second turn.
// TestRunTurnPlaysTwoTurnsWithoutDiplomacy checks two turns play cleanly with an
// Income Report each turn and no Diplomacy/Change Production stop anywhere.
func TestRunTurnPlaysTwoTurnsWithoutDiplomacy(t *testing.T) {
	perTurn := "   0000n" // income/status pauses, quit Spending/Attack/Covert/Trading, decline message
	// The Queen's refund is paid once a game day, so its pause is dismissed on
	// the first turn only.
	keys := " " + perTurn + "y" + perTurn + "n"
	f := &fakeSession{keys: []rune(keys)}
	w := newWorld()
	w.Player().Prefs.AutoPayMaint = true
	w.Player().Prefs.VisitCovert, w.Player().Prefs.VisitTrading, w.Player().Prefs.VisitMessage = true, true, true // this test walks every optional menu
	w.Player().Agents = 1                                                                                         // hold an agent so the Covert stage runs (it now gates on that)
	left := w.Player().TurnsLeft

	runTurn(f, w)
	out := f.out.String()

	if got := w.Player().TurnsLeft; got != left-2 {
		t.Fatalf("expected two turns consumed, TurnsLeft %d -> %d", left, got)
	}
	if strings.Contains(out, ansi.FgBrightWhite+"Diplomacy") {
		t.Errorf("Diplomacy should not appear in the Play flow:\n%s", out)
	}
	if n := strings.Count(out, "gold was earned in taxes."); n != 2 {
		t.Errorf("expected the income report once per turn (2), got %d\n%s", n, out)
	}
}

func TestRenderDailyBulletinRowsSignsAndColors(t *testing.T) {
	b := game.DailyBulletin{
		Totals: game.PlanetTotals{Population: 1865289, Regions: 53266, NetWorth: 34833000},
		Change: game.PlanetTotals{Population: -5838, Regions: 0, NetWorth: 1373000},
	}
	f := &fakeSession{}
	renderDailyBulletin(f, Term{UTF8: true}, b, "wildside")
	out := f.out.String()

	for _, want := range []string{
		"wildside — Daily Bulletin",
		// The sign carries its own colour, so it and its figure are asserted apart.
		"Total Population", "1,865,289", "5,838",
		"Total Regions", "53,266",
		"Total Net Worth", "34m", "1m",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	// BRE's colouring, as golden escape literals off bre-01-color.cap: the figure
	// is `96` bright cyan whichever way the day went, and only a rise paints its
	// `+` in `92` bright green. Direction is carried by the sign, so a reader who
	// sees no colour at all still gets it.
	if !strings.Contains(out, "\x1b[96m-\x1b[96m5,838") {
		t.Error("a falling figure and its minus sign should both be bright cyan")
	}
	if !strings.Contains(out, "\x1b[92m+\x1b[96m1m") {
		t.Error("a rising figure should carry a bright-green plus over a bright-cyan figure")
	}
	if !strings.Contains(out, "\x1b[96m+\x1b[96m0") {
		t.Error("an unchanged figure should not be painted as a rise")
	}
	if strings.Contains(out, ansi.FgRed) {
		t.Errorf("nothing in the bulletin is red — direction is not colour-coded:\n%q", out)
	}
}

func TestRenderDailyBulletinNoTitle(t *testing.T) {
	f := &fakeSession{}
	renderDailyBulletin(f, Term{UTF8: true}, game.DailyBulletin{}, "")
	if strings.Contains(f.out.String(), "—") {
		t.Error("expected no board-name prefix when title is empty")
	}
	if !strings.Contains(f.out.String(), "Daily Bulletin") {
		t.Error("expected the bare 'Daily Bulletin' header")
	}
}

func TestShowBulletinTodayVsYesterday(t *testing.T) {
	w := newWorld()
	w.BulletinToday = game.DailyBulletin{Totals: game.PlanetTotals{Population: 111}}
	w.NewsToday = []string{"today-line"}
	w.BulletinYesterday = game.DailyBulletin{Totals: game.PlanetTotals{Population: 222}}
	w.NewsYesterday = []string{"yesterday-line"}
	live := w.PlanetTotals() // today's Totals are computed live, not the stored 111 (#109)

	fToday := &fakeSession{keys: []rune(" ")}
	showBulletinToday(fToday, w)
	todayOut := fToday.out.String()
	if !strings.Contains(todayOut, "today-line") || strings.Contains(todayOut, "yesterday-line") {
		t.Errorf("showBulletinToday should show only today's news, got:\n%s", todayOut)
	}
	if !strings.Contains(todayOut, formatGold(live.Population, "")) {
		t.Error("showBulletinToday should render today's LIVE totals")
	}

	fYesterday := &fakeSession{keys: []rune(" ")}
	showBulletinYesterday(fYesterday, w)
	yesterdayOut := fYesterday.out.String()
	if !strings.Contains(yesterdayOut, "yesterday-line") || strings.Contains(yesterdayOut, "today-line") {
		t.Errorf("showBulletinYesterday should show only yesterday's news, got:\n%s", yesterdayOut)
	}
	if !strings.Contains(yesterdayOut, "222") {
		t.Error("showBulletinYesterday should render yesterday's totals")
	}
}

// Regression test for #109: rollNews only freezes a BulletinToday.Totals
// snapshot at daily maintenance, so a board still on its first game day (or
// any realm created since the last maintenance) had a zero-valued snapshot
// while the scoreboard beside it already showed living, populated realms.
// Today's News must reflect the CURRENT totals, not that stale snapshot.
func TestShowBulletinTodayNotStaleOnFreshBoard(t *testing.T) {
	w := newWorld()
	// A brand-new board: no maintenance has ever rolled a snapshot, so
	// BulletinToday is still its zero value, as game.NewWorldSeed leaves it.
	w.BulletinToday = game.DailyBulletin{}
	live := w.PlanetTotals()
	if live.Population == 0 {
		t.Fatal("test setup: expected the seeded world to already have living, populated empires")
	}

	f := &fakeSession{keys: []rune(" ")}
	showBulletinToday(f, w)
	out := f.out.String()

	if !strings.Contains(out, formatGold(live.Population, "")) {
		t.Errorf("expected Today's News to show the live population %d, got:\n%s", live.Population, out)
	}
}

func TestShowBulletinMastheadAndBox(t *testing.T) {
	w := newWorld()
	w.BulletinToday = game.DailyBulletin{Totals: game.PlanetTotals{Population: 111}}
	w.NewsToday = []string{"alpha-line", "beta-line"}
	f := &fakeSession{keys: []rune(" ")}
	showBulletinToday(f, w)
	out := f.out.String()

	for _, want := range []string{
		"News File",      // masthead header line
		newsBannerName,   // centered masthead banner
		"Daily Bulletin", // box title
		"alpha-line", "beta-line",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected news screen to contain %q, got:\n%s", want, out)
		}
	}
	// Golden literals, not the constants: BRE draws the Daily Bulletin in the
	// SINGLE-line CP437 set at 66 columns and leads the masthead banner with `═`,
	// not `»`. Both were wrong until the 2026-08-16 capture audit, so these must
	// fail loudly if anyone "tidies" them back (docs/dev/bre-screens.md).
	if strings.ContainsAny(out, "╔║╚╗╝") {
		t.Errorf("Daily Bulletin must use the single-line box, not ╔ ║ ╚, got:\n%s", out)
	}
	if !strings.Contains(out, "┌") || !strings.Contains(out, "│") || !strings.Contains(out, "└") {
		t.Errorf("expected a single-line boxed Daily Bulletin (┌ │ └), got:\n%s", out)
	}
	if !strings.Contains(out, "└"+strings.Repeat("─", 64)+"┘") {
		t.Errorf("expected the bulletin box to close at 64 inner columns, got:\n%s", out)
	}
	wantBanner := "\x1b[31m──\x1b[91m═\x1b[97m" + newsBannerName + "\x1b[91m═\x1b[31m──"
	if !strings.Contains(out, wantBanner) {
		t.Errorf("expected the masthead banner %q, got:\n%s", wantBanner, out)
	}
	if strings.Contains(out, "»"+newsBannerName) || strings.Contains(out, newsBannerName+"«") {
		t.Errorf("masthead banner must not use guillemets, got:\n%s", out)
	}
	// Each news item is led by the red arrow and the items are blank-line
	// separated, so the arrow appears once per item.
	if n := strings.Count(out, newsItemArrow); n != 2 {
		t.Errorf("expected 2 news-item arrows (one per item), got %d:\n%s", n, out)
	}
}

// Golden literals from the live capture in cap/kd3-01.cap, not from named
// constants: these are the fidelity contract, so a retune must fail here and
// produce new evidence. The other-realm case is the one that matters — BRE
// paints every realm `1;33`, giving the reader's own no distinct color, and a
// test covering only the player's own realm passed for months while other
// realms rendered in an invisible bright-white.
func TestNewsItemHighlights(t *testing.T) {
	w := newWorld()
	self := w.Player().Name
	var other string
	for _, e := range w.Empires {
		if e.Name != self {
			other = e.Name
			break
		}
	}
	if other == "" {
		t.Fatal("need a second empire to prove own and other share a color")
	}
	w.NewsToday = []string{
		self + " routed the " + game.PirateFactions[0] + " and took 1,375 regions from " +
			other + "; now Planetary Master!",
	}
	f := &fakeSession{keys: []rune(" ")}
	showBulletinToday(f, w)
	out := f.out.String()

	for what, want := range map[string]string{
		"own empire bright-yellow":   "\x1b[93m" + self,
		"other empire bright-yellow": "\x1b[93m" + other,
		"faction bright-red":         "\x1b[91m" + game.PirateFactions[0],
		"title bright-white":         "\x1b[97m" + "Planetary Master",
		"number bright-white":        "\x1b[97m" + "1,375",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%s: expected %q in output:\n%s", what, want, out)
		}
	}
}

func TestShowBulletinEmptyNewsShowsNoBulletinsNote(t *testing.T) {
	w := newWorld()
	w.NewsToday = nil
	f := &fakeSession{keys: []rune(" ")}
	showBulletinToday(f, w)
	if !strings.Contains(f.out.String(), "No planetary bulletins.") {
		t.Error("expected the empty-state note when there is no news")
	}
}

// Auto-Feed on + a food shortfall brings the Food Market up automatically so the
// player can buy food before their people starve (the old bug: nothing did).
func TestFeedStageAutoFeedOpensMarketWhenShort(t *testing.T) {
	w := newWorld()
	w.Player().Prefs.AutoFeed = true
	p := w.Player()
	p.Food = 0
	p.People = 100000 // ensures FoodUpkeep > 0
	// leading key dismisses BRE's pause before the market; then quit market,
	// give the default (0), decline the reconsider
	f := &fakeSession{keys: []rune(" 0\rn")}
	if _, err := feedStage(f, w, BuildMenus().Food, true); err != nil {
		t.Fatalf("feedStage: %v", err)
	}
	out := f.out.String()
	if !strings.Contains(out, "need") {
		t.Errorf("expected a food-shortfall warning; got:\n%s", out)
	}
	if !strings.Contains(out, "Chopper") { // the Food Market title
		t.Errorf("Auto-Feed should open the Food Market; got:\n%s", out)
	}
}

// The people and the armed forces are two separate obligations, and BRE prompts
// for each one in turn. Feeding the people in full but the army not at all must
// still raise the disastrous-results reconsider.
func TestFeedStageAsksForPeopleAndForcesSeparately(t *testing.T) {
	w := newWorld()
	w.Player().Prefs.AutoFeed = true
	p := w.Player()
	p.People = 100000 // eats 7,500
	p.Turrets = 500000
	p.Food = 7500 // covers the people exactly, nothing for the army
	if got := p.PeopleFoodUpkeep(); got != 7500 {
		t.Fatalf("test setup: people should eat 7,500, got %d", got)
	}
	if got := w.ForcesFoodDue(p); got != 50 {
		t.Fatalf("test setup: 500,000 turrets should eat 50, got %d", got)
	}
	// dismiss the pause, quit market, take each prompt's default, decline the
	// reconsider
	f := &fakeSession{keys: []rune(" 0\r\rn")}
	if _, err := feedStage(f, w, BuildMenus().Food, true); err != nil {
		t.Fatalf("feedStage: %v", err)
	}
	out := f.out.String()
	if !strings.Contains(out, "people need ") {
		t.Errorf("expected the people's obligation; got:\n%s", out)
	}
	if !strings.Contains(out, "armed forces require ") {
		t.Errorf("expected the armed forces' obligation as its own prompt; got:\n%s", out)
	}
	if !strings.Contains(out, "disastrous") {
		t.Errorf("an unfed army must still raise the reconsider; got:\n%s", out)
	}
}

func TestFeedStageNoNoticeWhenFed(t *testing.T) {
	w := newWorld()
	w.Player().Prefs.AutoFeed = true
	w.Player().Food = 10_000_000 // far more than needed
	f := &fakeSession{}
	if _, err := feedStage(f, w, BuildMenus().Food, true); err != nil {
		t.Fatalf("feedStage: %v", err)
	}
	if f.out.Len() != 0 {
		t.Errorf("a fed realm should get no feed notice; got:\n%s", f.out.String())
	}
}

// Auto-Feed off is BRE's manual food flow, and it runs EVERY turn — the market
// opens and both obligations are asked even with food to spare. A capture with
// 1608 food against a 150 need shows exactly that (cap/121125-666H4H_Camembert
// _Public.cap), and run_player_turn only skips the routine when the realm is
// covered AND Auto-Feed is on.
func TestFeedStageAutoFeedOffOpensMarketEvenWhenFed(t *testing.T) {
	w := newWorld()
	w.Player().Prefs.AutoFeed = false
	p := w.Player()
	p.People = 100000
	p.Food = 10_000_000 // far more than needed
	f := &fakeSession{keys: []rune(" 0\r\r")}
	if _, err := feedStage(f, w, BuildMenus().Food, true); err != nil {
		t.Fatalf("feedStage: %v", err)
	}
	out := f.out.String()
	if !strings.Contains(out, "Chopper") { // the Food Market title
		t.Errorf("Auto-Feed off must open the market every turn; got:\n%s", out)
	}
	if !strings.Contains(out, "people need ") {
		t.Errorf("expected the people's obligation; got:\n%s", out)
	}
	if strings.Contains(out, "disastrous") {
		t.Errorf("a fed realm must not raise the reconsider; got:\n%s", out)
	}
}

// Short of food with Auto-Feed OFF still gets the full flow: BRE's auto-feed
// only covers what the realm can afford ("if possible" in its own help text).
func TestFeedStageAutoFeedOffOpensMarketWhenShort(t *testing.T) {
	w := newWorld()
	w.Player().Prefs.AutoFeed = false
	p := w.Player()
	p.Food = 0
	p.People = 100000
	f := &fakeSession{keys: []rune(" 0\rn")}
	if _, err := feedStage(f, w, BuildMenus().Food, true); err != nil {
		t.Fatalf("feedStage: %v", err)
	}
	out := f.out.String()
	if !strings.Contains(out, "Chopper") {
		t.Errorf("expected the Food Market; got:\n%s", out)
	}
	if !strings.Contains(out, "disastrous") {
		t.Errorf("an unfed realm must raise the reconsider; got:\n%s", out)
	}
}

// BRE pauses between the maintenance total and the Food Market, and prints its
// "units of Food consumed." summary ONLY when it fed the realm silently — the
// market path goes straight from the two obligations to the bank
// (cap/eots-ibbs-01.cap: 8 market turns, none carrying that line, against 130
// silent ones that all do). The assertions below check the flow reached the
// market, not merely that it produced output: a script that lost a key to the
// new pause would fail on the market title.
func TestFeedStagePausesBeforeTheMarket(t *testing.T) {
	w := newWorld()
	w.Player().Prefs.AutoFeed = false
	p := w.Player()
	p.People = 100000
	p.Food = 10_000_000
	f := &fakeSession{keys: []rune(" 0\r\r")}
	silent, err := feedStage(f, w, BuildMenus().Food, true)
	if err != nil {
		t.Fatalf("feedStage: %v", err)
	}
	if silent {
		t.Error("the market ran, so feedStage must not report a silent feed")
	}
	out := f.out.String()
	if !strings.Contains(out, "Paused") {
		t.Errorf("expected the pause before the market; got:\n%s", out)
	}
	if !strings.Contains(out, "Chopper") {
		t.Errorf("the flow never reached the Food Market; got:\n%s", out)
	}
	if i, j := strings.Index(out, "Paused"), strings.Index(out, "Chopper"); i > j {
		t.Error("the pause must come before the market, not after it")
	}
}

// The pause has to survive the WHOLE turn path, not just a direct feedStage
// call: paymentStage is what reports the auto-pay summary, and a wrong return
// value there is invisible to a test that passes the flag in by hand. That is
// exactly what shipped in 91e0a42f — the unit tests below were green while a
// real turn showed "242,483 Gold paid." followed straight by the market.
func TestRunTurnPausesBetweenTheAutoPaySummaryAndTheMarket(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Prefs.AutoPayMaint = true
	p.Prefs.AutoFeed = false // Auto-Feed off opens the market every turn
	p.People = 100000
	p.Food = 10_000_000
	f := &fakeSession{keys: []rune("      000n")}
	runTurn(f, w)
	out := stripANSI(f.out.String())
	gold := strings.Index(out, "Gold paid.")
	market := strings.Index(out, "Chopper")
	if gold < 0 || market < 0 {
		t.Fatalf("the turn never reached the auto-pay summary and the market:\n%s", out)
	}
	if !strings.Contains(out[gold:market], "Paused") {
		t.Errorf("no pause between the summary and the market; between them:\n%q", out[gold:market])
	}
}

// Maintenance paid by hand gets NO pause before the market: BRE goes straight
// from the last payment prompt to it (cap/kd3-01.cap, the Queen Royale prompt
// followed immediately by "We have N units of food available today."). The
// pause belongs to the auto-pay summary, not to the food stage.
func TestFeedStageDoesNotPauseAfterManualMaintenance(t *testing.T) {
	w := newWorld()
	w.Player().Prefs.AutoFeed = false
	p := w.Player()
	p.People = 100000
	p.Food = 10_000_000
	f := &fakeSession{keys: []rune("0\r\r")}
	if _, err := feedStage(f, w, BuildMenus().Food, false); err != nil {
		t.Fatalf("feedStage: %v", err)
	}
	out := f.out.String()
	if !strings.Contains(out, "Chopper") {
		t.Fatalf("the flow never reached the Food Market; got:\n%s", out)
	}
	if strings.Contains(out, "Paused") {
		t.Errorf("hand-paid maintenance must not pause before the market; got:\n%s", out)
	}
}

func TestFeedStageReportsASilentFeed(t *testing.T) {
	w := newWorld()
	w.Player().Prefs.AutoFeed = true
	w.Player().Food = 10_000_000
	f := &fakeSession{}
	silent, err := feedStage(f, w, BuildMenus().Food, true)
	if err != nil {
		t.Fatalf("feedStage: %v", err)
	}
	if !silent {
		t.Error("a covered realm with Auto-Feed on is fed silently")
	}
	if strings.Contains(f.out.String(), "Paused") {
		t.Error("the silent path must not pause before a market it never opens")
	}
}

// The board name belongs in the bulletin title only in a league, where several
// planets file into one feed. A stand-alone board would otherwise read
// "local — Daily Bulletin" (#68).
func TestBulletinNamesTheBoardOnlyInALeague(t *testing.T) {
	show := func(ibbs bool) string {
		f := &fakeSession{keys: []rune(" ")}
		w := newWorld()
		w.Config.IBBS = ibbs
		w.Config.BoardID = "wildside"
		showBulletin(f, w, false)
		return f.out.String()
	}
	if out := show(false); strings.Contains(out, "wildside") {
		t.Errorf("stand-alone board should not name itself in the bulletin:\n%s", out)
	}
	if out := show(true); !strings.Contains(out, "wildside") {
		t.Errorf("league board should name itself in the bulletin:\n%s", out)
	}
}

// TestRunTurnSurfacesOffersDealsAndMail guards the WIRING of the three
// turn-start stops — treaty offers, trade-deal barters, and the mail reader —
// into runTurn. Each is well tested in isolation; without this test, deleting
// any of the three calls in runTurn passes the whole suite while the player
// never sees offers or mail again.
func TestRunTurnSurfacesOffersDealsAndMail(t *testing.T) {
	w := newWorld()
	w.Player().Prefs.AutoPayMaint = true
	rival := recipients(w)[0]
	w.World.ProposeTreaty(rival, w.Player(), "Free Trade Agreement")
	w.Player().TradeDeals = []game.TradeDeal{{From: rival.Name}}
	w.Player().Mail = []game.Message{{From: "Postmaster", To: "A", When: "07/24/2026  09:00:00", Body: "hi"}}
	w.Player().Events = []game.Event{{Text: "A dragon attacked your regions."}}

	// One key per stop — recap pause, decline the offer, decline the deal,
	// quit the mail reader — then filler for however much of the turn runs
	// before the script ends (the session ending mid-turn is fine: the three
	// stops have already rendered by then).
	f := &fakeSession{keys: []rune(" nnq   0000nn")}
	runTurn(f, w)
	out := f.out.String()

	for _, marker := range []string{
		"proposes a",              // reviewTreatyOffers
		"offers you a trade deal", // reviewTradeDeals
		"Postmaster",              // readTurnMail's message box
	} {
		if !strings.Contains(out, marker) {
			t.Errorf("runTurn should surface %q, got:\n%s", marker, out)
		}
	}
}

// The Queen's tax refund is paid once per game day, at the head of the first
// turn of the session, and never twice — playing a second turn must not draw it
// again (#93). The realm here is out of protection, so the figure is the plain
// 2% of the purse rather than the newcomer's capped share.
func TestQueenRefundPaidOncePerDay(t *testing.T) {
	perTurn := "   0000n" // income/status pauses, quit Spending/Attack/Covert/Trading, decline message
	keys := perTurn + "y" + perTurn + "n"
	f := &fakeSession{keys: []rune(keys)}
	w := newWorld()
	w.Player().Prefs.AutoPayMaint = true
	w.Player().Prefs.VisitCovert, w.Player().Prefs.VisitTrading, w.Player().Prefs.VisitMessage = true, true, true
	w.Player().Agents = 1
	w.Player().Protection = 0
	w.RefundPool = 500_000

	runTurn(f, w)
	out := f.out.String()

	if n := strings.Count(out, "refunds you"); n != 1 {
		t.Fatalf("expected the refund exactly once across two turns, got %d\n%s", n, out)
	}
	if !strings.Contains(out, "10,000") {
		t.Errorf("expected 2%% of a 500,000 purse (10,000) in:\n%s", out)
	}
	// The purse lost the 10,000 it paid, then took back the crown tax these two
	// turns handed over — so it lands between the two figures, not on either.
	if w.RefundPool < 490_000 || w.RefundPool >= 500_000 {
		t.Errorf("purse = %d, want 490,000 plus the tax paid since", w.RefundPool)
	}
	if !w.Player().RefundTaken {
		t.Error("the realm should be marked as having drawn today's refund")
	}
}

// The decontamination offer reaches the screen in the maintenance sequence and
// spends what the player gives it. A held-together strike is only half a
// mechanic if the defender is never shown the way out of it.
func TestDecontaminateStageOffersAndSpends(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Regions = game.RegionMix{Coastal: 1000, Waste: 200}
	p.Land = p.Regions.Total()
	p.Gold = 1_000_000_000

	cost := w.World.DecontaminateCost(p)
	before := p.Land
	// Accept the suggested amount, then send the cleaned land to Industrial.
	f := &fakeSession{keys: []rune("\rI40\r")}

	decontaminateStage(f, w)

	out := f.out.String()
	if !strings.Contains(out, "decontaminate") {
		t.Fatalf("expected the decontamination offer; got:\n%s", out)
	}
	if !strings.Contains(out, "clean again") {
		t.Fatalf("cleaned land should go through the type picker; got:\n%s", out)
	}
	p = w.Player()
	if p.Regions.Waste != 160 {
		t.Errorf("waste = %d after cleaning a turn's allowance of 40, want 160", p.Regions.Waste)
	}
	// Cleaned land has no type of its own — it is whatever the player named it.
	if p.Regions.Industrial != 40 {
		t.Errorf("Industrial = %d, want the 40 cleaned regions", p.Regions.Industrial)
	}
	// ...and it is not counted twice: the restore parked it in Coastal, the
	// picker took it back out again.
	if p.Regions.Coastal != 1000 {
		t.Errorf("Coastal = %d, want 1000 — the reclaimed regions were double-counted", p.Regions.Coastal)
	}
	if p.Land != before {
		t.Errorf("land changed across the clean-and-allocate round trip: %d -> %d", before, p.Land)
	}
	if p.Gold != 1_000_000_000-cost {
		t.Errorf("gold = %d, want %d", p.Gold, 1_000_000_000-cost)
	}
}

// A realm with no waste is not asked about it.
func TestDecontaminateStageSilentWithoutWaste(t *testing.T) {
	w := newWorld()
	w.Player().Gold = 1_000_000
	f := &fakeSession{}

	decontaminateStage(f, w)

	if out := f.out.String(); out != "" {
		t.Errorf("expected no output with nothing to clean; got:\n%s", out)
	}
}

// Running out of turns DURING play tells the player so. The loop used to fold
// the check into the continue prompt's condition — `turnsLeft <= 0 || !AskYesNo`
// — so the last turn ended by short-circuiting past both the prompt and any
// message, dropping the player on the opening menu with no word about why.
// Asserts it reached the end of a real turn as well as the message, so a key
// script that runs dry early cannot pass this vacuously.
func TestRunTurnSaysWhenTheLastTurnIsSpent(t *testing.T) {
	f := &fakeSession{keys: []rune(strings.Repeat("\r0n ", 400)), boot: true}
	w := newWorld()
	w.Player().TurnsLeft = 1

	func() {
		defer func() { recover() }() // an exhausted script ends the session by panic
		runTurn(f, w)
	}()

	out := f.out.String()
	if !strings.Contains(out, "End of Turn Statistics") {
		t.Fatalf("never reached the end of a turn, so the message could not be due:\n%s", out)
	}
	if !strings.Contains(out, "used all of your turns today") {
		t.Error("spending the last turn should say so rather than returning silently")
	}
}

// Six unit types used to print six near-identical sentences. They are a list
// under one heading now, and a count of one takes the singular name.
func TestManufacturedUnitsAreListedOnce(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")}
	w := newWorld()
	p := w.Player()
	p.Regions.Industrial = 200
	w.World.Manufacture(p)
	if p.MadeCarriers != 0 {
		p.MadeCarriers = 1 // pin the singular case rather than hoping the split lands on it
	}

	incomeReport(f, w)
	out := stripANSI(f.out.String())
	built := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Industrial Zones built") {
			built++
		}
		if strings.Contains(line, " 0  ") {
			t.Errorf("a unit type that built none should be left out: %q", line)
		}
		if n := len([]rune(line)); n > 80 {
			t.Errorf("line runs to %d columns: %q", n, line)
		}
	}
	if built != 1 {
		t.Errorf("manufacturing opened %d lines, want 1:\n%s", built, out)
	}
	if !strings.Contains(out, "1  Carrier\n") {
		t.Errorf("a count of one should take the singular name:\n%s", out)
	}
	if strings.Contains(out, "1 Carriers") {
		t.Errorf("plural on a count of one:\n%s", out)
	}
}

// The lottery offer reaches the screen with the day's first turn, right after
// the Queen's refund, and settles there: the price leaves gold in hand, the
// letters typed are the ticket, and the day's offer is spent whatever the
// answer. Scripted with six letters of its own so a flow change upstream that
// swallowed a key shows up as a missing draw rather than a quiet pass.
func TestLotteryOfferComesWithTheDaysFirstTurn(t *testing.T) {
	w := newWorld()
	w.World.Config.Lottery = true
	w.Player().Prefs.AutoPayMaint = true
	w.Player().Prefs.VisitCovert, w.Player().Prefs.VisitTrading, w.Player().Prefs.VisitMessage = false, false, false
	p := w.Player()
	p.Protection = 0
	p.Gold = 50_000
	p.Bank = 0
	w.RefundPool = 0 // the refund would otherwise print between the two prompts

	// Accept the ticket, type ABCDEF, then dismiss the turn's screens and quit.
	f := &fakeSession{keys: []rune("yABCDEF   0000n0")}
	runTurn(f, w)
	out := f.out.String()

	if !strings.Contains(out, "Drawn:") {
		t.Fatalf("the draw never reached the screen:\n%s", out)
	}
	if !strings.Contains(stripANSI(out), "letters: ABCDEF") {
		t.Errorf("the typed ticket was not echoed:\n%s", out)
	}
	if !p.LotteryTaken {
		t.Error("the day's offer should be spent once it has been made")
	}
	// 50,000 less the 5,000 ticket, plus whatever the turn earned — the ticket
	// price is the only thing that can have taken gold before the income report.
	if p.Gold >= 50_000 && p.Bank == 0 {
		t.Errorf("gold = %d: the ticket price was never charged", p.Gold)
	}
}

// A realm that cannot cover the price is not offered a ticket at all, and gets
// no second offer later the same day.
func TestNoLotteryOfferWithoutThePrice(t *testing.T) {
	w := newWorld()
	w.World.Config.Lottery = true
	w.Player().Prefs.AutoPayMaint = true
	w.Player().Prefs.VisitCovert, w.Player().Prefs.VisitTrading, w.Player().Prefs.VisitMessage = false, false, false
	p := w.Player()
	p.Protection = 0
	p.Gold = game.LotteryTicketPrice - 1
	w.RefundPool = 0

	f := &fakeSession{keys: []rune("   0000n0")}
	runTurn(f, w)
	out := f.out.String()

	if strings.Contains(out, "Drawn:") {
		t.Errorf("a realm with %d gold was sold a ticket:\n%s", game.LotteryTicketPrice-1, out)
	}
	if !p.LotteryTaken {
		t.Error("the day's offer is spent even when it could not be made")
	}
}

// The BBS Coordinator notice opens an inter-BBS turn, before the recap, and
// says one of three things: that you hold the office, who your vote is for, or
// that you have not voted. Each case asserts the line reached the screen and
// that the turn went on past it.
func TestCoordinatorNoticeOpensAnInterBBSTurn(t *testing.T) {
	perTurn := "   0000n0" // the turn's pauses, then quit each stage and the menu

	t.Run("holds the office", func(t *testing.T) {
		w := leagueCtx(t)
		w.Player().Prefs.AutoPayMaint = true
		w.Player().Prefs.VisitCovert, w.Player().Prefs.VisitTrading, w.Player().Prefs.VisitMessage = false, false, false
		f := &fakeSession{keys: []rune(perTurn)}
		runTurn(f, w)
		out := stripANSI(f.out.String())
		if !strings.Contains(out, "You hold the office of BBS Coordinator.") {
			t.Errorf("the office notice never reached the screen:\n%s", out)
		}
		if w.Player().TurnsLeft != w.Config.TurnsPerDay-1 {
			t.Errorf("the turn did not run past the notice: %d turns left", w.Player().TurnsLeft)
		}
	})

	t.Run("voted for someone else", func(t *testing.T) {
		w := leagueCtx(t)
		w.Player().Prefs.AutoPayMaint = true
		w.Player().Prefs.VisitCovert, w.Player().Prefs.VisitTrading, w.Player().Prefs.VisitMessage = false, false, false
		// An AI baron holds no BBS handle, so the vote needs a second human.
		other := w.AddHuman("rival", "Rivalia")
		w.VoteCoordinator(w.Player(), other.Owner)
		if w.Player().CoordinatorVote != other.Owner {
			t.Fatalf("the vote did not register: %q", w.Player().CoordinatorVote)
		}
		w.Player().Protection = 0 // a fresh realm starts protected; this case is the voter who can act
		f := &fakeSession{keys: []rune(perTurn)}
		runTurn(f, w)
		out := stripANSI(f.out.String())
		if !strings.Contains(out, "Your vote for BBS Coordinator is "+other.Name+".") {
			t.Errorf("the vote notice never reached the screen:\n%s", out)
		}
		if !strings.Contains(out, "System menu") {
			t.Errorf("an unprotected voter should be told where to change it:\n%s", out)
		}
	})

	t.Run("a protected realm is told why it cannot vote", func(t *testing.T) {
		w := leagueCtx(t)
		w.Player().Prefs.AutoPayMaint = true
		w.Player().Prefs.VisitCovert, w.Player().Prefs.VisitTrading, w.Player().Prefs.VisitMessage = false, false, false
		other := w.AddHuman("rival", "Rivalia")
		w.VoteCoordinator(w.Player(), other.Owner)
		w.Player().Protection = 5
		f := &fakeSession{keys: []rune(perTurn)}
		runTurn(f, w)
		out := stripANSI(f.out.String())
		if !strings.Contains(out, "Your vote for BBS Coordinator is Rivalia.") {
			t.Fatalf("the vote notice never reached the screen:\n%s", out)
		}
		// The System menu hides the Coordinator Vote item while protection lasts,
		// so sending a protected realm there (as the original does) is a dead end.
		if !strings.Contains(out, "until your new-realm protection ends, 5 turns from now") {
			t.Errorf("a protected realm is not told why the vote is closed to it:\n%s", out)
		}
		if strings.Contains(out, "You can change it from the System menu") {
			t.Errorf("a protected realm was still sent to a menu with no vote item:\n%s", out)
		}
	})

	t.Run("has not voted", func(t *testing.T) {
		w := leagueCtx(t)
		w.Player().Prefs.AutoPayMaint = true
		w.Player().Prefs.VisitCovert, w.Player().Prefs.VisitTrading, w.Player().Prefs.VisitMessage = false, false, false
		for _, e := range w.Empires {
			e.CoordinatorVote = ""
		}
		f := &fakeSession{keys: []rune(perTurn)}
		runTurn(f, w)
		out := stripANSI(f.out.String())
		// As the original does: the vote line always names somebody, and "no one"
		// is who it names when nobody has been chosen.
		if !strings.Contains(out, "Your vote for BBS Coordinator is no one.") {
			t.Errorf("the no-vote notice never reached the screen:\n%s", out)
		}
	})
}

// Outside a league there is no Coordinator and no notice.
func TestNoCoordinatorNoticeOffLeague(t *testing.T) {
	w := newWorld()
	w.Player().Prefs.AutoPayMaint = true
	w.Player().Prefs.VisitCovert, w.Player().Prefs.VisitTrading, w.Player().Prefs.VisitMessage = false, false, false
	f := &fakeSession{keys: []rune("   0000n0")}
	runTurn(f, w)
	if out := stripANSI(f.out.String()); strings.Contains(out, "BBS Coordinator") {
		t.Errorf("a standalone board should say nothing about a Coordinator:\n%s", out)
	}
}

// A player who has used every turn still gets their recap and their mailbox on
// entering, and is not asked about pending offers. BRE gates process_trade_offer
// and process_diplomatic_proposal on turns remaining (BRE.EXE 0x3842) but runs
// write_data_report and read_local_messages either way (0x385F), reaching
// "Sorry, you have used all of your turns today." only after them (0x3F8D).
func TestOutOfTurnsStillShowsRecapAndMail(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.TurnsLeft = 0
	rival := recipients(w)[0]
	p.TradeDeals = []game.TradeDeal{{From: rival.Name}}
	p.Mail = []game.Message{{From: "Postmaster", To: "A", When: "07/24/2026  09:00:00", Body: "hi"}}
	p.Events = []game.Event{{Text: "A dragon attacked your regions."}}

	// Recap pause, quit the mail reader, then the score-table pause.
	f := &fakeSession{keys: []rune(" q ")}
	runTurn(f, w)
	out := f.out.String()

	for _, marker := range []string{"dragon", "Postmaster", "used all of your turns"} {
		if !strings.Contains(out, marker) {
			t.Errorf("out of turns should still show %q, got:\n%s", marker, out)
		}
	}
	if strings.Contains(out, "offers you a trade deal") {
		t.Errorf("out of turns should not put the offer prompt, got:\n%s", out)
	}
	if len(w.Player().TradeDeals) != 1 {
		t.Errorf("the deal should stay pending for a turn they can play, got %d", len(w.Player().TradeDeals))
	}
}

// A day's worth of recap entries waits for a key rather than scrolling off an
// 80x24 screen (there is no scrollback on a terminal reading a door).
func TestTurnRecapPausesBeforeItRunsOffTheScreen(t *testing.T) {
	w := newWorld()
	p := w.Player()
	for i := range 12 {
		p.Events = append(p.Events, game.Event{Text: fmt.Sprintf("Raiders took %d gold.", (i+1)*11)})
	}

	f := &fakeSession{keys: []rune("   ")}
	showTurnEvents(f, w)

	out := stripANSI(f.out.String())
	if got := strings.Count(out, "Paused"); got < 2 {
		t.Errorf("12 entries should pause mid-recap as well as at the end, got %d pause(s):\n%s", got, out)
	}
	if !strings.Contains(out, "Raiders took 132 gold.") {
		t.Error("the last entry should still be drawn after the pause")
	}
}

// Entries repeating the same text are counted, not listed one by one — a day of
// covert operations resolving together otherwise fills the screen with the same
// sentence.
func TestTurnRecapCountsRepeatedEntries(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Events = []game.Event{
		{Text: "You demoralized Redmark's forces."},
		{Text: "You demoralized Redmark's forces."},
		{Text: "The operation failed and your agent was lost."},
		{Text: "You demoralized Redmark's forces."},
		{Text: "The operation failed and your agent was lost."},
	}

	f := &fakeSession{keys: []rune(" ")}
	showTurnEvents(f, w)

	out := stripANSI(f.out.String())
	if got := strings.Count(out, "You demoralized"); got != 1 {
		t.Errorf("the repeated line should be drawn once, got %d:\n%s", got, out)
	}
	if !strings.Contains(out, "(3 times)") || !strings.Contains(out, "(2 times)") {
		t.Errorf("both repeats should carry their count:\n%s", out)
	}
	if strings.Contains(out, "(1 times)") {
		t.Error("a one-off entry should carry no count")
	}
}
