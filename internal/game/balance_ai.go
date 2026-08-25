package game

// balance_ai.go — AI economic behaviour.
// Split out of balance.go; the provenance rules in that file's header
// apply here too, and each section below carries its own.

// --- AI economic behaviour (reconstructed / tunable) ---
//
// The AI mimics a human managing its realm: it keeps a few turns of food in
// reserve, and when its food production can't cover its army it expands
// Agriculture instead of buying more troops it can't feed (BRE realms grow food
// capacity alongside their military). Without this an AI buys troopers every
// turn until its food need outruns production and it starves.
const (
	AIFoodBufferTurns = 8  // turns of food consumption the AI keeps on hand (enough to ride out 5% per-turn spoilage without starving mid-day)
	AIAgriBuyMax      = 40 // Agricultural regions the AI buys per turn to keep food output ahead of a growing population

	// Force mix: once food-healthy, the AI spends AIMilitaryBudgetPct of its gold
	// on military each turn, split by gold-value across a mix chosen by its
	// personality (#36, #71, #72). Each profile's five shares sum to 100.
	//
	//   diplomat  — never attacks, so it buys defense and wastes nothing on punch
	//   balanced  — mixed; strikes only when overwhelmingly favoured
	//   aggressor — offense-heavy, and funds agents for pre-war covert work
	//
	// Carriers are NOT a share: jets cannot fight without them (JetsPerCarrier),
	// so the AI derives how many it needs from the jets it holds and buys those
	// first. Before this the AI bought no carriers at all and its seeded jets sat
	// inert for the life of the realm (#72).
	AIMilitaryBudgetPct = 50 // % of gold spent on military when food-healthy

	AIForceTrooperPct = 30 // diplomat: defensive mix
	AIForceTurretPct  = 45
	AIForceTankPct    = 10
	AIForceJetPct     = 10
	AIForceAgentPct   = 5

	AIForceTrooperPctMixed = 35 // balanced: can defend, can punish a weak neighbour
	AIForceTurretPctMixed  = 25
	AIForceTankPctMixed    = 25
	AIForceJetPctMixed     = 10
	AIForceAgentPctMixed   = 5

	AIForceTrooperPctWar = 35 // aggressor: offense-heavy
	AIForceTurretPctWar  = 5
	AIForceTankPctWar    = 40
	AIForceJetPctWar     = 12
	AIForceAgentPctWar   = 8

	// Banking (#36): rather than hoard the gold left after buying food and
	// military, the AI parks the clear surplus above a working reserve in
	// investments so idle gold earns instead of sitting.
	AIInvestPct = 50 // % of the surplus above the reserve to invest
	// The working reserve is AIReserveTurns turns of actual upkeep, floored at
	// AIGoldReserveMin so a tiny realm still keeps something back (#70). It was a
	// flat 50,000, which a grown realm earns back in a fraction of a turn — the
	// same threshold gated both land buying and investing however large the
	// economy got, so it stopped meaning anything.
	AIReserveTurns   = 3
	AIGoldReserveMin = 50_000

	// AILandBudgetPct is the share of the surplus above that reserve the AI puts
	// into land, leaving the rest for the army. Both spenders draw on the SAME
	// surplus: military spending used to take its cut of all gold including the
	// reserve, which meant a grown realm could never save past the reserve and
	// so stopped buying land for good. Giving expansion the entire surplus
	// instead ran realms straight into the daily land allowance and left wars
	// unwinnable, so the two split it.
	AILandBudgetPct = 40

	// Trading Market (#69). The AI used to ignore the market completely, so a
	// human's listings never sold and the AI never took the cheaper of two
	// prices. It now shops there first: it buys a good listed below what the
	// shop charges, capped so one baron cannot corner a listing, and it lists a
	// slice of its own surplus slightly under the shop price so the goods
	// actually move.
	AIMarketBuyDiscountPct = 10 // only buy when a listing undercuts the shop by at least this
	AIMarketBuyBudgetPct   = 25 // % of gold above the reserve spendable on market goods per turn
	AIMarketListPct        = 20 // % of a surplus good it will put up for sale
	// The undercut MUST exceed AIMarketBuyDiscountPct or no baron will ever meet
	// another's asking price and the market stays a museum piece — a sim with 5%
	// against a 10% threshold produced listings that nothing could ever buy.
	AIMarketUndercutPct = 15

	// Cash Relief loans (#69). The AI borrows only to cover a maintenance
	// shortfall — the thing that otherwise costs it desertion and revolts — and
	// never to fund expansion, which would compound debt it cannot service.
	AILoanDays          = 5   // term to borrow for; short, so the compounding stays small
	AILoanHeadroomPct   = 150 // borrow up to this % of the shortfall, for a small cushion
	AIDebtRepayPct      = 50  // % of the surplus above the reserve put toward debt
	AIMinSurplusToRepay = 10_000

	// Region rebalancing (#69). A realm that cannot feed itself and cannot
	// afford farmland is stuck; selling regions it has in surplus funds the
	// farmland instead of starving. Only fires when genuinely food-short.
	AIRebalanceSellMax = 50 // most regions sold in one rebalancing turn
	// AIDullLandBuyPct is the share of the affordable land-buy budget a dull-skill
	// AI spends each turn (a sharp AI spends it all). Because expansion compounds,
	// this is tuned empirically so a dull baron reaches ~700 regions over a day's
	// 15 turns against a sharp baron's ~1300. See aiExpandLand / aiSkill.
	AIDullLandBuyPct = 45

	// AIAgentsPerRegion caps covert stockpiling: agents are bought as a share of
	// the military budget, which on a large realm compounds into hundreds of
	// thousands of them with nothing to spend them on. A 30-day bot game left one
	// aggressor holding 360,000. The AI stops buying once it holds this many per
	// region it owns (#57).
	AIAgentsPerRegion = 2

	// Region strategy (#69 follow-on). The AI used to buy Coastal and nothing
	// else: a 30-day game left every baron at ~99% Coastal with ZERO Industrial,
	// zero River and only its five starting Mountains — so it manufactured no
	// units at all and never touched the mountain industry boost. Its whole army
	// was bought with gold.
	//
	// Each profile has a target share of its land per region type, and the AI buys
	// whichever type it is furthest below. Shares sum to 100. Agricultural is
	// absent on purpose: the food logic buys it on demand, so pinning a share too
	// would fight that.
	//
	// Mountain targets sit near 1/6 because the industry boost is
	// 1 + MountainIndustryNum*Mountain/Total capped at MountainIndustryCapPct —
	// at 3 and 150 that caps out at Mountain/Total = 1/6, and land past it buys
	// no more boost.
	AIRegionCoastalPct, AIRegionDesertPct, AIRegionMountainPct, AIRegionIndustrialPct, AIRegionRiverPct                          = 47, 25, 10, 8, 10
	AIRegionCoastalPctMixed, AIRegionDesertPctMixed, AIRegionMountainPctMixed, AIRegionIndustrialPctMixed, AIRegionRiverPctMixed = 37, 20, 15, 18, 10
	AIRegionCoastalPctWar, AIRegionDesertPctWar, AIRegionMountainPctWar, AIRegionIndustrialPctWar, AIRegionRiverPctWar           = 28, 15, 17, 30, 10

	// Production split (Set Industries). The AI never touched this and ran BRE's
	// default of 15% to every unit type, which is badly wrong once it owns
	// industry: a carrier costs 1,750 points against a jet's 140, so an even split
	// builds roughly one carrier per twelve jets when the fleet needs one per
	// hundred. A 60-day game left one baron holding 236,660 carriers for 389,667
	// jets, all of it drawing maintenance.
	//
	// Carriers are deliberately 0 here — aiBuyCarriers buys exactly the lift the
	// jets need, which is computable, so spending industrial capacity guessing at
	// it is waste. Each profile's shares total under 100; the remainder is paid as
	// industrial gold.
	AIProdTrooperPct, AIProdJetPct, AIProdTurretPct, AIProdBomberPct, AIProdTankPct                          = 15, 10, 45, 0, 5
	AIProdTrooperPctMixed, AIProdJetPctMixed, AIProdTurretPctMixed, AIProdBomberPctMixed, AIProdTankPctMixed = 20, 15, 30, 0, 20
	AIProdTrooperPctWar, AIProdJetPctWar, AIProdTurretPctWar, AIProdBomberPctWar, AIProdTankPctWar           = 20, 25, 5, 10, 35

	// Threat response. The AI built the same force mix whether or not a neighbour
	// could flatten it. When the strongest rival's offense outweighs this realm's
	// effective defense, it spends AIThreatBudgetPct of its gold on military
	// instead of the usual share, and buys the emergency mix below — turret-heavy,
	// because turrets are the cheapest defense per gold.
	//
	// Skill decides how early it notices: a sharp baron reacts as soon as a rival
	// can beat it, a dull one not until the rival is AIDullThreatFactor times
	// stronger, by which point it is usually too late. That is the difference
	// between a sharp and a dull realm under pressure.
	AIThreatBudgetPct  = 80
	AIDullThreatFactor = 2

	AIForceTrooperPctPanic = 25
	AIForceTurretPctPanic  = 65
	AIForceTankPctPanic    = 5
	AIForceJetPctPanic     = 0
	AIForceAgentPctPanic   = 5

	// Aggression gates: an AI attacks the weakest valid target only when its
	// offense exceeds that target's effective defense (units + land bonus) by this
	// %, so it picks winnable fights. Aggressors need a modest edge; balanced
	// realms need an overwhelming one, which is what makes them opportunists
	// rather than warmongers (#71). Diplomats never attack at all.
	AIWarOffenseMargin         = 130
	AIWarOffenseMarginCautious = 300

	// Tax policy (#73). The AI used to sit on its starting rate forever. Popular
	// support drifts by -(Tax-SupportTaxNeutral)/SupportTaxDivisor each turn, so
	// anything below the neutral rate GAINS support for free while riot risk is
	// Tax^2/10000 — a sharp baron therefore taxes just under neutral, where income
	// is highest and support still climbs. A dull one leaves money on the table.
	// The AI backs off to the recovery rate whenever support falls below the floor.
	AITaxSharp     = 28
	AITaxDull      = 18
	AITaxRecover   = 8 // below LowTaxBonusBelow, which buys support back outright
	AISupportFloor = 75

	// Diplomacy (#73). AI diplomacy was respond-only: it answered offers but never
	// made one, so a bot-only planet formed zero treaties. Each turn an AI may
	// propose one pact to one realm, at this percent chance, keeping the planet's
	// mail volume sane while pacts still accumulate over a game.
	AIProposeTreatyPct = 20

	// Covert (#57). The AI's only covert act was one demoralize immediately
	// before an attack, so agents piled up unused. Each turn it may run one op,
	// at this chance, against a rival — softening a war target if it has one, or
	// otherwise agitating the realm ahead of it on the scoreboard. It only spends
	// gold it can spare, so covert never starves food or maintenance.
	AICovertOpPct = 25
	// A realm shields itself when it is worth attacking: covert defense is worth
	// buying once a realm leads, since it is then everyone's target. The shield
	// needs a bribed agent inside the realm it guards against, so at this chance
	// the AI either buys that bribe or spends the shield it already has.
	AIExposeOpsPct = 10
)

