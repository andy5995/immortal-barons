package game

// The balance_*.go family — the game's tunable economy balance data, kept apart
// from the formulas that read it.
//
// This is a "config that isn't changed at runtime": compiled-in, type-safe
// constants that the economy formulas in turn.go, economy.go, specials.go and
// game.go reference by name. Keeping every balance number out of the formula
// code is what makes the economy legible and tunable.
//
// The set outgrew one file. It is split by subject — balance_regions.go,
// _crown, _start, _ai, _prices, _hq, _costs, _combat, _networth — and this file
// holds the header below plus what did not belong to any of them. A new number
// goes in the file for its mechanic; the rule is the separation from the
// formulas, not any one filename.
//
// Provenance is marked per group:
//   - "BRE-verified" values were read directly as immediate operands from a
//     disassembly of the original binary (BRE.OVR turn-economy routine, file
//     offsets 0x342C0–0x34A4E; HIGH confidence, cross-checked against bre.doc).
//     Do not "tune" these away from the recovered numbers without a new
//     disassembly — they are the fidelity contract.
//   - "reconstructed / tunable" values are IB's own, anchored to the BRE scale.
//     These are the playtest knobs.

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

	// Technology Agreement treaty (#11), BINARY-VERIFIED end to end (BRE.OVR
	// process_economic_production, unit ovr_033b64; the partner loop is +0x3c2 to
	// +0x4cb). Each partner adds
	//
	//	round( (min(myTechRegions, partnerTechRegions)^2 / myTotalRegions)^0.75 )
	//
	// — the same expression as your own research, over the SAME denominator, and
	// without the TechResearchMul. It carries no constants of its own, which is
	// why none appear here: the bound is the smaller of the two Technology REGION
	// counts (record field +0xb2 on both), so a pact accelerates a realm that is
	// already researching and does nothing for one holding no Technology.
	//
	// The loop's two guards on a partner are both existence tests, not diplomacy
	// conditions: it must hold Technology, and its record field +0x5d must be
	// positive. +0x5d is the slot's in-use marker — every unoccupied slot in a
	// live save reads -1 while the occupied one reads a positive serial, and the
	// binary reads or tests it at 83 sites and writes it at none. IB's alliesOf
	// filter on Alive is that same gate.
)

// Money ceilings. The money fields are int64 (Empire.Gold and friends), so what
// a realm may hold is a game rule rather than a machine-word limit: a 32-bit
// door build holds the same range as a 64-bit one. The ceiling used to be a
// hard-coded 2 billion because plain int is 32 bits on a 32-bit build, and every
// gold credit past it was silently discarded.
//
// The 2 billion is CONFIRMED BY PLAY of the original — it holds gold in hand,
// savings, and what may be invested in a day, all three at that figure. It is
// not a literal in either binary in 32-bit or Real48 form, which fits: a cap
// tested against a Turbo Pascal constant needs no constant of its own.
const (
	// What a realm may HOLD, on hand or in the bank, is the 2 billion above,
	// BRE's own figure, read through World.MoneyCap. It is no longer a setting
	// (#205) and no longer varies: World.MoneyCapBillions returns this whatever
	// Config.MoneyCapBillions says. A saved raised value was honoured until
	// 2026-09-01, on the reasoning that clamping one down would take gold a
	// league had been playing with; andy5995 knows of no board running one, so
	// the variation cost more than it bought.
	//
	// GoldPerBillion names the unit the cap is set in, so the multiplier is not
	// respelled at each site that converts between the two.
	GoldPerBillion = 1_000_000_000

	MoneyCapMinBillions = 2

	// MoneyCapMax is an ARITHMETIC ceiling, not a holding one: pure helpers that
	// project a future figure (ExpectedReturn, LoanTotalOwed) clamp to it so the
	// projection cannot overflow. It sits far above what a realm may hold,
	// because a projection legitimately runs past the hold cap before the gold
	// lands and is clamped to it.
	moneyCapMaxBillions       = 999
	MoneyCapMax         int64 = moneyCapMaxBillions * GoldPerBillion

	// MaxInvestment is the most gold ONE investment may lock away. Deposits and
	// withdrawals are deliberately unbounded (up to the money cap) — nothing gates the
	// bank per turn, so a per-action limit there only cost keystrokes. Locking
	// gold away is the case worth bounding, and a baron may still open as many
	// investments as they like.
	MaxInvestment = 2_000_000_000

	// MaxCountField is the largest value a figure held in COUNT width (a plain
	// int, 32 bits on a 32-bit door) may take: a per-unit market price, a
	// trade-basket amount. Money is int64 and bounded by MoneyCap instead.
	MaxCountField = 2_000_000_000
)

