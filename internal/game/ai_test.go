package game

import "testing"

func TestAIProfileFallbackFromName(t *testing.T) {
	e := &Empire{Name: "Crimson Horde"} // no AIProfile set (old-save case)
	if got := e.aiProfile(); got == "" {
		t.Error("aiProfile should derive a non-empty profile from the name")
	}
	// stable across calls
	if e.aiProfile() != e.aiProfile() {
		t.Error("aiProfile must be deterministic for a given name")
	}
}

func TestAIDiplomatAcceptsDefensePact(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	ai := w.AddHuman("ai", "AI") // any empire; we drive aiHandleDiplomacy directly
	ai.AIProfile = AIProfileDiplomat
	human := w.AddHuman("h", "Human")
	w.ProposeTreaty(human, ai, "Full Defense Alliance")

	w.aiHandleDiplomacy(ai)

	if !w.HasTreaty(ai, human, "Full Defense Alliance") {
		t.Error("diplomat AI should have accepted the Full Defense Alliance")
	}
	if len(ai.TreatyOffers) != 0 {
		t.Errorf("offers should be cleared after handling, got %d", len(ai.TreatyOffers))
	}
}

func TestAIAggressorRejectsDefensePact(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	ai := w.AddHuman("ai", "AI")
	ai.AIProfile = AIProfileAggressor
	human := w.AddHuman("h", "Human")
	w.ProposeTreaty(human, ai, "Full Defense Alliance")

	w.aiHandleDiplomacy(ai)

	if w.HasTreaty(ai, human, "Full Defense Alliance") {
		t.Error("aggressor AI should NOT accept a Full Defense Alliance")
	}
	if len(ai.TreatyOffers) != 0 {
		t.Error("declined offer should be discarded, not left pending")
	}
}

func TestAIAggressorAcceptsTradePact(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	ai := w.AddHuman("ai", "AI")
	ai.AIProfile = AIProfileAggressor
	human := w.AddHuman("h", "Human")
	w.ProposeTreaty(human, ai, "Tariff Trade Agreement")

	w.aiHandleDiplomacy(ai)

	if !w.HasTreaty(ai, human, "Tariff Trade Agreement") {
		t.Error("aggressor AI should accept a self-serving trade agreement")
	}
}

// warWorld builds a 2-empire world (no seeded AI) for deterministic war tests:
// one aggressor and one victim, both out of protection at full morale.
func warWorld(t *testing.T) (*World, *Empire, *Empire) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	agg := w.AddHuman("agg", "Aggressor")
	agg.AIProfile = AIProfileAggressor
	agg.Protection, agg.Morale = 0, 100
	vic := w.AddHuman("vic", "Victim")
	vic.Protection, vic.Morale = 0, 100
	vic.Troopers, vic.Turrets, vic.Tanks, vic.Jets, vic.Carriers = 100, 0, 0, 0, 0
	vic.Regions = RegionMix{Desert: 50}
	vic.syncLand()
	return w, agg, vic
}

func TestAIAggressorAttacksWeakNeighbor(t *testing.T) {
	w, agg, vic := warWorld(t)
	agg.Troopers, agg.Tanks = 5000, 500 // overwhelming offense
	before := vic.Land

	w.aiWageWar(agg)

	if vic.Land >= before {
		t.Errorf("aggressor should have captured land: before=%d after=%d", before, vic.Land)
	}
}

func TestAIAggressorHoldsWhenOutmatched(t *testing.T) {
	w, agg, vic := warWorld(t)
	agg.Troopers = 50 // far too weak to win
	vic.Troopers, vic.Turrets = 3000, 2000
	before := vic.Land

	w.aiWageWar(agg)

	if vic.Land != before {
		t.Errorf("aggressor should not attack when outmatched: before=%d after=%d", before, vic.Land)
	}
}

func TestAINonAggressorDoesNotStartWar(t *testing.T) {
	w, agg, vic := warWorld(t)
	agg.AIProfile = AIProfileDiplomat
	agg.Troopers, agg.Tanks = 5000, 500 // strong, but not warlike
	before := vic.Land

	w.aiWageWar(agg)

	if vic.Land != before {
		t.Errorf("non-aggressor should not start a war: before=%d after=%d", before, vic.Land)
	}
}

func TestAIAggressorRespectsProtection(t *testing.T) {
	w, agg, vic := warWorld(t)
	agg.Troopers, agg.Tanks = 5000, 500
	vic.Protection = 20 // shielded — no longer a valid target
	before := vic.Land

	w.aiWageWar(agg)

	if vic.Land != before {
		t.Errorf("aggressor must not attack a protected realm: before=%d after=%d", before, vic.Land)
	}
}
