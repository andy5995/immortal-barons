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
	a.Gold = 10_000_000 // enough to afford covert/WMD op costs; gold-asserting tests reset it
	d := w.Empires[0]
	d.Protection = 0
	return w, a, d
}

func TestNuclearStrike(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Gold = 10_000_000
	beforeLand := d.Land
	beforeEvents := len(d.Events)

	report, err := w.NuclearStrike(a, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == "" {
		t.Error("expected a non-empty attacker report")
	}
	if a.Gold != 10_000_000-NukeCost {
		t.Errorf("gold not deducted: got %d", a.Gold)
	}
	if d.Land >= beforeLand {
		t.Errorf("expected land reduced, before=%d after=%d", beforeLand, d.Land)
	}
	if len(d.Events) != beforeEvents+1 {
		t.Fatalf("expected one victim event, got %d new", len(d.Events)-beforeEvents)
	}
	if !strings.Contains(d.Events[len(d.Events)-1].Text, "nuclear") {
		t.Errorf("victim event should mention nuclear: %q", d.Events[len(d.Events)-1].Text)
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
	a.Gold = 10_000_000
	beforePeople := d.People
	beforeTroopers := d.Troopers

	report, err := w.ChemicalStrike(a, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == "" {
		t.Error("expected a non-empty attacker report")
	}
	if a.Gold != 10_000_000-ChemCost {
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
	a.Gold = 10_000_000
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
	if a.Gold != 10_000_000-BioCost {
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

func TestRaidFactionWinLose(t *testing.T) {
	w, a, _ := newAttackerAndTarget(t)

	// An overwhelming force against the easiest faction (Humans, index 0) wins
	// against a landed faction: the captured land is DEFERRED (returned for the
	// menu picker), so the attacker's own land does not change here (#21).
	a.Troopers = 1_000_000
	a.Jets = 0
	a.Tanks = 0
	w.Pirates[0].Land = 400 // ensure the faction holds land to capture
	beforeLand := a.Land

	report, captured := w.RaidFaction(a, 0, 1_000_000, 0, 0)
	if report == "" {
		t.Error("expected a non-empty report")
	}
	if captured <= 0 {
		t.Errorf("expected a positive captured-land count on a win, got %d", captured)
	}
	if a.Land != beforeLand {
		t.Errorf("captured land must be deferred, not auto-added: before=%d after=%d", beforeLand, a.Land)
	}
	if a.Land != a.Regions.Total() {
		t.Errorf("Land/Regions invariant broken: Land=%d Regions.Total()=%d", a.Land, a.Regions.Total())
	}

	// A token force against the last faction (Ammonians, index 8) loses
	// and the committed troopers drop.
	a.Troopers = 10
	beforeTroopers := a.Troopers

	report, _ = w.RaidFaction(a, 8, 10, 0, 0)
	if report == "" {
		t.Error("expected a non-empty report")
	}
	if a.Troopers >= beforeTroopers {
		t.Errorf("expected troopers lost on a loss, before=%d after=%d", beforeTroopers, a.Troopers)
	}
}
