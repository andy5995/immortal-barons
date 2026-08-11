package game

import "testing"

// Golden literals from BRE.OVR 0xF37B: effectiveness is morale x 0.6 + 50, so a
// fully motivated army fights ABOVE par at 110% and a broken one at 50%.
func TestMoraleFactor(t *testing.T) {
	for _, c := range []struct{ morale, want int }{{0, 50}, {50, 80}, {100, 110}} {
		if got := moraleFactor(c.morale); got != c.want {
			t.Errorf("moraleFactor(%d) = %d, want %d", c.morale, got, c.want)
		}
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
		w.GrowFood(e) // turn-start food yield, so the realm stays fed across the loop
		w.PlayTurn(e, "2026-07-03")
	}
	if e.Morale <= 60 {
		t.Errorf("morale should recover over quiet, paid turns, got %d", e.Morale)
	}
}
