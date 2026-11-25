package menu

import (
	"strings"
	"testing"
)

// Before a scheduled start, Play is refused with the date and the caller is
// returned to the menu: the turn is not spent and the session does not end.
func TestPlayRefusedBeforeScheduledStart(t *testing.T) {
	w := newWorld()
	w.Config.GameStartDate = "2026-07-10" // newWorld's today is 2026-07-03
	w.Player().TurnsLeft = 5
	f := &fakeSession{keys: []rune{'1', ' ', '0'}} // Play, dismiss the pause, Quit
	if err := Run(f, w, BuildMenus().Game); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if !strings.Contains(out, "The game has not started yet; it begins 2026-07-10.") {
		t.Fatalf("refusal not shown; output:\n%s", out)
	}
	if got := w.Player().TurnsLeft; got != 5 {
		t.Errorf("TurnsLeft = %d, want 5: a refused Play must not spend turns", got)
	}
	if f.pos != len(f.keys) {
		t.Errorf("%d keys unread: the session ended before the final Quit was reached", len(f.keys)-f.pos)
	}
}

// From the start date the same Play is not refused.
func TestPlayAllowedOnStartDate(t *testing.T) {
	w := newWorld()
	w.Config.GameStartDate = "2026-07-03"
	if got := gameNotStarted(w); got != "" {
		t.Errorf("gameNotStarted = %q on the start date, want empty", got)
	}
	w.Config.GameStartDate = ""
	if got := gameNotStarted(w); got != "" {
		t.Errorf("gameNotStarted = %q with no start date, want empty", got)
	}
}
