package game

import (
	"strings"
	"testing"
)

func newAttackerAndTarget(t *testing.T) (*World, *Empire, *Empire) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("att", "Attacker")
	a.Protection = 0
	d := w.Empires[0]
	d.Protection = 0
	return w, a, d
}

func TestNuclearStrike(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold = 100000
	beforeLand := d.Land
	beforeEvents := len(d.Events)

	report, err := w.NuclearStrike(a, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == "" {
		t.Error("expected a non-empty attacker report")
	}
	if a.Gold != 100000-NukeCost {
		t.Errorf("gold not deducted: got %d", a.Gold)
	}
	if d.Land >= beforeLand {
		t.Errorf("expected land reduced, before=%d after=%d", beforeLand, d.Land)
	}
	if len(d.Events) != beforeEvents+1 {
		t.Fatalf("expected one victim event, got %d new", len(d.Events)-beforeEvents)
	}
	if !strings.Contains(d.Events[len(d.Events)-1], "nuclear") {
		t.Errorf("victim event should mention nuclear: %q", d.Events[len(d.Events)-1])
	}
}

func TestNuclearStrikeCantAfford(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold = 0
	beforeLand := d.Land
	beforeEvents := len(d.Events)

	_, err := w.NuclearStrike(a, d)
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

func TestChemicalStrike(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold = 100000
	beforePeople := d.People
	beforeTroopers := d.Troopers

	report, err := w.ChemicalStrike(a, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == "" {
		t.Error("expected a non-empty attacker report")
	}
	if a.Gold != 100000-ChemCost {
		t.Errorf("gold not deducted: got %d", a.Gold)
	}
	if d.People >= beforePeople {
		t.Errorf("expected people reduced, before=%d after=%d", beforePeople, d.People)
	}
	if d.Troopers >= beforeTroopers {
		t.Errorf("expected troopers reduced, before=%d after=%d", beforeTroopers, d.Troopers)
	}
}

func TestBiologicalStrike(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold = 100000
	beforePeople := d.People
	beforeTroopers := d.Troopers
	beforeLand := d.Land

	report, err := w.BiologicalStrike(a, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == "" {
		t.Error("expected a non-empty attacker report")
	}
	if a.Gold != 100000-BioCost {
		t.Errorf("gold not deducted: got %d", a.Gold)
	}
	if d.People >= beforePeople {
		t.Errorf("expected people reduced, before=%d after=%d", beforePeople, d.People)
	}
	if d.Troopers >= beforeTroopers {
		t.Errorf("expected troopers reduced, before=%d after=%d", beforeTroopers, d.Troopers)
	}
	if d.Land != beforeLand {
		t.Errorf("land should be unchanged, before=%d after=%d", beforeLand, d.Land)
	}
}

func TestRaidPirates(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)
	a.Troopers = 1000
	beforeLand := a.Land

	report := w.RaidPirates(a)
	if report == "" {
		t.Error("expected a non-empty report")
	}
	if a.Land == beforeLand {
		t.Errorf("expected land to change (net raid effect), stayed at %d", beforeLand)
	}
}
