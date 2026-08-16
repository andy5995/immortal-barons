package game

import (
	"errors"
	"strings"
	"testing"
)

// A spy that gets through reports on the target and costs nothing. No agent
// count guarantees this any more — BRE's roll ignores both sides' agents (see
// spySuccess) — so the test spies until one lands.
func TestSendSpySuccess(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 50
	d.Agents = 0

	for i := 0; i < 50; i++ {
		before := a.Agents
		report, err := w.SendSpy(a, d)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(report, d.Name) {
			continue // caught; try again
		}
		if a.Agents != before {
			t.Errorf("a spy that got through cost an agent: %d, want %d", a.Agents, before)
		}
		return
	}
	t.Fatal("50 spies in a row were all caught; the roll is not landing near 55%")
}

// TestSpyRollIgnoresTheTarget pins the defect IB reproduces from BRE: the Send
// Spy roll draws both of its terms from the attacker, so neither realm's agent
// count moves the odds and every spy is a coin flip on top of a flat one-in-ten
// free pass. Several seeds, since one seed is one trajectory.
func TestSpyRollIgnoresTheTarget(t *testing.T) {
	const trials = 2000
	rate := func(seed int64, attackerAgents, targetAgents int) float64 {
		w := NewWorldSeed(DefaultConfig(), seed)
		a := w.AddHuman("alice", "Alethia")
		d := w.AddHuman("bob", "Bobland")
		a.Agents, d.Agents = attackerAgents, targetAgents
		won := 0
		for i := 0; i < trials; i++ {
			if w.spySuccess(a, d) {
				won++
			}
		}
		return float64(won) / trials
	}
	for _, seed := range []int64{1, 7, 99, 2026} {
		naked := rate(seed, 1, 1_000_000)
		swamped := rate(seed, 1_000_000, 1)
		for _, tc := range []struct {
			name string
			got  float64
		}{{"one agent against a million", naked}, {"a million against one", swamped}} {
			if tc.got < 0.50 || tc.got > 0.60 {
				t.Errorf("seed %d, %s: success rate %.3f, want near 0.55", seed, tc.name, tc.got)
			}
		}
	}
}

