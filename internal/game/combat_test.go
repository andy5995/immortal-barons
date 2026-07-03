package game

import (
	"strings"
	"testing"
)

func TestBombingRunDestroysGroundedJets(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := &Empire{Bombers: 10}
	d := &Empire{Jets: 100} // no turrets, no SDI
	kills, lost := w.bombingRun(a, d)
	if lost != 0 {
		t.Errorf("no turrets means no bombers lost, got %d", lost)
	}
	if kills != 10*BomberJetKills {
		t.Errorf("10 bombers should down %d jets, got %d", 10*BomberJetKills, kills)
	}
	if d.Jets != 100-kills {
		t.Errorf("defender jets not reduced: %d", d.Jets)
	}
}

func TestBombingRunTurretsDownBombersAndSDIBlunts(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := &Empire{Bombers: 10}
	d := &Empire{Jets: 1000, Turrets: 50, SDI: 50} // 50/25 = 2 bombers lost, SDI halves
	kills, lost := w.bombingRun(a, d)
	if lost != 2 {
		t.Errorf("50 turrets should down 2 bombers, got %d", lost)
	}
	// 8 survivors * 3 kills = 24, halved by 50% SDI = 12
	if kills != 12 {
		t.Errorf("expected 12 kills after SDI, got %d", kills)
	}
	if a.Bombers != 8 {
		t.Errorf("attacker should have 8 bombers left, got %d", a.Bombers)
	}
}

func TestBombingRunCapsAtDefenderJets(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := &Empire{Bombers: 100}
	d := &Empire{Jets: 5}
	kills, _ := w.bombingRun(a, d)
	if kills != 5 || d.Jets != 0 {
		t.Errorf("kills should cap at 5 and zero out jets, got kills=%d jets=%d", kills, d.Jets)
	}
}

func TestAttackRecordsVictimEvent(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("att", "Attacker")
	a.Troopers = 100000 // overwhelming, deterministic win
	d := w.Empires[0]
	d.Protection = 0

	before := len(d.Events)
	w.Attack(a, d)
	if len(d.Events) != before+1 {
		t.Fatalf("victim should get one event, got %d new", len(d.Events)-before)
	}
	if !strings.Contains(d.Events[len(d.Events)-1], a.Name) {
		t.Errorf("event should name the attacker: %q", d.Events[len(d.Events)-1])
	}
	if !strings.Contains(d.Events[len(d.Events)-1], "attacked you") {
		t.Errorf("event should be victim-perspective: %q", d.Events[len(d.Events)-1])
	}
}
