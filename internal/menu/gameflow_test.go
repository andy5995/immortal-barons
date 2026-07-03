package menu

import (
	"strings"
	"testing"
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
	incomeReport(f, w, w.Active)
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
	endOfTurnStats(f, w, w.Active)
	if f.out.Len() == 0 {
		t.Error("expected endOfTurnStats to write output")
	}
	if !strings.Contains(f.out.String(), "Turns left") {
		t.Error("expected turns-left line")
	}
}

func TestRunTurnNoTurnsLeftReturnsImmediately(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")} // pause keypress inside ok()
	w := newWorld()
	w.Active.TurnsLeft = 0
	runTurn(f, w)
	if !strings.Contains(f.out.String(), "no turns left") {
		t.Error("expected the no-turns-left message")
	}
}

// TestRunTurnConsumesATurn scripts a full pass through the pipeline for one
// turn: income/status pauses, Return out of Spending and Covert/Trading,
// Attack's Return key (X), decline the message prompt, the end-of-turn
// pause, then decline "continue" to stop after one turn.
func TestRunTurnConsumesATurn(t *testing.T) {
	keys := "  R\rX\rR\rR\rn\r n\r"
	f := &fakeSession{keys: []rune(keys)}
	w := newWorld()
	left := w.Active.TurnsLeft
	runTurn(f, w)
	if w.Active.TurnsLeft != left-1 {
		t.Errorf("expected TurnsLeft %d, got %d", left-1, w.Active.TurnsLeft)
	}
}
