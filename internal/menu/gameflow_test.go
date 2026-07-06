package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
)

func TestAskYesNo(t *testing.T) {
	cases := []struct {
		keys string
		want bool
	}{
		{"y\r", true},
		{"\r", true},
		{"n\r", false},
		{"N\r", false},
	}
	for _, c := range cases {
		f := &fakeSession{keys: []rune(c.keys)}
		got := askYesNo(f, "Continue?")
		if got != c.want {
			t.Errorf("askYesNo(%q) = %v, want %v", c.keys, got, c.want)
		}
	}
}

func TestIncomeReportWritesNonEmpty(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")} // pause keypress
	w := newWorld()
	incomeReport(f, w, w.Player())
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
	endOfTurnStats(f, w, w.Player())
	if f.out.Len() == 0 {
		t.Error("expected endOfTurnStats to write output")
	}
	if !strings.Contains(f.out.String(), "Turns left") {
		t.Error("expected turns-left line")
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

	incomeReport(f, w, p)

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

	paymentStage(f, w, p)

	if p.Gold != before-want {
		t.Errorf("auto-pay should deduct %d, gold %d -> %d", want, before, p.Gold)
	}
	if !strings.Contains(f.out.String(), "Maintenance paid") {
		t.Error("expected the auto-pay confirmation line")
	}
}

func TestPaymentStageManualFullPayNoDesertion(t *testing.T) {
	f := &fakeSession{keys: []rune("\r\r")} // accept suggested (full) for forces, then regions
	w := newWorld()
	w.AutoPayMaint = false
	p := w.Player()
	p.Gold = 100000
	p.Support = 100 // skips the optional support-boost prompt
	beforeTroopers := p.Troopers
	want := p.ForcesUpkeep() + p.RegionUpkeep()
	before := p.Gold

	paymentStage(f, w, p)

	if p.Troopers != beforeTroopers {
		t.Errorf("full pay should not desert troopers, %d -> %d", beforeTroopers, p.Troopers)
	}
	if p.Gold != before-want {
		t.Errorf("manual full pay should deduct %d, gold %d -> %d", want, before, p.Gold)
	}
}

func TestPaymentStageManualUnderpayDeserts(t *testing.T) {
	f := &fakeSession{keys: []rune("0\r0\r")} // give 0 to forces, then 0 to regions
	w := newWorld()
	w.AutoPayMaint = false
	p := w.Player()
	p.Gold = 100000
	p.Support = 100
	beforeTroopers := p.Troopers

	paymentStage(f, w, p)

	if p.Troopers >= beforeTroopers {
		t.Errorf("underpaying forces should desert troopers, %d -> %d", beforeTroopers, p.Troopers)
	}
	if !strings.Contains(f.out.String(), "deserted") {
		t.Error("expected a desertion notice")
	}
}

// TestRunTurnConsumesATurn scripts a full pass through the pipeline for one
// turn: income/status pauses, Quit ('0') out of Spending, Attack, Covert,
// and Trading, decline the message prompt, the end-of-turn pause, then
// decline "continue" to stop after one turn.
func TestRunTurnConsumesATurn(t *testing.T) {
	keys := "  0000n\r n\r"
	f := &fakeSession{keys: []rune(keys)}
	w := newWorld()
	w.AutoPayMaint = true // pay maintenance silently; this test is about the turn loop
	left := w.Player().TurnsLeft
	runTurn(f, w)
	if w.Player().TurnsLeft != left-1 {
		t.Errorf("expected TurnsLeft %d, got %d", left-1, w.Player().TurnsLeft)
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

func TestWriteStatTableDarkZebraColors(t *testing.T) {
	f := &fakeSession{}
	cols := []statCol{{name: "Money", val: 1234}, {name: "Regions", val: 5}}
	writeStatTable(f, "Overview", cols, 0, uniformWidth(cols))
	out := f.out.String()

	if !strings.Contains(out, ansi.BgHeader+ansi.FgBrightWhite) {
		t.Error("expected the header row to use BgHeader + FgBrightWhite")
	}
	if !strings.Contains(out, ansi.BgRow+ansi.FgBrightWhite) {
		t.Error("expected the value row to use BgRow + FgBrightWhite")
	}
	if strings.Contains(out, ansi.FgBlack) {
		t.Error("expected no black foreground text in a dark-scheme table")
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

func TestShowBulletinEmptyNewsShowsNoBulletinsNote(t *testing.T) {
	w := newWorld()
	w.NewsToday = nil
	f := &fakeSession{keys: []rune(" ")}
	showBulletinToday(f, w)
	if !strings.Contains(f.out.String(), "No planetary bulletins.") {
		t.Error("expected the empty-state note when there is no news")
	}
}
