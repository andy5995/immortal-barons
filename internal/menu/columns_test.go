package menu

import (
	"strings"
	"testing"
)

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
	out := f.out.String()
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
	if lineWithBoth(f.out.String(), "(R)", "(N)") {
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

// TestSystemMenuThreeColumn checks the System menu lays three items on one row
// and offers a Help item.
func TestSystemMenuThreeColumn(t *testing.T) {
	menus := BuildMenus()
	w := newWorld()

	f := &fakeSession{}
	draw(f, w, menus.System)
	out := f.out.String()
	if maxItemsPerLine(out) < 3 {
		t.Errorf("System menu should place three items on one line:\n%s", out)
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
	out := f.out.String()
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
