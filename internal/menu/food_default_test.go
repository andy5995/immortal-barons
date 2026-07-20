package menu

import "testing"

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
