package game

import (
	"errors"
	"strings"
	"testing"
)

func TestSendSpySuccess(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 50
	d.Agents = 0

	report, err := w.SendSpy(a, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == "" {
		t.Error("expected a non-empty intel report")
	}
	if !strings.Contains(report, d.Name) {
		t.Errorf("expected report to contain target name %q, got %q", d.Name, report)
	}
	if a.Agents != 50 {
		t.Errorf("expected attacker to keep all agents, got %d", a.Agents)
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

// TestSendSpyFailure forces the covert failure branch: newAttackerAndTarget
// seeds the world with NewWorldSeed(cfg, 1). With a.Agents=1 and
// d.Agents=1000000, covertSuccess needs w.rng.Intn(1000001) < 1 to succeed;
// verified against a standalone math/rand run with seed 1 that the first
// draw lands at 496783, well above the threshold, so the failure branch is
// deterministic here.
func TestSendSpyFailure(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 1
	d.Agents = 1000000
	beforeEvents := len(d.Events)

	report, err := w.SendSpy(a, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(report, "caught") {
		t.Errorf("expected report to indicate the spy was caught, got %q", report)
	}
	if a.Agents != 0 {
		t.Errorf("expected attacker to lose the caught agent, got %d agents", a.Agents)
	}
	if len(d.Events) != beforeEvents+1 {
		t.Fatalf("expected one victim event, got %d new", len(d.Events)-beforeEvents)
	}
}

// TestSupportDissensionsFailure forces the same covert failure branch as
// TestSendSpyFailure (see its comment for why seed 1 is deterministic here).
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
	if d.Support != 85 {
		t.Errorf("expected support 85, got %d", d.Support)
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
	a.Agents = 1
	d := w.AddHuman("d", "Delta") // unused target, required by the action signature

	if _, err := w.ExposeEnemyOps(a, d); err != nil {
		t.Fatal(err)
	}

	attacker := w.AddHuman("e", "Enemyland")
	attacker.Agents = 1_000_000
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
	before := d.Jets + d.Turrets + d.Tanks + d.Bombers + d.Carriers + d.Agents + d.Gold + d.Food
	for i := 0; i < 100; i++ {
		if _, err := w.SlappenheimerStrike(a, d); err != nil {
			t.Fatal(err)
		}
	}
	after := d.Jets + d.Turrets + d.Tanks + d.Bombers + d.Carriers + d.Agents + d.Gold + d.Food
	if after >= before {
		t.Errorf("expected R5-Slappenheimer strikes to reduce the target's resources, before=%d after=%d", before, after)
	}
}

func TestSpyOnRelationsRevealsTreaties(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
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
	a.Agents = 1_000_000
	d := w.AddHuman("d", "Delta")
	d.Agents = 4
	if _, err := w.Bribery(a, d); err != nil {
		t.Fatal(err)
	}
	// d now cannot land covert ops on a, even with overwhelming agents.
	d.Agents = 1_000_000
	before := a.Troopers
	if _, err := w.SupportDissensions(d, a); err != nil {
		t.Fatal(err)
	}
	if a.Troopers != before {
		t.Errorf("d's ops should fail from a's bribery immunity, troopers %d -> %d", before, a.Troopers)
	}
}
