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
	w.postCombatNews(&Empire{Name: "A"}, &Empire{Name: "D"}, true, false, 0)
	if len(w.Battles) != 1 {
		t.Fatalf("got %d entries, want 1", len(w.Battles))
	}
	if w.Battles[0].Weapon != "" {
		t.Errorf("conventional battle carried a weapon: %q", w.Battles[0].Weapon)
	}
}

// Two real battles between the same realms on the same day must both reach the
// other boards, while a resent packet must still add nothing. The merge drops
// exact matches, which once collapsed the repeats into one entry.
func TestRepeatedBattlesSurviveTheMerge(t *testing.T) {
	src := NewWorldSeed(DefaultConfig(), 1)
	src.Config.BoardID = "Alpha"
	src.Battles = nil
	for i := 0; i < 3; i++ {
		src.postCombatNews(&Empire{Name: "A"}, &Empire{Name: "D"}, true, false, 7)
	}
	dst := NewWorldSeed(DefaultConfig(), 1)
	dst.Battles = nil
	dst.mergeBattles("Alpha", src.ownBattles())
	dst.mergeBattles("Alpha", src.ownBattles()) // the resend
	if len(dst.Battles) != 3 {
		t.Fatalf("merged %d entries, want 3", len(dst.Battles))
	}
	if dst.Battles[0].Land != 7 {
		t.Errorf("Land = %d, want 7", dst.Battles[0].Land)
	}
}

// battleBoards is a sending and a receiving board with an empty log each.
func battleBoards() (src, dst *World) {
	src = NewWorldSeed(DefaultConfig(), 1)
	src.Config.BoardID = "Alpha"
	src.Battles = nil
	dst = NewWorldSeed(DefaultConfig(), 1)
	dst.Battles = nil
	return src, dst
}

// An entry the cap has already pushed out of the receiver's log must not come
// back as the newest when the sender resends it.
func TestTrimmedBattlesStayTrimmed(t *testing.T) {
	src, dst := battleBoards()
	for i := 0; i < 3; i++ {
		src.postCombatNews(&Empire{Name: "A"}, &Empire{Name: "D"}, true, false, i)
	}
	dst.mergeBattles("Alpha", src.ownBattles())
	dst.Battles = dst.Battles[2:] // as if the cap had pushed out the first two
	dst.mergeBattles("Alpha", src.ownBattles())
	if len(dst.Battles) != 1 || dst.Battles[0].Land != 2 {
		t.Fatalf("got %+v, want only the newest entry", dst.Battles)
	}
}

// A reset keeps the board's numbering, so the other boards, which remember the
// highest number they took from it, still take its battles afterward.
func TestBattlesAfterAResetStillArrive(t *testing.T) {
	src, dst := battleBoards()
	src.postCombatNews(&Empire{Name: "A"}, &Empire{Name: "D"}, true, false, 1)
	dst.mergeBattles("Alpha", src.ownBattles())
	src.ResetForLeague("11/12/26")
	src.postCombatNews(&Empire{Name: "A"}, &Empire{Name: "D"}, true, false, 1)
	dst.mergeBattles("Alpha", src.ownBattles())
	if len(dst.Battles) != 2 {
		t.Fatalf("merged %d entries, want 2", len(dst.Battles))
	}
}

// An arriving entry keeps the date it was fought on, even an empty one: filling
// in the receiver's date made a legacy entry differ from its own resend.
func TestMergedBattleKeepsItsDate(t *testing.T) {
	_, dst := battleBoards()
	dst.LastMaintDate = "11/12/26"
	in := []BattleLogEntry{{Planet: "Alpha", Attacker: "A", Defender: "D"}}
	dst.mergeBattles("Alpha", in)
	dst.mergeBattles("Alpha", in)
	if len(dst.Battles) != 1 || dst.Battles[0].Date != "" {
		t.Fatalf("got %+v, want one entry with no date", dst.Battles)
	}
}

// Only the signed sender's own entries are taken in, each number once: Battles
// is unsigned, so an entry naming another board, or repeated, came from a hop.
func TestMergeTakesOnlyTheSendersEntriesOnce(t *testing.T) {
	_, dst := battleBoards()
	dst.mergeBattles("Alpha", []BattleLogEntry{
		{Planet: "Beta", Attacker: "X", Seq: 9},
		{Planet: "Alpha", Attacker: "A", Seq: 1},
		{Planet: "Alpha", Attacker: "A", Seq: 1},
	})
	if len(dst.Battles) != 1 || dst.Battles[0].Planet != "Alpha" || len(dst.BattleSeen) != 1 {
		t.Fatalf("got %+v, marks %v; want one Alpha entry", dst.Battles, dst.BattleSeen)
	}
}
