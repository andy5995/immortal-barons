package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// The captured geometry (docs/dev/bre-screens.md, "InterBBS Scores"): every
// view right-aligns its metric to column 46, and the player views then put
// Planet LAST, beginning on column 52. IB had Planet second and the metric last
// until this was fixed (#196).
const (
	ipMetricRightEdge = 46
	ipPlanetColumn    = 52
)

// A name too long for its field is cut with a marker, and the marker's width is
// part of the column arithmetic. "…" is one column only while the caller reads
// UTF-8; the CP437 and plain-ASCII writers rewrite it as three dots below every
// layer that counts columns, so a fitted cell used to come out two columns wide
// and shove the Planet column right (#196) — for the default charset, at that.
// Both halves are checked here, and the ASCII case goes through the real writer
// rather than trusting the marker choice alone.
func TestIPScoresPlayerTableGeometry(t *testing.T) {
	// Both fixed-width cells need an over-long value: the Planet column is last,
	// so only a fitted NAME can shove the metric and Planet right.
	const longBoard = "ConstructiveChaos BBS and Interplanetary Trading Post"
	const longEmpire = "The Serene Republic of Mango Salsa"
	newCtx := func(utf8 bool) *ctx {
		w := newWorld()
		w.Term = Term{UTF8: utf8, ASCII: !utf8}
		w.With(func() {
			w.Config.IBBS = true
			w.Config.BoardID = "The X-Bit BBS"
			w.ImportBoard(game.RemoteBoard{BoardID: longBoard,
				Scores: []game.RemoteScore{{Empire: longEmpire, Land: 120, NetWorth: 5000, Score: 1491}}})
			w.ImportBoard(game.RemoteBoard{BoardID: "The uniX-Bit BBS",
				Scores: []game.RemoteScore{{Empire: "Charging Penguins", Land: 20, NetWorth: 500, Score: 1491}}})
		})
		return w
	}
	views := []struct {
		key    rune
		marker string // proves the view was reached
	}{
		{'5', "Top Players by Score"},
		{'6', "Top Players by Net Worth"},
		{'7', "Top Players by Land"},
		{'8', "Top Players by Net Worth Density"},
	}
	for _, charset := range []struct {
		name string
		utf8 bool
	}{{"UTF-8", true}, {"ASCII", false}} {
		for _, v := range views {
			t.Run(charset.name+"/"+v.marker, func(t *testing.T) {
				w := newCtx(charset.utf8)
				f := &fakeSession{keys: []rune{v.key, ' ', '0'}}
				var s session.Session = f
				if !charset.utf8 {
					s = session.NewASCIIWriter(f)
				}
				interbbsScores(s, w)
				out := stripANSI(f.out.String())
				if !strings.Contains(out, v.marker) {
					t.Fatalf("never reached the view (no %q):\n%s", v.marker, out)
				}
				rows := 0
				for _, ln := range strings.Split(out, "\n") {
					ln = strings.TrimRight(ln, " \r")
					r := []rune(ln)
					// The header and the data rows share one shape; pick them out by
					// the "(  n) " row marker and the leading "      Name".
					// A ranked row is "(  n) ": the view picker's own "(1) " items are
					// narrower and must not be measured as table rows.
					isRow := len(r) > 6 && r[0] == '(' && r[4] == ')' && r[5] == ' '
					if !isRow && !strings.HasPrefix(ln, "      Name") {
						continue
					}
					if isRow {
						rows++
					}
					// The metric ends on column 46 — the character there is not a space,
					// and the one after it is.
					if len(r) < ipPlanetColumn {
						t.Fatalf("row is only %d columns, too short to carry a Planet column:\n%q", len(r), ln)
					}
					if r[ipMetricRightEdge-1] == ' ' {
						t.Errorf("metric does not reach column %d:\n%q", ipMetricRightEdge, ln)
					}
					if r[ipMetricRightEdge] != ' ' {
						t.Errorf("metric runs past column %d:\n%q", ipMetricRightEdge, ln)
					}
					if r[ipPlanetColumn-1] == ' ' {
						t.Errorf("Planet column does not start on column %d:\n%q", ipPlanetColumn, ln)
					}
					if n := len(r); n > 80 {
						t.Errorf("%d-column line:\n%q", n, ln)
					}
				}
				if rows != 3 {
					t.Fatalf("expected 3 ranked rows, got %d:\n%s", rows, out)
				}
				// The whole point of the long board name: it must have been cut.
				if strings.Contains(out, longBoard) || strings.Contains(out, longEmpire) {
					t.Errorf("an over-long name was never fitted to its column:\n%s", out)
				}
			})
		}
	}
}

// The planet views share the player views' right edge, which is what makes the
// two table shapes agree under one 72-column rule.
func TestIPScoresPlanetTableGeometry(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.Config.IBBS = true
		w.Config.BoardID = "The X-Bit BBS"
		w.ImportBoard(game.RemoteBoard{BoardID: "The uniX-Bit BBS",
			Scores: []game.RemoteScore{{Empire: "Charging Penguins", Land: 20, NetWorth: 500, Score: 1491}}})
	})
	f := &fakeSession{keys: []rune{'4', ' ', '0'}}
	interbbsScores(f, w)
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Top Planets by Net Worth Density") {
		t.Fatalf("never reached the view:\n%s", out)
	}
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimRight(ln, " \r")
		if !strings.HasPrefix(ln, "(  ") && !strings.HasPrefix(ln, "      Name") {
			continue
		}
		if n := len([]rune(ln)); n != ipMetricRightEdge {
			t.Errorf("planet row is %d columns, want %d:\n%q", n, ipMetricRightEdge, ln)
		}
	}
}
