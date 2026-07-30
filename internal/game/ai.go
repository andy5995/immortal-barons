package game

import (
	"hash/fnv"
	"io"
)

// AI personality archetypes (#36). An AI's profile shapes its diplomacy — which
// treaty offers it accepts — and is the seam for its future military posture and
// willingness to go to war. Assigned at seed (addAIEmpire); an empty profile
// (human empire, or an AI saved before profiles existed) derives a stable
// fallback from the empire name so old saves still get a personality.
const (
	AIProfileDiplomat  = "diplomat"  // accepts most cooperative pacts
	AIProfileBalanced  = "balanced"  // accepts trade and defense; guards intel/tech
	AIProfileAggressor = "aggressor" // accepts only self-serving economic pacts
)

// aiProfiles is the assignment pool, cycled at seed for an even spread.
var aiProfiles = []string{AIProfileDiplomat, AIProfileBalanced, AIProfileAggressor}

// aiProfile returns e's personality, deriving a stable one from its name when
// unset so AI empires saved before profiles existed still behave consistently.
func (e *Empire) aiProfile() string {
	if e.AIProfile != "" {
		return e.AIProfile
	}
	h := fnv.New32a()
	io.WriteString(h, e.Name)
	return aiProfiles[h.Sum32()%uint32(len(aiProfiles))]
}

// AI economic skill: sharp barons expand at full tilt; dull barons throttle their
// land-buying (AIDullLandBuyPct) and so grow slower, giving a game a mix of strong
// and weak rivals. Assigned randomly at seed (addAIEmpire).
const (
	AISkillSharp = "sharp"
	AISkillDull  = "dull"
)

// aiSkill returns e's economic skill, deriving a stable one from its name when
// unset so AI empires saved before skill existed still behave consistently.
func (e *Empire) aiSkill() string {
	if e.AISkill != "" {
		return e.AISkill
	}
	h := fnv.New32a()
	io.WriteString(h, e.Name+"skill") // distinct salt from aiProfile's name hash
	if h.Sum32()%2 == 0 {
		return AISkillDull
	}
	return AISkillSharp
}

// aiAcceptsTreaty reports whether an AI of the given profile accepts a proposed
// treaty type. A diplomat takes any cooperative pact; an aggressor takes only
// self-serving economic ones (and keeps its hands free to attack); a balanced
// realm takes trade and defense but guards its intelligence and technology.
func aiAcceptsTreaty(profile, ttype string) bool {
	switch profile {
	case AIProfileDiplomat:
		return true
	case AIProfileAggressor:
		return ttype == "Tariff Trade Agreement" || ttype == "Technology Agreement"
	default: // balanced
		return ttype != "Intelligence Alliance" && ttype != "Technology Agreement"
	}
}

// aiForceMix is how one personality splits its military budget by gold value.
// The five shares sum to 100; carriers are derived from jets, not shared for.
type aiForceMix struct{ trooper, turret, tank, jet, agent int }

// aiForceShares returns the gold shares an AI of the given profile spends its
// military budget on (#36, #71, #72). A diplomat never attacks, so it buys
// defense; an aggressor leans into offense; a balanced realm sits between and
// can punish a weak neighbour.
func aiForceShares(profile string) aiForceMix {
	switch profile {
	case AIProfileAggressor:
		return aiForceMix{AIForceTrooperPctWar, AIForceTurretPctWar, AIForceTankPctWar, AIForceJetPctWar, AIForceAgentPctWar}
	case AIProfileBalanced:
		return aiForceMix{AIForceTrooperPctMixed, AIForceTurretPctMixed, AIForceTankPctMixed, AIForceJetPctMixed, AIForceAgentPctMixed}
	default: // diplomat
		return aiForceMix{AIForceTrooperPct, AIForceTurretPct, AIForceTankPct, AIForceJetPct, AIForceAgentPct}
	}
}

// aiWarMargin is how far ahead an AI must be before it attacks, as a % of the
// target's effective defense. A diplomat returns 0, meaning it never attacks
// (#71) — its budget goes to defense instead, so it is passive by choice rather
// than by hoarding offense it never uses.
func aiWarMargin(profile string) int {
	switch profile {
	case AIProfileAggressor:
		return AIWarOffenseMargin
	case AIProfileBalanced:
		return AIWarOffenseMarginCautious
	default:
		return 0
	}
}

