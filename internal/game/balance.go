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
// hydropower turns every figure was a clean yield*Rate/100 + Base, with no
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

	// Force mix (#36): once food-healthy, the AI spends this share of its gold
	// on military each turn, split by gold-value across a defensive-capable mix
	// instead of buying only troopers. Troopers are versatile (1 offense / 1
	// defense), turrets give cheap defense (0/2) the old trooper-only AI lacked,
	// and tanks (4/4) add punch when it can afford them. Shares sum to 100.
	AIMilitaryBudgetPct = 50 // % of gold spent on military when food-healthy
	AIForceTrooperPct   = 50 // % of the military budget spent on troopers
	AIForceTurretPct    = 30 // % on turrets (defense)
	AIForceTankPct      = 20 // % on tanks

	// Banking (#36): rather than hoard the gold left after buying food and
	// military, the AI parks the clear surplus above a working reserve in
	// investments so idle gold earns instead of sitting.
	AIGoldReserve = 50_000 // gold kept on hand for food/maintenance/expansion
	AIInvestPct   = 50     // % of the surplus above the reserve to invest
	// AIDullLandBuyPct is the share of the affordable land-buy budget a dull-skill
	// AI spends each turn (a sharp AI spends it all). Because expansion compounds,
	// this is tuned empirically so a dull baron reaches ~700 regions over a day's
	// 15 turns against a sharp baron's ~1300. See aiExpandLand / aiSkill.
	AIDullLandBuyPct = 45

	// War (#36): aggressor-profile AIs lean into offense (tanks + troopers, few
	// turrets) so they can actually wage the wars they start, where the other
	// profiles keep the defensive default mix above. They also fund a few agents
	// for pre-war covert ops. The four shares sum to 100.
	AIForceTrooperPctWar = 40
	AIForceTurretPctWar  = 5
	AIForceTankPctWar    = 45
	AIForceAgentPctWar   = 10 // aggressors buy agents to demoralize a target first

	// AIWarOffenseMargin gates aggression: an aggressor attacks the weakest valid
	// target only when its offense exceeds the target's effective defense (units
	// + land bonus) by this %, so it picks winnable fights instead of throwing
	// its army away.
	AIWarOffenseMargin = 130
)

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

// --- Misc gold costs (reconstructed / tunable) ---
const (
	HQCost = 5104 // gold to start HeadQuarters construction (BRE live snapshot)

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
	// SupportFedBoost: a well-run realm (people fed AND maintenance paid in full)
	// recovers this many points of popular support per turn, for free — a
	// placeholder for BRE's pay-to-boost-support mechanic (#39), not yet built.
	SupportFedBoost = 5
	// Food production per region per turn. Agri calibrated to live BRE (97 Agri
	// → 29,197 and 16 Agri → 4,864 food → ~300/Agri, no flat base).
	FoodPerAgri = 300
	// Rivers do EITHER hydropower (gold, as usual) OR fishing (food) each turn —
	// an either/or the empire flips per turn (BRE #29; live: ~50/50, fishing
	// yields ~124 food/river and gives NO river gold that turn). Tunable.
	RiverFishFood   = 124 // food per River region on a fishing turn
	RiverFishChance = 50  // percent chance the rivers fish (vs hydropower) each turn
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

	// --- Industry (live-verified; one capacity pool per region, split between
	// units and gold — see World.industrialGold / ProjectedProduction) ---
	// Industrial GOLD uses the same yield×Rate/100 + Base shape as every other
	// region (binary-verified, BRE.OVR 0x34545–0x345D1), giving 2,500–2,555 per
	// region. It is paid only on the share NOT allocated to units.
	IndustryGoldRate, IndustryGoldBase = 55, 2500
	// UnitPointsPerRegion is the separate, smaller pool BRE spends on UNITS.
	// Verified in BRE.OVR (0x34F49-0x3517A): each unit type's multiplier is
	// exactly 2100/cost (21, 15, 14, 4.2, 1.4, 1.2). Gold and units do NOT share
	// one pool size — gold uses IndustryPointsPerRegion above.
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

// --- Upkeep / maintenance (reconstructed / tunable) ---
//
// Per-unit gold maintenance per turn (Technology reduces the total via
// TechFactor — see Empire.ForcesUpkeep). Ratios follow BRE's guide table
// (×10 of the 0.60/1.20/0.90/1.30/0.60/0.10 figures).
const (
	MaintTrooper = 6
	MaintJet     = 12
	MaintTurret  = 9
	MaintBomber  = 13
	MaintTank    = 6
	MaintCarrier = 1

	RegionUpkeepPerLand = 2 // gold per region of land, per turn
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
	// Technology now ACCUMULATES over turns rather than applying instantly
	// (BRE-verified via live play: the bonus ramps up slowly and saturates,
	// faster when tech is a denser share of the realm). These shape IB's own
	// reconstructed curve — tune freely; see advanceTech and TechFactor.
	// Each played turn adds share²/TechGainDiv (in tenths of a %) to TechLevel,
	// capped at share×TechCeilMul tenths (i.e. the bonus tops out near the tech
	// share), and hard-capped at TechFactorCap.
	TechGainDiv   = 250 // per-turn TechLevel gain (tenths %) = share² / this
	TechCeilMul   = 10  // per-share TechLevel ceiling (tenths %) = share × this
	TechFactorCap = 60  // max % bonus/reduction Technology grants (was 40, pre-cumulative)

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
