package menu

import (
	"strings"
	"testing"
)

// On the first pass of a turn the player sees the income report and the status
// screen. On a REPLAY after an idle-boot the income report (this turn's
// production, already collected) is skipped, but the status screen is shown
// unconditionally so the player sees their current state before landing at the
// first unfinished stage (#10).
func TestTurnIntroSkipsIncomeButShowsStatusOnReplay(t *testing.T) {
	w := newWorld()

	first := &fakeSession{keys: []rune("   ")} // keys to satisfy the intro pauses
	showTurnIntro(first, w, false)
	if !strings.Contains(first.out.String(), "gold was earned in taxes.") {
		t.Error("first pass of a turn should show the income report")
	}

	replay := &fakeSession{}
	showTurnIntro(replay, w, true)
	out := replay.out.String()
	if strings.Contains(out, "gold was earned in taxes.") {
		t.Errorf("a replay should skip the income report, got:\n%s", out)
	}
	// "Popular Support:" is the status block's own line and appears nowhere else
	// in the intro; the title carries the realm name rather than a screen name.
	if !strings.Contains(out, "Popular Support:") {
		t.Errorf("a replay should still show the empire status, got:\n%s", out)
	}
}

// The bank's two returns close the income report: the interest credited at the
// end of the last turn, then the day's matured investments (#216). BRE prints
// the investment line in that position (cap/eots-ibbs-01.cap); the interest line
// is IB's own addition, above it.
func TestIncomeReportShowsBankReturns(t *testing.T) {
	w := newWorld()
	p := w.World.FindByOwner("tester")
	p.LastInterest = 1_234_567
	p.InvestReturnsToday = 7_654_321

	s := &fakeSession{keys: []rune("   ")}
	showTurnIntro(s, w, false)
	out := s.out.String()

	if !strings.Contains(out, "gold was earned in taxes.") {
		t.Fatal("the income report was never reached")
	}
	interest := strings.Index(out, "1,234,567")
	invested := strings.Index(out, "7,654,321")
	if interest < 0 {
		t.Error("the interest earned last turn is not reported")
	}
	if invested < 0 {
		t.Error("the day's investment returns are not reported")
	}
	if interest > 0 && invested > 0 && interest > invested {
		t.Error("the interest line belongs above the investment returns line")
	}
}

// Nothing earned, nothing said: a realm with no bank balance and no matured
// investment gets neither line, the way every other zero line is left out.
func TestIncomeReportOmitsBankReturnsWhenZero(t *testing.T) {
	w := newWorld()

	s := &fakeSession{keys: []rune("   ")}
	showTurnIntro(s, w, false)
	out := s.out.String()

	for _, unwanted := range []string{"bank interest", "investment returns"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("report mentions %q with nothing earned:\n%s", unwanted, out)
		}
	}
}
