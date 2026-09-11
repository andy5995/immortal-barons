package game

import "testing"

// The hashed BYTES are the contract: shift one and every price walk and region
// draw in every saved world moves. These are the figures the hand-built hashes
// produced before they were folded into one chain (#210), so the chain cannot
// quietly relayer them.
func TestDeterministicDrawsAreFrozen(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("a", "Testrealm")
	w.GameDay, e.TurnsPlayed, e.TurnsLeft = 7, 21, 14

	for _, c := range []struct {
		what string
		got  int
		want int
	}{
		{"FoodBuyPrice", w.FoodBuyPrice(), 42},
		{"priceJitter hq", w.priceJitter(e, "hq", 97), 77},
		{"priceJitter hqcap", w.priceJitter(e, "hqcap", 97), 21},
		{"walkRoll troopers k=0", w.walkRoll(e, "troopers", 0, 89), 31},
		{"walkRoll troopers k=1", w.walkRoll(e, "troopers", 1, 89), 42},
		{"regionDraw salt=1", w.regionDraw(e, 1, 83), 60},
		{"regionDraw salt=5", w.regionDraw(e, 5, 83), 6},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d — the key material's byte layout moved", c.what, c.got, c.want)
		}
	}
}

// A non-positive bound draws nothing rather than dividing by zero, which each
// caller used to test for itself.
func TestDrawBoundIsGuarded(t *testing.T) {
	for _, n := range []int{0, -1} {
		if got := newDraw().num(1).text("x").roll(n); got != 0 {
			t.Errorf("roll(%d) = %d, want 0", n, got)
		}
	}
}
