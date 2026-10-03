package menu

import (
	"strings"
	"testing"
)

// An indented report line keeps its indent, on the line and on any line it
// wraps onto; help.Wrap alone collapses it, which flattened the tallies of a
// returning strike's report.
func TestWrapReportKeepsALinesIndent(t *testing.T) {
	short := "Date: 09/01/2026  20:35:16 UTC    Result: SUCCESS\n  20k Troopers returned."
	if got := wrapReport(short); got != short {
		t.Errorf("a report that fits was respaced:\n%q\nwant\n%q", got, short)
	}
	got := wrapReport("Header\n  You lost 638 Troopers, 433k Jets, 46k Tanks, and 1500 Bombers, and a great many more besides that.")
	lines := strings.Split(got, "\n")
	if len(lines) < 3 || lines[0] != "Header" {
		t.Fatalf("wrapped to %q", lines)
	}
	for _, l := range lines[1:] {
		if !strings.HasPrefix(l, "  ") || strings.HasPrefix(l, "   ") || len(l) >= 80 {
			t.Errorf("line %q should keep the two-space indent and fit 80 columns", l)
		}
	}
}
