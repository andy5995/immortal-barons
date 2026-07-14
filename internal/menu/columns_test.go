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
	if !lineWithAll(out, "Diplomacy", "Empire Status", "Food Market") {
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
