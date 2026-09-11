package game

import "testing"

// newEventTestEmpire builds a bare empire with every resource at a fixed,
// nonzero value so both gain and lose rolls have somewhere to land.
func newEventTestEmpire() *Empire {
	return &Empire{
		Troopers: 50, Jets: 10, Turrets: 10, Tanks: 10, Agents: 10,
		Food: 1000, People: 500,
	}
}

func TestMaybeRandomEventDeterministicWithFixedSeed(t *testing.T) {
	w1 := NewWorldSeed(DefaultConfig(), 42)
	e1 := newEventTestEmpire()
	maybeRandomEvent(w1, e1)

	w2 := NewWorldSeed(DefaultConfig(), 42)
	e2 := newEventTestEmpire()
	maybeRandomEvent(w2, e2)

	if len(e1.Events) != len(e2.Events) {
		t.Fatalf("event counts differ: %d vs %d", len(e1.Events), len(e2.Events))
	}
	// Compare the TEXT, not the Event: it carries a wall-clock stamp, so two
	// runs never produce equal structs. The seed governs the wording and the
	// deltas, which is what this test is about.
	if len(e1.Events) == 1 && e1.Events[0].Text != e2.Events[0].Text {
		t.Errorf("same seed produced different events: %q vs %q", e1.Events[0].Text, e2.Events[0].Text)
	}
	if *resourcePtr(e1, eventTroopers) != *resourcePtr(e2, eventTroopers) {
		t.Errorf("same seed produced different resource deltas")
	}
}

// TestMaybeRandomEventNeverGoesNegative exercises many seeds/iterations to hit
// every category and both gain/lose branches, and checks the hard rule that a
// resource never drops below 0. (The don't-report-a-loss-you-couldn't-take rule
// is guarded numerically by TestMaybeRandomEventSkipsLoseOnZeroResource below —
// the event TEXT is not inspected anywhere.)
func TestMaybeRandomEventNeverGoesNegative(t *testing.T) {
	for seed := int64(0); seed < 200; seed++ {
		w := NewWorldSeed(DefaultConfig(), seed)
		e := &Empire{} // every resource starts at 0
		for i := 0; i < 20; i++ {
			maybeRandomEvent(w, e)
		}
		for r := eventResource(0); r < numEventResources; r++ {
			if *resourcePtr(e, r) < 0 {
				t.Fatalf("seed %d: resource %d went negative: %d", seed, r, *resourcePtr(e, r))
			}
		}
	}
}

func TestMaybeRandomEventGainIncreasesResourceAndAppendsOneLine(t *testing.T) {
	// Search a small range of seeds for one that fires a gain event, then
	// verify the resource increased and exactly one line was appended.
	for seed := int64(0); seed < 500; seed++ {
		w := NewWorldSeed(DefaultConfig(), seed)
		e := newEventTestEmpire()
		snapshot := map[eventResource]int{}
		for r := eventResource(0); r < numEventResources; r++ {
			snapshot[r] = *resourcePtr(e, r)
		}
		maybeRandomEvent(w, e)
		if len(e.Events) == 0 {
			continue // didn't fire this seed; try another
		}
		if len(e.Events) != 1 {
			t.Fatalf("seed %d: expected exactly 1 event line, got %d", seed, len(e.Events))
		}
		changed := false
		for r := eventResource(0); r < numEventResources; r++ {
			if *resourcePtr(e, r) != snapshot[r] {
				changed = true
			}
		}
		if !changed {
			t.Fatalf("seed %d: an event fired but no resource changed", seed)
		}
		return
	}
	t.Fatal("no seed in range fired an event; RandomEventChancePct or rng usage may have changed")
}

