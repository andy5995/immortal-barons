package game

import "testing"

func TestMoraleFactor(t *testing.T) {
	if got := moraleFactor(100); got != 100 {
		t.Errorf("moraleFactor(100) = %d, want 100", got)
	}
	if got := moraleFactor(0); got != MoraleCombatFloor {
		t.Errorf("moraleFactor(0) = %d, want %d (floor)", got, MoraleCombatFloor)
	}
	if got := moraleFactor(50); got != (MoraleCombatFloor+100)/2 {
		t.Errorf("moraleFactor(50) = %d, want %d", got, (MoraleCombatFloor+100)/2)
	}
}

func TestBoostMoraleCapsPerTurn(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Morale = 40
	e.Gold = 10_000_000

	pts := w.BoostMorale(e, 1_000_000) // far more than enough to max it
	if pts != MaxMoraleBoostPerTurn {
		t.Errorf("one turn's morale boost = %d, want the per-turn cap %d", pts, MaxMoraleBoostPerTurn)
	}
	if e.Morale != 40+MaxMoraleBoostPerTurn {
		t.Errorf("morale = %d, want %d (capped rise from 40)", e.Morale, 40+MaxMoraleBoostPerTurn)
	}
}

func TestPayForcesShortfallLowersMorale(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Morale = 100
	e.Troopers = 1000
	e.Gold = 1_000_000
	req := w.ForcesDue(e)
	if req == 0 {
		t.Skip("no forces upkeep to underpay")
	}
	w.PayForces(e, 0) // pay nothing
	if e.Morale >= 100 {
		t.Errorf("unpaid forces should lower morale, got %d", e.Morale)
	}
}

func TestLowMoraleCausesDesertion(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Morale = 20 // at/below the desert threshold even after a turn's drift
	e.Troopers = 1000
	e.Gold = 10_000_000 // plenty, so this isn't non-payment desertion
	before := e.Troopers

	w.PlayTurn(e, "2026-07-03")

	if e.Troopers >= before {
		t.Errorf("low morale should shrink the army: troopers %d -> %d", before, e.Troopers)
	}
	if e.LastMoraleDesertion <= 0 {
		t.Errorf("LastMoraleDesertion = %d, want > 0", e.LastMoraleDesertion)
	}
}

func TestMoraleRecoversOverTurns(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Morale = 60
	e.Gold = 10_000_000 // pay upkeep freely so nothing drags morale down
	for i := 0; i < 10; i++ {
		w.PlayTurn(e, "2026-07-03")
	}
	if e.Morale <= 60 {
		t.Errorf("morale should recover over quiet, paid turns, got %d", e.Morale)
	}
}
