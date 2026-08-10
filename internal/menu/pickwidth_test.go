package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
)

// TestPickerFitsScreen renders the treaty picker's roster with the longest
// treaty names in the Relations column and asserts every line fits the screen.
// The Relations column is IB's addition to BRE's table, so it is the one that
// can push a row past 80 columns.
func TestPickerFitsScreen(t *testing.T) {
	w := newWorld()
	p := w.Player()
	for i, e := range recipients(w) {
		e.Name = strings.Repeat("W", 26) // widest name the column allows
		if i == 0 {
			w.World.ProposeTreaty(e, p, "Full Defense Alliance")
			w.World.AcceptTreaty(p, e.Name, "Full Defense Alliance")
			w.World.ProposeTreaty(e, p, "Tariff Trade Agreement")
			w.World.AcceptTreaty(p, e.Name, "Tariff Trade Agreement")
		}
	}
	f := &fakeSession{keys: []rune("?0")}
	negotiateTreaty("Free Trade Agreement")(f, w)
	for _, l := range strings.Split(sgr.ReplaceAllString(f.out.String(), ""), "\n") {
		if n := len([]rune(strings.TrimRight(l, " "))); n >= ansi.ScreenCols {
			t.Errorf("line is %d columns, over the %d-column screen:\n%s", n, ansi.ScreenCols, l)
		}
	}
}