func TestDailyMaintenancePersistsHumanEventsButClearsAIEvents(t *testing.T) {
	// A human empire's random event must survive maintenance (shown at their
	// next login), while an AI/idle empire's events are cleared same-day
	// since nobody will ever read them. The event now fires at the END OF A
	// TURN (BRE's process_end_of_turn), so the turn is what has to be played
	// -- driving DailyMaintenance alone can no longer produce one.
	found := false
	for seed := int64(0); seed < 300; seed++ {
		cfg := DefaultConfig()
		cfg.AICount = 0
		w := NewWorldSeed(cfg, seed)
		w.Pirates = nil
		human := w.AddHuman("h", "Realm")
		human.Troopers, human.Jets, human.Turrets = 50_000, 10_000, 10_000
		human.Tanks, human.Agents, human.Food, human.People = 10_000, 10_000, 100_000, 50_000
		human.Protection = 0

		ai := w.AddHuman("", "AIRealm")
		ai.Owner = ""
		ai.Troopers, ai.Jets, ai.Turrets = 50_000, 10_000, 10_000
		ai.Tanks, ai.Agents, ai.Food, ai.People = 10_000, 10_000, 100_000, 50_000
		ai.Protection = 0

		w.PlayTurn(human, "2026-07-01")
		w.PlayTurn(ai, "2026-07-01")
		if len(human.Events) == 0 {
			continue
		}
		w.LastMaintDate = "2026-07-01"
		w.DailyMaintenance("2026-07-02")

		if len(ai.Events) != 0 {
			t.Fatalf("seed %d: AI empire events should be cleared same-day, got %v", seed, ai.Events)
		}
		if len(human.Events) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no seed in range produced a surviving human event; RandomEventChancePct or placement may be wrong")
	}
}

// A realm holding nothing gets no event at all: the amount is a SHARE of what
// is held, so it truncates to zero and the original returns without a word.
// This is the behavior that replaced a flat 1-20 roll, and it is the reason
// the old "lose on zero is skipped" test could no longer fire anything.
func TestRandomEventLeavesAnEmptyRealmAlone(t *testing.T) {
	for seed := int64(0); seed < 500; seed++ {
		w := NewWorldSeed(DefaultConfig(), seed)
		e := &Empire{}
		maybeRandomEvent(w, e)
		if len(e.Events) != 0 {
			t.Fatalf("seed %d: an empire holding nothing was given an event: %v", seed, e.Events)
		}
	}
}

// A realm under New Realm Protection is skipped before the roll is even made
// (is_under_protection at resolve_random_game_event +0x05e6).
func TestRandomEventSkipsProtectedRealms(t *testing.T) {
	for seed := int64(0); seed < 500; seed++ {
		w := NewWorldSeed(DefaultConfig(), seed)
		e := &Empire{Protection: 1, Troopers: 1_000_000, Jets: 1_000_000, Turrets: 1_000_000,
			Tanks: 1_000_000, Agents: 1_000_000, Food: 1_000_000, People: 1_000_000}
		maybeRandomEvent(w, e)
		if len(e.Events) != 0 {
			t.Fatalf("seed %d: a protected realm was given an event: %v", seed, e.Events)
		}
	}
}

// The amount moved is exactly the resource's binary-verified share of what is
// held. Asserted as golden literals rather than against eventMagnitudePct, so
// a retune has to bring new evidence (see AGENTS.md).
func TestRandomEventMovesTheVerifiedShare(t *testing.T) {
	const held = 1_000_000
	want := map[eventResource]int{
		eventTroopers: 30_000,  // 3%
		eventJets:     50_000,  // 5%
		eventTurrets:  50_000,  // 5%
		eventTanks:    70_000,  // 7%
		eventAgents:   50_000,  // 5%
		eventFood:     150_000, // 15%
		eventPeople:   40_000,  // 4%
	}
	for seed := int64(0); seed < 400; seed++ {
		w := NewWorldSeed(DefaultConfig(), seed)
		e := &Empire{Troopers: held, Jets: held, Turrets: held,
			Tanks: held, Agents: held, Food: held, People: held}
		maybeRandomEvent(w, e)
		if len(e.Events) == 0 {
			continue
		}
		for r, w2 := range want {
			got := *resourcePtr(e, r)
			if got == held {
				continue // untouched
			}
			if got != held+w2 && got != held-w2 {
				t.Fatalf("seed %d: resource %d moved to %d, want %d +/- %d", seed, r, got, held, w2)
			}
		}
	}
}
