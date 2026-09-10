package menu

import (
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The desk counts down to the weapon's own instant, in the same short form the
// group-attack table uses. It read "in 0 hours" for the whole of launch day
// while the schedule was a game day, which is what sent us looking.
func TestGooieDeskCountsDownToTheInstant(t *testing.T) {
	for _, c := range []struct {
		name string
		at   time.Time
		want string
	}{
		{"hours out", time.Now().Add(2 * time.Hour), "in 2h"},
		{"minutes out", time.Now().Add(90 * time.Minute), "in 90m"},
		{"its hour has come", time.Now().Add(-time.Hour), "on the next run"},
	} {
		w := newWorld()
		w.With(func() {
			w.World.Config.GooieKablooie = true
			w.Player().Protection = 0
			w.World.Annihilator = &game.Annihilator{
				TargetBoard: "Mars", Funded: true, Intact: 100,
				CostMillion: 10, PaidMillion: 10, LaunchAt: c.at,
			}
		})
		f := &fakeSession{keys: []rune("\r\r")}
		gooieKablooie(f, w)

		out := stripANSI(f.out.String())
		if !strings.Contains(out, "The Gooie Kablooie is complete") {
			t.Fatalf("%s: never reached the funded branch:\n%s", c.name, out)
		}
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: want %q in:\n%s", c.name, c.want, out)
		}
		if strings.Contains(out, "in 0 hours") {
			t.Errorf("%s: the day-grained countdown is back:\n%s", c.name, out)
		}
	}
}
