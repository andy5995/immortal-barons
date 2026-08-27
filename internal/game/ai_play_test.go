package game

import (
	"fmt"
	"testing"
)

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
		w.resolveCovertQueue() // the AI queues like a player; the effect lands at maintenance
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

// A pair of empires holds exactly ONE relation, matching BRE's enum (#88).
// Forming a new one replaces whatever stood before, so diplomacy is a choice
// rather than an accumulation.
func TestOneRelationPerPair(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")

	form := func(ttype string) {
		w.ProposeTreaty(a, b, ttype)
		if !w.AcceptTreaty(b, a.Name, ttype) {
			t.Fatalf("setup: %s was not accepted", ttype)
		}
	}
	form("Full Defense Alliance")
	if !w.AreAllied(a, b) {
		t.Fatal("setup: the defense alliance did not take effect")
	}

	form("Tariff Trade Agreement")
	if got := w.TreatiesBetween(a, b); len(got) != 1 || got[0] != "Tariff Trade Agreement" {
		t.Errorf("a second pact should replace the first, got %v", got)
	}
	if w.AreAllied(a, b) {
		t.Error("trading away a defense alliance should end it")
	}
}

// Breaking a pact by attacking a partner ends the relation and costs the
// breaker NOTHING. That is the original's shape and it is deliberately the
// opposite way round from intuition: the crown charges for the DECLARATION
// (TestDeclareWarCostsSupportAndMorale), not for the betrayal, which the other
// players are left to punish. IB charged 10 support here until it was read.
func TestAttackingAPartnerBreachesAndCostsNothing(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")
	w.ProposeTreaty(a, b, "Tariff Trade Agreement")
	w.AcceptTreaty(b, a.Name, "Tariff Trade Agreement")
	a.Support, a.Morale, a.Protection, b.Protection = 90, 80, 0, 0

	w.Attack(a, b, FullForce(a), true)

	if got := w.Relation(a, b); got != RelationEnemy {
		t.Errorf("attacking a partner should leave the pair at %q, got %q", RelationEnemy, got)
	}
	if a.Support != 90 {
		t.Errorf("breaching a pact should cost no support: 90 -> %d", a.Support)
	}
	// Attacking a realm you had no pact with is not a breach.
	c := w.AddHuman("c", "Gamma")
	c.Protection = 0
	a.Support = 90
	w.Attack(a, c, FullForce(a), true)
	if a.Support != 90 {
		t.Errorf("attacking a non-partner is no breach, 90 -> %d", a.Support)
	}
}

// A save written before #88 could stack several relations on one pair; loading
// must collapse each pair to a single one.
func TestEnsureTreatiesCollapsesStackedPairs(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Treaties = []Treaty{
		{Type: "Full Defense Alliance", A: "Alpha", B: "Beta"},
		{Type: "Tariff Trade Agreement", A: "Alpha", B: "Beta"},
		{Type: "Free Trade Agreement", A: "Alpha", B: "Gamma"},
	}

	w.EnsureTreaties()

	if len(w.Treaties) != 2 {
		t.Fatalf("want one relation per pair (2 pairs), got %v", w.Treaties)
	}
	for _, tr := range w.Treaties {
		if tr.A == "Alpha" && tr.B == "Beta" && tr.Type != "Tariff Trade Agreement" {
			t.Errorf("the last recorded relation should win, got %q", tr.Type)
		}
	}
}

