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
)

// Tax coefficient (reconstructed / tunable). BRE stores population/tax income
// as an inline "6 − f(tax)" × Population shape that was only partially
// recovered; this flat per-capita gold figure is anchored to the new income
// scale (was an inline ×8 before the rebalance). Top playtest knob.
const TaxGoldPerCapita = 1200

// --- Unit / land / food prices (reconstructed / tunable) ---
//
// BRE does not store these as constants (they're computed inline), so they are
// IB's reconstructed values, re-anchored to the BRE income magnitude while
// preserving IB's ratios (which match BRE: ~7 troopers per tank, ~6 jets per
// tank). Used as the DefaultConfig / NewWorldSeed price defaults.
const (
	// Unit buy prices — BRE early-game live snapshot (2026-07-11). BRE recomputes
	// these inline so they fluctuate turn to turn; IB uses the snapshot as a
	// fixed base for now (the per-turn ±% fluctuation is the still-open part of
	// #30). Sell price is buy/3 (see sellUnit), except agents (SellAgentPrice).
	PriceTrooper = 263
	PriceJet     = 345
	PriceTurret  = 380
	PriceTank    = 2172
	PriceCarrier = 5943
	PriceAgent   = 608
	PriceBomber  = 2572
	PriceFood    = 300 // food is bought via the food market (Chopper's), not this
	// Region price rises with land owned: BRE ≈ 917 + Land×33 (live-sampled). See
	// World.LandPrice. PriceLand is the base; LandPerRegion the per-owned climb.
	PriceLand     = 917
	LandPerRegion = 33
	// SellAgentPrice: agents sell at a flat 100 in BRE, not buy/3 like other units.
	SellAgentPrice = 100
)

// --- Misc gold costs (reconstructed / tunable) ---
const (
	HQCost = 5104 // gold to start HeadQuarters construction (BRE live snapshot)

	// Food market (issue #19). BRE prices vary daily within buy∈[20,60] /
	// sell∈[7,20] with sell=buy/3; IB keeps its ~150× economy scale, so buy
	// varies within [FoodBuyPriceMin, 3×FoodBuyPriceMin] and sell = buy/3. The
	// old fixed 3000/1000 was the band floor. See World.FoodBuyPrice/FoodSellPrice.
	FoodBuyPriceMin = 3000 // gold per unit at the cheapest; buys range up to 3× this
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
	// Score penalties (IB's own — BRE leaves Score untouched by economy events).
	// A riot or food spoilage shaves a small fraction of a turn's Score award.
	ScoreRiotPenaltyDiv  = 10 // a riot costs ScorePerTurn/10 Score
	ScoreSpoilPenaltyDiv = 10 // food spoilage costs ScorePerTurn/10 Score
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
	TechFactorCap = 40 // max % bonus/reduction Technology regions grant (see TechFactor)
)

// Money ceilings (BRE-scale). Kept under int32 max so 32-bit door builds stay
// correct — do not raise past ~2.1e9 without widening the money fields to int64.
const (
	InterestCap = 1_599_999_999 // bank balance above this earns no more interest
	MoneyCap    = 2_000_000_000 // hard cap on gold on hand / in bank
)
