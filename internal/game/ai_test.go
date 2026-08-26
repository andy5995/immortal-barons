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
	want := "AI rejected your Full Defense Alliance proposal."
	if len(human.Events) != 1 || human.Events[0].Text != want {
		t.Errorf("proposer's events = %v, want [%q]", human.Events, want)
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

func TestAIAggressorBuysAgents(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("agg", "Aggressor")
	e.AIProfile = AIProfileAggressor
	e.Regions = RegionMix{Agricultural: 50}
	e.syncLand()
	e.People = 1000
	e.Troopers, e.Turrets, e.Tanks, e.Agents = 0, 0, 0, 0
	e.Food = 1_000_000
	e.Gold = 500_000
	e.Protection = 0 // past protection: the aggressor buys agents for covert ops (under protection it expands land)

	w.aiManageEconomy(e)

	if e.Agents == 0 {
		t.Error("aggressor should buy agents for covert ops, got 0")
	}
}

// An attack no longer carries a covert strike in front of it. BRE queues every
// effect covert operation and resolves it at daily maintenance, so an agent sent
// this turn cannot soften the realm being invaded this turn — and an AI that
// still sent one would be spending a fee and an agent for nothing. The pre-war
// demoralize the AI used to run here belongs to aiCovertOps, a day earlier.
func TestAIAttackSendsNoAgentAhead(t *testing.T) {
	w, agg, vic := warWorld(t)
	agg.Troopers, agg.Tanks = 5000, 500 // clearly favored
	agg.Agents = 50
	agg.Gold = 1_000_000
	vic.Morale = 100
	beforeLand, beforeAgents, beforeGold := vic.Land, agg.Agents, agg.Gold

	w.aiWageWar(agg)

	if vic.Land >= beforeLand {
		t.Fatalf("precondition: the attack should still have happened, land %d->%d", beforeLand, vic.Land)
	}
	if len(w.CovertQueue) != 0 {
		t.Errorf("the war path queued %d covert operations, want 0", len(w.CovertQueue))
	}
	if agg.Agents != beforeAgents || agg.Gold != beforeGold {
		t.Errorf("the war path spent covert resources: agents %d->%d, gold %d->%d",
			beforeAgents, agg.Agents, beforeGold, agg.Gold)
	}
	if vic.Morale != 100 {
		t.Errorf("the target's morale moved before the battle: %d", vic.Morale)
	}
}

// Expansion must not collapse the moment New Realm Protection lapses. Two AIs
// identical except for Protection (one shielded, one exposed), both food-healthy
// and budget-limited, should buy the SAME amount of land in a turn: the exposed
// one additionally funds a defensive force, but that must come out of what's left
// AFTER expansion, not out of the expansion budget. Regression guard for the
// "strong first day, stalls afterwards" behavior — the AI was skimming half its
// gold for military before the land step, starving the compounding land engine
// right when protection ended.
func TestAIExpansionSurvivesProtectionLapse(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	build := func(owner string, protection int) *Empire {
		e := w.AddHuman(owner, owner)
		e.AISkill = AISkillSharp                // no dull throttle
		e.AIProfile = AIProfileDiplomat         // non-aggressor: no agent buys to muddy the compare
		e.Regions = RegionMix{Agricultural: 50} // food-healthy: reaches the land step
		e.syncLand()
		e.People = 1000
		e.Food = 1_000_000
		e.Gold = 500_000 // below the per-turn region cap, so the budget (not the cap) sets land bought
		e.Protection = protection
		e.Investments = nil
		return e
	}
	protected := build("prot", 20) // shielded: skips military, all surplus to land
	exposed := build("expo", 0)    // out of protection: also builds a force
	protBefore, expoBefore := protected.Land, exposed.Land

	w.aiManageEconomy(protected)
	w.aiManageEconomy(exposed)

	protBought, expoBought := protected.Land-protBefore, exposed.Land-expoBefore
	if protBought <= 0 {
		t.Fatalf("precondition: protected AI should expand, bought %d", protBought)
	}
	if expoBought < protBought {
		t.Errorf("expansion collapsed when protection lapsed: protected bought %d, exposed bought %d "+
			"(exposed should expand at least as much, funding military from the remainder)",
			protBought, expoBought)
	}
}

// A dull-skill AI throttles its land-buying, so it expands less than a sharp one
// from the same position (both still expand). Two tiers give a game a mix of
// strong and weak rivals.
func TestAIDullExpandsLessThanSharp(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	build := func(skill string) *Empire {
		e := w.AddHuman(skill, skill)
		e.AISkill = skill
		e.Regions = RegionMix{Agricultural: 50} // food-healthy: goes straight to land
		e.syncLand()
		e.People = 1000
		e.Food = 1_000_000
		e.Gold = 500_000  // budget-limited (below the per-turn region cap), so the throttle shows
		e.Protection = 20 // under protection: expand, no military spend
		e.Investments = nil
		return e
	}
	sharp, dull := build(AISkillSharp), build(AISkillDull)
	sharpBefore, dullBefore := sharp.Land, dull.Land
	w.aiManageEconomy(sharp)
	w.aiManageEconomy(dull)
	sharpBought, dullBought := sharp.Land-sharpBefore, dull.Land-dullBefore
	if dullBought <= 0 {
		t.Errorf("dull AI should still expand, bought %d", dullBought)
	}
	if dullBought >= sharpBought {
		t.Errorf("dull AI should expand less than sharp: dull bought %d, sharp bought %d", dullBought, sharpBought)
	}
}

// A Max Tax Rate of 0 held every player to 0% while the AI went on taxing at
// its normal rate, because aiSetTax read 0 as "no cap configured" (#203). Zero
// is a ceiling of zero for both sides.
func TestAIObeysAZeroTaxCeiling(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxTaxRate = 0
	w := NewWorldSeed(cfg, 1)
	ai := w.AddHuman("ai", "AI")
	ai.AIProfile = AIProfileAggressor
	ai.Tax = 25 // a rate to be pulled DOWN, so a no-op would fail rather than pass
	w.aiSetTax(ai)
	if ai.Tax != 0 {
		t.Errorf("AI taxed at %d%% under a Max Tax Rate of 0", ai.Tax)
	}
	// The ceiling still binds where it is not zero, and still lets a lower rate be.
	w.Config.MaxTaxRate = 10
	w.aiSetTax(ai)
	if ai.Tax != 10 {
		t.Errorf("AI taxed at %d%% under a Max Tax Rate of 10, want the ceiling", ai.Tax)
	}
}