// The AI builds a mixed realm rather than buying one region type forever. It
// used to buy only Coastal, ending a 30-day game at ~99% Coastal with zero
// Industrial and only its five starting Mountains.
func TestAIRegionSharesSumTo100(t *testing.T) {
	for _, p := range []string{AIProfileDiplomat, AIProfileBalanced, AIProfileAggressor} {
		m := aiRegionShares(p)
		if sum := m.coastal + m.desert + m.mountain + m.industrial + m.river; sum != 100 {
			t.Errorf("%s region shares sum to %d, want 100", p, sum)
		}
	}
	if aiRegionShares(AIProfileAggressor).industrial <= aiRegionShares(AIProfileDiplomat).industrial {
		t.Error("an aggressor should build more industry than a diplomat: it manufactures its army")
	}
	// Mountain targets should not overshoot the industry-boost cap, which tops out
	// at Mountain/Total = MountainIndustryCapPct-100 over MountainIndustryNum*100.
	capShare := (MountainIndustryCapPct - 100) / MountainIndustryNum
	for _, p := range []string{AIProfileDiplomat, AIProfileBalanced, AIProfileAggressor} {
		if got := aiRegionShares(p).mountain; got > capShare+2 {
			t.Errorf("%s targets %d%% mountain, past the ~%d%% where the boost caps", p, got, capShare)
		}
	}
}

// The AI buys whichever type it is furthest short of, so a realm of nothing but
// Coastal expands into something else next.
func TestAIBuysTheTypeItIsShortOf(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.AIProfile = AIProfileAggressor
	e.Regions = RegionMix{Coastal: 1000}
	e.syncLand()

	if got := e.aiNextRegionType(); got == &e.Regions.Coastal {
		t.Error("an all-Coastal realm should buy something other than Coastal next")
	}
	// Industry is the aggressor's largest target, so it should be first in line.
	if got := e.aiNextRegionType(); got != &e.Regions.Industrial {
		t.Error("an aggressor short of everything should buy Industrial first")
	}
}

// Industry must not manufacture carriers: aiBuyCarriers buys exactly the lift
// the jets need. BRE's even 15% default built roughly one carrier per twelve
// jets when one per hundred is needed.
func TestAIProductionNeverBuildsCarriers(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	for _, p := range []string{AIProfileDiplomat, AIProfileBalanced, AIProfileAggressor} {
		e := w.AddHuman("h"+p, "Realm "+p)
		e.AIProfile = p
		w.aiSetProduction(e)
		if e.ProdCarriers != 0 {
			t.Errorf("%s allocates %d%% to carriers; aiBuyCarriers owns that", p, e.ProdCarriers)
		}
		sum := e.ProdTroopers + e.ProdJets + e.ProdTurrets + e.ProdBombers + e.ProdTanks + e.ProdCarriers
		if sum > 100 {
			t.Errorf("%s allocates %d%% of industry, over 100", p, sum)
		}
		if sum == 100 {
			t.Errorf("%s leaves nothing for industrial gold", p)
		}
	}
}

// A realm that a rival can beat shifts to defense; skill decides how early it
// notices. A dull baron waits until the rival is several times stronger.
func TestAIThreatResponseBySkill(t *testing.T) {
	mk := func(skill string) (*World, *Empire, *Empire) {
		w := NewWorldSeed(DefaultConfig(), 1)
		me := w.AddHuman("me", "Mine")
		me.AISkill, me.Troopers, me.Turrets, me.Tanks = skill, 0, 0, 0
		me.Regions = RegionMix{Coastal: 10}
		me.syncLand()
		bully := w.AddHuman("b", "Bully")
		bully.Troopers = 1
		return w, me, bully
	}
	// A rival just strong enough to win: the sharp baron sees it, the dull one not.
	w, sharp, bully := mk(AISkillSharp)
	bully.Troopers = effectiveDefense(sharp) + 1
	if !w.aiUnderThreat(sharp) {
		t.Error("a sharp baron should react as soon as a rival can beat it")
	}
	w2, dull, bully2 := mk(AISkillDull)
	bully2.Troopers = effectiveDefense(dull) + 1
	if w2.aiUnderThreat(dull) {
		t.Error("a dull baron should not yet react to a rival only just ahead")
	}
	bully2.Troopers = effectiveDefense(dull)*AIDullThreatFactor + 1
	if !w2.aiUnderThreat(dull) {
		t.Error("a dull baron should react once badly outgunned")
	}

	// An ally is never a threat, or defense pacts would be pointless.
	w3, e, ally := mk(AISkillSharp)
	ally.Troopers = effectiveDefense(e) * 100
	w3.ProposeTreaty(e, ally, "Full Defense Alliance")
	w3.AcceptTreaty(ally, e.Name, "Full Defense Alliance")
	if w3.aiUnderThreat(e) {
		t.Error("a defense-pact partner should not count as a threat")
	}
}