// effectiveDefense is a realm's total defensive strength as the battle math
// sees it: its unit defense plus the per-region land bonus. The AI uses this to
// judge whether a target is winnable (#36).
func effectiveDefense(e *Empire) int {
	return e.Defense() + e.Land*LandDefenseBonus
}

// aiWageWar lets an AI strike the weakest valid target when it is clearly
// favored (#36, #71). It reuses World.Targets (which already excludes dead,
// protected, and allied realms) and only commits when its offense beats the
// target's effective defense by its profile's margin, so it starts winnable
// wars rather than suicidal ones. Diplomats and protected AIs never start a war.
// At most one attack per call (one per turn, BRE-style).
func (w *World) aiWageWar(e *Empire) {
	margin := aiWarMargin(e.aiProfile())
	if e.Protection > 0 || margin == 0 {
		return
	}
	if !w.CanAttack(e) {
		return // used up the day's individual-attack allotment
	}
	var target *Empire
	for _, t := range w.Targets(e) {
		if target == nil || effectiveDefense(t) < effectiveDefense(target) {
			target = t
		}
	}
	if target == nil {
		return
	}
	if e.Offense() > effectiveDefense(target)*margin/100 {
		// Soften the target with a covert strike first: demoralized forces defend
		// worse, so the aggressor's agents pave the way for the attack (#36). The
		// op now carries a gold fee, so only attempt it when the AI can pay.
		if e.Agents > 0 && e.Gold >= CostDemoralizeForces {
			w.DemoralizeForces(e, target)
		}
		w.Attack(e, target, FullForce(e), true) // the AI commits its whole army
	}
}

// aiHandleDiplomacy responds to an AI's pending treaty offers (#36): it accepts
// the ones its personality favors and drops the rest, so a player (or another
// AI) proposing a pact to an AI gets a real answer instead of silence. Declined
// offers are discarded rather than left to re-prompt every turn.
func (w *World) aiHandleDiplomacy(e *Empire) {
	if len(e.TreatyOffers) == 0 {
		return
	}
	profile := e.aiProfile()
	for _, o := range append([]TreatyOffer(nil), e.TreatyOffers...) {
		if aiAcceptsTreaty(profile, o.Type) {
			w.AcceptTreaty(e, o.From, o.Type) // matches, consumes the offer, forms the treaty
		}
	}
	e.TreatyOffers = nil // discard any the AI declined
}

// aiSetTax picks the AI's tax rate each turn (#73). It never touched the rate
// before, so every bot realm ran its starting 15% for the life of the game.
//
// The choice follows the support model rather than a fixed number: support
// drifts by -(Tax-SupportTaxNeutral)/SupportTaxDivisor per turn, so a rate just
// under neutral earns the most gold while still gaining support, and riot risk
// (Tax²/10000) stays low there. When support slips below AISupportFloor the AI
// drops to AITaxRecover, under LowTaxBonusBelow, where the original hands back
// support outright — so a realm in trouble buys its way out instead of taxing
// itself into a riot spiral. Skill decides how well it plays this: a dull baron
// under-taxes even when healthy.
func (w *World) aiSetTax(e *Empire) {
	want := AITaxDull
	if e.aiSkill() == AISkillSharp {
		want = AITaxSharp
	}
	if e.Support < AISupportFloor {
		want = AITaxRecover
	}
	if cap := w.Config.MaxTaxRate; cap > 0 && want > cap {
		want = cap
	}
	e.Tax = want
}

// aiProposeTreaty lets an AI open diplomacy instead of only answering it (#73).
// It offers one pact per call, and only one its own personality would accept —
// an aggressor does not sue for a defense pact it would refuse if offered, so
// the planet's treaty web still reflects who these realms are.
//
// Chance-gated so pacts accumulate over a game rather than all forming on turn
// one, and keyed to the empire and turn so a replayed turn proposes the same
// thing (the same determinism the region yields use).
func (w *World) aiProposeTreaty(e *Empire) {
	if w.regionDraw(e, 91, 100) >= AIProposeTreatyPct {
		return
	}
	profile := e.aiProfile()
	var wanted []string
	for _, t := range TreatyTypes {
		if aiAcceptsTreaty(profile, t) {
			wanted = append(wanted, t)
		}
	}
	if len(wanted) == 0 {
		return
	}
	ttype := wanted[w.regionDraw(e, 92, len(wanted))]
	// Offer to the realm it is least likely to be fighting: the strongest other
	// realm, which is the one worth binding by treaty rather than by arms.
	var to *Empire
	for _, other := range w.Empires {
		// A pair holds one relation (#88), so proposing to a realm it is already
		// bound to would REPLACE that pact. An AI should not churn its own
		// diplomacy; it only courts realms it has no agreement with, or ones it is
		// currently at war with (suing for peace).
		if other == e || !other.Alive {
			continue
		}
		if rel := w.Relation(e, other); rel != "" && rel != RelationEnemy {
			continue
		}
		if to == nil || w.NetWorth(other) > w.NetWorth(to) {
			to = other
		}
	}
	if to != nil {
		w.ProposeTreaty(e, to, ttype)
	}
}

