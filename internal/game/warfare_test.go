package game

import "testing"

func TestFundSDI(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)
	a.Gold = 100_000_000

	level, err := w.FundSDI(a, 3*SDIStep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if level != 3 {
		t.Errorf("expected SDI 3, got %d", level)
	}
	if want := 100_000_000 - 3*SDIStep; a.Gold != want {
		t.Errorf("expected gold %d, got %d", want, a.Gold)
	}
}

func TestFundSDICapsAtMax(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)
	a.Gold = 200_000_000

	level, err := w.FundSDI(a, 200_000_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if level != SDIMax {
		t.Errorf("expected SDI capped at %d, got %d", SDIMax, level)
	}
	wantSpent := SDIMax * SDIStep
	if a.Gold != 200_000_000-wantSpent {
		t.Errorf("expected only %d gold spent, gold now %d", wantSpent, a.Gold)
	}
}

func TestFundSDICantAfford(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)
	a.Gold = 100

	_, err := w.FundSDI(a, 3*SDIStep)
	if err != ErrCantAfford {
		t.Fatalf("expected ErrCantAfford, got %v", err)
	}
	if a.Gold != 100 {
		t.Errorf("gold should be unchanged, got %d", a.Gold)
	}
}

func TestNuclearStrikeSDIReducesDamage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1

	// The defenders need REAL regions, not a bare Land figure: the strike ends
	// in syncLand, which recomputes Land from the mix, so a fixture that only
	// sets Land measures the fixture's lie, wipes the realm in both worlds, and
	// makes the two losses equal no matter what SDI does (the old shape of this
	// test).
	w1 := NewWorldSeed(cfg, 42)
	a1 := w1.AddHuman("att", "Attacker")
	a1.Gold = 10_000_000
	d1 := w1.Empires[0]
	d1.Protection, a1.Protection = 0, 0
	d1.Regions = RegionMix{Agricultural: 1000}
	d1.syncLand()
	d1.SDI = 0

	w2 := NewWorldSeed(cfg, 42)
	a2 := w2.AddHuman("att", "Attacker")
	a2.Gold = 10_000_000
	d2 := w2.Empires[0]
	d2.Protection, a2.Protection = 0, 0
	d2.Regions = RegionMix{Agricultural: 1000}
	d2.syncLand()
	d2.SDI = 50

	beforeLand1, beforeLand2 := d1.Land, d2.Land

	if _, err := w1.NuclearStrike(a1, d1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := w2.NuclearStrike(a2, d2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loss1 := beforeLand1 - d1.Land
	loss2 := beforeLand2 - d2.Land

	if loss1 <= 0 {
		t.Errorf("expected SDI=0 defender to lose land, lost %d", loss1)
	}
	// Strictly less: both worlds share seed 42 and an identical call sequence,
	// so if the SDI mitigation were deleted the losses would be EQUAL and a
	// non-strict check would pass with the mechanic gone.
	if loss2 >= loss1 {
		t.Errorf("expected SDI=50 defender to lose strictly less land than SDI=0 defender: loss2=%d loss1=%d", loss2, loss1)
	}
}
