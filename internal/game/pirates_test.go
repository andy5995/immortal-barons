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

func TestRaidFactionDrainsOverSeveralHits(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("me", "Mine")
	a.Troopers = 1_000_000
	p := &w.Pirates[0]
	p.Forces = 100
	p.LootTroopers = 1000

	// One hit must not clear it; several hits should drain most of it.
	w.RaidFaction(a, 0, 1_000_000, 0, 0)
	if p.LootTroopers == 0 {
		t.Error("a single hit should not fully drain a faction")
	}
	for i := 0; i < 5; i++ {
		p.Forces = 100 // keep it beatable each hit
		w.RaidFaction(a, 0, 1_000_000, 0, 0)
	}
	if p.LootTroopers > 1000*PirateReclaimPct/100 {
		t.Errorf("after several hits most loot should be reclaimed, %d left", p.LootTroopers)
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
