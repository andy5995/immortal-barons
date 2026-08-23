package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A long board name must not widen any screen that shows it. The roster cap sits
// at 512 bytes deliberately — routing compares board names, so the stored value
// cannot be shortened — which makes fitting the column at draw time the only
// defence, on every screen rather than the two that were looked at first.
//
// Each case must REACH the table it tests: interbbsScores opens on a view picker
// and renders no rows until one is chosen, which is how an earlier version of
// this test passed with the fix removed.
func TestLongBoardNameNeverWidensAScreen(t *testing.T) {
	const long = "Wideawake Interplanetary Communications Exchange And Trading Post Number Seven"
	// Travel Times leaves the CALLING board out of its list, so a long name has to
	// exist on a remote planet too or that screen renders nothing to measure.
	const longRemote = "Farflung Orbital Freeport And Regional Clearing House Number Nineteen"
	newCtx := func() *ctx {
		w := newWorld()
		w.With(func() {
			w.Config.IBBS = true
			w.Config.BoardID = long
			w.Config.LeagueNumber = 7
			w.LeagueNodes = []game.LeagueNode{
				{Number: 1, Name: long, City: "Somewhere"},
				{Number: 2, Name: longRemote, City: "Elsewhere"},
				// A short neighbour, so a column that shifts under the long name has
				// something to be out of line WITH.
				{Number: 3, Name: "Zed BBS", City: "Nowhere"},
			}
			w.ImportBoard(game.RemoteBoard{BoardID: longRemote,
				Scores: []game.RemoteScore{{Empire: "Iron Dominion", Land: 120, NetWorth: 5000, Score: 6100}}})
			w.TravelTimes = map[string]float64{longRemote: 0.5, "Zed BBS": 0.5}
		})
		return w
	}

	cases := []struct {
		name   string
		keys   string
		marker string // proves the screen was reached
		// aligned, when set, is a column every row carrying it must start at: a
		// name that overflows its field pushes the next column right without
		// necessarily passing 80, so width alone would miss it.
		aligned string
		run     func(*fakeSession, *ctx)
	}{
		{"IPScores planet view", "1 0", "Top Planets by Score", "",
			func(f *fakeSession, w *ctx) { interbbsScores(f, w) }},
		{"IPScores player view", "5 0", "Top Players by Score", "",
			func(f *fakeSession, w *ctx) { interbbsScores(f, w) }},
		{"Travel Times", " ", "Average Turn Around Times", "hours",
			func(f *fakeSession, w *ctx) { travelTimes(f, w) }},
		{"Game Setup", "  ", "League number", "",
			func(f *fakeSession, w *ctx) { gameSetup(f, w) }},
		{"Daily Bulletin", " ", "Daily Bulletin", "",
			func(f *fakeSession, w *ctx) { showBulletinToday(f, w) }},
		{"List of Planets", " ", "Somewhere", "",
			func(f *fakeSession, w *ctx) { showPlanetList(f, knownPlanets(w)) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newCtx()
			f := &fakeSession{keys: []rune(tc.keys)}
			tc.run(f, w)
			out := stripANSI(f.out.String())
			if !strings.Contains(out, tc.marker) {
				t.Fatalf("never reached the screen (no %q):\n%s", tc.marker, out)
			}
			col := -1
			for _, ln := range strings.Split(out, "\n") {
				if n := len([]rune(strings.TrimRight(ln, " "))); n > 80 {
					t.Errorf("%d-column line from a long board name:\n%q", n, ln)
				}
				if tc.aligned == "" {
					continue
				}
				// Rune index, not byte: the ellipsis a fitted name ends with is one
				// rune and three bytes, so byte offsets would report a phantom shift.
				b := strings.Index(ln, tc.aligned)
				if b < 0 {
					continue
				}
				at := len([]rune(ln[:b]))
				if col < 0 {
					col = at
				} else if at != col {
					t.Errorf("%q starts at column %d on one row and %d on another — the name overflowed its field:\n%q",
						tc.aligned, col, at, ln)
				}
			}
		})
	}
}
