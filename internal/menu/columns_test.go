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

// TestGameMenuTwoColumn checks the opt-in two-column layout puts two items on
// one line for the Game menu, while a plain menu (Attack) stays one per line.
func TestGameMenuTwoColumn(t *testing.T) {
	menus := BuildMenus()
	w := newWorld()

	f := &fakeSession{}
	draw(f, w, menus.Game)
	if !lineWithBoth(f.out.String(), "(1)", "(2)") {
		t.Errorf("Game menu should place (1) and (2) on one line:\n%s", f.out.String())
	}

	f = &fakeSession{}
	draw(f, w, menus.Attack)
	if lineWithBoth(f.out.String(), "(R)", "(N)") {
		t.Errorf("Attack menu should draw one item per line:\n%s", f.out.String())
	}
}
