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

func TestAttackScoreAttackerWins(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := &Empire{Name: "A", Troopers: 100000, Morale: 100, Alive: true}
	d := &Empire{Name: "D", Turrets: 1000, Morale: 100, Land: 100,
		Regions: RegionMix{Mountain: 100}, Gold: 1000, People: 1000, Alive: true, Score: 1_000_000}

	w.Attack(a, d)

	// aloss = 15% of 100000 troopers; dloss = 15% of 1000 turrets.
	battle := 100000*RegularAttackLossPct/100 + 1000*RegularAttackLossPct/100
	gain := battle / CombatScoreDivisor
	if a.Score != gain {
		t.Errorf("attacker Score = %d, want %d", a.Score, gain)
	}
	wantD := 1_000_000 - gain*CombatLoserPenaltyPct/100
	if d.Score != wantD {
		t.Errorf("defender Score = %d, want %d (loses less than the winner gains)", d.Score, wantD)
	}
	if gain-(1_000_000-d.Score) <= 0 {
		t.Errorf("loser penalty should be smaller than the winner's gain")
	}
}

func TestAttackScoreDefenderWinsWorthMore(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := &Empire{Name: "A", Troopers: 10, Morale: 100, Alive: true, Score: 1_000_000}
	d := &Empire{Name: "D", Turrets: 100000, Morale: 100, Land: 100, People: 1000, Alive: true}

	w.Attack(a, d)

	// aloss = 15% of 10 troopers; dloss = 15% of 100000 turrets.
	battle := 10*RegularAttackLossPct/100 + 100000*RegularAttackLossPct/100
	gain := battle / CombatScoreDivisor * DefenseWinBonusPct / 100
	if d.Score != gain {
		t.Errorf("defending winner Score = %d, want %d", d.Score, gain)
	}
	wantA := 1_000_000 - gain*CombatLoserPenaltyPct/100
	if a.Score != wantA {
		t.Errorf("repelled attacker Score = %d, want %d", a.Score, wantA)
	}
	// A defensive win awards more than the same-size attack would (150% vs 100%).
	if gain <= battle/CombatScoreDivisor {
		t.Errorf("defensive win (%d) should out-award an equivalent attack win (%d)", gain, battle/CombatScoreDivisor)
	}
}

func TestAttackPostsPlanetaryNews(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("att", "Attacker")
	a.Troopers = 100000 // overwhelming, deterministic win
	d := w.Empires[0]
	d.Protection = 0

	before := len(w.NewsToday)
	w.Attack(a, d)
	if len(w.NewsToday) != before+1 {
		t.Fatalf("attack should post one planetary news line, got %d new", len(w.NewsToday)-before)
	}
	if last := w.NewsToday[len(w.NewsToday)-1]; !strings.Contains(last, "Attacker") {
		t.Errorf("news should name the attacker: %q", last)
	}
}