// Jets lost in battle strand their carriers, which keep drawing maintenance and
// lift nothing. The AI sells the surplus back.
func TestAISellsIdleCarriers(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Jets, e.Carriers, e.Gold = 0, 50, 0

	w.aiSellIdleCarriers(e)

	if e.Carriers != 0 {
		t.Errorf("carriers with no jets are useless; want 0 kept, got %d", e.Carriers)
	}
	if e.Gold <= 0 {
		t.Error("selling the surplus should return gold")
	}
	// It must keep the lift the jets it still has need.
	e.Jets, e.Carriers = 150, 10
	w.aiSellIdleCarriers(e)
	if want := (150 + JetsPerCarrier - 1) / JetsPerCarrier; e.Carriers != want {
		t.Errorf("should keep %d carriers for 150 jets, kept %d", want, e.Carriers)
	}
}

// Exercises the real call path: a threatened realm going through aiBuildForces
// must come out turret-heavy. The unit tests above call aiUnderThreat directly,
// so they kept passing when the call site in aiBuildForces was accidentally
// reverted and the behaviour became unreachable from actual play.
func TestThreatResponseReachesBuildForces(t *testing.T) {
	build := func(threatened bool) *Empire {
		w := NewWorldSeed(DefaultConfig(), 1)
		e := w.AddHuman("me", "Mine")
		e.AIProfile, e.AISkill = AIProfileAggressor, AISkillSharp
		e.Regions = RegionMix{Coastal: 50}
		e.syncLand()
		e.Troopers, e.Turrets, e.Tanks, e.Jets = 0, 0, 0, 0
		e.Gold = 50_000_000
		bully := w.AddHuman("b", "Bully")
		if threatened {
			bully.Troopers = effectiveDefense(e) * 10
		} else {
			bully.Troopers = 0
		}
		w.aiBuildForces(e)
		return e
	}
	calm, scared := build(false), build(true)
	if scared.Turrets <= calm.Turrets {
		t.Errorf("a threatened aggressor should buy more turrets: calm %d, threatened %d", calm.Turrets, scared.Turrets)
	}
	if scared.Tanks >= calm.Tanks {
		t.Errorf("a threatened aggressor should buy fewer tanks: calm %d, threatened %d", calm.Tanks, scared.Tanks)
	}
}

// A winning AI allocates the land it captures into the type it is short of,
// the way a human does at the capture prompt, rather than inheriting whatever
// mix the loser happened to hold.
func TestAIAllocatesCapturedLand(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Attacker")
	a.AIProfile, a.AISkill = AIProfileAggressor, AISkillSharp
	a.Protection, a.Gold = 0, 10_000_000
	a.Regions = RegionMix{Coastal: 400} // nothing but Coastal: short of industry
	a.syncLand()
	a.Troopers, a.Tanks = 500_000, 200_000

	d := w.AddHuman("d", "Defender")
	d.Protection = 0
	d.Regions = RegionMix{Desert: 300} // the loser holds only Desert
	d.syncLand()
	d.Troopers, d.Turrets, d.Tanks = 1, 0, 0

	beforeIndustrial, beforeDesert := a.Regions.Industrial, a.Regions.Desert
	w.aiWageWar(a)

	if a.Regions.Industrial <= beforeIndustrial {
		t.Errorf("captured land should go where the realm is short (Industrial), got %d -> %d",
			beforeIndustrial, a.Regions.Industrial)
	}
	if a.Regions.Desert != beforeDesert {
		t.Errorf("the attacker should not simply inherit the loser's Desert: %d -> %d",
			beforeDesert, a.Regions.Desert)
	}
}

// --- AI economic levers (#69): market, loans, region rebalancing ---

