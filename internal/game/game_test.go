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

func TestHQBoostsTanks(t *testing.T) {
	w := testWorld()
	e := w.AddHuman("tester", "Testland")
	e.Tanks = 10

	e.HQ = 0
	baseOffense := e.Offense()
	baseDefense := e.Defense()

	e.HQ = 100
	boostedOffense := e.Offense()
	boostedDefense := e.Defense()

	if boostedOffense <= baseOffense {
		t.Errorf("Offense should rise with HQ=100: base=%d boosted=%d", baseOffense, boostedOffense)
	}
	if boostedDefense <= baseDefense {
		t.Errorf("Defense should rise with HQ=100: base=%d boosted=%d", baseDefense, boostedDefense)
	}

	wantOffenseDelta := e.Tanks * 4
	if boostedOffense-baseOffense != wantOffenseDelta {
		t.Errorf("Offense delta: want %d (tanks doubled), got %d", wantOffenseDelta, boostedOffense-baseOffense)
	}
	wantDefenseDelta := e.Tanks * 4
	if boostedDefense-baseDefense != wantDefenseDelta {
		t.Errorf("Defense delta: want %d (tanks doubled), got %d", wantDefenseDelta, boostedDefense-baseDefense)
	}
}

func TestTechFactor(t *testing.T) {
	e := &Empire{Land: 100, Regions: RegionMix{Coastal: 100}}
	if got := e.techFactor(); got != 0 {
		t.Errorf("no Technology regions: want 0, got %d", got)
	}

	e = &Empire{Land: 100, Regions: RegionMix{Coastal: 80, Technology: 20}}
	if got := e.techFactor(); got != 20 {
		t.Errorf("20%% Technology: want 20, got %d", got)
	}

	e = &Empire{Land: 100, Regions: RegionMix{Technology: 100}}
	if got := e.techFactor(); got != TechFactorCap {
		t.Errorf("all-Technology empire should be capped at %d, got %d", TechFactorCap, got)
	}
}

func TestTechBoostsMilitary(t *testing.T) {
	base := &Empire{Land: 100, Regions: RegionMix{Coastal: 100},
		Troopers: 100, Jets: 20, Carriers: 1, Turrets: 30, Tanks: 10}
	tech := &Empire{Land: 100, Regions: RegionMix{Technology: 40},
		Troopers: 100, Jets: 20, Carriers: 1, Turrets: 30, Tanks: 10}

	if tech.Offense() <= base.Offense() {
		t.Errorf("Offense should rise with Technology regions: base=%d tech=%d", base.Offense(), tech.Offense())
	}
	if tech.Defense() <= base.Defense() {
		t.Errorf("Defense should rise with Technology regions: base=%d tech=%d", base.Defense(), tech.Defense())
	}
}

func TestRemoveEmpireAbdicate(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	before := len(w.Empires)
	e := w.AddHuman("bob", "Bobland")
	if w.FindByOwner("bob") == nil {
		t.Fatal("empire not registered")
	}
	w.RemoveEmpire(e)
	if got := len(w.Empires); got != before {
		t.Errorf("empire count = %d, want %d after abdication", got, before)
	}
	if w.FindByOwner("bob") != nil {
		t.Error("abdicated empire still findable by owner")
	}
}

func TestRealmNameTaken(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.AddHuman("alice", "Khanate")
	if !w.RealmNameTaken("Khanate") {
		t.Error("exact name should be taken")
	}
	if !w.RealmNameTaken("  khanate  ") {
		t.Error("case-insensitive, trimmed name should be taken")
	}
	if w.RealmNameTaken("Empire") {
		t.Error("unused name should be free")
	}
}