// Trade-deal sending (BRE-verified live, 2026-07-21): sending a trade deal
// consumes one carrier to transport the goods and costs TradeDealGoldPerDay per
// day for a chosen span of TradeDealMinDays..TradeDealMaxDays days; the offered
// goods are escrowed and arrive on the recipient's next turn. (BRE adds a
// cargo-weighted component on top of the 100,000/day base that IB does not
// model — BRE.OVR 0x0513e7 sums the nine goods against fixed weights, divides by
// 5, and adds the base at 0x05154e.)
const (
	TradeDealGoldPerDay = 100_000 // binary: the flat part of the per-day transit cost
	// TradeDealGoldBase and TradeDealCostDivisor are the two halves of the
	// original's cost formula: the nine goods are summed against the per-good
	// ShipWeight figures in units.go, divided by five, and this base is added
	// (BRE.OVR 0x0513e7, constants decoded at unit offsets 0x0746 and 0x0753).
	// BINARY-VERIFIED. The base is the same 100,000 the per-day rate uses.
	TradeDealGoldBase    = 100_000
	TradeDealCostDivisor = 5
	// carrierScale is the fixed-point unit the carrier requirement is summed in
	// — hundred-thousandths of a carrier. Every per-carrier capacity below
	// divides it exactly, which is what keeps the arithmetic in integers.
	carrierScale = 100_000
	// GoldPerCarrier is gold's own carrier capacity, kept here rather than on
	// the Good row because gold is not one: it is the basket good held in money
	// width and handled beside the loop. BINARY-VERIFIED (BRE.OVR
	// ovr_050dfb_entry_0436, unit offset 0x0489).
	GoldPerCarrier = 100_000
	// GoldShipWeight is gold's weight in the cost formula, in shipWeightScale
	// units — the original's 0.01 (unit offset 0x0682). Same reasoning as
	// GoldPerCarrier for why it is not on the row.
	GoldShipWeight = 1
	// shipWeightScale is what a Good's ShipWeight is expressed in hundredths of.
	// The original's weights include 0.05 and 0.01, which have no exact binary
	// form, so IB holds them as exact integers and can differ from the original
	// by a gold or two on an enormous basket.
	shipWeightScale = 100
	// The sysop's Trade Deal Costs ladder, applied by Level.TradeCostScaled.
	// BINARY-VERIFIED (BRE.OVR 0x5158F): Low divides by six and High multiplies
	// by three, which is its own spread — not the generic preset ladder and not
	// the attack pair's divide-by-five.
	TradeCostLowDivisor   = 6
	TradeCostHighMultiple = 3
	// The Maintenance Costs ladder, applied by Level.MaintCostScaled.
	// BINARY-VERIFIED (BRE.OVR 0x2E836 and 0x2E948): a quarter at Low, four
	// times at High — a sixteenfold spread, where IB applied half and double.
	MaintCostLowDivisor   = 4
	MaintCostHighMultiple = 4
	// The span a deal is sent for is also its LIFETIME: BRE stores now + days as
	// the deal's expiry and drops it unanswered past that (create_trade_offer
	// 0x2256 adds the day count to the clock global; process_trade_offer 0x24E5
	// compares the stored stamp against it). BINARY-VERIFIED: the prompt refuses
	// anything under two days (0x21B2) and offers ten as its default (0x1F7D).
	// There is no upper bound in the original — what the sender can pay for is
	// the only limit — so IB has none either, where it used to cap the span at
	// five days.
	TradeDealMinDays     = 2  // shortest a deal may be sent for
	TradeDealDefaultDays = 10 // what the prompt offers

	// ProtectiveTradeCostDivisor is what a Protective Trade agreement takes off
	// the transit cost — the manual's "making trade deals cheaper to send and
	// maintain". BINARY-VERIFIED (BRE.OVR 0x0268bc, create_trade_offer): the
	// recipient's relation is compared against 2 (Protective Trade) and, when it
	// matches, the PER-DAY cost is divided by three before the span is chosen and
	// before days x cost is deducted. There is no separate upkeep charge in the
	// binary — the one up-front payment is the whole cost, so "maintain" is
	// covered by the same discount.
	ProtectiveTradeCostDivisor = 3
)

