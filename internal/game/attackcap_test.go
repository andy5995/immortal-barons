package game

import "testing"

func TestIndividualAttackCap(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1) // MaxIndividualAttacks defaults to 3
	a := w.AddHuman("att", "Attacker")
	d := w.AddHuman("def", "Defender")
	// A big attacker and a big, deep defender so the defender survives several
	// attacks (elimination would end the test early).
	a.Troopers = 500_000
	d.Regions = RegionMix{Mountain: 5000}
	d.syncLand()
	d.Turrets = 2_000_000

	for i := 1; i <= 3; i++ {
		if !w.CanAttack(a) {
			t.Fatalf("attack %d should be allowed", i)
		}
		w.Attack(a, d, FullForce(a))
		if a.AttacksToday != i {
			t.Fatalf("after attack %d, AttacksToday=%d", i, a.AttacksToday)
		}
	}
	if w.CanAttack(a) {
		t.Error("a 4th attack should be blocked by the daily cap")
	}
	if got := w.AttacksLeft(a); got != 0 {
		t.Errorf("AttacksLeft=%d, want 0", got)
	}

	// Disabled cap means unlimited.
	w.Config.MaxIndividualAttacks = 0
	if !w.CanAttack(a) {
		t.Error("cap 0 should mean unlimited attacks")
	}
	if got := w.AttacksLeft(a); got != -1 {
		t.Errorf("AttacksLeft with cap disabled=%d, want -1", got)
	}
}

func TestAttackCapResetsAtDailyMaintenance(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.AttacksToday = 3
	w.LastMaintDate = "2026-07-16"
	w.DailyMaintenance("2026-07-17")
	if e.AttacksToday != 0 {
		t.Errorf("AttacksToday=%d after a day rolled over, want 0", e.AttacksToday)
	}
}
