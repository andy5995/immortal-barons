package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/session"
)

// scoreRowWidth is what one rendered row of the score table occupies: the id
// cell, the name field, and the three right-aligned figure fields with a space
// before each. Every row is the same width, so a row that measures differently
// has moved the figures beside it.
const scoreRowWidth = scoreIDCellWidth + scoreNameWidth + 1 + 10 + 1 + 11 + 1 + 11

// ValidRealmName accepts any printable rune, so a realm may legally be named
// "Iron—Fist Horde". The CP437 and plain-ASCII writers rewrite that em dash as
// two hyphens BELOW every layer that counts columns, so a name field measured in
// runes came out one column wide for the caller and walked the figures right —
// on the default charset, which is where it would be seen. Same defect as #192
// and #196.
//
// The ASCII case goes through the real writer, so the substitution has happened
// on the wire before anything is measured.
func TestScoreRowsHoldTheirColumnsWithAnExpandedName(t *testing.T) {
	const emDashName = "Iron—Fist Horde"
	// Long enough to be clipped, and carrying the em dash inside the clip.
	const longEmDashName = "Iron—Fist Horde of the Everlasting Ash—Wastes"

	for _, charset := range []struct {
		name string
		utf8 bool
	}{{"UTF-8", true}, {"ASCII", false}} {
		t.Run(charset.name, func(t *testing.T) {
			w, shielded, other := protectionWorld(t)
			w.Term = Term{UTF8: charset.utf8, ASCII: !charset.utf8}
			// One flagged and one not: the (P) flag rides in the same field, so
			// both states have to leave the figures where they are.
			shielded.Name = emDashName
			other.Name = longEmDashName

			f := &fakeSession{}
			var s session.Session = f
			if !charset.utf8 {
				s = session.NewASCIIWriter(f)
			}
			printScores(s, w)
			out := stripANSI(f.out.String())
			if !strings.Contains(out, "Net Worth") {
				t.Fatalf("never reached the score table:\n%s", out)
			}
			seen := 0
			for _, ln := range strings.Split(out, "\n") {
				ln = strings.TrimRight(ln, "\r")
				// A data row is the only line that opens with a bracketed id.
				r := []rune(ln)
				if len(r) < 3 || r[0] != '[' || r[2] != ']' {
					continue
				}
				seen++
				if len(r) != scoreRowWidth {
					t.Errorf("row is %d columns, want %d — the figures moved:\n%q",
						len(r), scoreRowWidth, ln)
				}
			}
			if seen < 3 {
				t.Fatalf("measured %d rows, want the player and both renamed rivals:\n%s", seen, out)
			}
			if !charset.utf8 && strings.Contains(out, "—") {
				t.Errorf("an em dash reached an ASCII session:\n%s", out)
			}
		})
	}
}
