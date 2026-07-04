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

func TestSabotageSuccess(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 50
	d.Agents = 0
	d.Troopers = 1000
	beforeEvents := len(d.Events)

	report, err := w.Sabotage(a, d)
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

func TestSabotageNoAgents(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 0

	_, err := w.Sabotage(a, d)
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

// TestSabotageFailure forces the same covert failure branch as
// TestSendSpyFailure (see its comment for why seed 1 is deterministic here).
func TestSabotageFailure(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 1
	d.Agents = 1000000
	d.Troopers = 1000
	beforeEvents := len(d.Events)

	report, err := w.Sabotage(a, d)
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

func TestBombAirbasesDestroysGroundedJets(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents, d.Jets = 50, 0, 400
	if _, err := w.BombAirbases(a, d); err != nil {
		t.Fatal(err)
	}
	if d.Jets != 300 {
		t.Errorf("expected 300 jets after a 25%% strike, got %d", d.Jets)
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

func TestBombHQWeakensAndClamps(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents, d.HQ = 50, 0, 10
	if _, err := w.BombHQ(a, d); err != nil {
		t.Fatal(err)
	}
	if d.HQ != 0 { // 10 - 20, clamped at 0
		t.Errorf("HQ should clamp at 0, got %d", d.HQ)
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

func TestBombIntelligenceKillsAgents(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents = 1_000_000, 8 // overwhelming odds -> success
	if _, err := w.BombIntelligence(a, d); err != nil {
		t.Fatal(err)
	}
	if d.Agents != 6 { // 8 - 8/4
		t.Errorf("expected 6 agents after a 25%% strike, got %d", d.Agents)
	}
}

func TestCovertOpNeedsAnAgent(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents = 0
	if _, err := w.BombAirbases(a, d); !errors.Is(err, ErrNoAgents) {
		t.Errorf("expected ErrNoAgents, got %v", err)
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
	if _, err := w.Sabotage(d, a); err != nil {
		t.Fatal(err)
	}
	if a.Troopers != before {
		t.Errorf("d's sabotage should fail from a's bribery immunity, troopers %d -> %d", before, a.Troopers)
	}
}
