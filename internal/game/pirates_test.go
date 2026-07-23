package game

import "testing"

func TestSeedPiratesRandomStrengthNoLadder(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	if len(w.Pirates) != len(PirateFactions) {
		t.Fatalf("want %d factions, got %d", len(PirateFactions), len(w.Pirates))
	}
	ascending := true
	for i, p := range w.Pirates {
		if p.Forces < pirateForcesMin || p.Forces >= pirateForcesMax {
			t.Errorf("%s forces %d out of seed range", p.Name, p.Forces)
		}
		if i > 0 && p.Forces < w.Pirates[i-1].Forces {
			ascending = false
		}
	}
	if ascending {
		t.Error("seeded forces are strictly ascending — that's the old index ladder, not random")
	}
}

func TestPirateRaidTakesUnitsAndGrantsLand(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	v := w.AddHuman("me", "Mine")
	v.Troopers, v.Jets, v.Tanks = 1000, 200, 100
	p := &w.Pirates[0]
	landBefore := p.Land

	w.pirateRaidVictim(p, v)

	if v.Troopers != 1000-1000*PirateRaidUnitPct/100 {
		t.Errorf("victim troopers not reduced: %d", v.Troopers)
	}
	if p.LootTanks != 100*PirateRaidUnitPct/100 {
		t.Errorf("faction should hold looted tanks, got %d", p.LootTanks)
	}
	if p.Land <= landBefore {
		t.Errorf("faction should be granted regions by the game, land %d -> %d", landBefore, p.Land)
	}
	if len(v.PirateRaids) == 0 {
		t.Error("human victim should get a raid notice")
	}
}

func TestPirateRaidOnAIRecordsNoNotice(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 1)
	ai := w.Empires[0] // seeded AI empire (Owner == "")
	ai.Tanks = 100
	p := &w.Pirates[0]

	w.pirateRaidVictim(p, ai)

	if ai.Tanks != 100-100*PirateRaidUnitPct/100 {
		t.Errorf("AI victim should still lose units, got %d tanks", ai.Tanks)
	}
	if len(ai.PirateRaids) != 0 {
		t.Errorf("AI victim should not accumulate raid notices, got %d", len(ai.PirateRaids))
	}
}

func TestRaidFactionWinReclaimsPortion(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("me", "Mine")
	a.Troopers = 1_000_000 // overwhelming, deterministic win
	p := &w.Pirates[0]
	p.Forces = 100
	p.LootTanks = 400
	p.Land = 400

	before := a.Tanks
	w.RaidFaction(a, 0, 1_000_000, 0, 0)

	if p.LootTanks != 400-400*PirateReclaimPct/100 {
		t.Errorf("faction loot should drop by reclaim pct, got %d", p.LootTanks)
	}
	if a.Tanks != before+400*PirateReclaimPct/100 {
		t.Errorf("attacker should recover reclaimed tanks, got %d", a.Tanks)
	}
	if p.Land != 400-400*PirateReclaimPct/100 {
		t.Errorf("faction land should drop by reclaim pct, got %d", p.Land)
	}
}

// A win against a faction that holds no land captures no regions, so the caller
// shows no picker (#21 — BRE: raiding a landless band yields gold/military only).
func TestRaidFactionLandlessCapturesNoRegions(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("me", "Mine")
	a.Troopers = 1_000_000
	p := &w.Pirates[0]
	p.Forces = 100
	p.Land = 0 // landless band
	p.Gold = 50_000

	beforeLand := a.Land
	_, captured := w.RaidFaction(a, 0, 1_000_000, 0, 0)
	if captured != 0 {
		t.Errorf("landless faction should capture no regions, got %d", captured)
	}
	if a.Land != beforeLand {
		t.Errorf("attacker land must be unchanged, got %d want %d", a.Land, beforeLand)
	}
}

func TestPiratesSkipProtectedEmpires(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	safe := w.AddHuman("safe", "Safe")
	safe.Protection = 100
	safe.Troopers = 1_000_000
	exposed := w.AddHuman("exp", "Exposed")
	exposed.Protection = 0
	exposed.Troopers = 1_000_000

	for i := 0; i < 200; i++ {
		w.piratesRaid()
	}
	if len(safe.PirateRaids) != 0 {
		t.Errorf("empire under protection was raided %d times, want 0", len(safe.PirateRaids))
	}
	if len(exposed.PirateRaids) == 0 {
		t.Error("unprotected empire should have been raided at least once")
	}
}

