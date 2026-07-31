package game

// balance.go — the game's tunable economy balance data, in one place.
//
// This is a "config that isn't changed at runtime": compiled-in, type-safe
// constants that the economy formulas in turn.go, economy.go, specials.go, and
// game.go reference by name. Keeping every balance number here (rather than
// scattered as literals across the formula code) makes the whole economy
// legible and tunable from a single file.
//
// Provenance is marked per group:
//   - "BRE-verified" values were read directly as immediate operands from a
//     disassembly of the original binary (BRE.OVR turn-economy routine, file
//     offsets 0x342C0–0x34A4E; HIGH confidence, cross-checked against bre.doc).
//     Do not "tune" these away from the recovered numbers without a new
//     disassembly — they are the fidelity contract.
//   - "reconstructed / tunable" values are IB's own, anchored to the BRE scale.
//     These are the playtest knobs.

// --- Region income (BRE-verified: BRE.OVR 0x342C0–0x34A4E) ---
//
// Per region, per turn: perRegion = Base + a uniform draw in [0, Rate), so Rate
// is the full width of the swing and Base its floor (see World.regionDraw, which
// mirrors BRE's Random(Rate) + Base). Each region type draws independently —
// there is no planet-wide year factor; in one live turn desert drew near the top
// of its band while agriculture drew zero.
//
// The band is live-verified across four region types in two separate games, by
// back-computing the draw from these Rate/Base pairs: desert 2.6–99.8% of Rate,
// tourism 0.4–99.8, ore 0.5–99.8, hydro 2.0–99.0. Two earlier readings were
// wrong and are recorded so nobody re-derives them: a reconstructed 1.0–1.5
// multiplier ran high, and a later 0.30–0.80 band came from 12 turns — the
// sample size at which any uniform looks narrow.
//
// Coastal is additionally multiplied by a support factor (see IncomeThisTurn);
// Urban and Technology produce NO direct gold (housing / maintenance-reduction
// only).
const (
	MountainRate, MountainBase = 400, 3550  // ore — smallest Rate, most stable
	CoastalRate, CoastalBase   = 1000, 3750 // tourism — × support factor
	DesertRate, DesertBase     = 2000, 3000 // solar — widest swing
	RiverRate, RiverBase       = 100, 5000  // hydro — highest base, occasional bad-year dud
)

// RiverDudChancePct is the chance a River's hydropower has a "bad year" and pays
// half. Kept at the ~10% the game has always used.
//
// UNVERIFIED, and the live evidence is against it: across roughly 29 captured
// hydropower turns every figure was a clean Base + [0, Rate) draw, with no
// halved value. That is not conclusive on its own (a true 10% rate survives 29
// samples about 5% of the time), so the mechanic is left in place rather than
// removed on borderline evidence — but it should be settled before anyone tunes
// it. Removing it entirely is the likelier correct answer.
const RiverDudChancePct = 10

// CrownTaxSupportPenalty bounds the popular support a baron loses by failing to
// pay the Queen's tax in full. It is a ceiling the penalty approaches but never
// reaches: the +1 on each side means paying nothing costs 14 points, not 15.
// Binary-verified (BRE.OVR 0x2FA90–0x2FAEE):
//
//	penalty = trunc((1 - (paid+1)/(required+1)) * CrownTaxSupportPenalty)
//
// BRE uses the same channel for region-maintenance shortfall but at 50, and
// docks morale rather than support at 40 for unpaid forces. The tax is the
// mildest of the three.
const CrownTaxSupportPenalty = 15

// --- New-realm starting setup ---
//
// A fresh realm's regions and units. The region mix, trooper count, and food
// are BRE-verified from a live BRE new-empire screen (2 Agricultural, 5 Desert,
// 5 Mountain, 3 Coastal = 15 regions; 100 troopers; 1000 food; full morale, no
// other units). Gold, population, and tax are IB's own start values.
const (
	StartAgricultural = 2
	StartDesert       = 5
	StartMountain     = 5
	StartCoastal      = 3
	StartTroopers     = 100
	StartFood         = 1000
	StartGold         = 10000
	StartPeople       = 2000
	StartTax          = 15
)

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
	// buying once a realm leads, since it is then everyone's target.
	AIExposeOpsPct = 10
)