// TestAIBuysBelowShopPrice: the AI ignored the general market entirely, so a
// human's listings never sold and it paid shop price for stock it could get
// cheaper. It should take a clearly-undercut listing.
func TestAIBuysBelowShopPrice(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	seller := w.AddHuman("seller", "Sellville")
	seller.Protection, seller.Troopers = 0, 5000
	ai := w.AddAIEmpires(1)
	_ = ai
	bot := w.Empires[len(w.Empires)-1]
	bot.Protection, bot.Gold = 0, 10_000_000
	before := bot.Troopers

	cheap := w.TrooperPrice(bot) / 2 // well under the discount threshold
	if err := w.SetMarketListing(seller, "Trooper", 1000, cheap); err != nil {
		t.Fatalf("list: %v", err)
	}
	w.aiShopMarket(bot)

	if bot.Troopers <= before {
		t.Errorf("the AI should buy troopers listed at half the shop price (had %d, now %d)", before, bot.Troopers)
	}
	if w.MarketForSale(seller.Name, "Trooper") == 1000 {
		t.Error("the seller's listing should have been drawn down")
	}
}

// It must NOT buy a listing priced at or above the shop — that is strictly worse
// than the shop, and buying it would make the market a trap for the AI.
func TestAIIgnoresOverpricedListings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	seller := w.AddHuman("seller", "Sellville")
	seller.Troopers = 5000
	w.AddAIEmpires(1)
	bot := w.Empires[len(w.Empires)-1]
	bot.Protection, bot.Gold = 0, 10_000_000
	before := bot.Troopers

	w.SetMarketListing(seller, "Trooper", 1000, w.TrooperPrice(bot)*2)
	w.aiShopMarket(bot)

	if bot.Troopers != before {
		t.Errorf("the AI must not pay above shop price on the market (had %d, now %d)", before, bot.Troopers)
	}
}

// TestAIBorrowsToCoverShortfall: rather than eat desertion and revolts, the AI
// takes a short loan when it cannot meet the turn's maintenance.
func TestAIBorrowsToCoverShortfall(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	w.AddAIEmpires(1)
	bot := w.Empires[0]
	bot.Regions = RegionMix{Coastal: 500}
	bot.syncLand()
	bot.Gold = 0 // cannot pay a thing

	w.aiManageDebt(bot)

	if len(bot.Loans) == 0 {
		t.Fatal("a broke AI facing maintenance should borrow")
	}
	if bot.Gold <= 0 {
		t.Errorf("the loan should have credited gold, got %d", bot.Gold)
	}
	// It borrows what the ceiling allowed, which may not cover the whole bill:
	// LoanCeiling is 10 x net worth discounted by the term's compound factor, and
	// a region's net worth (12.5) is tiny beside its 913 upkeep, so a land-heavy
	// realm cannot borrow its way out entirely.
	// Partial cover still buys down the desertion/revolt penalty. (The ceiling
	// reads 0 afterwards because the loan it just took is now outstanding.)
	if got, want := bot.Loans[0].Principal, bot.Gold; got != want {
		t.Errorf("gold %d should equal the %d borrowed", want, got)
	}
	if bot.Loans[0].Principal <= 0 {
		t.Error("borrowed nothing")
	}
}

// A solvent AI must not borrow — debt it does not need still compounds.
func TestAISolventDoesNotBorrow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	w.AddAIEmpires(1)
	bot := w.Empires[0]
	bot.Gold = 100_000_000

	w.aiManageDebt(bot)

	if len(bot.Loans) != 0 {
		t.Error("a solvent AI should not take a loan")
	}
}

