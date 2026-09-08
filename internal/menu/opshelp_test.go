package menu

import (
	"strings"
	"testing"
)

// Both InterPlanetary submenus gained a (?) Help browser (IB's own divergence:
// the original lists the ops and nothing else). It is the Attack Type menu's
// browser, so nine ops can be read one after another without leaving.
func TestTerrorOpHelpBrowsesEveryOp(t *testing.T) {
	// "bomb food" completes to Bomb Food Stores, then Enter leaves.
	f := &fakeSession{keys: []rune("bomb food s\r\r")}
	if got := showTerrorOpHelp(f, &ctx{}); got != Stay {
		t.Errorf("help returned %v, want Stay -- the reader should land back on the menu", got)
	}
	out := stripANSI(f.out.String())
	for _, tp := range terrorOpTopics {
		if !strings.Contains(out, tp.name) {
			t.Errorf("the topic list is missing %q:\n%s", tp.name, out)
		}
	}
	if !strings.Contains(out, "29 percent") {
		t.Errorf("the chosen topic's body never printed:\n%s", out)
	}
}

func TestIPSpecialOpHelpBrowsesEveryOp(t *testing.T) {
	f := &fakeSession{keys: []rune("\r")}
	if got := showIPSpecialOpHelp(f, &ctx{}); got != Stay {
		t.Errorf("help returned %v, want Stay", got)
	}
	out := stripANSI(f.out.String())
	for _, tp := range ipSpecialOpTopics {
		if !strings.Contains(out, tp.name) {
			t.Errorf("the topic list is missing %q:\n%s", tp.name, out)
		}
	}
}

// Every op on the menu has a topic and every topic names an op. The menu and
// the help are two lists of the same set, so they drift apart the moment one is
// edited alone -- which is the whole failure mode these tables invite.
func TestOpHelpTopicsMatchTheMenus(t *testing.T) {
	m := BuildMenus()
	for _, tc := range []struct {
		what   string
		topics []attackTypeTopic
		menu   *Menu
	}{
		{"Terrorist Ops", terrorOpTopics, m.TerrorOps},
		{"Special Operations", ipSpecialOpTopics, m.IPSpecial},
	} {
		ops := map[string]bool{}
		for _, it := range tc.menu.Items {
			// Help and Quit are menu furniture, not operations.
			if it.Key == '?' || it.Key == '0' {
				continue
			}
			ops[it.Label] = true
		}
		topics := map[string]bool{}
		for _, tp := range tc.topics {
			topics[tp.name] = true
			if strings.TrimSpace(tp.body) == "" {
				t.Errorf("%s: topic %q has no body", tc.what, tp.name)
			}
			if !ops[tp.name] {
				t.Errorf("%s: topic %q is not an item on the menu", tc.what, tp.name)
			}
		}
		for op := range ops {
			if !topics[op] {
				t.Errorf("%s: menu item %q has no help topic", tc.what, op)
			}
		}
	}
}

// The (?) item is actually on both menus -- the browsers above can pass while
// nothing reaches them.
func TestOpMenusCarryAHelpItem(t *testing.T) {
	m := BuildMenus()
	for what, menu := range map[string]*Menu{"Terrorist Ops": m.TerrorOps, "Special Operations": m.IPSpecial} {
		found := false
		for _, it := range menu.Items {
			if it.Key == '?' {
				found = true
			}
		}
		if !found {
			t.Errorf("%s has no (?) Help item", what)
		}
	}
}