// ProclamationChancePct is how often the Queen Royale's daily proclamation
// appears in the planetary news (see news_quotes.go). Flavour only — it moves
// nothing in the economy, so it is tuned purely for how often a joke stays
// funny.
const ProclamationChancePct = 35

// TreatyBreachSupportPenalty is the popular support a baron loses for ending an
// agreement by attacking a partner instead of declaring war first. BRE's manual
// says Declaration Of War breaks a pact "without causing internal troubles in
// your realm", so the route that skips it must cause them — the original does
// not publish the size, and this is IB's own figure. Tunable.
const TreatyBreachSupportPenalty = 10

// Tax coefficient (reconstructed / tunable). BRE stores population/tax income
// as an inline "6 − f(tax)" × Population shape that was only partially
// recovered. Calibrated to BRE's first-turn income report: a new realm
// (People 2000, Tax 15%) earns 2000·0.15·17 = 5100 gold in taxes, matching BRE's
// ~5183 — so taxes are a minor part of income (region income dominates, as in
// BRE), not the runaway 360k the old 1200 produced. Top playtest knob.
const TaxGoldPerCapita = 17

// --- Unit / land / food prices (reconstructed / tunable) ---
//
// BRE does not store these as constants (they're computed inline), so they are
// IB's reconstructed values, re-anchored to the BRE income magnitude while
// preserving IB's ratios (which match BRE: ~7 troopers per tank, ~6 jets per
// tank). Used as the DefaultConfig / NewWorldSeed price defaults.
const (
	// Unit buy prices — BRE early-game live snapshot (2026-07-11). These are the
	// BASE each empire's per-turn price walk starts from and stays near (see the
	// PriceWalkStep*/PriceWalkBandPct block below and World.stepPrices, #30). Sell
	// price is buy/3 (see sellUnit), except agents (SellAgentPrice).
	PriceTrooper = 263
	PriceJet     = 345
	PriceTurret  = 380
	PriceTank    = 2172
	PriceCarrier = 5943
	PriceAgent   = 608
	PriceBomber  = 2572
	// Region price rises with land owned: BRE ≈ 917 + Land×33 (live-sampled). See
	// World.LandPrice. PriceLand is the base; LandPerRegion the per-owned climb.
	PriceLand     = 917
	LandPerRegion = 33
	// SellAgentPrice: agents sell at a flat 100 in BRE, not buy/3 like other units.
	SellAgentPrice = 100
)

// Covert Operations gold costs, charged per op on top of the agent risk
// (live-sampled from BRE's Covert Operations menu at the DEFAULT/medium game
// setup, 2026-07-21 — other BRE setups scale these; tune here as needed). Bomb
// Enemy Targets is one 100k menu entry in BRE; IB splits it into a submenu and
// charges each variant CostBombEnemyTargets.
const (
	CostSendSpy            = 5_000
	CostStirRevolts        = 25_000
	CostSetUp              = 50_000
	CostSupportDissensions = 80_000
	CostDemoralizeForces   = 80_000
	CostSpyOnRelations     = 100_000
	CostBombEnemyTargets   = 100_000
	CostBribery            = 200_000
	CostExposeEnemyOps     = 600_000
)

