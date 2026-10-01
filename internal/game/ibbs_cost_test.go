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
	// 5,000 regions at 63 gold each is 315,000 at Medium — the rate for the day's
	// FIRST op, which IB quotes unclamped so it matches what is charged.
	for _, tc := range []struct {
		level Level
		want  int64
	}{
		{None, 0},
		{Low, 63_000},
		{Medium, 315_000},
		{High, 945_000},
	} {
		cfg := DefaultConfig()
		cfg.TerrorCosts = tc.level
		w := NewWorldSeed(cfg, 1)
		e := w.AddHuman("alice", "Alethia")
		e.Land = 5000
		if got := w.TerrorOpGoldRate(e); got != tc.want {
			t.Errorf("TerrorCosts %s: rate = %d, want %d", tc.level, got, tc.want)
		}
	}
}

// BINARY-VERIFIED: the rate rises by a gold piece a region with each op
// launched that day, and nothing clamps the count — the charge routine adds 63
// to it as it stands. (The original's QUOTE clamps it to 1..100; IB quotes the
// charge, a recorded divergence.)
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
		{0, 63_000},    // (0+63) * 1000 — the original quotes 64 here and charges 63
		{1, 64_000},    // (1+63) * 1000
		{2, 65_000},    // (2+63) * 1000
		{100, 163_000}, // (100+63) * 1000
		{200, 263_000}, // (200+63) * 1000: the charge has no ceiling
	} {
		e.TerrorOpsToday = tc.opsToday
		if got := w.TerrorOpGoldRate(e); got != tc.want {
			t.Errorf("opsToday=%d: rate = %d, want %d", tc.opsToday, got, tc.want)
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
	e.Land, e.Agents, e.Gold = 1000, 10, 1_000_000

	// Four agents at the first-op rate of 63 a region, Medium.
	want := int64(4 * 1000 * 63)
	if _, err := w.SendTerror(e, "faraway", "Rome", 4, TerrorOpSpy); err != nil {
		t.Fatalf("SendTerror: %v", err)
	}
	if e.Gold != 1_000_000-want {
		t.Errorf("gold = %d, want %d charged", e.Gold, want)
	}

	e.Gold = want - 1
	if _, err := w.SendTerror(e, "faraway", "Rome", 4, TerrorOpSpy); err != ErrCantAfford {
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

// Each agent on a terrorist op is one operation: it pays its own share of the
// fee and takes its own slot out of the day's allowance. CAPTURE-VERIFIED
// against cap/eots-ibbs-02.cap, whose four sends the formula reproduces to the
// gold. Golden literals: mirroring the constants would follow a retune silently.
func TestTerrorOpChargesPerAgent(t *testing.T) {
	for _, tc := range []struct {
		agents, opsToday, regions int
		want                      int64
	}{
		{8, 0, 8957, 4_514_328},
		{7, 8, 8957, 4_451_629},
		{7, 0, 6835, 3_014_235},
		{8, 7, 6835, 3_827_600},
	} {
		cfg := DefaultConfig()
		cfg.TerrorCosts = Medium
		w := NewWorldSeed(cfg, 1)
		e := w.AddHuman("alice", "Alethia")
		e.Land, e.TerrorOpsToday = tc.regions, tc.opsToday
		if got := w.TerrorOpGoldCost(e, tc.agents); got != tc.want {
			t.Errorf("%d agents, %d ops used, %d regions: cost = %d, want the captured %d",
				tc.agents, tc.opsToday, tc.regions, got, tc.want)
		}
	}
}

// The charge is worked in the original's six-byte Real, regions divided by 100
// first, and truncated. BINARY-VERIFIED (BRE.OVR 0x2ad8d); the expected figures
// come from scripts/bre_real48.py, the port of BRE's own runtime, and are
// golden literals. Two of them are not what exact arithmetic gives: High on
// 8,957 regions loses a gold piece to rounding (exact: 1,692,873), and Low on
// 7 regions keeps the fraction a per-agent floor would drop (IB billed 704).
func TestTerrorOpChargeRoundsAsTheOriginal(t *testing.T) {
	for _, tc := range []struct {
		level                     Level
		agents, opsToday, regions int
		want                      int64
	}{
		{High, 1, 0, 8957, 1_692_872},
		{Low, 8, 0, 7, 705},
		{Low, 8, 0, 8957, 902_865},
		{Low, 8, 7, 6835, 765_520},
		{None, 8, 0, 8957, 0},
	} {
		cfg := DefaultConfig()
		cfg.TerrorCosts = tc.level
		w := NewWorldSeed(cfg, 1)
		e := w.AddHuman("alice", "Alethia")
		e.Land, e.TerrorOpsToday = tc.regions, tc.opsToday
		if got := w.TerrorOpGoldCost(e, tc.agents); got != tc.want {
			t.Errorf("%s, %d agents, %d ops used, %d regions: cost = %d, want %d",
				tc.level, tc.agents, tc.opsToday, tc.regions, got, tc.want)
		}
	}
}

// And the allowance is counted in AGENTS, so eight sent out of fifteen leaves
// seven — the number the original's next prompt offers.
func TestTerrorOpAllowanceCountsAgents(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID, cfg.MaxTerrorOps = true, "boardA", 15
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")
	e.Land, e.Agents, e.Gold, e.Protection = 1000, 100, 100_000_000_000, 0
	w.RemoteBoards = []RemoteBoard{{BoardID: "boardB"}}

	if _, err := w.SendTerror(e, "boardB", "Victim", 8, TerrorOpDemoralize); err != nil {
		t.Fatalf("send 8: %v", err)
	}
	if e.TerrorOpsToday != 8 {
		t.Errorf("eight agents should spend eight ops, got %d", e.TerrorOpsToday)
	}
	if got := w.TerrorOpsLeft(e); got != 7 {
		t.Errorf("allowance left = %d, want 7", got)
	}
	// Eight asked with seven left sends the seven.
	if n, err := w.SendTerror(e, "boardB", "Victim", 8, TerrorOpDemoralize); err != nil || n != 7 {
		t.Fatalf("eight asked with seven left: sent %d, %v; want 7 sent", n, err)
	}
	if _, err := w.SendTerror(e, "boardB", "Victim", 1, TerrorOpDemoralize); err != ErrTerrorOpsExhausted {
		t.Errorf("a send with none left: %v, want ErrTerrorOpsExhausted", err)
	}
	if w.CanTerrorOp(e) {
		t.Error("the day's allowance is spent; CanTerrorOp should be false")
	}
}

// The engine clamps a send to what can go: at most TerrorAgentsPerSendMax at
// once, and no more agents than are held, which can fall while the prompt is
// open when an arriving Bomb Intelligence is processed.
func TestSendTerrorClampsToWhatCanGo(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID, cfg.MaxTerrorOps = true, "boardA", 0
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")
	e.Land, e.Agents, e.Gold, e.Protection = 1000, 1000, 100_000_000_000, 0
	w.RemoteBoards = []RemoteBoard{{BoardID: "boardB"}}

	if n, err := w.SendTerror(e, "boardB", "Victim", 256, TerrorOpDemoralize); err != nil || n != 255 {
		t.Errorf("256 asked: sent %d, %v; want 255", n, err)
	}
	if e.Agents != 1000-255 || e.TerrorOpsToday != 255 {
		t.Errorf("after the capped send: agents %d, ops %d", e.Agents, e.TerrorOpsToday)
	}
	e.Agents = 3
	gold, cost3 := e.Gold, w.TerrorOpGoldCost(e, 3)
	if n, err := w.SendTerror(e, "boardB", "Victim", 5, TerrorOpDemoralize); err != nil || n != 3 {
		t.Errorf("five asked with three held: sent %d, %v; want 3", n, err)
	}
	if e.Gold != gold-cost3 {
		t.Errorf("charged %d, want the price of three agents, %d", gold-e.Gold, cost3)
	}
	if _, err := w.SendTerror(e, "boardB", "Victim", 1, TerrorOpDemoralize); err != ErrNoAgents {
		t.Errorf("a send with no agents: %v, want ErrNoAgents", err)
	}
}
