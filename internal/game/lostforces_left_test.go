package game

import "testing"

// The countdown the sysop panel shows is the rule ReturnLostForces applies, so
// an item it says has days left is one the timer keeps.
func TestLostForcesDaysLeft(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Config.LostForcesDays = 3
	w.GameDay = 10
	for _, c := range []struct {
		name string
		f    InFlightStrike
		want int
	}{
		{"wait just up", InFlightStrike{LaunchedDay: 7}, 0},
		{"one day in", InFlightStrike{LaunchedDay: 9}, 2},
		{"held: backstop only", InFlightStrike{LaunchedDay: 7, Held: true}, 12},
		{"a cleared hold extends it", InFlightStrike{LaunchedDay: 7, HeldDays: 2}, 2},
		{"backstop caps a long hold", InFlightStrike{LaunchedDay: -4, HeldDays: 14}, 1},
	} {
		got, ok := w.LostForcesDaysLeft(c.f)
		if !ok || got != c.want {
			t.Errorf("%s: LostForcesDaysLeft = %d, %v; want %d, true", c.name, got, ok, c.want)
		}
		w.InFlight = []InFlightStrike{c.f}
		w.InFlight[0].Kind = "special"
		held := map[string]bool{}
		if c.f.Held {
			held[""] = true
		}
		kept := w.ReturnLostForces(held) == 0
		if kept != (c.want > 0) {
			t.Errorf("%s: %d days left, but ReturnLostForces kept it: %v", c.name, c.want, kept)
		}
	}
	w.Config.LostForcesDays = 0
	if _, ok := w.LostForcesDaysLeft(InFlightStrike{}); ok {
		t.Error("LostForcesDaysLeft reported a countdown with recovery off")
	}
}
