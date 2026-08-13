package menu

import (
	"regexp"
	"strings"
	"testing"
)

// ansiEsc matches SGR/cursor escape sequences.
var ansiEsc = regexp.MustCompile("\x1b\\[[0-9;?]*[A-Za-z]")

// stripANSI removes escape sequences so structural checks can match "(K)" key
// markers that the menu now colors piece by piece (dim parens, bright key).
func stripANSI(s string) string { return ansiEsc.ReplaceAllString(s, "") }

// lineWithBoth reports whether any single line of out contains both a and b.
func lineWithBoth(out, a, b string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, a) && strings.Contains(line, b) {
			return true
		}
	}
	return false
}

// lineIndexOf returns the index of the first line containing sub, or -1.
func lineIndexOf(out, sub string) int {
	for i, line := range strings.Split(out, "\n") {
		if strings.Contains(line, sub) {
			return i
		}
	}
	return -1
}

// maxItemsPerLine is the most items on any one line, counted by the "(" of each
// "(K)" key marker (none of these menus' labels contain a literal "(").
func maxItemsPerLine(out string) int {
	max := 0
	for _, line := range strings.Split(out, "\n") {
		if n := strings.Count(line, "("); n > max {
			max = n
		}
	}
	return max
}

// TestGameMenuTwoColumn checks the two-column layout is column-major: keys run
// DOWN the left column ((1) then (2) on consecutive lines, not side by side),
// while a plain menu (Attack) stays one per line.
func TestGameMenuTwoColumn(t *testing.T) {
	menus := BuildMenus()
	w := newWorld()

	f := &fakeSession{}
	draw(f, w, menus.Game)
	out := stripANSI(f.out.String())
	if maxItemsPerLine(out) < 2 {
		t.Errorf("Game menu should have two columns:\n%s", out)
	}
	// Column-major: (1) heads the left column and (2) sits directly below it.
	if lineWithBoth(out, "(1)", "(2)") {
		t.Errorf("column-major: (1) and (2) should not share a line:\n%s", out)
	}
	if got := lineIndexOf(out, "(2)") - lineIndexOf(out, "(1)"); got != 1 {
		t.Errorf("(2) should sit one line below (1), gap was %d:\n%s", got, out)
	}

	f = &fakeSession{}
	draw(f, w, menus.Attack)
	if lineWithBoth(stripANSI(f.out.String()), "(R)", "(N)") {
		t.Errorf("Attack menu should draw one item per line:\n%s", f.out.String())
	}
}

// lineWithAll reports whether any single line of out contains every substring.
func lineWithAll(out string, subs ...string) bool {
	for _, line := range strings.Split(out, "\n") {
		ok := true
		for _, s := range subs {
			if !strings.Contains(line, s) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// TestSystemMenuTwoColumn checks the System menu lays two items on one row (2
// columns, not BRE's 3 — wide/translated labels overflow 3; see tree.go) and
// offers a Help item.
func TestSystemMenuTwoColumn(t *testing.T) {
	menus := BuildMenus()
	w := newWorld()

	f := &fakeSession{}
	draw(f, w, menus.System)
	out := stripANSI(f.out.String())
	if maxItemsPerLine(out) < 2 {
		t.Errorf("System menu should place two items on one line:\n%s", out)
	}
	if !lineWithAll(out, "(?)", "Help") {
		t.Errorf("System menu should include a (?) Help item:\n%s", out)
	}
}

// TestGameMenuMessagesAndHelp checks the opening menu merged Read/Send into a
// single Messages item and renamed the help hotkey to (?) Help.
func TestGameMenuMessagesAndHelp(t *testing.T) {
	menus := BuildMenus()
	w := newWorld()

	f := &fakeSession{}
	draw(f, w, menus.Game)
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Messages") {
		t.Errorf("Game menu should contain a Messages item:\n%s", out)
	}
	if strings.Contains(out, "Read Messages") || strings.Contains(out, "Send Message") {
		t.Errorf("Game menu should not contain Read Messages / Send Message:\n%s", out)
	}
	if !lineWithAll(out, "(?)", "Help") {
		t.Errorf("Game menu should include a (?) Help item:\n%s", out)
	}
	if strings.Contains(out, "Help Database") || lineWithAll(out, "(B)", "Help") {
		t.Errorf("Game menu should not contain the old Help Database / (B) item:\n%s", out)
	}
}
