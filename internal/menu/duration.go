package menu

import (
	"fmt"
	"time"
)

// duration.go — how IB spells a short wait in a table cell (#269). One spelling,
// because a player reading a force leaving here and a force arriving here is
// reading the same kind of figure and must not have to learn two forms (#268
// wants it for the incoming views).
//
// The form is one figure and one letter, at most four columns, so it fits where
// the original's whole-hours figure sat without widening the column around it.
// It steps down as the wait shortens — hours, then minutes, then seconds —
// because the decision it feeds changes as it does: with three hours left the
// question is whether to commit forces at all, and with ninety seconds left it is
// whether there is time to reach the menu.

// shortDurationMinutes is where the form steps from hours to minutes. Under 100
// minutes a whole-hours figure is at its coarsest — "1h" spans anything from a
// second to an hour — and three digits and a letter still fit the cell.
const shortDurationMinutes = 100

// shortDuration spells d for a table cell. It rounds UP, so a wait never reads
// as less than it is: an attack filed with an eight-hour delay reads "8h" for
// its first hour rather than dropping to "7h" the moment it is created.
func shortDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "0s"
	case d > shortDurationMinutes*time.Minute:
		return fmt.Sprintf("%dh", int((d+time.Hour-1)/time.Hour))
	case d > time.Minute:
		return fmt.Sprintf("%dm", int((d+time.Minute-1)/time.Minute))
	}
	return fmt.Sprintf("%ds", int((d+time.Second-1)/time.Second))
}