// Per-turn price WALK (#30). BRE recomputes each unit's buy price every turn as a
// persistent random walk with per-empire state — confirmed live (2026-07-15): a
// fresh empire started at base while a veteran on the same day had drifted (bomber
// +22%, agent +100%), and the drift carried across the day boundary. IB matches
// that: each empire stores its own current prices (Empire.Prices) and steps them
// once per turn in PlayTurn (World.stepPrices). A step moves a price up or down by
// up to PriceWalkStep*% of its base, clamped to ±PriceWalkBandPct% of the base so
// it drifts like BRE but can't run away. The stored price is what the Spending menu
// shows AND what a buy/sell charges within the turn (shown == charged); it persists
// across days via the save. Steps are deterministic (keyed per empire+turn like
// riversFish) so play is reproducible and concurrency-safe. Cheap units take small
// steps; agents step wider (matching the calibration). Regions do NOT walk — their
// price is holdings-only (917+owned×33), which BRE held exact every turn. Tunable.
const (
	PriceWalkStepTrooper = 3
	PriceWalkStepJet     = 3
	PriceWalkStepTurret  = 3
	PriceWalkStepTank    = 3
	PriceWalkStepBomber  = 3
	PriceWalkStepCarrier = 3
	PriceWalkStepAgent   = 8
	// PriceWalkBandPct caps how far any price may drift from its base. BRE's bomber
	// reached ~+22% in a 14-turn sample and was still climbing, so keep it generous.
	PriceWalkBandPct = 30
)

// BombMarketLossPct is the share (percent) of a target's listed goods and pending
// market proceeds destroyed by a successful Bomb Trading Market covert op (#17).
const BombMarketLossPct = 25

// MarketCommissionPct is the cut (percent) the general Trading Market takes from
// a seller's proceeds at day-end settlement (#17). BRE's exact value was not
// observable live (a real cross-empire sale needs two non-protected empires,
// which the protection grind + self-buy refusal blocked), so it defaults to 0 —
// tunable once the real figure is known.
const MarketCommissionPct = 0

// --- HeadQuarters (BRE-verified: BRE.OVR 0xD010, 0x12C8C, 0x40241/0x4043D) ---
//
// A tank's strength in hundredths of a trooper, rising with HQ completion:
// 300 at HQ 0%, 400 at 50%, 500 at 100%. BRE computes it as the real
// (1.5 + HQ/100) against a trooper's 0.5 and a jet's/turret's 1.0 — the same
// 1 : 2 : 3..5 ratio in whole troopers. breins.txt calls a tank "about the
// equivalent of four Troopers", which is the HQ-50 value, not a base: the
// manual and the binary agree once the HQ term is read.
//
// IB previously used 4 rising to 8, which both started a third too high and
// doubled the HQ's marginal value.
const (
	TankStrengthPctBase  = 300
	TankStrengthPctPerHQ = 2
)

// tankStrength values n tanks in troopers, given HQ completion percent.
func tankStrength(n, hq int) int {
	return n * (TankStrengthPctBase + TankStrengthPctPerHQ*hq) / 100
}

// HQBuildStart is the completion percent a newly bought HeadQuarters begins at,
// and HQBuildPerTurn is what it gains each turn until it reaches 100 — so it
// takes 20 turns. Both read from the original (the turn routine advances the
// field only while it sits in 1..99, then clamps to [0,100]).
const (
	HQBuildStart   = 5
	HQBuildPerTurn = 5
)

// The HeadQuarters price rises with the empire's lifetime turn count — unlike
// every other unit price, which random-walks around a fixed base. BRE.OVR
// 0x128BA computes it as:
//
//	min(HQPriceBase + HQPricePerTurn×TurnsPlayed + Random(HQPriceJitter),
//	    HQPriceCap − Random(HQPriceCapJitter))
//
// Checked against 163 distinct captured prices over seven captures and two
// empires: 161 land inside the jitter window and are spread flat across it. The
// two strays sit one turn's worth outside, from a day-boundary off-by-one in the
// score-to-turns derivation used to check them, not from the formula.
//
// So an old realm pays far more for a HeadQuarters than a young one — the
// building rewards committing early, and the cap keeps a very old realm from
// being priced out entirely.
const (
	HQPriceBase      = 5000
	HQPricePerTurn   = 75
	HQPriceJitter    = 300
	HQPriceCap       = 100_000
	HQPriceCapJitter = 1000
)

