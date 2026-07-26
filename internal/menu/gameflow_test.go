package menu

import (
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
	if !strings.Contains(f.out.String(), "Income Report") {
		t.Error("expected income report heading")
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
		"Troopers were trained by Industrial Zones",
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
		"Troopers were trained by Industrial Zones",
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
	p.PirateRaids = []string{"The Sharks pirates raided you, carrying off 40 troopers, 0 jets, and 5 tanks!"}

	incomeReport(f, w)

	if !strings.Contains(f.out.String(), "Sharks pirates raided you") {
		t.Error("income report should show the pirate raid notice")
	}
	if len(p.PirateRaids) != 0 {
		t.Error("income report should clear the raid notice after showing it")
	}
}

func TestPaymentStageAutoPays(t *testing.T) {
	f := &fakeSession{}
	w := newWorld()
	w.AutoPayMaint = true
	p := w.Player()
	p.Gold = 100000
	before := p.Gold
	want := p.ForcesUpkeep() + p.RegionUpkeep()

	paymentStage(f, w, BuildMenus().Bank)

	if p.Gold != before-want {
		t.Errorf("auto-pay should deduct %d, gold %d -> %d", want, before, p.Gold)
	}
	if !strings.Contains(f.out.String(), "Maintenance paid") {
		t.Error("expected the auto-pay confirmation line")
	}
}

func TestPaymentStageManualFullPayNoDesertion(t *testing.T) {
	f := &fakeSession{keys: []rune("n\r\r")} // decline bank, accept suggested (full) for forces then regions
	w := newWorld()
	w.AutoPayMaint = false
	p := w.Player()
	p.Gold = 100000
	p.Support = 100 // skips the optional support-boost prompt
	beforeTroopers := p.Troopers
	want := p.ForcesUpkeep() + p.RegionUpkeep()
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
	f := &fakeSession{keys: []rune("n0\r0\r")} // decline bank, give 0 to forces then 0 to regions
	w := newWorld()
	w.AutoPayMaint = false
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
// turn: Quit ('0') out of the pre-turn Diplomacy stop, decline Change
// Production, income/status pauses, Quit ('0') out of Spending, Attack,
// Covert, and Trading, decline the message prompt, then decline "continue"
// to stop after one turn.
func TestRunTurnConsumesATurn(t *testing.T) {
	keys := "0\r   0000nn"
	f := &fakeSession{keys: []rune(keys)}
	w := newWorld()
	w.AutoPayMaint = true // pay maintenance silently; this test is about the turn loop
	w.Player().Agents = 1 // hold an agent so the Covert stage runs (it now gates on that)
	left := w.Player().TurnsLeft
	runTurn(f, w)
	if w.Player().TurnsLeft != left-1 {
		t.Errorf("expected TurnsLeft %d, got %d", left-1, w.Player().TurnsLeft)
	}
}

// TestRunTurnCovertGatedBeforeSpending checks the Covert stage's placement and
// gate: with an agent (and VisitCovert on) it runs right before Spending; with
// no agent it is skipped entirely — a fresh realm starts with zero agents.
func TestRunTurnCovertGatedBeforeSpending(t *testing.T) {
	// With an agent, Covert Operations appears before the Spending Menu.
	f := &fakeSession{keys: []rune("0\r   0000nn")}
	w := newWorld()
	w.AutoPayMaint = true
	w.Player().Agents = 1
	runTurn(f, w)
	out := f.out.String()
	cov, spend := strings.Index(out, "Covert Operations"), strings.Index(out, "Spending Menu")
	if cov == -1 || spend == -1 {
		t.Fatalf("expected both the Covert and Spending menus; got:\n%s", out)
	}
	if cov > spend {
		t.Errorf("Covert should come before Spending (offsets cov=%d spend=%d)", cov, spend)
	}

	// With no agent, the Covert stage is skipped (one fewer menu to quit).
	f2 := &fakeSession{keys: []rune("0\r   000nn")}
	w2 := newWorld()
	w2.AutoPayMaint = true
	w2.Player().Agents = 0
	runTurn(f2, w2)
	out2 := f2.out.String()
	if strings.Contains(out2, "Covert Operations") {
		t.Errorf("Covert should be skipped with no agents; got:\n%s", out2)
	}
	if !strings.Contains(out2, "Spending Menu") {
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
	w.AutoPayMaint = true
	w.Player().Agents = 1 // hold an agent so the Covert stage runs (it now gates on that)
	w.Player().Events = []string{"A dragon attacked your regions."}

	runTurn(f, w)
	out := f.out.String()

	eventsAt := strings.Index(out, "Since your last play, this has happened:")
	incomeAt := strings.Index(out, "Income Report")
	if eventsAt == -1 || incomeAt == -1 {
		t.Fatalf("expected the event log and Income Report, got:\n%s", out)
	}
	if eventsAt > incomeAt {
		t.Errorf("event log should precede the Income Report, got offsets %d, %d", eventsAt, incomeAt)
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
	keys := perTurn + "y" + perTurn + "n"
	f := &fakeSession{keys: []rune(keys)}
	w := newWorld()
	w.AutoPayMaint = true
	w.Player().Agents = 1 // hold an agent so the Covert stage runs (it now gates on that)
	left := w.Player().TurnsLeft

	runTurn(f, w)
	out := f.out.String()

	if got := w.Player().TurnsLeft; got != left-2 {
		t.Fatalf("expected two turns consumed, TurnsLeft %d -> %d", left, got)
	}
	if strings.Contains(out, ansi.FgBrightWhite+"Diplomacy") {
		t.Errorf("Diplomacy should not appear in the Play flow:\n%s", out)
	}
	if n := strings.Count(out, "Income Report"); n != 2 {
		t.Errorf("expected Income Report once per turn (2), got %d\n%s", n, out)
	}
}

func TestRenderDailyBulletinRowsSignsAndColors(t *testing.T) {
	b := game.DailyBulletin{
		Totals: game.PlanetTotals{Population: 1865289, Regions: 53266, NetWorth: 34833000},
		Change: game.PlanetTotals{Population: -5838, Regions: 0, NetWorth: 1373000},
	}
	f := &fakeSession{}
	renderDailyBulletin(f, b, "wildside")
	out := f.out.String()

	for _, want := range []string{
		"wildside — Daily Bulletin",
		"Total Population", "1,865,289", "-5,838",
		"Total Regions", "53,266", "+0",
		"Total Net Worth", "34,833k", "+1,373k",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	// Population change is negative -> red; net worth change is positive -> green;
	// regions change is zero -> neutral white.
	if !strings.Contains(out, ansi.FgRed+"-5,838") {
		t.Error("expected the negative population change to be red")
	}
	if !strings.Contains(out, ansi.FgGreen+"+1,373k") {
		t.Error("expected the positive net-worth change to be green")
	}
	if !strings.Contains(out, ansi.FgWhite+"+0") {
		t.Error("expected the zero regions change to be neutral white")
	}
}

func TestRenderDailyBulletinNoTitle(t *testing.T) {
	f := &fakeSession{}
	renderDailyBulletin(f, game.DailyBulletin{}, "")
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

	fToday := &fakeSession{keys: []rune(" ")}
	showBulletinToday(fToday, w)
	todayOut := fToday.out.String()
	if !strings.Contains(todayOut, "today-line") || strings.Contains(todayOut, "yesterday-line") {
		t.Errorf("showBulletinToday should show only today's news, got:\n%s", todayOut)
	}
	if !strings.Contains(todayOut, "111") {
		t.Error("showBulletinToday should render today's totals")
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
	// The Daily Bulletin is now drawn as a box, not a solid bar.
	if !strings.Contains(out, "╔") || !strings.Contains(out, "║") || !strings.Contains(out, "╚") {
		t.Errorf("expected a boxed Daily Bulletin (╔ ║ ╚), got:\n%s", out)
	}
	// Each news item is led by the red arrow and the items are blank-line
	// separated, so the arrow appears once per item.
	if n := strings.Count(out, newsItemArrow); n != 2 {
		t.Errorf("expected 2 news-item arrows (one per item), got %d:\n%s", n, out)
	}
}

func TestNewsItemHighlights(t *testing.T) {
	w := newWorld()
	self := w.Player().Name
	w.NewsToday = []string{
		self + " routed the " + game.PirateFactions[0] + " and took 1,375 regions; now Planetary Master!",
	}
	f := &fakeSession{keys: []rune(" ")}
	showBulletinToday(f, w)
	out := f.out.String()

	for what, want := range map[string]string{
		"own empire bright-yellow": ansi.FgBrightYellow + self,
		"faction bright-cyan":      ansi.FgBrightCyan + game.PirateFactions[0],
		"title bright-white":       ansi.FgBrightWhite + "Planetary Master",
		"number bright-yellow":     ansi.FgBrightYellow + "1,375",
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
	w.AutoFeed = true
	p := w.Player()
	p.Food = 0
	p.People = 100000                       // ensures FoodUpkeep > 0
	f := &fakeSession{keys: []rune("0\rn")} // quit market, give default (0), decline reconsider
	if err := feedStage(f, w, BuildMenus().Food); err != nil {
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

func TestFeedStageNoNoticeWhenFed(t *testing.T) {
	w := newWorld()
	w.AutoFeed = true
	w.Player().Food = 10_000_000 // far more than needed
	f := &fakeSession{}
	if err := feedStage(f, w, BuildMenus().Food); err != nil {
		t.Fatalf("feedStage: %v", err)
	}
	if f.out.Len() != 0 {
		t.Errorf("a fed realm should get no feed notice; got:\n%s", f.out.String())
	}
}

func TestFeedStageWarnsButNoMarketWhenAutoFeedOff(t *testing.T) {
	w := newWorld()
	w.AutoFeed = false
	p := w.Player()
	p.Food = 0
	p.People = 100000
	f := &fakeSession{}
	if err := feedStage(f, w, BuildMenus().Food); err != nil {
		t.Fatalf("feedStage: %v", err)
	}
	out := f.out.String()
	if !strings.Contains(out, "need") {
		t.Errorf("expected a warning; got:\n%s", out)
	}
	if strings.Contains(out, "Chopper") {
		t.Errorf("Auto-Feed off must NOT open the market; got:\n%s", out)
	}
}
