package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// terms are the three charsets a caller can arrive on. A cell has to come out
// the same width on all of them, and each measures a rune differently.
var terms = map[string]Term{
	"utf8":  {UTF8: true},
	"cp437": {},
	"ascii": {ASCII: true},
}

// TestPadColumnHoldsWidth is the contract a table cell needs and a "%-*s" verb
// does not give: exactly width columns, whatever the name and whatever the
// charset. fmt counts runes and never truncates, so a long realm name overran
// its column and a name carrying an expanding character under-padded it.
func TestPadColumnHoldsWidth(t *testing.T) {
	names := []string{
		"Asgard",
		strings.Repeat("A", game.RealmNameMaxChars), // the longest name onboarding allows
		"Iron—Fist", // em dash: two columns on CP437, one on UTF-8
		"±Tatooine", // one column on CP437, three under -ascii
		"Grüße",
	}
	for label, term := range terms {
		for _, name := range names {
			for _, width := range []int{8, 16, 21, 40} {
				got := padColumn(term, name, width)
				if n := visWidth(term, got); n != width {
					t.Errorf("%s: padColumn(%q, %d) is %d columns:\n%q", label, name, width, n, got)
				}
			}
		}
	}
}

// TestPadColumnMatchesTheWire checks visWidth against what the CP437 encoder
// really puts on the wire, so the cell is measured in the columns the caller
// sees rather than in a metric of our own.
func TestPadColumnMatchesTheWire(t *testing.T) {
	for _, name := range []string{"Asgard", "Iron—Fist", "±Tatooine", "Grüße", "Ünderdog"} {
		out := &cp437Buf{}
		s := session.NewCP437Writer(out)
		if _, err := s.Write([]byte(padColumn(Term{}, name, 20))); err != nil {
			t.Fatal(err)
		}
		if n := out.buf.Len(); n != 20 {
			t.Errorf("%q encodes to %d CP437 bytes, want 20: %q", name, n, out.buf.String())
		}
	}
}

// TestAllyRowHoldsItsColumns covers the Alliance Strength table, where the name
// cell is narrower than a realm name may be: a 30-character ally used to push
// Troopers, Tanks and Agents right off their headings. The width is asserted
// from the column constants rather than by drawing the heading a second time.
func TestAllyRowHoldsItsColumns(t *testing.T) {
	want := allyNameWidth + 3*allyColumnWidth
	for label, term := range terms {
		for _, name := range []string{"Asgard", strings.Repeat("A", game.RealmNameMaxChars), "Iron—Fist"} {
			f := &fakeSession{}
			allyRow(f, term, name, 1000, 2000, 3000)
			line := plainLines(f.out.String())[0]
			if n := visWidth(term, line); n != want {
				t.Errorf("%s: ally row for %q is %d columns, want %d:\n%q", label, name, n, want, line)
			}
		}
	}
}
