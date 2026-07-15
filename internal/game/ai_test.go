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