// --- Misc gold costs (reconstructed / tunable) ---
const (

	// Food market (issue #19). BRE food prices vary daily within buy∈[20,60] /
	// sell∈[7,20] with sell=buy/3. IB's economy is BRE-native scale (units at the
	// BRE live snapshot, food quantities calibrated to live BRE), so the price is
	// BRE's own band: buy varies within [FoodBuyPriceMin, 3×FoodBuyPriceMin], sell
	// = buy/3. See World.FoodBuyPrice/FoodSellPrice. (Was 3000, a leftover ~150×
	// factor from before the economy was re-anchored to BRE magnitude — food then
	// cost 150× a trooper, wildly out of line with the rest of the economy.)
	FoodBuyPriceMin = 20 // gold per unit at the cheapest; buys range up to 3× this
	// FoodMarketDailySupply is the planet-wide pool of food available to buy each
	// day (BRE seeds ~1,000,000); buying depletes it, selling replenishes it,
	// unless the sysop's Food Unlimited toggle is on.
	FoodMarketDailySupply = 1_000_000
	// FoodSpoilPct: each turn this % of the ENTIRE stored food stock spoils —
	// floor(0.05 × food) — with NO floor below which nothing spoils. BRE-verified
	// by driving the original (2026-07-16): spoilage matched floor(5% × food) to
	// the unit across food stocks from 1.4k to 13.9k. Technology decreases it (via
	// TechFactor). Was an unverified "4% of food above 1000" guess.
	FoodSpoilPct = 5
	// Food-shortfall penalties (IB reconstruction — BRE publishes no rate;
	// calibrated to a live BRE point: ~73% short dropped support ~50 points in one
	// turn). Scaled by how much of the turn's food need went unmet (0–100%): the
	// hungrier the realm, the more popular support falls and the more people leave.
	FoodShortfallSupportDrop   = 70 // support points lost per turn at 100% unfed
	FoodShortfallMoraleDrop    = 80 // morale points lost per turn at 100% unfed (hungry troops demoralize faster than the public, as under pay shortfall)
	FoodShortfallEmigrationPct = 10 // % of population that leaves per turn at 100% unfed
	// Agricultural food follows the same Base + [0, Rate) draw the gold regions
	// use — one roll per turn, multiplied by the region count. BRE-verified: over
	// six captures and nine different region counts (2 … 194) every printed total
	// divided exactly by its count, and the per-region figure landed on exactly
	// 300, 301, 302, 303 or 304 — all five values, so the width is 5 and the floor
	// is 300. IB previously paid a flat 300, i.e. always the floor of the band.
	FoodAgriBase = 300
	FoodAgriRate = 5
	// Rivers produce BOTH gold and food every turn — a DELIBERATE DIVERGENCE.
	// In BRE a river does hydropower OR fishing each turn, never both, and it
	// fished on 21 of 73 captured turns (~29%). IB pays the same expected value
	// with none of the swing: RiverFishShare of a river's output is taken as
	// food, the rest as hydropower gold. A player committed to rivers otherwise
	// watches millions of gold appear and vanish turn to turn with no way to plan
	// around it, and an un-plannable food source is close to useless.
	//
	// One constant drives both halves so they cannot drift apart when tuned.
	RiverFishShare = 30  // % of a river's output taken as food; the rest is gold
	RiverFishFood  = 124 // food per River region at a full (100%) fishing turn
	// PeopleFoodPerThousand is the food the population eats per 1000 people per
	// turn. BRE's maintenance shows "Your People Need ~150 units of food"; at IB's
	// ~2000-person start that lands on ~150 (2000×75/1000), so a fresh realm's 1000
	// food comfortably covers it. Was an unscaled 1:1 with People, which starved a
	// BRE-faithful 1000-food start. Tunable.
	PeopleFoodPerThousand = 75
	// ArmyFoodDivisor: the army eats ~1 food per this many army-units. Live-verified
	// against BRE (2026-07 IBBS capture): 42,259 troopers → "Armed Forces Require 211
	// units of food" (42,259/200 = 211). Jets and tanks weigh 2× a trooper (crews +
	// fuel). Was 1 food/trooper — ~200× too heavy, which made a standing army
	// food-crippling instead of nearly free as in BRE. Tunable.
	ArmyFoodDivisor = 200

	// --- Popular support: tax drift, riots, and the pay-to-boost prompt ---
	// BINARY-VERIFIED against BRE.OVR's end-of-turn routine (0xCE97) and the
	// maintenance routine (0x2EEBB), and reproduced exactly by a live capture
	// (tax 12, riot: 100 − 12/3 − (12−30)/10 = 97).
	//
	// Every turn:  Support = clamp(Support − riotPenalty − (Tax−30)/10, 0, 100)
	// so a rate under SupportTaxNeutral recovers support for free and a high one
	// bleeds it. Integer division truncates toward zero in both Go and the
	// original's Pascal, so the two agree on negative values.
	SupportTaxNeutral  = 30 // tax rate at which the per-turn drift is zero
	SupportTaxDivisor  = 10 // tax points per point of drift either side of neutral
	RiotTaxFloor       = 10 // riots need Tax strictly above this
	RiotSupportDivisor = 3  // a riot costs Tax/this many support points
	RiotPeopleDivisor  = 15 // a riot costs People/this many people
	RiotChanceDenom    = 10000
	// Below LowTaxSupportCeil, a rate under LowTaxBonusBelow additionally buys
	// back (LowTaxBonusBelow − Tax) points — BRE's reward for taxing very lightly
	// while unpopular. It does nothing to an already-content realm.
	//
	// LowTaxBonusBelow is 10 in the original, the same value as RiotTaxFloor, but
	// it is a separate mechanic and must stay a separate constant: sharing one
	// would make a change to the riot threshold silently move a support gift.
	LowTaxBonusBelow  = 10
	LowTaxSupportCeil = 85
	// Very low support demoralizes the army: below MoraleDrainSupport, morale
	// falls by the shortfall each turn.
	MoraleDrainSupport = 10

	// The pay-to-boost-support prompt. Cost is a flat amount per missing point
	// plus a share of the population, so a big realm pays more to please the same
	// fraction of it. BINARY-VERIFIED (BRE.OVR 0x2F4C4 / 0x2F740), reproduced
	// exactly by two live prompts (216,366 gold at 23,874M people and 218,139 at
	// 24,071M, both three points short):
	//
	//	deficit = min(100 − Support, MaxSupportBoostPerTurn)
	//	cost    = deficit × (SupportBoostPerPerson × People + SupportBoostFlat)
	//	points  = deficit × (given + 1) / (cost + 1)
	//
	// The cap is on the deficit the crown CHARGES for, not on the award — paying
	// the SupportBoostMaxPct maximum buys proportionally more. That is the
	// original's behaviour; a test pins it so it does not get "fixed".
	MaxSupportBoostPerTurn = 15
	SupportBoostPerPerson  = 3
	SupportBoostFlat       = 500
	SupportBoostMaxPct     = 150 // most a baron may pay, as a % of the request

	// --- Industry (live-verified; one capacity pool per region, split between
	// units and gold — see World.industrialGold / ProjectedProduction) ---
	// Industrial GOLD uses the same Base + [0, Rate) shape as every other region
	// (binary-verified, BRE.OVR 0x34545–0x345D1), giving 2,500–2,554 per region.
	// It is paid only on the share NOT allocated to units.
	IndustryGoldRate, IndustryGoldBase = 55, 2500
	// UnitPointsPerRegion is the separate, smaller pool BRE spends on UNITS.
	// Verified in BRE.OVR (0x34F49-0x3517A): each unit type's multiplier is
	// exactly 2100/cost (21, 15, 14, 4.2, 1.4, 1.2). Gold and units do NOT share
	// one pool size — gold uses IndustryGoldRate/IndustryGoldBase above.
	UnitPointsPerRegion = 2100
	// Mountain regions boost unit manufacturing: the pool is multiplied by
	// 1 + MountainIndustryNum*Mountain/TotalRegions, capped at
	// MountainIndustryCapPct. Read from BRE.OVR and validated against 11 of 11
	// live turret figures across three captures.
	MountainIndustryNum    = 3
	MountainIndustryCapPct = 150
	// DefaultProdPct is each unit type's default production percentage — BRE's
	// default is all six at 15% (90% to units, 10% remainder → industrial gold).
	DefaultProdPct = 15
	// Point cost to manufacture one unit. BINARY-VERIFIED: BRE stores the
	// reciprocal (units per point) as 21 / 15 / 14 / 4.2 / 1.4 / 1.2, which is
	// exactly UnitPointsPerRegion/cost for these six values.
	CostTrooper = 100
	CostJet     = 140
	CostTurret  = 150
	CostTank    = 500
	CostBomber  = 1500
	CostCarrier = 1750
	// Specialization efficiency. BINARY-VERIFIED: the multipliers are exactly
	// 1.25 for the specialized unit and 0.85 for every other.
	SpecialtyBonusPct   = 25
	SpecialtyPenaltyPct = 15

	NukeCost   = 7_500_000
	ChemCost   = 6_000_000
	BioCost    = 6_000_000
	DoomerCost = 75_000_000
	SDIStep    = 1_500_000 // gold per +1% SDI
	// TerrorUnitLossDenom: each successful terror hit removes 1/N of one random
	// unit type. BRE's disassembled hit applier uses a 6/7 ratio (removes ~1/7),
	// so N = 7.
	TerrorUnitLossDenom = 7
	// ScorePerTurn is the flat Score a played turn earns. BRE's observed award
	// for a standard start was 213 (= round of the standard-start net worth
	// 212.5), constant within a day; day-over-day growth was never shown to
	// change it, so IB awards a flat constant to every empire per turn rather
	// than tracking net worth. Combat and covert score (combat.go) are on top.
	ScorePerTurn = 213
	// Riots and food spoilage do NOT affect Score — Score is the cumulative earned
	// metric, and BRE leaves it untouched by economy events (Andy's call, reversing
	// IB's earlier per-event dings).
	// Combat score (IB's own): a battle's Score award scales with the forces used
	// (units both sides lose). The winner gains, the loser loses a bit less, and a
	// successful DEFENSE is worth more than a successful attack.
	CombatScoreDivisor    = 2   // Score award = (units lost by both sides) / this
	CombatLoserPenaltyPct = 80  // the loser loses this % of the winner's gain
	DefenseWinBonusPct    = 150 // a defender's win awards this % of an attacker's win
	// AllyDefenseContribPct is the share of a Full Defense Alliance partner's
	// MOBILE forces (troopers + tanks — not turrets/jets/bombers/carriers) that it
	// sends to reinforce an ally under attack. BRE-verified live (2026-07): an ally
	// sent exactly 30% of its troopers, tanks, and agents to the defender's aid.
	// See docs/mechanics-reference.md "Diplomacy" and the bre-binary-verified-math
	// memory. The sent detachment adds to the defender's battle power and takes the
	// same casualty rate as the defender.
	AllyDefenseContribPct = 30
	// PirateScoreDivisor keeps raids on pirate factions worth only a little Score
	// (win or lose) — far less than a battle against another empire.
	PirateScoreDivisor = 50
	// GroupAttackBomberOffense values a committed bomber's offense in a group
	// attack (tunable; BRE's exact figure not recovered — set to a tank's).
	GroupAttackBomberOffense = 4
	// GroupAttackLossPct is the share of a committed force lost in the strike;
	// the rest returns. 15% matches attack.hlp's normal-attack losses.
	GroupAttackLossPct = 15
)