// aiCovertOps gives the AI a covert repertoire beyond the single pre-war
// demoralize it used to run (#57). Once a turn it may spend agents and gold on
// one operation, chosen by personality:
//
//   - an aggressor softens the realm it is most likely to hit next, stirring
//     revolts to cut the target's popular support (which cuts its income) or
//     demoralizing its forces to make them defend worse
//   - a balanced realm agitates whoever is ahead of it on net worth, which is
//     the cheapest way to slow a leader it cannot yet fight
//   - a diplomat plays defense only, shielding itself when it is the realm worth
//     attacking
//
// Everything is gated on affordability against the AI's working reserve, so
// covert spending never eats the food and maintenance budget. Chance-gated and
// keyed to the empire and turn, matching the determinism the rest of AI play
// uses.
func (w *World) aiCovertOps(e *Empire) {
	if e.Agents <= 0 || w.regionDraw(e, 93, 100) >= AICovertOpPct {
		return
	}
	spare := e.Gold - w.aiReserve(e)

	// A diplomat plays defense only. It shields itself when it is the realm worth
	// attacking, and never runs an offensive op — the same personality line that
	// stops it starting wars.
	if e.aiProfile() == AIProfileDiplomat {
		if w.aiIsLeader(e) && spare >= CostExposeEnemyOps && e.ShieldedUntilDay <= w.GameDay &&
			w.regionDraw(e, 94, 100) < AIExposeOpsPct {
			// ExposeEnemyOps shields the CALLER and ignores its target argument
			// entirely, so there is no rival to name here.
			w.ExposeEnemyOps(e, e)
		}
		return
	}

	target := w.aiCovertTarget(e)
	if target == nil {
		return
	}
	// Revolts are the cheaper op and hit income through popular support;
	// demoralizing is the pre-battle one and is reserved for a realm this AI
	// could actually follow up against.
	if spare >= CostDemoralizeForces && e.Offense() > effectiveDefense(target) {
		w.DemoralizeForces(e, target)
		return
	}
	if spare >= CostStirRevolts {
		w.StirRevolts(e, target)
	}
}

// aiCovertTarget picks who an AI works against: the realm it would attack if it
// could, else the strongest rival it is not bound to by treaty. Allies are never
// targeted — an AI that knifes its own treaty partners makes diplomacy
// meaningless.
func (w *World) aiCovertTarget(e *Empire) *Empire {
	var weakest, strongest *Empire
	for _, t := range w.Targets(e) {
		if weakest == nil || effectiveDefense(t) < effectiveDefense(weakest) {
			weakest = t
		}
		if strongest == nil || w.NetWorth(t) > w.NetWorth(strongest) {
			strongest = t
		}
	}
	if e.aiProfile() == AIProfileAggressor && weakest != nil {
		return weakest
	}
	return strongest
}

// aiIsLeader reports whether e currently tops the planet on net worth — the
// realm every aggressor is measuring itself against.
func (w *World) aiIsLeader(e *Empire) bool {
	for _, other := range w.Empires {
		if other != e && other.Alive && w.NetWorth(other) > w.NetWorth(e) {
			return false
		}
	}
	return true
}

// aiRegionMix is one personality's target share of land per region type. The
// five shares sum to 100; Agricultural is deliberately not among them because
// the food logic buys it on demand.
type aiRegionMix struct{ coastal, desert, mountain, industrial, river int }

// aiRegionShares returns the land mix an AI of the given profile builds toward.
// An aggressor leans industrial (it manufactures its own army) and holds enough
// mountains to cap the industry boost; a diplomat leans on income regions.
func aiRegionShares(profile string) aiRegionMix {
	switch profile {
	case AIProfileAggressor:
		return aiRegionMix{AIRegionCoastalPctWar, AIRegionDesertPctWar, AIRegionMountainPctWar, AIRegionIndustrialPctWar, AIRegionRiverPctWar}
	case AIProfileBalanced:
		return aiRegionMix{AIRegionCoastalPctMixed, AIRegionDesertPctMixed, AIRegionMountainPctMixed, AIRegionIndustrialPctMixed, AIRegionRiverPctMixed}
	default:
		return aiRegionMix{AIRegionCoastalPct, AIRegionDesertPct, AIRegionMountainPct, AIRegionIndustrialPct, AIRegionRiverPct}
	}
}

