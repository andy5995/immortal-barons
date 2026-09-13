package menu

import (
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// travelWorld sets up a league board with three peers: one measured recently,
// one whose link stopped days ago, and one saved before the arrival stamp was
// kept.
func travelWorld(t *testing.T) *ctx {
	t.Helper()
	w := newWorld()
	w.With(func() {
		w.World.Config.IBBS = true
		w.World.Config.BoardID = "Alpha"
		for _, b := range []string{"Fresh Board", "Nite Eyes", "Legacy Board"} {
			w.World.ImportBoard(game.RemoteBoard{BoardID: b})
		}
		w.World.TravelTimes = map[string]float64{
			"Fresh Board":  2.0 / (24 * 60),
			"Nite Eyes":    40.0 / (24 * 60),
			"Legacy Board": 40.0 / (24 * 60),
		}
		w.World.TravelSeen = map[string]string{
			"Fresh Board": time.Now().Add(-20 * time.Minute).Format(time.RFC3339),
			"Nite Eyes":   time.Now().Add(-73 * time.Hour).Format(time.RFC3339),
		}
	})
	return w
}

// The screen's whole job is telling a fast link from a dead one, and the
// average cannot: it stays at the last good figure when packets stop moving, so
// a board three days silent reads exactly like one answering in seconds. The
// age has to be on the row: a board showed 40 minutes through a three-day
// outage. Asserts the row was REACHED, not merely that output appeared.
func TestTravelTimesMarkAFigureNothingHasRefreshed(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")}
	travelTimes(f, travelWorld(t))
	out := stripANSI(f.out.String())

	var stale, fresh, legacy string
	for _, ln := range strings.Split(out, "\n") {
		switch {
		case strings.Contains(ln, "Nite Eyes"):
			stale = ln
		case strings.Contains(ln, "Fresh Board"):
			fresh = ln
		case strings.Contains(ln, "Legacy Board"):
			legacy = ln
		}
	}
	if stale == "" || fresh == "" || legacy == "" {
		t.Fatalf("the screen did not list all three boards:\n%s", out)
	}
	if !strings.Contains(stale, "40 minutes") {
		t.Errorf("the stale row lost its figure — the age replaces nothing: %q", stale)
	}
	if !strings.Contains(stale, "3 days old") {
		t.Errorf("a three-day-old measurement was not marked: %q", stale)
	}
	if strings.Contains(fresh, "old") {
		t.Errorf("a measurement from twenty minutes ago was marked stale: %q", fresh)
	}
	// Unknown vintage says nothing rather than guessing either way.
	if strings.Contains(legacy, "old") {
		t.Errorf("a board with no arrival stamp was called stale: %q", legacy)
	}
}

// The note is words, not a color: a reader on a monochrome terminal, or one who
// cannot tell this yellow from the green beside it, gets the same information.
// It also has to stay inside the 80-column door screen.
func TestTravelTimesStaleNoteSurvivesWithoutColor(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")}
	travelTimes(f, travelWorld(t))
	for _, ln := range strings.Split(stripANSI(f.out.String()), "\n") {
		ln = strings.TrimRight(ln, " \r")
		if n := len([]rune(ln)); n > 80 {
			t.Errorf("row of %d columns: %q", n, ln)
		}
	}
}
