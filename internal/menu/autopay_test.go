package menu

import (
	"strings"
	"testing"
)

// With auto-pay maintenance on and enough gold, the maintenance screen (shown
// after the empire status, before the pause) must report the gold paid, and
// the food upkeep must be a real number to display beside it. Regression: the
// income-timing bug left the player broke at maintenance so nothing was paid.
func TestAutoPayReportsMaintenance(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold = 5_000_000
	p.Troopers, p.Land = 700, 100
	w.AutoPayMaint = true

	f := &fakeSession{}
	paymentStage(f, w, p)

	if out := f.out.String(); !strings.Contains(out, "Maintenance paid") {
		t.Errorf("auto-pay should report maintenance paid.\n--- output ---\n%s", out)
	}
	if p.FoodUpkeep() <= 0 {
		t.Errorf("FoodUpkeep should be positive for a populated empire, got %d", p.FoodUpkeep())
	}
}
