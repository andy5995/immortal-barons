package game

import (
	"strings"
	"testing"
)

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
}
