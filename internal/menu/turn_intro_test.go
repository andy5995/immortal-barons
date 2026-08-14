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
	if !strings.Contains(first.out.String(), "Income Report") {
		t.Error("first pass of a turn should show the income report")
	}

	replay := &fakeSession{}
	showTurnIntro(replay, w, true)
	out := replay.out.String()
	if strings.Contains(out, "Income Report") {
		t.Errorf("a replay should skip the income report, got:\n%s", out)
	}
	// "Popular Support:" is the status block's own line and appears nowhere else
	// in the intro; the title carries the realm name rather than a screen name.
	if !strings.Contains(out, "Popular Support:") {
		t.Errorf("a replay should still show the empire status, got:\n%s", out)
	}
}
