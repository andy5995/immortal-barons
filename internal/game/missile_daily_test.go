package game

import "testing"

// Each of the three missiles is its own once-a-day gate, and they do NOT share
// the bombing allowance. BRE keeps three separate flag bytes on the empire
// record (+0x27e, +0x27f, +0x280), sets one per launch, and clears all three at
// daily maintenance; MaxBombingOps governs the four bombing ops alone.
func TestEachMissileIsOncePerDay(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Config.IBBS, w.Config.MaxBombingOps = true, 5
	e := w.AddHuman("a", "Alpha")
	e.Protection, e.Bombers, e.Gold = 0, BombingBombersRequired, 1<<40
	w.RemoteBoards = []RemoteBoard{{BoardID: "Bravo BBS"}}

	for _, op := range []SpecialOp{OpNuclear, OpChemical, OpSabre} {
		if !w.CanSpecialOp(e, op) {
			t.Fatalf("%s should be available before it is used", op)
		}
		if err := w.SendSpecialOp(e, "Bravo BBS", "Anyone", op, 0); err != nil {
			t.Fatalf("%s first launch: %v", op, err)
		}
		if w.CanSpecialOp(e, op) {
			t.Errorf("%s is still offered after its one launch", op)
		}
		if err := w.SendSpecialOp(e, "Bravo BBS", "Anyone", op, 0); err != ErrMissileSpentToday {
			t.Errorf("%s second launch: %v, want ErrMissileSpentToday", op, err)
		}
	}
	// Spending all three missiles must not have eaten the bombing allowance.
	if e.BombingOpsToday != 0 {
		t.Errorf("missiles charged the bombing allowance: BombingOpsToday = %d", e.BombingOpsToday)
	}
	if !w.CanSpecialOp(e, OpBombFood) {
		t.Error("a bombing op should still be available after all three missiles")
	}
}

// The bombing ops keep their counted allowance, which is a different rule from
// the missiles' one-shot flags.
func TestBombingOpsShareTheCountedAllowance(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Config.IBBS, w.Config.MaxBombingOps = true, 2
	e := w.AddHuman("a", "Alpha")
	e.Protection, e.Bombers, e.Gold = 0, BombingBombersRequired, 1<<40
	w.RemoteBoards = []RemoteBoard{{BoardID: "Bravo BBS"}}

	for i, op := range []SpecialOp{OpBombFood, OpBombMarket} {
		if err := w.SendSpecialOp(e, "Bravo BBS", "", op, 0); err != nil {
			t.Fatalf("bombing op %d: %v", i, err)
		}
	}
	if err := w.SendSpecialOp(e, "Bravo BBS", "", OpBombRoutes, 0); err != ErrBombingOpsExhausted {
		t.Errorf("third bombing op: %v, want ErrBombingOpsExhausted", err)
	}
	// ... and a missile is untouched by that allowance being spent.
	if !w.CanSpecialOp(e, OpNuclear) {
		t.Error("a spent bombing allowance should not block a missile")
	}
}

// Daily maintenance clears all three together, as BRE clears them.
func TestMissilesClearAtDailyMaintenance(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("a", "Alpha")
	// Get past the fresh-game branch, which returns before the daily reset.
	w.DailyMaintenance("2026-09-01")
	e.MissileUsedToday = map[SpecialOp]bool{OpNuclear: true, OpChemical: true, OpSabre: true}

	w.DailyMaintenance("2026-09-02")

	for _, op := range []SpecialOp{OpNuclear, OpChemical, OpSabre} {
		if !w.CanSpecialOp(e, op) {
			t.Errorf("%s is still spent after daily maintenance", op)
		}
	}
}
