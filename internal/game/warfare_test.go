package game

import "testing"

func TestFundSDI(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)
	a.Gold = 100000

	level, err := w.FundSDI(a, 30000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if level != 3 {
		t.Errorf("expected SDI 3, got %d", level)
	}
	if a.Gold != 70000 {
		t.Errorf("expected gold 70000, got %d", a.Gold)
	}
}

func TestFundSDICapsAtMax(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)
	a.Gold = 2_000_000

	level, err := w.FundSDI(a, 1_000_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if level != SDIMax {
		t.Errorf("expected SDI capped at %d, got %d", SDIMax, level)
	}
	wantSpent := SDIMax * SDIStep
	if a.Gold != 2_000_000-wantSpent {
		t.Errorf("expected only %d gold spent, gold now %d", wantSpent, a.Gold)
	}
}

func TestFundSDICantAfford(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)
	a.Gold = 100

	_, err := w.FundSDI(a, 30000)
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

	w1 := NewWorldSeed(cfg, 42)
	a1 := w1.AddHuman("att", "Attacker")
	a1.Gold = 100000
	d1 := w1.Empires[0]
	d1.Protection, a1.Protection = 0, 0
	d1.Land = 1000
	d1.SDI = 0

	w2 := NewWorldSeed(cfg, 42)
	a2 := w2.AddHuman("att", "Attacker")
	a2.Gold = 100000
	d2 := w2.Empires[0]
	d2.Protection, a2.Protection = 0, 0
	d2.Land = 1000
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
	if loss2 > loss1 {
		t.Errorf("expected SDI=50 defender to lose no more land than SDI=0 defender: loss2=%d loss1=%d", loss2, loss1)
	}
}

func TestGooieKablooie(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 2
	w := NewWorldSeed(cfg, 7)
	a := w.AddHuman("att", "Attacker")
	a.Gold = 1_000_000
	a.Protection = 0
	beforeAttackerLand := a.Land

	beforeLand := make([]int, len(w.Empires))
	beforeEvents := make([]int, len(w.Empires))
	for i, e := range w.Empires {
		e.Protection = 0
		beforeLand[i] = e.Land
		beforeEvents[i] = len(e.Events)
	}

	report, err := w.GooieKablooie(a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == "" {
		t.Error("expected a non-empty report")
	}
	if a.Gold != 1_000_000-GooieCost {
		t.Errorf("gold not deducted: got %d", a.Gold)
	}
	if a.Land != beforeAttackerLand {
		t.Errorf("attacker's own land should be unchanged, before=%d after=%d", beforeAttackerLand, a.Land)
	}

	for i, e := range w.Empires {
		if e == a || !e.Alive {
			continue
		}
		if e.Land >= beforeLand[i] {
			t.Errorf("%s: expected land reduced, before=%d after=%d", e.Name, beforeLand[i], e.Land)
		}
		if len(e.Events) != beforeEvents[i]+1 {
			t.Errorf("%s: expected one new victim event, got %d", e.Name, len(e.Events)-beforeEvents[i])
		}
	}
}

func TestGooieKablooieCantAfford(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold = 100
	beforeLand := d.Land
	beforeEvents := len(d.Events)

	_, err := w.GooieKablooie(a)
	if err != ErrCantAfford {
		t.Fatalf("expected ErrCantAfford, got %v", err)
	}
	if d.Land != beforeLand {
		t.Errorf("target land should be unchanged, before=%d after=%d", beforeLand, d.Land)
	}
	if len(d.Events) != beforeEvents {
		t.Errorf("target should have no new events")
	}
}