// ProclamationChancePct is how often the Queen Royale's daily proclamation
// appears in the planetary news (see news_quotes.go). Flavour only — it moves
// nothing in the economy, so it is tuned purely for how often a joke stays
// funny.
const ProclamationChancePct = 35

// Breaking a pact by ATTACKING a partner costs nothing — see breachTreaty. IB
// used to charge 10 popular support here; the original charges nothing, because
// no attack path reads the relation at all. Declaring war is the route that
// costs (a quarter of both support and morale, DeclareWar).

// Declaring war costs a quarter of BOTH popular support and military morale.
// BINARY-VERIFIED (BRE.OVR 0x01a838, break_diplomatic_treaty): on the
// confirmation, popular support at record +0x92 and military morale at +0x8e are
// each divided by four and multiplied by three, in that order, before either
// side's relation row is cleared. The truncation is on the divide, so 99 keeps
// 72 rather than 74 — hence a numerator and denominator here instead of a
// percentage.
//
// This CONTRADICTS the manual, which says the declaration breaks a pact
// "without causing internal troubles in your realm". The binary's own message on
// the same screen speaks of revolts and morale dropping severely, so the manual
// is describing an intention the shipped game does not honour, and the code
// wins (a disassembly outranks the docs for mechanics).
const (
	DeclareWarKeepNumerator   = 3
	DeclareWarKeepDenominator = 4
)

