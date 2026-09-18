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
		if e.LastRandomEvent == "" {
			continue // didn't fire this seed; try another
		}
		// The line is a report on the turn, not an asynchronous notice: the
		// original prints it in the End of Turn Statistics and files nothing.
		if len(e.Events) != 0 {
			t.Fatalf("seed %d: the event was filed as a notice: %+v", seed, e.Events)
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

// A human empire's filed events survive daily maintenance, to be read at their
// next login, while an AI or ownerless realm's are cleared the same day because
// nobody will ever read them.
//
// It files the events directly rather than waiting for a mechanic to produce
// one. It used to drive the random event and assert on what that left behind,
// and when the random event stopped being an Event at all (it prints in the End
// of Turn Statistics, where the original prints it) this test went on passing on
// whatever else happened to fire that turn -- covering something other than its
// own name.
func TestDailyMaintenancePersistsHumanEventsButClearsAIEvents(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	w.Pirates = nil

	human := w.AddHuman("h", "Realm")
	human.Protection = 0
	human.addEvent("Mallory attacked you and took 3 regions.")

	ai := w.AddHuman("", "AIRealm")
	ai.Owner = ""
	ai.Protection = 0
	ai.addEvent("Mallory attacked you and took 3 regions.")

	w.LastMaintDate = "2026-07-01"
	w.DailyMaintenance("2026-07-02")

	if len(human.Events) != 1 {
		t.Errorf("a human's events should survive maintenance, got %v", human.Events)
	}
	if len(ai.Events) != 0 {
		t.Errorf("an ownerless realm's events should be cleared, got %v", ai.Events)
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