// --- Upkeep / maintenance (BRE-verified — live capture, Maintenance Costs
// "Medium") ---
//
// Per-unit maintenance per turn, in TENTHS of a gold (Technology reduces the
// total via TechFactor — see Empire.ForcesUpkeep). These are literal gold, not a
// ratio to scale: a turret-only empire was charged exactly trunc(0.9 × turrets)
// at five army sizes from 49,691 turrets (44,721 gold) to 219,032 (197,128).
//
// TWO RATES ARE LIVE-VERIFIED and the rest are not. The turret figure above
// matches the guide table, but a live 100-trooper realm was charged 40 gold —
// 0.40 per trooper, where the guide's Medium column says 0.60. Both readings
// come from games whose config reads "Maintenance Costs: Medium" and whose
// region upkeep is the verified 913, so the settings are the same and the guide
// is simply wrong on that row. Treat the three unmeasured rates below as
// SUSPECT for the same reason, not as fidelity contract: measuring them needs a
// realm holding one unit type at a time and reading "Your Armed Forces Require".
const (
	MaintTrooperTenths = 4  // live-verified: 100 troopers → 40 gold
	MaintJetTenths     = 12 // guide table, UNVERIFIED
	MaintTurretTenths  = 9  // live-verified across five army sizes
	MaintBomberTenths  = 13 // guide table, UNVERIFIED
	MaintTankTenths    = 6  // guide table, UNVERIFIED
	MaintCarrierTenths = 1  // guide table, UNVERIFIED

	MaintTenthsPerGold = 10

	// RegionUpkeepPerLand is gold per region of land, per turn — flat, with no
	// dependence on region type or empire size. Live-verified across four empire
	// sizes in two separate games, exact every time: 15 regions → 13,695 gold,
	// 5,917 → 5,402,221, 6,397 → 5,840,461, 6,837 → 6,242,181. Region upkeep is
	// the dominant drain in BRE (that last empire owed 6.2M on land against
	// 197k on a 219,032-turret army); IB's old figure of 2 made expansion
	// effectively free.
	RegionUpkeepPerLand = 913
)

