package game

import "testing"

func testWorld() *World {
	cfg := DefaultConfig()
	cfg.AICount = 2
	return NewWorldSeed(cfg, 1)
}

func TestNewWorldSeedsAIOnly(t *testing.T) {
	w := testWorld()
	if len(w.Empires) != 2 {
		t.Fatalf("want 2 AI empires, got %d", len(w.Empires))
	}
	for _, e := range w.Empires {
		if e.Owner != "" {
			t.Errorf("AI empire should have empty Owner, got %q", e.Owner)
		}
	}
	if w.Player() != nil {
		t.Error("no active empire yet, Player() should be nil")
	}
}

func TestAddHumanAndFindByOwner(t *testing.T) {
	w := testWorld()
	e := w.AddHuman("Khan", "New Barony")
	if e.Owner != "khan" {
		t.Errorf("owner should be normalized to lowercase, got %q", e.Owner)
	}
	if e.Name != "New Barony" {
		t.Errorf("realm name: got %q", e.Name)
	}
	if w.FindByOwner("KHAN") != e {
		t.Error("FindByOwner should match case-insensitively")
	}
	if w.FindByOwner("nobody") != nil {
		t.Error("FindByOwner should return nil for unknown handle")
	}
}

func TestTargetsExcludeSelfAndProtected(t *testing.T) {
	w := testWorld()
	me := w.AddHuman("me", "Mine")
	me.Protection = 0
	w.Empires[0].Protection = 5 // protected AI
	w.Empires[1].Protection = 0
	got := w.Targets(me)
	for _, e := range got {
		if e == me {
			t.Error("Targets must not include the attacker")
		}
		if e.Protection > 0 {
			t.Error("Targets must exclude protected empires")
		}
	}
	if len(got) != 1 {
		t.Errorf("want 1 targetable empire, got %d", len(got))
	}
}
