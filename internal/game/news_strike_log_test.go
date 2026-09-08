package game

import "testing"

// A WMD strike enters the world-report log alongside conventional battles
// (#233 follow-up). It used to be excluded, which left the report blank in a
// league that fights mostly with missiles.
func TestStrikesAreLoggedForTheWorldReport(t *testing.T) {
	for _, weapon := range []string{"nuclear", "chemical", "biological"} {
		w := NewWorldSeed(DefaultConfig(), 1)
		w.Battles = nil
		a := &Empire{Name: "Attacker"}
		d := &Empire{Name: "Defender"}
		w.postStrikeNews(a, d, weapon)
		if len(w.Battles) != 1 {
			t.Fatalf("%s: got %d log entries, want 1", weapon, len(w.Battles))
		}
		b := w.Battles[0]
		if b.Weapon != weapon {
			t.Errorf("%s: Weapon = %q", weapon, b.Weapon)
		}
		if b.Attacker != "Attacker" || b.Defender != "Defender" {
			t.Errorf("%s: sides wrong: %+v", weapon, b)
		}
		// A strike takes no ground, so it must not read as a land grab.
		if b.Land != 0 || b.Crushed {
			t.Errorf("%s: a strike should take no land and crush nobody: %+v", weapon, b)
		}
	}
}

// A conventional battle still logs with no weapon, so the report can tell the
// two apart -- and so an older board's entries, which carry no Weapon at all,
// keep reading as attacks.
func TestConventionalBattleHasNoWeapon(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Battles = nil
	w.postCombatNews(&Empire{Name: "A"}, &Empire{Name: "D"}, true, false)
	if len(w.Battles) != 1 {
		t.Fatalf("got %d entries, want 1", len(w.Battles))
	}
	if w.Battles[0].Weapon != "" {
		t.Errorf("conventional battle carried a weapon: %q", w.Battles[0].Weapon)
	}
}
