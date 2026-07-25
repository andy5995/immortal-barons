package menu

import (
	"strings"
	"testing"
)

// BRE's Spending menu footer counts the turns remaining AFTER the current one,
// so the last turn of the day reads "0 turns" (the Empire Status screen still
// shows 1). See tree.go spendingStatus.
func TestSpendingFooterShowsTurnsAfterCurrent(t *testing.T) {
	w := newWorld()
	p := w.Player()

	p.TurnsLeft = 1 // last turn of the day
	if got := spendingStatus(w); !strings.Contains(got, "0 turns") {
		t.Errorf("last turn should read 0 turns in the spending footer, got %q", got)
	}

	p.TurnsLeft = 5
	if got := spendingStatus(w); !strings.Contains(got, "4 turns") {
		t.Errorf("5 turns left should read 4 in the spending footer, got %q", got)
	}
}