// Trade-treaty income, per turn, per partner. BINARY-VERIFIED inside
// process_economic_production: the Tariff branch at BRE.OVR 0x03416b and the
// Free Trade branch at 0x0341f0 each take
//
//	min(myPopulation, partnerPopulation) x rate
//
// and add it to the turn's gold. The rate is assembled inline from the two
// realms' New Realm Protection flags — 6 - 3*protected(self) - 2*protected(partner)
// for a Tariff, 11 - 5*protected(self) - 5*protected(partner) for Free Trade —
// so a sheltered newcomer cannot open a trade pact and farm it, and the pact is
// worth less to you while YOU are the sheltered one.
//
// UNITS: these are gold per BRE population unit, i.e. per MILLION (record
// +0x62), so tradePactIncome converts IB's count down through PopBREUnitScale
// before applying them — the same way peopleFood and SupportBoostCost do. At
// starting scale that is 600 gold a turn from an equal-sized tariff partner
// against a 5,100-gold tax take, which is the "extra income... promising for
// those empires with large numbers of people" the manual describes. Applying
// them to IB's People count directly pays twenty times that.
const (
	TariffTradeGoldPerHead = 6  // binary
	FreeTradeGoldPerHead   = 11 // binary

	TariffTradeProtectedSelfCut    = 3 // binary
	TariffTradeProtectedPartnerCut = 2 // binary
	FreeTradeProtectedSelfCut      = 5 // binary
	FreeTradeProtectedPartnerCut   = 5 // binary
)

// Tax coefficient (reconstructed / tunable). BRE stores population/tax income
// as an inline "6 − f(tax)" × Population shape that was only partially
// recovered. Calibrated to BRE's first-turn income report: a new realm
// (People 2000, Tax 15%) earns 2000·0.15·17 = 5100 gold in taxes, matching BRE's
// ~5183 — so taxes are a minor part of income (region income dominates, as in
// BRE), not the runaway 360k the old 1200 produced. Top playtest knob.
const TaxGoldPerCapita = 17