func TestRaidFactionScoreIsSmall(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("me", "Mine")
	a.Troopers = 1_000_000 // overwhelming, deterministic win
	p := &w.Pirates[0]
	p.Forces = 100

	w.RaidFaction(a, 0, 1_000_000, 0, 0)
	if want := 100 / PirateScoreDivisor; a.Score != want {
		t.Errorf("pirate-win Score = %d, want %d (scaled by faction strength, not army size)", a.Score, want)
	}

	// A loss shaves the same small amount, and never below zero.
	b := w.AddHuman("you", "Yours")
	b.Troopers = 10
	b.Score = 1000
	q := &w.Pirates[1]
	q.Forces = 1000
	w.RaidFaction(b, 1, 10, 0, 0)
	if want := 1000 - 1000/PirateScoreDivisor; b.Score != want {
		t.Errorf("pirate-loss Score = %d, want %d", b.Score, want)
	}
}

func TestRaidFactionDrainsOverSeveralHits(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("me", "Mine")
	a.Troopers = 1_000_000
	p := &w.Pirates[0]
	p.Forces = 100
	p.LootTroopers = 1000

	// One hit must not clear it; many hits should drain most of it.
	w.RaidFaction(a, 0, 1_000_000, 0, 0)
	if p.LootTroopers == 0 {
		t.Error("a single hit should not fully drain a faction")
	}
	for i := 0; i < 12; i++ {
		p.Forces = 100 // keep it beatable each hit
		w.RaidFaction(a, 0, 1_000_000, 0, 0)
	}
	if p.LootTroopers > 200 {
		t.Errorf("after many hits most loot should be reclaimed, %d left", p.LootTroopers)
	}
}

func TestEnsurePiratesSeedsOldSave(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Pirates = nil // simulate a save predating the pirate model
	w.EnsurePirates()
	if len(w.Pirates) != len(PirateFactions) {
		t.Errorf("EnsurePirates should seed %d factions, got %d", len(PirateFactions), len(w.Pirates))
	}
}

func TestPirateRaidStealsAllButBombersAndCarriers(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	v := w.AddHuman("me", "Mine")
	v.Troopers, v.Jets, v.Turrets, v.Tanks, v.Agents, v.Gold = 1000, 1000, 1000, 1000, 1000, 1000
	v.Bombers, v.Carriers = 500, 500
	p := &w.Pirates[0]

	w.pirateRaidVictim(p, v)

	if v.Bombers != 500 || v.Carriers != 500 {
		t.Errorf("pirates must not take bombers/carriers: bombers=%d carriers=%d", v.Bombers, v.Carriers)
	}
	stolen := map[string]int{
		"troopers": v.Troopers, "jets": v.Jets, "turrets": v.Turrets,
		"tanks": v.Tanks, "agents": v.Agents, "gold": v.Gold,
	}
	for name, got := range stolen {
		if got != 950 { // 1000 - 5%
			t.Errorf("%s: expected 950 after a 5%% raid, got %d", name, got)
		}
	}
	if p.LootAgents != 50 || p.Gold != 50 || p.LootTurrets != 50 {
		t.Errorf("faction should hold looted agents/gold/turrets, got A=%d G=%d U=%d",
			p.LootAgents, p.Gold, p.LootTurrets)
	}
}

func TestPirateTakeCappedAtMax(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	v := w.AddHuman("me", "Mine")
	v.Troopers = 100_000_000 // 5% would be 5,000,000 — far over the take cap
	p := &w.Pirates[0]
	before := v.Troopers

	w.pirateRaidVictim(p, v)

	if before-v.Troopers != PirateRaidMaxTake {
		t.Errorf("a single raid should take at most %d, took %d", PirateRaidMaxTake, before-v.Troopers)
	}
}

func TestPirateHoldingClampsToCap(t *testing.T) {
	if got := capAdd(PirateCapTanks-10, 1000, PirateCapTanks); got != PirateCapTanks {
		t.Errorf("holdings should clamp to cap %d, got %d", PirateCapTanks, got)
	}
}
