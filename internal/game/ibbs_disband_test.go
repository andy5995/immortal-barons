package game

import (
	"strings"
	"testing"
	"time"
)

// The Coordinator can call off a party still forming, and every contributor gets
// their own detachment back — not the Coordinator, and not whoever filed it
// (#270). The gold is not refunded, exactly as a dismantled Gooie refunds none.
func TestCoordinatorCallsOffAGroupAttack(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "boardA"
	w := NewWorldSeed(cfg, 1)
	leader := w.AddHuman("l", "Leader")
	ally := w.AddHuman("a", "Ally")
	coord := w.AddHuman("c", "Coord")
	for _, e := range []*Empire{leader, ally, coord} {
		e.Regions = RegionMix{Desert: 5000}
		e.Troopers, e.Tanks, e.Gold = 100_000, 20_000, 1_000_000
		e.CoordinatorVote = "c"
	}
	if w.BBSCoordinator() != coord {
		t.Fatalf("the test needs Coord elected, got %v", w.BBSCoordinator())
	}

	ga, err := w.CreateGroupAttack(leader, "boardB", "Victim", GroupAttackHoursMin, AttackForce{Troopers: 40_000})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := w.JoinGroupAttack(ally, ga.ID, AttackForce{Tanks: 5_000}); err != nil {
		t.Fatalf("join: %v", err)
	}
	goldLeft, goldAlly := leader.Gold, ally.Gold

	if err := w.DisbandGroupAttackByCoordinator(leader, ga.ID, time.Now()); err != ErrNotPlanetCO {
		t.Errorf("a baron who is not Coordinator called off a party: %v", err)
	}
	if err := w.DisbandGroupAttackByCoordinator(coord, ga.ID, time.Now()); err != nil {
		t.Fatalf("call off: %v", err)
	}

	if len(w.GroupAttacks) != 0 {
		t.Errorf("the party is still forming: %+v", w.GroupAttacks)
	}
	if leader.Troopers != 100_000 {
		t.Errorf("the filer got back %d troopers, want all 100,000", leader.Troopers)
	}
	if ally.Tanks != 20_000 {
		t.Errorf("the joiner got back %d tanks, want all 20,000", ally.Tanks)
	}
	if ally.Troopers != 100_000 {
		t.Errorf("a contributor was handed somebody else's forces: %d troopers", ally.Troopers)
	}
	if leader.Gold != goldLeft || ally.Gold != goldAlly {
		t.Errorf("gold was refunded: %d and %d", leader.Gold, ally.Gold)
	}
	for _, e := range []*Empire{leader, ally} {
		if len(e.Events) == 0 || !strings.Contains(e.Events[len(e.Events)-1].Text, "called off") {
			t.Errorf("%s was not told: %+v", e.Name, e.Events)
		}
	}
	if !strings.Contains(w.NewsToday.Join("\n"), "called off") {
		t.Errorf("the planet was not told: %v", w.NewsToday)
	}
	if err := w.DisbandGroupAttackByCoordinator(coord, ga.ID, time.Now()); err != ErrNoAttack {
		t.Errorf("calling off a party twice: %v, want ErrNoAttack", err)
	}
}

// A force that has left is in flight; the lost-forces timer governs it from
// there, and nothing on the planet calls it back.
func TestCallingOffADepartedPartyIsRefused(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "boardA"
	w := NewWorldSeed(cfg, 1)
	coord := w.AddHuman("c", "Coord")
	coord.Regions = RegionMix{Desert: 5000}
	coord.Troopers, coord.Gold = 100_000, 1_000_000
	coord.CoordinatorVote = "c"

	ga, err := w.CreateGroupAttack(coord, "boardB", "Victim", GroupAttackHoursMin, AttackForce{Troopers: 10_000})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	gone := ga.DepartAt.Add(time.Second)
	if err := w.DisbandGroupAttackByCoordinator(coord, ga.ID, gone); err != ErrDeparted {
		t.Errorf("called off a force already away: %v, want ErrDeparted", err)
	}
	if len(w.GroupAttacks) != 1 {
		t.Errorf("the party was removed anyway: %+v", w.GroupAttacks)
	}
}
