package menu

import (
	"strings"
	"testing"
)

// At the Food Market "How much food to buy?" prompt, hitting Enter should take
// the suggested default, and that default should cover the current shortfall
// (what the realm needs this turn minus what it has) — not a hard-coded 0.
func TestBuyFoodDefaultsToShortfall(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Food = 100
	p.People = 5_000_000
	p.Gold = 1_000_000_000
	w.Config.FoodUnlimited = true

	shortfall := p.FoodUpkeep() - p.Food
	if shortfall <= 0 {
		t.Fatalf("test setup needs a positive shortfall, got %d", shortfall)
	}

	f := &fakeSession{keys: []rune("\r")} // Enter -> accept the suggested default
	buyFoodMarket(f, w)

	if got := p.Food; got != 100+shortfall {
		t.Errorf("Enter at buy-food should buy the shortfall: Food = %d, want %d", got, 100+shortfall)
	}
}

// The Food Market carries one holding behind two rows, so it belongs in the
// footer beside the gold — as the original prints it ("You have 37,505 gold and
// 1608 units of food.", cap/121125-666H4H_Camembert_Public.cap) — and not in a
// "# Owned" column, which printed the same figure on the Buy row and the Sell
// row. The supply line sits above the title rule, also as the original draws it:
// joined to the footer the pair ran to 89 columns.
func TestFoodMarketShowsTheHoldingOnce(t *testing.T) {
	f := &fakeSession{keys: []rune("0")}
	w := newWorld()
	p := w.Player()
	p.Food = 9003
	p.Gold = 400634
	if err := Run(f, w, BuildMenus().Food); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := stripANSI(f.out.String())
	if n := strings.Count(out, "9,003"); n != 1 {
		t.Errorf("the food holding should appear once, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, "You have 400,634 gold and 9,003 units of food.") {
		t.Errorf("want the original's footer sentence, got:\n%s", out)
	}
	if strings.Contains(out, "# Owned") {
		t.Errorf("the Owned column should be gone:\n%s", out)
	}
	for _, l := range strings.Split(out, "\n") {
		if n := len([]rune(strings.TrimRight(l, " "))); n > 80 {
			t.Errorf("line runs to %d columns: %q", n, l)
		}
	}
}
