package game

import "testing"

// The two cost Levels use BRE's own spread, NOT the even 0/50/100/200 ladder
// these knobs once shared.
// Golden literals, per the fidelity rule: mirroring the constants would follow a
// retune silently, and these figures came out of the binary.
func TestAttackGoldCostByLevel(t *testing.T) {
	f := AttackForce{Troopers: 1000, Tanks: 200, Jets: 300, Bombers: 100}
	// A realm too small for any of the four divisors to bite pays the addends
	// alone: 1000x1 + 200x2 + 300x2 + 100x1 = 2100 gold at Medium.
	for _, tc := range []struct {
		level Level
		want  int64
	}{
		{None, 0},
		{Low, 420},
		{Medium, 2100},
		{High, 6300},
	} {
		cfg := DefaultConfig()
		cfg.AttackCosts = tc.level
		w := NewWorldSeed(cfg, 1)
		e := w.AddHuman("a", "Alpha")
		e.Regions = RegionMix{}
		if got := w.AttackGoldCost(e, f); got != tc.want {
			t.Errorf("AttackCosts %s: cost = %d, want %d", tc.level, got, tc.want)
		}
	}
}

// The whole quoted price, against a live capture (#252). A realm of 9,003
// regions joining with 12,141 troopers, 12,378,520 jets, no tanks and 105,520
// bombers is quoted 99,572,437 gold in cap/eots-ibbs-02.cap. A golden literal:
// mirroring the constants would follow a retune silently.
func TestAttackGoldCostMatchesTheCapturedQuote(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AttackCosts = Medium
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("a", "Alpha")
	e.Regions = RegionMix{Desert: 9003}

	f := AttackForce{Troopers: 12_141, Jets: 12_378_520, Bombers: 105_520}
	if got := w.AttackGoldCost(e, f); got != 99_572_437 {
		t.Errorf("cost = %d, want the captured 99,572,437", got)
	}
}

// BRE clamps the quoted attack price at 200 million however big the detachment.
func TestAttackGoldCostCapped(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AttackCosts = High
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("a", "Alpha")
	f := AttackForce{Troopers: 500_000_000}
	if got := w.AttackGoldCost(e, f); got != 200_000_000 {
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

// A group attack is charged at both ends (#252): creating one and joining one
// each pay the same price a lone strike of that weight pays. Until this was
// fixed the group attack was the free way to move an army between planets.
func TestGroupAttackIsChargedAtBothEnds(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "boardA"
	w := NewWorldSeed(cfg, 1)
	leader := w.AddHuman("l", "Leader")
	ally := w.AddHuman("a", "Ally")
	for _, e := range []*Empire{leader, ally} {
		e.Regions = RegionMix{Desert: 5000}
		e.Troopers, e.Gold = 100_000, 1_000_000
	}

	f := AttackForce{Troopers: 40_000}
	want := w.AttackGoldCost(leader, f)
	if want == 0 {
		t.Fatal("this test needs a non-zero price to prove anything")
	}

	ga, err := w.CreateGroupAttack(leader, "boardB", "Victim", GroupAttackHoursMin, f)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if leader.Gold != 1_000_000-want {
		t.Errorf("creating cost %d, want %d", 1_000_000-leader.Gold, want)
	}
	if err := w.JoinGroupAttack(ally, ga.ID, f); err != nil {
		t.Fatalf("join: %v", err)
	}
	if ally.Gold != 1_000_000-want {
		t.Errorf("joining cost %d, want %d", 1_000_000-ally.Gold, want)
	}

	// And a baron who cannot pay is refused, with the units left where they are.
	broke := w.AddHuman("b", "Broke")
	broke.Regions, broke.Troopers, broke.Gold = RegionMix{Desert: 5000}, 100_000, want-1
	if err := w.JoinGroupAttack(broke, ga.ID, f); err != ErrCantAfford {
		t.Errorf("a baron who cannot pay: %v, want ErrCantAfford", err)
	}
	if broke.Troopers != 100_000 {
		t.Errorf("a refused join still committed units: %d troopers left", broke.Troopers)
	}
}
