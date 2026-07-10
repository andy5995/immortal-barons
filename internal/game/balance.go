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
	MountainRate, MountainBase     = 400, 3550  // ore — smallest Rate, most stable
	CoastalRate, CoastalBase       = 1000, 3750 // tourism — × support factor
	DesertRate, DesertBase         = 2000, 3000 // solar — widest swing
	IndustrialRate, IndustrialBase = 100, 2500  // × industry-efficiency modifier
	RiverRate, RiverBase           = 100, 5000  // hydro — highest base, occasional bad-year dud
)

// Yield band (reconstructed / tunable). The exact BRE distribution lives in an
// unmapped helper segment; [1.0, 1.5] keeps income at or above BASE. Expressed
// as integer percents so the math stays in integers.
const (
	YieldMin = 100 // income = BASE at yield 1.0
	YieldMax = 150 // BRE's ~1.5 amplitude (reconstructed)
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
	PriceLand    = 15000
	PriceFood    = 300
	PriceTrooper = 7500
	PriceJet     = 9000
	PriceTurret  = 9000
	PriceTank    = 50000
	PriceCarrier = 6000
	PriceAgent   = 15000
	PriceBomber  = 30000
)

// --- Misc gold costs (reconstructed / tunable) ---
//
// Scaled to the BRE income magnitude alongside the prices above. (LandPriceStep
// is a divisor ratio, not a gold amount, so it lives in economy.go unchanged.)
const (
	HQCost = 750_000 // gold to start HeadQuarters construction

	FoodBuyPrice  = 3000 // gold per unit bought from the food market
	FoodSellPrice = 1000 // gold per unit the market pays you

	NukeCost   = 7_500_000
	ChemCost   = 6_000_000
	BioCost    = 6_000_000
	DoomerCost = 75_000_000
	SDIStep    = 1_500_000 // gold per +1% SDI
)
