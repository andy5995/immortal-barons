package game

import (
	"fmt"
	"testing"
)

// pactPair builds two realms on one seed holding the given relation, with the
// given morale/support figures. Every test below runs across many seeds, because
// one seed is one trajectory of a 1-in-3 roll.
func pactPair(seed int64, rel string, aMorale, aSupport, bMorale, bSupport int) (*World, *Empire, *Empire) {
	w := NewWorldSeed(DefaultConfig(), seed)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")
	a.Morale, a.Support = aMorale, aSupport
	b.Morale, b.Support = bMorale, bSupport
	if rel != "" {
		w.setRelation(a.Name, b.Name, rel)
	}
	return w, a, b
}

// The contagion runs ONE way. BRE's routine has no branch that raises a stat, so
// the sicker partner is never pulled up and the healthier one never falls past
// it: each pass either does nothing or lands exactly on 100-24=76, and once the
// gap is inside a day's drain it lands exactly on the partner's figure.
func TestFreeTradeContagionOnlyDragsTheHealthierRealmDown(t *testing.T) {
	fired := 0
	const passes = 600
	for seed := int64(1); seed <= passes; seed++ {
		w, a, b := pactPair(seed, freeTradeAgreement, 100, 100, 30, 30)
		w.freeTradeContagion()
		if b.Morale != 30 || b.Support != 30 {
			t.Fatalf("seed %d: the worse partner moved to %d/%d, want 30/30 — nothing may raise it", seed, b.Morale, b.Support)
		}
		for _, got := range []int{a.Morale, a.Support} {
			if got != 100 && got != 76 {
				t.Fatalf("seed %d: healthy realm at %d, want 100 (no roll) or 76 (100 - one day's 24)", seed, got)
			}
		}
		if a.Morale == 76 {
			fired++
		}
	}
	// A third of the passes should fire. The band is wide enough that a change of
	// RNG or of seed cannot trip it, and narrow enough to catch a lost or doubled
	// roll: 600 passes at 1-in-3 has a standard deviation of about 11.5.
	if fired < 140 || fired > 260 {
		t.Errorf("morale roll fired on %d of %d passes, want about %d (1 in %d)", fired, passes, passes/FreeTradeContagionOdds, FreeTradeContagionOdds)
	}
}

// The drain is clamped at the partner's figure, never below it. A gap smaller
// than one day's drain closes exactly, and a pair already level costs nothing.
func TestFreeTradeContagionNeverOvershootsThePartner(t *testing.T) {
	for seed := int64(1); seed <= 200; seed++ {
		w, a, b := pactPair(seed, freeTradeAgreement, 50, 50, 45, 40)
		w.freeTradeContagion()
		if a.Morale != 50 && a.Morale != 45 {
			t.Fatalf("seed %d: morale at %d, want 50 or the partner's 45 — a 5-point gap may not drain 24", seed, a.Morale)
		}
		if a.Support != 50 && a.Support != 40 {
			t.Fatalf("seed %d: support at %d, want 50 or the partner's 40", seed, a.Support)
		}
		if b.Morale != 45 || b.Support != 40 {
			t.Fatalf("seed %d: the worse partner moved to %d/%d, want 45/40", seed, b.Morale, b.Support)
		}

		w, a, b = pactPair(seed, freeTradeAgreement, 60, 60, 60, 60)
		w.freeTradeContagion()
		if a.Morale != 60 || a.Support != 60 || b.Morale != 60 || b.Support != 60 {
			t.Fatalf("seed %d: level partners moved to %d/%d and %d/%d, want 60 throughout", seed, a.Morale, a.Support, b.Morale, b.Support)
		}
	}
}

// Only relation 3 spreads it. BRE compares the pair's relation against exactly
// 3 and skips everything else, so no other pact — and no absence of one — moves
// a figure.
func TestFreeTradeContagionIgnoresEveryOtherRelation(t *testing.T) {
	rels := append([]string{"", RelationEnemy}, TreatyTypes...)
	for _, rel := range rels {
		if rel == freeTradeAgreement {
			continue
		}
		for seed := int64(1); seed <= 100; seed++ {
			w, a, b := pactPair(seed, rel, 100, 100, 10, 10)
			w.freeTradeContagion()
			if a.Morale != 100 || a.Support != 100 || b.Morale != 10 || b.Support != 10 {
				t.Fatalf("relation %q, seed %d: figures moved to %d/%d and %d/%d, want them untouched", rel, seed, a.Morale, a.Support, b.Morale, b.Support)
			}
		}
	}
}

// Morale and support are drawn separately, so a pass can pass the misery along
// in one and not the other. Both mixed outcomes must occur; a single shared roll
// would produce neither.
func TestFreeTradeContagionRollsEachStatIndependently(t *testing.T) {
	moraleOnly, supportOnly := 0, 0
	for seed := int64(1); seed <= 200; seed++ {
		w, a, _ := pactPair(seed, freeTradeAgreement, 100, 100, 0, 0)
		w.freeTradeContagion()
		switch {
		case a.Morale < 100 && a.Support == 100:
			moraleOnly++
		case a.Morale == 100 && a.Support < 100:
			supportOnly++
		}
	}
	if moraleOnly == 0 || supportOnly == 0 {
		t.Errorf("over 200 passes morale moved alone %d times and support alone %d times; both must happen if the two rolls are independent", moraleOnly, supportOnly)
	}
}

// It runs in daily maintenance, so it reaches a realm whose owner is not playing
// — which is the point of a contagion. A whole chain drains: C is sick, B holds
// a pact with C, A holds one with B.
func TestFreeTradeContagionRunsInDailyMaintenance(t *testing.T) {
	drained := 0
	for seed := int64(1); seed <= 100; seed++ {
		w := NewWorldSeed(DefaultConfig(), seed)
		var es []*Empire
		for i, name := range []string{"Alpha", "Beta", "Gamma"} {
			e := w.AddHuman(fmt.Sprintf("h%d", i), name)
			e.LastPlayed = "2026-07-16" // played once, so maintenance does not reap it
			e.Morale, e.Support = 100, 100
			es = append(es, e)
		}
		es[2].Morale, es[2].Support = 0, 0
		w.setRelation("Alpha", "Beta", freeTradeAgreement)
		w.setRelation("Beta", "Gamma", freeTradeAgreement)
		w.LastMaintDate = "2026-07-16"
		w.DailyMaintenance("2026-07-17")
		if es[2].Morale != 0 || es[2].Support != 0 {
			t.Fatalf("seed %d: the sick realm rose to %d/%d during maintenance", seed, es[2].Morale, es[2].Support)
		}
		if es[1].Morale < 100 {
			drained++
		}
	}
	if drained == 0 {
		t.Error("maintenance never drained a Free Trade partner over 100 days; the pass is not wired in")
	}
}
