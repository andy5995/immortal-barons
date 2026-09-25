package menu

import (
	"strings"
	"testing"
)

// The SDI takes gold in whole thousands only. The screen shows the allowance as
// the original does, a remainder under a thousand included, but the result has to report what actually
// went in rather than echo what was typed (#290).
func TestSDIReportsWhatWentIn(t *testing.T) {
	// A program of 2,592,000 at the start of the turn allows 518,400 (a captured
	// figure); with 518,000 of it spent, 400 is left, which no deposit can use.
	cases := []struct {
		name        string
		keys        string
		fundedSoFar int64
		want        string
		spent       int64
	}{
		{"a remainder under a thousand", "400\r", 518_000, "Nothing added", 0},
		{"a figure that is not whole thousands", "1500\r", 0, "1,000 Gold added.", 1_000},
	}
	for _, c := range cases {
		w := newWorld()
		p := w.Player()
		p.Gold = 10_000_000
		p.SDIFunding = 2_592_000 + c.fundedSoFar
		p.TurnProgress.SDIFunded = c.fundedSoFar
		f := &fakeSession{keys: []rune(c.keys + " ")}
		sdiProgram(f, w)
		out := stripANSI(f.out.String())
		if !strings.Contains(out, "Add how much gold for funding?") {
			t.Fatalf("%s: never reached the funding prompt:\n%s", c.name, out)
		}
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: want %q in:\n%s", c.name, c.want, out)
		}
		if got := 10_000_000 - p.Gold; got != c.spent {
			t.Errorf("%s: spent %d gold, want %d", c.name, got, c.spent)
		}
	}
}