// --- Net-worth weights (BRE-verified — guide net-worth table) ---
//
// Contribution to net worth per unit / per region, in thousandths of a gold
// (World.NetWorth divides by 1000 for exactness).
const (
	NetWorthLand    = 12500
	NetWorthTrooper = 250
	NetWorthJet     = 325
	NetWorthTurret  = 425
	NetWorthBomber  = 3000
	NetWorthAgent   = 500
	NetWorthTank    = 1250
	NetWorthCarrier = 1000
)

// Cash Relief loans (#40) — term-based borrowing, gathered live from BRE
// (2026-07-17): the daily rate rises with the term and compounds daily.
// The daily rate and its per-term math are VERIFIED (matched three live loans);
// the borrowing ceiling and default penalty are IB reconstructions — BRE's
// ceiling formula stays unverified (the gathered points were confounded by
// growing debt), so treat LoanCeilingMultiple as a tunable placeholder.
const (
	LoanMinDays            = 1 // BRE prompt "(1; 10)"
	LoanMaxDays            = 10
	LoanBaseRateTenths     = 80 // daily rate at term 0, in tenths of a % (8.0%/day)
	LoanRatePerDayTenths   = 2  // +0.2%/day of daily rate per term-day (2d→8.4, 5d→9.0, 10d→10.0 — verified)
	LoanCeilingMultiple    = 8  // max borrowable ≈ this × NetWorth less outstanding (UNVERIFIED: one confounded BRE point ≈8.5×NW; term-dependence unmodeled)
	LoanDefaultPenaltyPct  = 25 // an unpaid loan at its due date rolls into Debt grown by this % (IB's late-payment penalty)
	LoanDefaultSupportDrop = 10 // popular-support points lost when a loan defaults
)