// Covert ops charge their BRE gold cost per op (on top of the agent risk), and
// an op the attacker can't afford does nothing.
func TestCovertOpChargesGold(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 50
	d.Agents = 0
	a.Gold = 10_000
	if _, err := w.SendSpy(a, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Gold != 10_000-CostSendSpy {
		t.Errorf("Send Spy should charge %d gold: Gold = %d, want %d", CostSendSpy, a.Gold, 10_000-CostSendSpy)
	}
}

func TestCovertOpTooPoor(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 50
	a.Gold = CostBribery - 1 // can't cover Bribery's fee
	agentsBefore := a.Agents
	if _, err := w.Bribery(a, d); !errors.Is(err, ErrCantAfford) {
		t.Fatalf("expected ErrCantAfford when too poor, got %v", err)
	}
	if a.Gold != CostBribery-1 || a.Agents != agentsBefore {
		t.Error("a failed (unaffordable) op must charge no gold and lose no agent")
	}
}

// BRE caps EFFECT covert ops at one per turn ("Limit one try per turn!"): the
// first works, any second effect op (of any type) is blocked, but the info ops
// (Send Spy, Spy on Relations) stay exempt. The cap clears next turn.
func TestCovertEffectOpCappedOncePerTurn(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents = 50, 0 // d has no agents, so ops reliably succeed
	a.Gold = 10_000_000

	if _, err := w.StirRevolts(a, d); err != nil {
		t.Fatalf("first effect op should work: %v", err)
	}
	if _, err := w.SetUp(a, d); !errors.Is(err, ErrCovertCapReached) {
		t.Fatalf("a second effect op (any type) should be capped, got %v", err)
	}
	if _, err := w.SendSpy(a, d); err != nil {
		t.Errorf("Send Spy is an info op, exempt from the cap: %v", err)
	}
	if _, err := w.SpyOnRelations(a, d); err != nil {
		t.Errorf("Spy on Relations is an info op, exempt from the cap: %v", err)
	}
	a.TurnProgress = TurnProgress{} // next turn
	if _, err := w.StirRevolts(a, d); err != nil {
		t.Errorf("the cap should reset next turn: %v", err)
	}
}

func TestSendSpyNoAgents(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 0

	_, err := w.SendSpy(a, d)
	if !errors.Is(err, ErrNoAgents) {
		t.Fatalf("expected ErrNoAgents, got %v", err)
	}
}

func TestSupportDissensionsSuccess(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 50
	d.Agents = 0
	d.Troopers = 1000
	beforeEvents := len(d.Events)

	report, err := w.SupportDissensions(a, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == "" {
		t.Error("expected a non-empty attacker report")
	}
	if d.Troopers != 900 {
		t.Errorf("expected 900 troopers remaining, got %d", d.Troopers)
	}
	if len(d.Events) != beforeEvents+1 {
		t.Fatalf("expected one victim event, got %d new", len(d.Events)-beforeEvents)
	}
}

func TestSupportDissensionsNoAgents(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 0

	_, err := w.SupportDissensions(a, d)
	if !errors.Is(err, ErrNoAgents) {
		t.Fatalf("expected ErrNoAgents, got %v", err)
	}
}

// A caught spy costs the agent and alerts the victim. Stacking agents cannot
// force the branch any more, so the test spies until one is caught.
func TestSendSpyFailure(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 50
	d.Agents = 1000000

	for i := 0; i < 50; i++ {
		before, beforeEvents := a.Agents, len(d.Events)
		report, err := w.SendSpy(a, d)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(report, "caught") {
			continue // got through; try again
		}
		if a.Agents != before-1 {
			t.Errorf("caught spy: attacker has %d agents, want %d", a.Agents, before-1)
		}
		if len(d.Events) != beforeEvents+1 {
			t.Fatalf("caught spy raised %d victim events, want 1", len(d.Events)-beforeEvents)
		}
		return
	}
	t.Fatal("50 spies in a row all got through; the roll is not landing near 55%")
}

// The effect ops still weigh attacker against defender (covertSuccess), so one
// agent against a million fails on any seed and the branch stays deterministic.
func TestSupportDissensionsFailure(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 1
	d.Agents = 1000000
	d.Troopers = 1000
	beforeEvents := len(d.Events)

	report, err := w.SupportDissensions(a, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == "" {
		t.Error("expected a non-empty attacker report")
	}
	if d.Troopers != 1000 {
		t.Errorf("expected troopers unchanged at 1000, got %d", d.Troopers)
	}
	if a.Agents != 0 {
		t.Errorf("expected attacker to lose the caught agent, got %d agents", a.Agents)
	}
	if len(d.Events) != beforeEvents+1 {
		t.Fatalf("expected one victim event, got %d new", len(d.Events)-beforeEvents)
	}
}

func TestBombFoodDestroysReserve(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents, d.Food = 50, 0, 1000
	if _, err := w.BombFood(a, d); err != nil {
		t.Fatal(err)
	}
	if d.Food != 500 {
		t.Errorf("expected 500 food after a 50%% strike, got %d", d.Food)
	}
}

func TestStirRevoltsLowersSupport(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents, d.Support = 50, 0, 100
	if _, err := w.StirRevolts(a, d); err != nil {
		t.Fatal(err)
	}
	// Golden literal from BRE.OVR 0x4AE61: support is SCALED by 11/13, not
	// docked a flat number of points.
	if d.Support != 84 {
		t.Errorf("expected support 84, got %d", d.Support)
	}
}

func TestCovertOpNeedsAnAgent(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 0
	if _, err := w.DemoralizeForces(a, d); !errors.Is(err, ErrNoAgents) {
		t.Errorf("expected ErrNoAgents, got %v", err)
	}
}

func TestDemoralizeForcesLowersMorale(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents, d.Morale = 50, 0, 100
	if _, err := w.DemoralizeForces(a, d); err != nil {
		t.Fatal(err)
	}
	if d.Morale != 85 {
		t.Errorf("expected morale 85, got %d", d.Morale)
	}
}

func TestSetUpVoidsAlliance(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	a.Gold = 10_000_000 // afford covert op fees
	a.Agents = 1_000_000
	d := w.AddHuman("d", "Delta")
	d.Agents = 0
	partner := w.AddHuman("p", "PartnerLand")
	w.ProposeTreaty(d, partner, "Full Defense Alliance")
	w.AcceptTreaty(partner, d.Name, "Full Defense Alliance")

	report, err := w.SetUp(a, d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, partner.Name) {
		t.Errorf("expected report to name the tricked ally %q, got %q", partner.Name, report)
	}
	if w.HasTreaty(d, partner, "Full Defense Alliance") {
		t.Error("expected the Full Defense Alliance to be voided")
	}
}

func TestExposeEnemyOpsShieldsAgainstCovertOps(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	a.Gold = 10_000_000 // afford covert op fees
	a.Agents = 1
	d := w.AddHuman("d", "Delta") // unused target, required by the action signature

	if _, err := w.ExposeEnemyOps(a, d); err != nil {
		t.Fatal(err)
	}

	attacker := w.AddHuman("e", "Enemyland")
	attacker.Agents = 1_000_000
	attacker.Gold = 10_000_000
	before := a.Troopers
	if _, err := w.SupportDissensions(attacker, a); err != nil {
		t.Fatal(err)
	}
	if a.Troopers != before {
		t.Errorf("Expose Enemy Ops should shield a from incoming covert ops, troopers %d -> %d", before, a.Troopers)
	}
}

func TestBombTradingMarketDrainsGold(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents, d.Gold = 50, 0, 1000
	if _, err := w.BombTradingMarket(a, d); err != nil {
		t.Fatal(err)
	}
	if d.Gold != 750 {
		t.Errorf("expected 750 gold after a 25%% strike, got %d", d.Gold)
	}
}

func TestBombTradeRoutesSeversTreaty(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	a.Gold = 10_000_000 // afford covert op fees
	a.Agents = 1_000_000
	d := w.AddHuman("d", "Delta")
	d.Agents = 0
	partner := w.AddHuman("p", "PartnerLand")
	w.ProposeTreaty(d, partner, "Free Trade Agreement")
	w.AcceptTreaty(partner, d.Name, "Free Trade Agreement")

	if _, err := w.BombTradeRoutes(a, d); err != nil {
		t.Fatal(err)
	}
	if w.HasTreaty(d, partner, "Free Trade Agreement") {
		t.Error("expected the Free Trade Agreement to be severed")
	}
}

func TestUndermineInvestmentsReducesPrincipal(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents = 50, 0
	d.Investments = []Investment{{Amount: 1000, Return: 1100, MaturesDay: 5}}
	if _, err := w.UndermineInvestments(a, d); err != nil {
		t.Fatal(err)
	}
	if d.Investments[0].Amount != 750 {
		t.Errorf("expected 750 principal remaining, got %d", d.Investments[0].Amount)
	}
}

func TestSlappenheimerStrikeDamagesTarget(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	// Give the attacker overwhelming agents so covertSuccess always passes, no
	// Troopers on the target so it can never backfire, and a stock of every
	// strikeable resource. Only ~3 in 10 launches land, so fire many.
	a.Agents, d.Agents, d.Troopers, d.SDI = 50, 0, 0, 0
	d.Jets, d.Turrets, d.Tanks, d.Bombers, d.Carriers, d.Gold, d.Food = 1000, 1000, 1000, 1000, 1000, 1000, 1000
	before := int64(d.Jets+d.Turrets+d.Tanks+d.Bombers+d.Carriers+d.Agents+d.Food) + d.Gold
	for i := 0; i < 100; i++ {
		if _, err := w.SlappenheimerStrike(a, d); err != nil {
			t.Fatal(err)
		}
	}
	after := int64(d.Jets+d.Turrets+d.Tanks+d.Bombers+d.Carriers+d.Agents+d.Food) + d.Gold
	if after >= before {
		t.Errorf("expected R5-Slappenheimer strikes to reduce the target's resources, before=%d after=%d", before, after)
	}
}

func TestSpyOnRelationsRevealsTreaties(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	a.Gold = 10_000_000 // afford covert op fees
	a.Agents = 50
	d := w.AddHuman("d", "Delta")
	d.Agents = 0
	c := w.AddHuman("c", "Gamma")
	w.ProposeTreaty(d, c, "Tariff Trade Agreement")
	w.AcceptTreaty(c, d.Name, "Tariff Trade Agreement")

	report, err := w.SpyOnRelations(a, d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, "Tariff Trade Agreement") || !strings.Contains(report, "Gamma") {
		t.Errorf("report should reveal Delta's treaty with Gamma, got: %s", report)
	}
}

func TestBriberyGrantsImmunity(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	a.Gold = 10_000_000 // afford covert op fees
	a.Agents = 1_000_000
	d := w.AddHuman("d", "Delta")
	d.Agents = 4
	if _, err := w.Bribery(a, d); err != nil {
		t.Fatal(err)
	}
	// d now cannot land covert ops on a, even with overwhelming agents.
	d.Agents = 1_000_000
	d.Gold = 10_000_000
	before := a.Troopers
	if _, err := w.SupportDissensions(d, a); err != nil {
		t.Fatal(err)
	}
	if a.Troopers != before {
		t.Errorf("d's ops should fail from a's bribery immunity, troopers %d -> %d", before, a.Troopers)
	}
}

// Gold is one of the R5-Slappenheimer's targets. It is held in money width, so
// it sits outside the loop over the count-width assets — easy to leave out of
// the target roll entirely, which would quietly make the weapon weaker.
func TestSlappenheimerCanStrikeGold(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents, d.Troopers, d.SDI = 50, 0, 0, 0
	d.Gold = 1_000_000
	before := d.Gold
	for i := 0; i < 300; i++ {
		if _, err := w.SlappenheimerStrike(a, d); err != nil {
			t.Fatal(err)
		}
		if d.Gold < before {
			return // it reached gold at least once
		}
	}
	t.Errorf("300 landed-or-fizzled launches never touched gold (still %d)", d.Gold)
}

// BINARY-VERIFIED: a caught agent gives the attacker away; an operation that
// succeeds does not (BRE.OVR 0x016d67 for the local spy report, 0x04a96b and
// 0x04a09a for the two interplanetary paths). The whole family has to carry it,
// so this walks every covert op rather than the one that was changed first — a
// sibling left on the old wording would still pass a single-op test.
func TestFoiledCovertOpsNameTheAttacker(t *testing.T) {
	ops := []struct {
		name string
		run  func(*World, *Empire, *Empire) (string, error)
	}{
		{"SendSpy", (*World).SendSpy},
		{"SupportDissensions", (*World).SupportDissensions},
		{"DemoralizeForces", (*World).DemoralizeForces},
		{"SetUp", (*World).SetUp},
		{"SpyOnRelations", (*World).SpyOnRelations},
		{"Bribery", (*World).Bribery},
		{"StirRevolts", (*World).StirRevolts},
		{"BombFood", (*World).BombFood},
		{"BombTradingMarket", (*World).BombTradingMarket},
		{"BombTradeRoutes", (*World).BombTradeRoutes},
		{"UndermineInvestments", (*World).UndermineInvestments},
		{"SlappenheimerStrike", (*World).SlappenheimerStrike},
	}
	for _, op := range ops {
		w, a, d := newAttackerAndTarget(t)
		a.Gold, a.Agents = 1_000_000_000, 100
		// Expose Enemy Ops turns every roll in both covertSuccess and
		// spySuccess, so the foiled branch is reached without depending on the
		// seed.
		d.ShieldedUntilDay = w.GameDay + 1
		before := len(d.Events)

		if _, err := op.run(w, a, d); err != nil {
			t.Fatalf("%s: %v", op.name, err)
		}
		if len(d.Events) != before+1 {
			t.Fatalf("%s: filed %d victim events, want 1", op.name, len(d.Events)-before)
		}
		if text := d.Events[before].Text; !strings.Contains(text, a.Name) {
			t.Errorf("%s: foiled event does not name the attacker %q: %q", op.name, a.Name, text)
		}
	}
}

// The other half of the same rule: an operation that lands leaves the target
// guessing. Naming the attacker there would make Expose Enemy Ops and Bribery
// pointless, since the name is the only thing they buy.
func TestSuccessfulCovertOpsStayAnonymous(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold, a.Agents = 1_000_000_000, 1_000
	d.Agents, d.Troopers = 0, 10_000
	before := len(d.Events)

	if _, err := w.SupportDissensions(a, d); err != nil {
		t.Fatal(err)
	}
	if d.Troopers == 10_000 {
		t.Fatal("the op did not land, so this proves nothing about a success")
	}
	if len(d.Events) != before+1 {
		t.Fatalf("filed %d victim events, want 1", len(d.Events)-before)
	}
	if text := d.Events[before].Text; strings.Contains(text, a.Name) {
		t.Errorf("a successful covert op named the attacker: %q", text)
	}
}
