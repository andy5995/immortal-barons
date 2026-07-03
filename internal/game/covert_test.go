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