// --- Other economy tunables ---
const (
	DebtGrowthPct = 10 // % a loan's outstanding debt grows each turn
	// --- Technology (BRE-verified: BRE.OVR 0x33E85-0x34029 and 056d:1a07) ---
	//
	// Research is fifteen independent counters, of which only six do anything —
	// the other nine are pure dilution, so 9 of every 15 points are wasted. Per
	// turn an empire earns
	//
	//	TechResearchMul * round( (techRegions^2 / totalRegions)^0.75 )
	//
	// points, each landing in a slot chosen uniformly at random, plus a smaller
	// unmultiplied contribution per Technology Agreement partner. The level NEVER
	// decays: selling the regions stops research and freezes what you have. The
	// benefit divides by total regions, so expanding dilutes it.
	//
	// See docs/mechanics-reference.md, "Technology (binary-verified)".
	TechSlotCount   = 15 // research slots; only the six named below have effects
	TechResearchMul = 4  // own-research multiplier (ally contributions are unmultiplied)
	// TechExpClamp bounds level/(regions+1) before exp(), as the original does.
	TechExpClamp = 50

	// The six slots that do anything. Slot 0 drives three separate effects, each
	// with its own ceiling; slots 6-14 exist only to dilute research.
	TechSlotGold     = 0 // gold income, food production and unit production
	TechSlotSDI      = 1
	TechSlotTax      = 2 // population tax income
	TechSlotMaint    = 3
	TechSlotDecay    = 4 // food spoilage
	TechSlotMilitary = 5

	// Effect ceilings, in hundredths (150 = x1.50). A factor approaches its
	// ceiling but never reaches it. Food decay has by far the most headroom,
	// which is why it moves first and furthest — at low levels Technology is
	// effectively a spoilage technology.
	TechCapGold     = 150
	TechCapTax      = 150
	TechCapFood     = 200
	TechCapUnits    = 135
	TechCapMaint    = 140
	TechCapDecay    = 500
	TechCapSDI      = 200
	TechCapMilitary = 140

	// TechFactorUnit is the fixed-point scale the factor helpers return:
	// 10000 == x1.0. Kept off floating point so money stays integral.
	TechFactorUnit = 10000

	// Technology Agreement treaty (#11): BRE's manual says the pact "allows an
	// empire to gain some of the technological advances of its partner" — a
	// tech-sharing effect. The magnitude isn't in the manual (and isn't
	// disassembly-recovered), so these are IB's own reconstructed tunables. A
	// Technology Agreement raises your TechLevel ceiling to TechAgreementCapPct%
	// of your highest-tech partner's level, and you catch up 1/TechAgreementGainDiv
	// of the remaining gap each turn — so even a low-Technology realm slowly gains
	// from a strong partner. See advanceTech.
	TechAgreementCapPct  = 60 // reach this % of the best partner's TechLevel
	TechAgreementGainDiv = 20 // per-turn catch-up = (partner ceiling − yours) / this
)

// Money ceilings (BRE-scale). Kept under int32 max so 32-bit door builds stay
// correct — do not raise past ~2.1e9 without widening the money fields to int64.
const (
	InterestCap = 1_599_999_999 // bank balance above this earns no more interest
	MoneyCap    = 2_000_000_000 // hard cap on gold on hand / in bank
)

// Trade-deal sending (BRE-verified live, 2026-07-21): sending a trade deal
// consumes one carrier to transport the goods and costs TradeDealGoldPerDay per
// day for a chosen span of TradeDealMinDays..TradeDealMaxDays days; the offered
// goods are escrowed and arrive on the recipient's next turn. (BRE adds a small
// deal-size component on top of the 100,000/day base that IB does not model.)
const (
	TradeDealCarriers   = 1       // carriers consumed to send one deal
	TradeDealGoldPerDay = 100_000 // gold cost per day of transit
	TradeDealMinDays    = 2       // shortest a deal may be sent for
	TradeDealMaxDays    = 5       // longest a deal may be sent for
)
