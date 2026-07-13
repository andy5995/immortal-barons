package game

import (
	"strings"
	"testing"
)

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

func TestAddAIEmpires(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)

	if got := w.AddAIEmpires(3); got != 3 {
		t.Fatalf("AddAIEmpires(3) added %d, want 3", got)
	}
	if len(w.Empires) != 3 {
		t.Fatalf("want 3 empires, got %d", len(w.Empires))
	}
	seen := map[string]bool{}
	for _, e := range w.Empires {
		if e.Owner != "" {
			t.Errorf("injected empire should be AI (empty Owner), got %q", e.Owner)
		}
		if e.Jets != 5 || e.Turrets != 40 {
			t.Errorf("AI setup: Jets=%d Turrets=%d, want 5/40", e.Jets, e.Turrets)
		}
		key := strings.ToLower(e.Name)
		if seen[key] {
			t.Errorf("duplicate AI name %q", e.Name)
		}
		seen[key] = true
	}

	// A second call skips names already used and keeps names distinct.
	if got := w.AddAIEmpires(2); got != 2 {
		t.Fatalf("second AddAIEmpires(2) added %d, want 2", got)
	}
	names := map[string]bool{}
	for _, e := range w.Empires {
		key := strings.ToLower(e.Name)
		if names[key] {
			t.Errorf("second call duplicated name %q", e.Name)
		}
		names[key] = true
	}
	if len(w.Empires) != 5 {
		t.Fatalf("want 5 empires after two calls, got %d", len(w.Empires))
	}
}

func TestAddAIEmpiresExhaustsPool(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)

	pool := len(aiBaronNames)
	if got := w.AddAIEmpires(pool + 10); got != pool {
		t.Fatalf("requesting more than the pool added %d, want %d", got, pool)
	}
	if len(w.Empires) != pool {
		t.Fatalf("want %d empires (whole pool), got %d", pool, len(w.Empires))
	}
	// No names left; a further request adds nothing.
	if got := w.AddAIEmpires(1); got != 0 {
		t.Errorf("exhausted pool should add 0, got %d", got)
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
	// The bonus is the accumulated TechLevel, NOT the instantaneous share: an
	// empire with Technology regions but no ramp yet still reads 0.
	e := &Empire{Land: 100, Regions: RegionMix{Coastal: 80, Technology: 20}}
	if got := e.TechFactor(); got != 0 {
		t.Errorf("un-ramped Technology: want 0, got %d", got)
	}

	e.TechLevel = 200 // TechLevel is tenths of a percent → 20%
	if got := e.TechFactor(); got != 20 {
		t.Errorf("TechLevel 200 → 20%%: want 20, got %d", got)
	}

	e.TechLevel = 10000 // well over the cap
	if got := e.TechFactor(); got != TechFactorCap {
		t.Errorf("TechFactor should cap at %d, got %d", TechFactorCap, got)
	}
}

// TestTechRampsOverTurns: a fresh empire with Technology regions starts at 0
// bonus and builds up (never decreasing) as it plays turns.
func TestTechRampsOverTurns(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Regions = RegionMix{Coastal: 40, Technology: 60} // 60% tech share
	e.Land = e.Regions.Total()
	if e.TechFactor() != 0 {
		t.Fatalf("fresh Technology empire should start at 0 bonus, got %d", e.TechFactor())
	}
	prev := 0
	for i := 0; i < 10; i++ {
		w.advanceTech(e)
		if e.TechFactor() < prev {
			t.Errorf("TechFactor should not decrease while holding tech: %d -> %d", prev, e.TechFactor())
		}
		prev = e.TechFactor()
	}
	if e.TechFactor() <= 0 {
		t.Errorf("TechFactor should have ramped above 0 after 10 turns, got %d", e.TechFactor())
	}
}

// TestTechHigherShareRampsFaster: a tech-denser realm advances faster.
func TestTechHigherShareRampsFaster(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	lo := w.AddHuman("lo", "Lo")
	lo.Regions = RegionMix{Coastal: 74, Technology: 26}
	lo.Land = lo.Regions.Total()
	hi := w.AddHuman("hi", "Hi")
	hi.Regions = RegionMix{Coastal: 40, Technology: 60}
	hi.Land = hi.Regions.Total()
	for i := 0; i < 10; i++ {
		w.advanceTech(lo)
		w.advanceTech(hi)
	}
	if hi.TechFactor() <= lo.TechFactor() {
		t.Errorf("denser tech should ramp faster: lo(26%%)=%d hi(60%%)=%d", lo.TechFactor(), hi.TechFactor())
	}
}

// TestTechSaturatesAtShareCeiling: the bonus tops out near the tech share.
func TestTechSaturatesAtShareCeiling(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Regions = RegionMix{Coastal: 74, Technology: 26} // ceiling ≈ 26%
	e.Land = e.Regions.Total()
	for i := 0; i < 1000; i++ {
		w.advanceTech(e)
	}
	if got := e.TechFactor(); got != 26 {
		t.Errorf("26%% tech share should saturate at 26%% bonus, got %d", got)
	}
}

func TestTechBoostsMilitary(t *testing.T) {
	base := &Empire{Land: 100, Regions: RegionMix{Coastal: 100},
		Troopers: 100, Jets: 20, Carriers: 1, Turrets: 30, Tanks: 10}
	tech := &Empire{Land: 100, Regions: RegionMix{Technology: 40}, TechLevel: 400, // 40% ramped
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
