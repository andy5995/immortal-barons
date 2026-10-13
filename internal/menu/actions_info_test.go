package menu

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The info screen names a trade's floor where the lost-forces setting is under
// it, or "Never" reads as covering deals too.
func TestLostForcesStrNamesTheTradeFloor(t *testing.T) {
	f := &fakeSession{}
	for _, c := range []struct {
		days int
		want string
	}{{0, "Never; trades 3 days"}, {1, "1 day; trades 3 days"}, {3, "3 days"}, {7, "7 days"}} {
		cfg := game.DefaultConfig()
		cfg.LostForcesDays = c.days
		if got := lostForcesStr(f, cfg); got != c.want {
			t.Errorf("LostForcesDays %d: %q, want %q", c.days, got, c.want)
		}
	}
}
