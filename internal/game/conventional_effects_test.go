package game

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// battleWorld is a seeded world where a player with `troops` troopers can
// attack an AI realm of the given size.
func battleWorld(seed int64, land, people, troopers, attackers int) (*World, *Empire, *Empire, AttackForce) {
	w, a, d := strikeWorld(seed, land, people, troopers)
	a.Troopers, a.Tanks, a.Jets, a.Bombers = attackers, attackers/5, attackers/9, attackers/13
	a.Morale = 70
	d.Morale = 70
	return w, a, d, AttackForce{Troopers: a.Troopers, Tanks: a.Tanks, Jets: a.Jets, Bombers: a.Bombers}
}

func clearStamps(es ...*Empire) {
	for _, e := range es {
		for i := range e.Events {
			e.Events[i].When = time.Time{}
		}
	}
}

func TestBaseConventionalAttackMatchesTheOriginalRoutine(t *testing.T) {
	sizes := []struct{ land, people, troopers int }{
		{0, 0, 0}, {1, 5, 1}, {3, 1000, 7}, {40, 99_999, 333}, {200, 3_000_000, 50_000},
		{1000, 90_000_000, 2_000_000}, {100_000, 2_000_000_000, 100_000_000},
	}
	forces := []int{1, 40, 3_000, 90_000, 4_000_000, 150_000_000}
	won, lost := 0, 0
	for _, sz := range sizes {
		for _, force := range forces {
			for seed := int64(1); seed <= 25; seed++ {
				w1, a1, d1, f1 := battleWorld(seed, sz.land, sz.people, sz.troopers, force)
				w2, a2, d2, f2 := battleWorld(seed, sz.land, sz.people, sz.troopers, force)
				r1, c1 := w1.Attack(a1, d1, f1)
				r2, c2 := refAttack(w2, a2, d2, f2)
				if strings.Contains(r1, "Victory") {
					won++
				} else {
					lost++
				}
				clearStamps(a1, d1, a2, d2)
				if r1 != r2 || c1 != c2 || !reflect.DeepEqual(a1, a2) || !reflect.DeepEqual(d1, d2) {
					t.Fatalf("%v force %d seed %d: attack differs\n got  %q %d\n want %q %d", sz, force, seed, r1, c1, r2, c2)
				}
				if x, y := w1.rng.Intn(1<<30), w2.rng.Intn(1<<30); x != y {
					t.Fatalf("%v force %d seed %d: rng streams diverged", sz, force, seed)
				}
			}
		}
	}
	if won < 100 || lost < 100 {
		t.Errorf("the comparison covered %d wins and %d losses, want at least 100 of each", won, lost)
	}
}

func moddedConventional(w *World, kind EffectKind, edit func(fx *AttackEffects)) {
	rules := BaseRules()
	fx := rules.Attacks[kind]
	edit(&fx)
	rules.Attacks[kind] = fx
	w.Rules = rules
}

func TestModdedWonSetAddsToAWin(t *testing.T) {
	base, ba, bd, bf := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	w, a, d, f := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	moddedConventional(w, EffectConventionalWon, func(fx *AttackEffects) {
		fx.Target.Kill[KillIndex(Tank)] = Roll{Pct: 10}
		fx.Target.Waste = Roll{Pct: 5}
	})
	baseReport, _ := base.Attack(ba, bd, bf)
	report, captured := w.Attack(a, d, f)
	if !strings.Contains(baseReport, "Victory") {
		t.Fatal("setup: the attack should win")
	}
	if captured != 100 {
		t.Errorf("captured %d, want the unchanged 100", captured)
	}
	wantTanks := bd.Tanks - pctOf(bd.Tanks, 10)
	if d.Tanks != wantTanks || d.Tanks == bd.Tanks {
		t.Errorf("tanks %d, want %d (base %d)", d.Tanks, wantTanks, bd.Tanks)
	}
	if d.Land != bd.Land || d.Regions.Waste != bd.Regions.Waste+pctOf(bd.Land, 5) {
		t.Errorf("land %d waste %d, want %d and %d", d.Land, d.Regions.Waste, bd.Land, bd.Regions.Waste+pctOf(bd.Land, 5))
	}
	want := baseReport + "\nIt also took " + "45 Regions laid waste and 114 Tanks.\n"
	if report != want {
		t.Errorf("report\n%q\nwant\n%q", report, want)
	}
}

func TestModdedLostSetCostsTheAttacker(t *testing.T) {
	base, ba, bd, bf := battleWorld(8, 1000, 1_000_000, 2_000_000, 40)
	w, a, d, f := battleWorld(8, 1000, 1_000_000, 2_000_000, 40)
	moddedConventional(w, EffectConventionalLost, func(fx *AttackEffects) {
		fx.Attacker.People = Roll{Pct: 2}
	})
	baseReport, _ := base.Attack(ba, bd, bf)
	report, _ := w.Attack(a, d, f)
	if !strings.Contains(baseReport, "Defeat") {
		t.Fatal("setup: the attack should be repelled")
	}
	if a.People != ba.People-pctOf(ba.People, 2) || a.People == ba.People {
		t.Errorf("attacker people %d, want %d", a.People, ba.People-pctOf(ba.People, 2))
	}
	want := baseReport + "\nIt also cost you 40 People.\n"
	if report != want {
		t.Errorf("report\n%q\nwant\n%q", report, want)
	}
	if got := d.Events[len(d.Events)-1].Msg.English(); !strings.HasSuffix(got, "It also cost "+a.Name+" 40 People.") {
		t.Errorf("defender's event %q lacks the attacker's cost", got)
	}
}

