package game

import "testing"

// Diplomats never attack (#71): their budget goes to defense instead, so they
// are passive by choice rather than by hoarding offense they never use.
func TestAIWarMarginByProfile(t *testing.T) {
	if got := aiWarMargin(AIProfileDiplomat); got != 0 {
		t.Errorf("a diplomat should never attack, got margin %d", got)
	}
	if aiWarMargin(AIProfileBalanced) <= aiWarMargin(AIProfileAggressor) {
		t.Error("a balanced realm should need a bigger edge than an aggressor before striking")
	}
}

// Every profile's five shares must sum to 100, or the AI silently spends less
// than its military budget.
func TestAIForceSharesSumTo100(t *testing.T) {
	for _, p := range []string{AIProfileDiplomat, AIProfileBalanced, AIProfileAggressor} {
		m := aiForceShares(p)
		if sum := m.trooper + m.turret + m.tank + m.jet + m.agent; sum != 100 {
			t.Errorf("%s shares sum to %d, want 100", p, sum)
		}
	}
	// A diplomat buys defense; an aggressor buys punch.
	if aiForceShares(AIProfileDiplomat).turret <= aiForceShares(AIProfileAggressor).turret {
		t.Error("a diplomat should buy more turret defense than an aggressor")
	}
	if aiForceShares(AIProfileAggressor).tank <= aiForceShares(AIProfileDiplomat).tank {
		t.Error("an aggressor should buy more tanks than a diplomat")
	}
}

// Jets cannot reach a battle without carriers, so the AI must buy the lift for
// the jets it holds (#72). Before this it bought none and its seeded jets were
// inert for the life of the realm.
func TestAIBuysCarriersForItsJets(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Jets, e.Carriers, e.Gold = 450, 0, 100_000_000

	w.aiBuyCarriers(e)

	want := (450 + JetsPerCarrier - 1) / JetsPerCarrier
	if e.Carriers != want {
		t.Errorf("carriers for 450 jets: want %d, got %d", want, e.Carriers)
	}
	if FullForce(e).Jets != 450 {
		t.Errorf("all 450 jets should be committable, got %d", FullForce(e).Jets)
	}
	// It must not keep buying hulls it does not need.
	before := e.Carriers
	w.aiBuyCarriers(e)
	if e.Carriers != before {
		t.Errorf("carriers over-bought: %d -> %d", before, e.Carriers)
	}
}

// The AI must not stockpile agents without bound (#57): as a flat share of the
// military budget they compounded into six-figure hoards on a large realm.
func TestAIAgentStockpileIsCapped(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.AIProfile = AIProfileAggressor
	e.Regions = RegionMix{Coastal: 100}
	e.syncLand()
	e.Agents = e.Land * AIAgentsPerRegion // already at the cap
	e.Gold = 1_000_000_000

	before := e.Agents
	w.aiBuildForces(e)
	if e.Agents != before {
		t.Errorf("agents should stop at the cap: %d -> %d", before, e.Agents)
	}
}

// The AI sets its tax rate from the support model rather than sitting on the
// starting rate forever (#73), and backs off when support slips.
func TestAISetTaxRespondsToSupport(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	sharp := w.AddHuman("s", "Sharp")
	sharp.AISkill, sharp.Support = AISkillSharp, 100
	w.aiSetTax(sharp)
	if sharp.Tax != AITaxSharp {
		t.Errorf("a healthy sharp baron should tax at %d, got %d", AITaxSharp, sharp.Tax)
	}
	if sharp.Tax >= SupportTaxNeutral {
		t.Errorf("the chosen rate must stay under the neutral rate to keep gaining support, got %d", sharp.Tax)
	}

	dull := w.AddHuman("d", "Dull")
	dull.AISkill, dull.Support = AISkillDull, 100
	w.aiSetTax(dull)
	if dull.Tax >= sharp.Tax {
		t.Errorf("a dull baron should under-tax a sharp one: dull %d, sharp %d", dull.Tax, sharp.Tax)
	}

	sharp.Support = AISupportFloor - 1
	w.aiSetTax(sharp)
	if sharp.Tax != AITaxRecover || sharp.Tax >= LowTaxBonusBelow {
		t.Errorf("an unpopular realm should drop under the low-tax buy-back rate, got %d", sharp.Tax)
	}
}

// A realm may only buy land its Daily Land Creation allowance covers, and the
// purchase draws that allowance down (BRE.OVR 0x12D30 / 0x12EF9).
func TestLandAllowanceLimitsPurchases(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxRegions = 100000 // isolate the allowance from the per-turn buy cap
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("me", "Mine")
	e.Gold = 1_000_000_000
	e.LandAvailable = 10

	if err := w.BuyRegions(e, &e.Regions.Coastal, 11); err != ErrNoLand {
		t.Errorf("buying past the allowance should fail with ErrNoLand, got %v", err)
	}
	if err := w.BuyRegions(e, &e.Regions.Coastal, 10); err != nil {
		t.Fatalf("buying exactly the allowance should succeed, got %v", err)
	}
	if e.LandAvailable != 0 {
		t.Errorf("allowance should be spent down to 0, got %d", e.LandAvailable)
	}
	if err := w.BuyRegions(e, &e.Regions.Coastal, 1); err != ErrNoLand {
		t.Errorf("an exhausted allowance should refuse further land, got %v", err)
	}
}

// Daily Land Creation tops every living realm's allowance up each day, which is
// what makes the config field mean something.
func TestDailyLandCreationTopsUpAllowance(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("me", "Mine")
	e.LandAvailable = 0
	w.LastMaintDate = "2026-07-31"

	w.DailyMaintenance("2026-08-01")

	if e.LandAvailable < cfg.LandPerDay {
		t.Errorf("a day of maintenance should grant %d land, got %d", cfg.LandPerDay, e.LandAvailable)
	}
}

// The AI runs covert operations of its own accord, not just one demoralize
// immediately before an attack (#57).
func TestAIAggressorRunsCovertOps(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Aggressor")
	a.AIProfile, a.Agents, a.Gold = AIProfileAggressor, 50, 1_000_000_000
	a.Regions = RegionMix{Coastal: 500}
	a.syncLand()
	victim := w.AddHuman("v", "Victim")
	victim.Regions = RegionMix{Coastal: 10}
	victim.syncLand()
	victim.Protection, victim.Support = 0, 100

	// Drive several turns' worth of the chance gate so the op fires.
	fired := false
	for day := 0; day < 40 && !fired; day++ {
		w.GameDay = day
		before := victim.Support
		w.aiCovertOps(a)
		if victim.Support != before || len(victim.Events) > 0 {
			fired = true
		}
	}
	if !fired {
		t.Error("an aggressor with agents and gold never ran a covert op across 40 turns")
	}
}

// Covert targets are never treaty partners: an AI that knifes its own allies
// makes diplomacy meaningless.
func TestAICovertSparesAllies(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Aggressor")
	a.AIProfile = AIProfileAggressor
	ally := w.AddHuman("b", "Ally")
	ally.Protection = 0
	w.ProposeTreaty(a, ally, "Full Defense Alliance")
	w.AcceptTreaty(ally, a.Name, "Full Defense Alliance")
	if !w.HasTreaty(a, ally, "Full Defense Alliance") {
		t.Fatal("test setup: the defense pact did not form")
	}

	if got := w.aiCovertTarget(a); got == ally {
		t.Error("an AI should not run covert ops against its own defense-pact partner")
	}
}