// aiNextRegionType picks which region type the AI should buy this turn: the one
// whose share of its land is furthest below the target for its personality.
// Buying the whole turn's purchase into that one type converges on the target
// over several turns and keeps each purchase a single BuyRegions call, so the
// per-turn cap and rising price apply exactly as they do for a human.
func (e *Empire) aiNextRegionType() *int {
	mix := aiRegionShares(e.aiProfile())
	total := max(1, e.Land)
	type want struct {
		field *int
		gap   int
	}
	// Gap is measured in regions, not percentage points, so a type that is short
	// by a lot of land outranks one short by a large fraction of very little.
	candidates := []want{
		{&e.Regions.Coastal, mix.coastal*total/100 - e.Regions.Coastal},
		{&e.Regions.Desert, mix.desert*total/100 - e.Regions.Desert},
		{&e.Regions.Mountain, mix.mountain*total/100 - e.Regions.Mountain},
		{&e.Regions.Industrial, mix.industrial*total/100 - e.Regions.Industrial},
		{&e.Regions.River, mix.river*total/100 - e.Regions.River},
	}
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.gap > best.gap {
			best = c
		}
	}
	if best.gap <= 0 {
		return &e.Regions.Coastal // at target everywhere: keep growing the income base
	}
	return best.field
}

// aiSetProduction points the AI's industry at the units its personality
// actually fields, replacing BRE's even 15%-to-everything default. Carriers are
// never manufactured: aiBuyCarriers buys precisely the lift the jets need, so
// industrial capacity spent guessing at it is wasted (see balance.go).
//
// Whatever the shares leave unallocated is paid out as industrial gold, so a
// defensive realm converts more of its industry to cash and an aggressor turns
// nearly all of it into an army.
func (w *World) aiSetProduction(e *Empire) {
	switch e.aiProfile() {
	case AIProfileAggressor:
		e.ProdTroopers, e.ProdJets, e.ProdTurrets = AIProdTrooperPctWar, AIProdJetPctWar, AIProdTurretPctWar
		e.ProdBombers, e.ProdTanks = AIProdBomberPctWar, AIProdTankPctWar
	case AIProfileBalanced:
		e.ProdTroopers, e.ProdJets, e.ProdTurrets = AIProdTrooperPctMixed, AIProdJetPctMixed, AIProdTurretPctMixed
		e.ProdBombers, e.ProdTanks = AIProdBomberPctMixed, AIProdTankPctMixed
	default:
		e.ProdTroopers, e.ProdJets, e.ProdTurrets = AIProdTrooperPct, AIProdJetPct, AIProdTurretPct
		e.ProdBombers, e.ProdTanks = AIProdBomberPct, AIProdTankPct
	}
	e.ProdCarriers = 0
	e.ProdInitialized = true
}

// aiUnderThreat reports whether some rival can currently beat this realm's
// defense. A sharp baron treats "a rival could win" as the trigger; a dull one
// waits until the rival is several times stronger, which is usually too late —
// the same skill split that governs how hard each expands.
//
// Allies are excluded: a Full Defense Alliance partner is not a threat, and
// panicking about one would make defense pacts pointless.
func (w *World) aiUnderThreat(e *Empire) bool {
	trigger := effectiveDefense(e)
	if e.aiSkill() == AISkillDull {
		trigger *= AIDullThreatFactor
	}
	for _, other := range w.Empires {
		if other == e || !other.Alive || w.AreAllied(e, other) {
			continue
		}
		if other.Offense() > trigger {
			return true
		}
	}
	return false
}

// aiSellIdleCarriers converts hulls the realm cannot use back into gold. Jets
// lost in battle leave their carriers behind, still drawing maintenance and
// lifting nothing; one carrier per JetsPerCarrier jets is all that is ever
// useful. Small, but it is gold the AI was simply burning.
func (w *World) aiSellIdleCarriers(e *Empire) {
	need := (e.Jets + JetsPerCarrier - 1) / JetsPerCarrier
	if surplus := e.Carriers - need; surplus > 0 {
		w.SellCarriers(e, surplus) // same sell path a human uses (buy price / 3)
	}
}