// The Gooie Kablooie. It is not a purchase
// but a public works: one planet funds one weapon, in millions of gold, over as
// many days as it takes to raise the money, and the target planet can see it
// coming and shoot at it.
//
// The funding cost is BINARY-VERIFIED from the construction routine in BRE.OVR's
// overlay unit at 0x27441 (0x277A0-0x27950):
//
//	cost := round(targetPlanetLand * AnnihilatorCostPerLand) + AnnihilatorCostBase
//	cost = min(cost, AnnihilatorCostCap)
//	switch ratio := targetPlanetLand / ourPlanetLand; {
//	case ratio > 4: cost *= 2
//	case ratio > 2: cost *= 1.5   // AnnihilatorSurchargeBigPct
//	case ratio > 1: cost *= 1.2   // AnnihilatorSurchargeAheadPct
//	}
//
// where targetPlanetLand is the sum of every living realm's regions on the target
// planet, floored at AnnihilatorMinTargetLand. So the weapon is priced against how
// much bigger than you the planet you are aiming at is: attacking upward is
// affordable, and a giant planet flattening a small one pays the most.
//
// The siege and interception figures below are BINARY-VERIFIED too, from the
// daily resolver at BRE.OVR 0x47c52 and the jet-attack routine at 0x2827b.
const (
	AnnihilatorCostPerLand       = 44743   // per region of the target planet, in millionths of a million
	AnnihilatorCostPerLandDenom  = 1000000 // ... so the rate is 0.0044743
	AnnihilatorCostBase          = 100     // million gold, added after the per-land part
	AnnihilatorCostCap           = 5000    // million gold, before the size surcharge
	AnnihilatorMinTargetLand     = 1000    // a tiny planet still costs as if it were this big
	AnnihilatorSurchargeAheadPct = 120     // target is larger than us
	AnnihilatorSurchargeBigPct   = 150     // more than twice our size
	AnnihilatorSurchargeHugePct  = 200     // more than four times our size
	AnnihilatorMillion           = 1_000_000

	// Construction runs for AnnihilatorBuildDays after the last gold is in, and
	// then the weapon launches itself: the funding routine sets its launch date
	// to now + 3.0 days (BRE.OVR 0x27e47-0x27ac9, a Real48 3.0) and announces the
	// remaining hours. Nothing asks a baron whether to launch (#114).
	AnnihilatorBuildDays = 3
	// The weapon is visible to its target for the whole flight, which is what
	// makes the warning worth having. BRE's flight is the league's real packet
	// transit; IB fixes it, which is a divergence of convenience.
	AnnihilatorFlightDays = 2

	// The weapon is a siege, not an explosion (#112). It sits on the planet for
	// AnnihilatorSiegeDays, taking AnnihilatorFirstDayPct of every realm's
	// regions the day it lands and AnnihilatorLaterDayPct every day after, then
	// self-destructs. The resolver divides each realm's region total by 10 and
	// then by 20 (BRE.OVR 0x4783a, 0x47b0c); it reads neither the weapon's
	// remaining strength nor the defender's SDI, so a battered weapon bites just
	// as deep as a fresh one (#111). A realm under new-realm protection is
	// skipped outright (is_under_protection, 056d:19b5).
	AnnihilatorSiegeDays   = 5
	AnnihilatorFirstDayPct = 10
	AnnihilatorLaterDayPct = 5

	// Interception. Only jets can reach it — the original is explicit that
	// nothing else can — and the number needed scales with the whole planet's
	// land, which is what makes killing one a planet-wide effort:
	//
	//	required = min(2e9, land x (land/750 + 15))
	//	knocked  = min(75, jets x 100 / required)
	//
	// (BRE.OVR 0x28897-0x28961). The 75% ceiling means no single sortie can
	// finish the weapon, however many jets it carries. The jets sent are spent
	// either way, at 33% give or take five points: the routine adds one
	// Random(5) and subtracts another (0x2879d-0x287d3).
	AnnihilatorJetsLandDivisor = 750
	AnnihilatorJetsLandBase    = 15
	AnnihilatorJetsRequiredCap = 2_000_000_000
	AnnihilatorMaxSortiePct    = 75
	AnnihilatorJetLossPct      = 33
	AnnihilatorJetLossSpread   = 5
)

// Travel Times — how the average packet round trip to another board is kept.
// BINARY-VERIFIED from BRE.OVR's TIME_CHECK handler (0x44745-0x44772): each
// completed round trip folds in as avg = (avg + 2*elapsed) / 3, and the display
// routine (0x23D70) reads in hours below two days and in days at or above it.
const (
	TravelAvgNewWeight = 2 // weight on the newest sample
	TravelAvgDenom     = 3 // ... over (old + weight*new) / denom
	TravelHoursCutoff  = 2 // days; under this the screen reads in hours
	// Below an hour BRE's hours figure rounds to 0.00, which reads as "never
	// measured" rather than "fast". A modern always-on link is routinely under a
	// minute, so IB adds two tiers BRE has no need of. A readability divergence,
	// not a mechanic — the stored figure is unchanged.
	TravelMinutesCutoff = 1.0 / 24        // days; under this the screen reads in minutes
	TravelSecondsCutoff = 1.0 / (24 * 60) // days; under this the screen reads in seconds
)

