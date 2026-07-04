package menu

import (
	"strings"
	"testing"
)

// With auto-pay maintenance on and enough gold, the end-of-turn summary must
// still report what was paid and consumed (regression: the income-timing bug
// left the player broke at maintenance, so nothing was paid and the summary
// was empty).
func TestAutoPayShowsGoldPaidAndFoodConsumed(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold = 5_000_000
	p.Troopers, p.Land = 700, 100
	w.AutoPayMaint = true

	f := &fakeSession{}
	paymentStage(f, w, p)
	w.PlayTurn(p, w.Today)
	endOfTurnStats(f, w, p)

	out := f.out.String()
	if !strings.Contains(out, "Gold paid") {
		t.Errorf("end-of-turn missing 'Gold paid'.\n--- output ---\n%s", out)
	}
	if !strings.Contains(out, "Food consumed") {
		t.Errorf("end-of-turn missing 'Food consumed'.\n--- output ---\n%s", out)
	}
}
