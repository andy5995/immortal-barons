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
// Per region, per turn: perRegion = yield*Rate/100 + Base, where yield is a
// per-(game-day, empire, region) percent in [YieldMin, YieldMax]. Coastal is
// additionally multiplied by a support factor (see IncomeThisTurn); Urban and
// Technology produce NO direct gold (housing / maintenance-reduction only).
const (
	MountainRate, MountainBase = 400, 3550  // ore — smallest Rate, most stable
	CoastalRate, CoastalBase   = 1000, 3750 // tourism — × support factor
	DesertRate, DesertBase     = 2000, 3000 // solar — widest swing
	RiverRate, RiverBase       = 100, 5000  // hydro — highest base, occasional bad-year dud
)

// Yield band (live-calibrated). Each turn a planet-wide year factor is drawn in
// this band and multiplies each region's Rate on top of its Base. Live BRE data
// (mountain 7 turns, coastal 3, river 2 — all with the disassembled Bases) pins
// the band to ~[0.30, 0.80], NOT the earlier reconstructed [1.0, 1.5]. Integer
// percents so the math stays in integers.
const (
	YieldMin = 30 // live-verified: mountain/coastal/river all land in [30, 80]
	YieldMax = 80
)

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
	AIFoodBufferTurns = 3 // turns of food consumption the AI keeps on hand
	AIAgriBuyMax      = 5 // Agricultural regions the AI buys per turn when food-tight

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
	// FoodSpoilFloor: stored food at or below this never spoils (BRE gates
	// spoilage above ~1000 units); above it, a fraction of the EXCESS decays,
	// reduced by Technology regions (via TechFactor). Exact rate is an IB tunable.
	FoodSpoilFloor = 1000
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

	// --- Industry (live-verified; one capacity pool per region, split between
	// units and gold — see World.industrialGold / ProjectedProduction) ---
	// IndustryPointsPerRegion is the gold-valued capacity each Industrial region
	// yields per turn (live BRE ~2,548/region; IB uses a round 2,600).
	IndustryPointsPerRegion = 2600
	// DefaultProdPct is each unit type's default production percentage — BRE's
	// default is all six at 15% (90% to units, 10% remainder → industrial gold).
	DefaultProdPct = 15
	// Point cost to manufacture one unit, from live BRE (114 ind @ 15% →
	// 455/325/303/30/91/26 → ratios 1 : 1.4 : 1.5 : 15 : 5 : 17.5).
	CostTrooper = 100
	CostJet     = 140
	CostTurret  = 150
	CostTank    = 500
	CostBomber  = 1500
	CostCarrier = 1750
	// Specialization efficiency (live BRE: specialize a unit → it +25%, others −15%).
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
