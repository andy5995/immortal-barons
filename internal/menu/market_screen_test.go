package menu

import (
	"strings"
	"testing"
)

// The Trading Market's geometry is pinned to the live capture in
// docs/dev/bre-screens.md, as golden literals rather than as the constants that
// produce them: a retune of the constants should fail here and force new
// evidence, which is the point of the fidelity contract.
//
// From that capture: every figure is right-aligned with its header label
// right-aligned to the same edge, at columns 33, 45, 59 and 76, and the divider
// is BRE's inset rule — 5 single, 15 double, then single to 78.
func TestMarketTableMatchesTheCapturedGeometry(t *testing.T) {
	w := newWorld()
	f := &fakeSession{}
	printMarketTable(f, w)
	lines := strings.Split(stripANSI(f.out.String()), "\n")

	var header, divider, firstRow string
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "Key") && header == "":
			header = l
		case strings.HasPrefix(l, "─") && divider == "":
			divider = l
		case strings.HasPrefix(l, "(1)") && firstRow == "":
			firstRow = l
		}
	}
	if header == "" || divider == "" || firstRow == "" {
		t.Fatalf("market table did not render header/divider/row, got:\n%s", f.out.String())
	}

	wantRule := strings.Repeat("─", 5) + strings.Repeat("═", 15) + strings.Repeat("─", 58)
	if divider != wantRule {
		t.Errorf("divider is not BRE's inset rule:\n got %q (len %d)\nwant %q (len %d)",
			divider, len([]rune(divider)), wantRule, len([]rune(wantRule)))
	}

	// Right edges of the four figure columns, 0-indexed, from the capture.
	for _, want := range []int{33, 45, 59, 76} {
		if got := len([]rune(firstRow)); got <= want {
			t.Fatalf("row is too short (%d runes) to carry a column ending at %d: %q", got, want, firstRow)
		}
	}
	if got := columnEnds(firstRow); !equalInts(got, []int{33, 45, 59, 76}) {
		t.Errorf("figure columns end at %v, want [33 45 59 76]\nrow: %q", got, firstRow)
	}
	// The header labels contain spaces ("Your Prices"), so locate each one
	// rather than splitting the line on whitespace.
	for _, c := range []struct {
		label string
		end   int
	}{{"Your Prices", 33}, {"Owned", 45}, {"For Sale", 59}, {"Total For Sale", 76}} {
		i := strings.Index(header, c.label)
		if i < 0 {
			t.Errorf("header is missing %q: %q", c.label, header)
			continue
		}
		if got := i + len([]rune(c.label)) - 1; got != c.end {
			t.Errorf("header label %q ends at column %d, want %d\nheader: %q", c.label, got, c.end, header)
		}
	}
}

// BRE marks the goods that are not military units with a leading '*' in its
// shared goods table (BRE.EXE 0x157b7: Trooper, Jet, Turret, Bomber, *Food,
// *Gold, Agent, Tank, Carrier). Only Food appears on this screen, so the table
// carries exactly one marker and the seven units carry none.
func TestMarketMarksTheNonUnitGood(t *testing.T) {
	w := newWorld()
	f := &fakeSession{}
	printMarketTable(f, w)
	out := stripANSI(f.out.String())

	if !strings.Contains(out, "*Food") {
		t.Errorf("food row should be marked *Food, got:\n%s", out)
	}
	for _, unit := range []string{"Trooper", "Jet", "Turret", "Bomber", "Agent", "Tank", "Carrier"} {
		if strings.Contains(out, "*"+unit) {
			t.Errorf("%s is a military unit and must not carry the '*' marker", unit)
		}
	}
	if n := strings.Count(out, "*"); n != 1 {
		t.Errorf("market shows %d '*' markers, want exactly 1 (Food)\n%s", n, out)
	}
}

// The market's keys are 1-5 and 7-9 — BRE leaves 6 out, because regions are not
// tradeable (docs/dev/bre-screens.md).
func TestMarketSkipsKeySix(t *testing.T) {
	w := newWorld()
	f := &fakeSession{}
	printMarketTable(f, w)
	out := stripANSI(f.out.String())
	for _, k := range []string{"(1)", "(2)", "(3)", "(4)", "(5)", "(7)", "(8)", "(9)"} {
		if !strings.Contains(out, k) {
			t.Errorf("market is missing key %s", k)
		}
	}
	if strings.Contains(out, "(6)") {
		t.Error("market must not offer key 6 — regions are not tradeable")
	}
}

// columnEnds returns the 0-indexed end column of each run of non-space text
// that follows two or more spaces, i.e. the right edge of every aligned column
// after the leading label.
func columnEnds(line string) []int {
	r := []rune(line)
	var ends []int
	for i := 0; i < len(r); i++ {
		if r[i] == ' ' {
			continue
		}
		j := i
		for j < len(r) && r[j] != ' ' {
			j++
		}
		// A column is a field preceded by at least two spaces.
		if i >= 2 && r[i-1] == ' ' && r[i-2] == ' ' {
			ends = append(ends, j-1)
		}
		i = j
	}
	return ends
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
