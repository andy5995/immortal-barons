package game

import (
	"slices"
	"testing"
)

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
		{"wait just up", InFlightStrike{LaunchedDay: 6}, 0},
		{"launched yesterday", InFlightStrike{LaunchedDay: 9}, 3},
		{"held: backstop only", InFlightStrike{LaunchedDay: 7, Held: true}, 13},
		{"a cleared hold extends it", InFlightStrike{LaunchedDay: 7, HeldDays: 2}, 3},
		{"backstop caps a long hold", InFlightStrike{LaunchedDay: -4, HeldDays: 14}, 2},
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

// The news says whose attack came home and whether it was a group's, and a
// trade deal or terror op, being private, stays out of it (#326).
func TestLostForcesNewsNamesTheAttack(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Config.LostForcesDays = 3
	w.GameDay = 10
	e := w.AddHuman("baron-a", "Iron Coast")
	c := []Contribution{{Owner: "baron-a", AttackForce: AttackForce{Troopers: 10}}}
	w.InFlight = []InFlightStrike{
		{ID: 1, Kind: "attack", TargetBoard: "Far BBS", TargetEmpire: "Salt Marsh", Contributors: c},
		{ID: 2, Kind: "attack", TargetBoard: "Far BBS", Group: true, Whole: true, Contributors: c},
		{ID: 3, Kind: "deal", TargetBoard: "Far BBS", TargetEmpire: "Salt Marsh", Owner: "baron-a", Goods: &TradeBasket{Food: 1}},
		{ID: 4, Kind: "terror", TargetBoard: "Far BBS", TargetEmpire: "Salt Marsh", Owner: "baron-a", Agents: 3, TerrorOp: TerrorOpDemoralize},
	}
	if got := w.ReturnLostForces(nil); got != 4 {
		t.Fatalf("recovered %d, want 4", got)
	}
	// Each owner hears what came home and from where, whatever the kind.
	var events []string
	for _, ev := range e.Events {
		events = append(events, ev.Text)
	}
	wantEvents := []string{
		"No word came back from Far BBS. Your force sent against Salt Marsh has come home: 10 Troopers.",
		"No word came back from Far BBS. Your force sent against the whole planet has come home: 10 Troopers.",
		"No word came back from Far BBS. Your trade deal to Salt Marsh has come home: 1 Food.",
		"No word came back from Far BBS. Your Demoralize against Salt Marsh has come home: 3 agents.",
	}
	if len(events) < len(wantEvents) || !slices.Equal(events[len(events)-len(wantEvents):], wantEvents) {
		t.Fatalf("events = %q, want them to end %q", events, wantEvents)
	}
	var news []string
	for _, n := range w.NewsToday {
		news = append(news, n.Text)
	}
	want := []string{
		"Iron Coast's attack on Salt Marsh of Far BBS heard nothing back, and its forces came home.",
		"A group attack on Far BBS heard nothing back, and its forces came home.",
	}
	if !slices.Equal(news, want) {
		t.Fatalf("news = %q, want %q", news, want)
	}
	if e.Troopers < 20 {
		t.Fatalf("troopers not returned: %d", e.Troopers)
	}
}
