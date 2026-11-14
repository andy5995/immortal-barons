package menu

import (
	"strings"
	"testing"
)

// Income the cap cuts off at turn start is the baron's own turn, so it is
// reported under the Spending menu, after the footer and before the prompt, and
// files no event (an event would surface as "While you were at the menus").
// A treasury that next turn's income would overflow gets a warning there too.
func TestSpendingMenuReportsAndWarnsOfIncomeLostToTheCap(t *testing.T) {
	menus := BuildMenus()
	w := newWorld()
	p := w.Player()
	p.Prefs.DepositEndTurn = false
	p.Gold = w.World.MoneyCap() - 1
	before := len(p.Events)

	w.World.CollectIncome(p)

	if len(p.Events) != before {
		t.Errorf("income lost to the cap filed %d events, want none", len(p.Events)-before)
	}
	if p.TurnStartGoldLost <= 0 {
		t.Fatalf("TurnStartGoldLost = %d, want the overflow recorded", p.TurnStartGoldLost)
	}

	f := &fakeSession{}
	draw(f, w, menus.Spending)
	out := stripANSI(f.out.String())
	footer := strings.Index(out, "You have ")
	lost := strings.Index(out, "Treasury full")
	warn := strings.Index(out, "Warning: about")
	if footer < 0 || lost < footer || warn < lost {
		t.Fatalf("want footer, then the loss, then the warning; got:\n%s", out)
	}

	// Banking enough removes the warning; the loss already taken stays.
	p.Gold = 0
	f = &fakeSession{}
	draw(f, w, menus.Spending)
	out = stripANSI(f.out.String())
	if strings.Contains(out, "Warning: about") {
		t.Errorf("warning still shown with an empty treasury:\n%s", out)
	}
	if !strings.Contains(out, "Treasury full") {
		t.Errorf("this turn's loss should still be reported:\n%s", out)
	}
}
