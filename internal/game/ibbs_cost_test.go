package game

import "testing"

// The two cost Levels use BRE's own spread, NOT Level.Percent()'s 0/50/100/200.
// Golden literals, per the fidelity rule: mirroring the constants would follow a
// retune silently, and these figures came out of the binary.
func TestAttackGoldCostByLevel(t *testing.T) {
	f := AttackForce{Troopers: 1000, Tanks: 200, Jets: 300, Bombers: 100}
	// 1600 units at the captured 1 gold each.
	for _, tc := range []struct {
		level Level
		want  int64
	}{
		{None, 0},
		{Low, 320},
		{Medium, 1600},
		{High, 4800},
	} {
		cfg := DefaultConfig()
		cfg.AttackCosts = tc.level
		w := NewWorldSeed(cfg, 1)
		if got := w.AttackGoldCost(f); got != tc.want {
			t.Errorf("AttackCosts %s: cost = %d, want %d", tc.level, got, tc.want)
		}
	}
}

// BRE clamps the quoted attack price at 200 million however big the detachment.
func TestAttackGoldCostCapped(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AttackCosts = High
	w := NewWorldSeed(cfg, 1)
	f := AttackForce{Troopers: 500_000_000}
	if got := w.AttackGoldCost(f); got != 200_000_000 {
		t.Errorf("cost = %d, want the 200,000,000 ceiling", got)
	}
}

func TestTerrorOpGoldCostByLevel(t *testing.T) {
	// 5,000 regions at (0+63)=64 gold each is 320,000 at Medium.
	for _, tc := range []struct {
		level Level
		want  int64
	}{
		{None, 0},
		{Low, 64_000},
		{Medium, 320_000},
		{High, 960_000},
	} {
		cfg := DefaultConfig()
		cfg.TerrorCosts = tc.level
		w := NewWorldSeed(cfg, 1)
		e := w.AddHuman("alice", "Alethia")
		e.Land = 5000
		if got := w.TerrorOpGoldCost(e); got != tc.want {
			t.Errorf("TerrorCosts %s: cost = %d, want %d", tc.level, got, tc.want)
		}
	}
}

// BINARY-VERIFIED: the per-region cost rises with each op launched that day.
// capped = clamp(opsToday, 1, 100); cost = (capped + 63) * regions * configMult.
func TestTerrorOpGoldCostByCounter(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TerrorCosts = Medium
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")
	e.Land = 1000

	for _, tc := range []struct {
		opsToday int
		want     int64
	}{
		{0, 64_000},    // clamp to 1 → (1+63) * 1000
		{1, 64_000},    // same: (1+63) * 1000
		{2, 65_000},    // (2+63) * 1000
		{100, 163_000}, // cap: (100+63) * 1000
		{200, 163_000}, // clamped at 100
	} {
		e.TerrorOpsToday = tc.opsToday
		if got := w.TerrorOpGoldCost(e); got != tc.want {
			t.Errorf("opsToday=%d: cost = %d, want %d", tc.opsToday, got, tc.want)
		}
	}
}

// The price is charged, and an op the launcher cannot pay for is refused with
// nothing spent — no agents committed, no packet queued.
func TestSendTerrorChargesTheOp(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.BoardID = "here"
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")
	e.Land, e.Agents, e.Gold = 1000, 10, 100_000

	want := int64(1000 * 64) // Medium
	if err := w.SendTerror(e, "faraway", "Rome", 4, TerrorOpSpy); err != nil {
		t.Fatalf("SendTerror: %v", err)
	}
	if e.Gold != 100_000-want {
		t.Errorf("gold = %d, want %d charged", e.Gold, want)
	}

	e.Gold = want - 1
	if err := w.SendTerror(e, "faraway", "Rome", 4, TerrorOpSpy); err != ErrCantAfford {
		t.Fatalf("a broke launcher: err = %v, want ErrCantAfford", err)
	}
	if e.Agents != 6 {
		t.Errorf("agents = %d, want 6 — a refused op must commit none", e.Agents)
	}
	if len(w.Outbox) != 1 || len(w.Outbox[0].Terrors) != 1 {
		t.Errorf("a refused op queued a packet: %+v", w.Outbox)
	}
}