// Online indicator — how long after a baron's last menu action the roster
// screens still mark them online. IB's own; BRE has no such display.
//
// Short on purpose. A clean logoff (and a caught disconnect) zeroes the stamp
// outright, so this window only covers a session that died without unwinding —
// a hard crash or a reset board. Of the two ways the indicator can be wrong,
// showing an absent baron as online is the misleading one: it invites a player
// to wait for a reply that is not coming, or to hold off a strike. Showing a
// present baron as absent costs nothing.
const OnlineWindowSecs = 300

// MoraleDesertBands are the per-turn desertion rate draws, worst band first:
// the rate is Base + Random(Up) − Random(Down), and the index is
// morale/MoraleDesertBandWidth. BINARY-VERIFIED (BRE.OVR 0xC20C, 0xC245, 0xC27D
// and 0xC2B5, one band each). The two draws are wide enough that the milder
// bands often come out zero or negative, which costs nothing — a realm at 35
// morale usually loses nobody, one at 5 always loses somebody.
//
// A var rather than a const only because Go has no constant arrays; treat it as
// data, and do not tune it without new evidence.
var MoraleDesertBands = [MoraleDesertBandTop / MoraleDesertBandWidth]struct{ Base, Up, Down int }{
	{22, 7, 17}, // morale 0-9
	{17, 5, 12}, // morale 10-19
	{10, 3, 8},  // morale 20-29
	{5, 2, 5},   // morale 30-39
}

// Interplanetary Special Operations prices (#49).
//
// The four bombing ops carry the figures the original prints in that menu's own
// price column, read off a live capture (docs/dev/bre-screens.md). They are
// fidelity constants, not playtest knobs.
//
// The three missiles print NO price there. The local versions price off the
// TARGET's size, which a board cannot know about a realm on another planet, so
// IB prices them off the launcher's own land the way terror ops already are.
// That per-region rate IS a playtest knob — it is IB's, with nothing in the
// original to match it against.
const (
	IPBombFoodCost   int64 = 10_000_000
	IPBombMarketCost int64 = 25_000_000
	IPBombRoutesCost int64 = 25_000_000
	IPUndermineCost  int64 = 75_000_000

	IPMissileGoldPerRegion int64 = 20_000
)

// UndermineInvestmentDivisor is the share of each investment's principal an
// Undermine Investments op destroys: a quarter.
const UndermineInvestmentDivisor = 4

// TerrorPctLoss is the band one landed terror agent takes off the field its
// operation aims at: Base + Random(Spread) percent, truncated.
type TerrorPctLoss struct {
	Base   int
	Spread int
}

// TerrorOpLosses is what each percentage-based interplanetary terrorist
// operation destroys. BINARY-VERIFIED from the received-op resolver
// (`resolve_received_covert_operation`, BRE.OVR 0x04a96b), which dispatches on
// the operation byte the packet carries and, for these five, computes
// `Random(n)/100 + base` as a Real48 and multiplies the target's field by it:
//
//	Bomb Intelligence  agents    (+0x26F)  0x56A  Random(3)/100 + 0.02
//	Cause Dissensions  troopers  (+0x76)   0x63C  Random(3)/100 + 0.02
//	Bomb Air Bases     jets      (+0x7E)   0x6BB  Random(5)/100 + 0.03
//	Stir Emigrations   population(+0x62)   0x73A  Random(7)/100 + 0.04
//	Bomb Food Storages food      (+0x6E)   0x80C  Random(30)/100
//
// Turbo Pascal's Random(n) returns 0..n-1, so Bomb Intelligence takes 2, 3 or 4
// percent and Bomb Food Storages takes anything from nothing to 29 percent.
var TerrorOpLosses = map[TerrorOpType]TerrorPctLoss{
	TerrorOpBombIntel:    {Base: 2, Spread: 3},
	TerrorOpDissensions:  {Base: 2, Spread: 3},
	TerrorOpBombAirBases: {Base: 3, Spread: 5},
	TerrorOpEmigrations:  {Base: 4, Spread: 7},
	TerrorOpBombFood:     {Base: 0, Spread: 30},
}