// A region a won set captures is a captured region like the battle's own: it is
// in the report, the event, the score and the returned total the caller parks.
func TestModdedWonCaptureCountsLikeTheBuiltInOne(t *testing.T) {
	base, ba, _, bf := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	w, a, d, f := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	moddedConventional(w, EffectConventionalWon, func(fx *AttackEffects) {
		fx.Target.Capture = Roll{Pct: 10}
	})
	baseReport, baseCaptured := base.Attack(ba, battleDefender(base), bf)
	report, captured := w.Attack(a, d, f)
	if baseCaptured != 100 || captured != 190 {
		t.Fatalf("captured %d (base %d), want 190 (base 100)", captured, baseCaptured)
	}
	if !strings.Contains(baseReport, "Your score rose by 8200.") || !strings.Contains(report, "Your score rose by 15580.") {
		t.Errorf("score lines:\n%s", report)
	}
	if !strings.Contains(report, "Victory! You captured 190 regions.") || strings.Contains(report, "seized") {
		t.Errorf("report:\n%s", report)
	}
	if got := d.Events[len(d.Events)-1].Msg.English(); !strings.Contains(got, "took 190 regions") {
		t.Errorf("defender's event %q", got)
	}
	if d.Land != 810 || a.Score != ba.Score+15580-8200 {
		t.Errorf("target land %d score %d, want 810 and %d", d.Land, a.Score, ba.Score+15580-8200)
	}
	if last := w.Battles[len(w.Battles)-1]; last.Land != 190 {
		t.Errorf("battle log records %d regions, want 190", last.Land)
	}
	w.ParkUntyped(a, captured)
	if a.PendingRegions != 190 {
		t.Errorf("parked %d regions, want the one total of 190", a.PendingRegions)
	}
}

// battleDefender is the AI realm of a battleWorld's world.
func battleDefender(w *World) *Empire { return w.Empires[0] }

// A won set that takes the last regions makes the win a total conquest, paid at
// the conquest rate for every region the battle and the set took together.
func TestModdedWonSetThatTakesTheLastRegionsIsATotalConquest(t *testing.T) {
	for _, c := range []struct {
		name     string
		edit     func(fx *AttackEffects)
		captured int
		score    int
	}{
		{"destroy", func(fx *AttackEffects) { fx.Target.Destroy = Roll{Pct: 100} }, 100, 100 * 192},
		{"capture", func(fx *AttackEffects) { fx.Target.Capture = Roll{Pct: 100} }, 1000, 1000 * 192},
	} {
		w, a, d, f := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
		moddedConventional(w, EffectConventionalWon, c.edit)
		before := a.Score
		report, captured := w.Attack(a, d, f)
		if d.Land != 0 || d.Alive {
			t.Errorf("%s: target land %d alive %v, want a conquered realm", c.name, d.Land, d.Alive)
		}
		if !strings.Contains(report, "You crushed") {
			t.Errorf("%s: report lacks the total conquest:\n%s", c.name, report)
		}
		if captured != c.captured || a.Score-before != c.score {
			t.Errorf("%s: captured %d score %d, want %d and %d", c.name, captured, a.Score-before, c.captured, c.score)
		}
		if last := w.Battles[len(w.Battles)-1]; !last.Crushed || last.Land != c.captured {
			t.Errorf("%s: battle log %+v, want a conquest of %d regions", c.name, last, c.captured)
		}
	}
}

// A won set's attacker side runs before the capture is placed, so it cannot
// waste or destroy regions the session then parks for the picker.
func TestWonAttackerSetLeavesTheCaptureToBeParked(t *testing.T) {
	w, a, d, f := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	moddedConventional(w, EffectConventionalWon, func(fx *AttackEffects) {
		fx.Attacker.Destroy = Roll{Pct: 50}
		fx.Attacker.Waste = Roll{Pct: 20}
	})
	a.Regions = RegionMix{Agricultural: 100}
	a.syncLand()
	report, captured := w.Attack(a, d, f)
	if !strings.Contains(report, "Victory") || captured == 0 {
		t.Fatalf("setup: the attack should win and capture land:\n%s", report)
	}
	w.ParkUntyped(a, captured)
	if a.Regions.Coastal != 0 || a.PendingRegions != captured {
		t.Errorf("coastal %d pending %d, want 0 and the %d captured", a.Regions.Coastal, a.PendingRegions, captured)
	}
}

// A realm the won set conquers receives nothing back from the attacker set.
func TestConqueredRealmTakesNoCaptureBack(t *testing.T) {
	w, a, d, f := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	moddedConventional(w, EffectConventionalWon, func(fx *AttackEffects) {
		fx.Target.Destroy = Roll{Pct: 100}
		fx.Attacker.Capture = Roll{Pct: 10}
	})
	before := a.Land
	report, captured := w.Attack(a, d, f)
	if !strings.Contains(report, "You crushed") || d.Alive || d.Land != 0 {
		t.Fatalf("land %d alive %v, want a conquest:\n%s", d.Land, d.Alive, report)
	}
	if a.Land != before+captured {
		t.Errorf("attacker land %d, want %d + %d captured", a.Land, before, captured)
	}
}