// TestAISellsRegionsWhenStarving: an AI whose population outgrew its farmland
// and which cannot afford more should convert surplus land into gold rather
// than starve holding it.
func TestAISellsRegionsWhenStarving(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	w.AddAIEmpires(1)
	bot := w.Empires[0]
	bot.Regions = RegionMix{Coastal: 400, Agricultural: 1}
	bot.syncLand()
	bot.People = 5_000_000
	bot.Gold = 0
	bot.LandAvailable = 100
	coastalBefore := bot.Regions.Coastal

	if w.FoodGrown(bot) >= bot.FoodUpkeep() {
		t.Fatal("test setup: the realm is not actually food-short")
	}
	w.aiRebalanceRegions(bot)

	if bot.Regions.Coastal >= coastalBefore {
		t.Errorf("a starving, broke AI should sell surplus regions (coastal %d -> %d)", coastalBefore, bot.Regions.Coastal)
	}
	if bot.Gold <= 0 {
		t.Errorf("selling regions should raise gold, got %d", bot.Gold)
	}
}

// A well-fed AI must not sell land — that would dismantle a healthy realm.
func TestAIWellFedKeepsItsRegions(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 0
	w := NewWorldSeed(cfg, 1)
	w.AddAIEmpires(1)
	bot := w.Empires[0]
	bot.Regions = RegionMix{Coastal: 100, Agricultural: 200}
	bot.syncLand()
	bot.People = 1000
	before := bot.Regions.Coastal

	w.aiRebalanceRegions(bot)

	if bot.Regions.Coastal != before {
		t.Errorf("a well-fed AI must keep its regions (%d -> %d)", before, bot.Regions.Coastal)
	}
}

// TestGroundDownRealmsGetFinished guards the #87 balance fix: a realm beaten
// down must actually die, rather than rebuy land faster than attackers can take
// it and linger as a permanently-farmed "zombie".
//
// When #87 was filed, a bot-only probe ran 60 days with ~12 battles a day and
// eliminated NOBODY. What changed is not the capture rule (10% + a 15-region
// floor is BRE-verified and ratio-independent, so it is not a knob) but the
// cost of holding land: at 913 gold per region per turn, a stripped realm can
// no longer afford to rebuild, so the death spiral completes.
//
// SEVERAL seeds, deliberately. An earlier version of this test ran one seed and
// also asserted that no realm ground below 200 regions ever survived the 60
// days. That second claim is simply false — measured across 24 seeds it happens
// about 0.9 times per run, at both 15 and 20 turns of protection. The old test
// passed because seed 23 happens not to hit it, which came to light only when a
// protection-default change reshuffled the trajectory. A guard that holds on one
// seed is not measuring the property it names.
//
// So this asserts what is actually true and is what #87 was about: wars
// conclude. If it fails, the economy has drifted back to making conquest
// impossible.
func TestGroundDownRealmsGetFinished(t *testing.T) {
	const seeds = 8
	total, barren := 0, 0
	for seed := int64(1); seed <= seeds; seed++ {
		cfg := DefaultConfig()
		w := NewWorldSeed(cfg, seed)
		w.AddAIEmpires(8)
		start := len(w.Empires)

		for d := 1; d <= 60; d++ {
			w.DailyMaintenance(fmt.Sprintf("2026-%02d-%02d", 8+d/28, 1+d%28))
		}

		alive := 0
		for _, e := range w.Empires {
			if e.Alive {
				alive++
			}
		}
		killed := start - alive
		total += killed
		if killed == 0 {
			barren++
			t.Errorf("seed %d: 60 days and %d battles eliminated nobody — wars cannot conclude again (#87)",
				seed, w.BattlesTotal)
		}
	}
	// Measured 1.75 per run across 24 seeds (2026-08-26), with no seed at zero.
	// It read 3.5 at 20 turns of protection and 2.7 at 15 while IB's capture
	// carried a net-worth density boost of its own; that boost let a soft realm
	// lose twice the original's share and was removed under #200, which is what
	// halved the rate — the no-people conquest trigger, removed alongside it,
	// moved the figure not at all. One is under the measured mean and still
	// clear of the zero this ticket was filed for.
	if mean := float64(total) / seeds; mean < 1 {
		t.Errorf("mean eliminations %.1f per 60-day run across %d seeds — too few; conquest is stalling (#87)",
			mean, seeds)
	}
}
