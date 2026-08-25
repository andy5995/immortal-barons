package game

// balance_prices.go — unit, land and food prices.
// Split out of balance.go; the provenance rules in that file's header
// apply here too, and each section below carries its own.

// --- Unit / land / food prices ---
//
// The six military units are BINARY-VERIFIED. BRE gives every empire its own
// stored price per unit and walks it one step per turn inside a fixed band. The
// three tables below are three arrays of six words in BRE.EXE's data segment
// (DS:0x508 step, DS:0x514 low, DS:0x520 high), read by the walk at BRE.OVR
// 0x12633. Sell price is UnitSellPriceDivisor (see UnitSellPrice), except
// agents (SellAgentPrice).
//
// Part of the fidelity contract: retuning one of these needs new evidence. Every
// buy price across the 30-turn capture `cap/eots-covert-agents.cap` lands inside
// its band, and they cluster near the middle rather than spreading out — that is
// the walk's mean reversion (see PriceWalkDampMax).
const (
	PriceLoTrooper = 200
	PriceLoJet     = 250
	PriceLoTurret  = 300
	PriceLoBomber  = 2500
	PriceLoTank    = 1125
	PriceLoCarrier = 4750

	PriceHiTrooper = 350
	PriceHiJet     = 400
	PriceHiTurret  = 475
	PriceHiBomber  = 5000
	PriceHiTank    = 2250
	PriceHiCarrier = 6000

	PriceStepTrooper = 25
	PriceStepJet     = 25
	PriceStepTurret  = 25
	PriceStepBomber  = 125
	PriceStepTank    = 75
	PriceStepCarrier = 125
)

const (
	// Region price rises with land owned: BRE ≈ 917 + Land×33 (live-sampled). See
	// World.LandPrice. PriceLand is the base; LandPerRegion the per-owned climb.
	PriceLand     = 917
	LandPerRegion = 33
	// The Region Cost Change knob is a BIG-REALM SURCHARGE on the per-region
	// climb, not a scale on the price. BINARY-VERIFIED (BRE.OVR 0x3019C): the
	// level selects one of the values below, a flag turns it on only at
	// RegionCostSurchargeAt regions or more, and the result is ADDED to
	// LandPerRegion. Below the threshold the knob does nothing at all.
	RegionCostSurchargeAt     = 300 // binary: cmp against 0x12C
	RegionCostSurchargeNone   = 0
	RegionCostSurchargeLow    = 15
	RegionCostSurchargeMedium = 35
	RegionCostSurchargeHigh   = 55
	// UnitSellPriceDivisor: BRE buys a unit back at a third of what it charges
	// for one. Named here rather than written as a 3 at each site — the rule was
	// spelled once in the sell path and again in the Sell menu's price column, so
	// the figure the player was quoted and the figure they were paid were two
	// separate literals.
	UnitSellPriceDivisor = 3
	// SellAgentPrice: agents sell at a flat 100 in BRE, not buy/3 like other units.
	// Binary-verified — the Sell menu's agent column is the literal 100 pushed at
	// BRE.OVR 0x16AEB, and every sell capture shows it.
	SellAgentPrice = 100
)

// The covert roll, BINARY-VERIFIED from BRE.OVR 0x4BA48 (the roll) and 0x4CAB7
// (the agent pools). docs/mechanics-reference.md carries the full routine and the
// evidence, including the defect IB reproduces: both sides of the comparison are
// drawn from the ATTACKER, so the defender's agents never enter it.
const (
	// CovertAutoSuccessOdds: one roll in this many succeeds before agents are
	// weighed at all.
	CovertAutoSuccessOdds = 10
	// ExposeOpsSlipOdds: an operation against a realm that has exposed you still
	// slips through one time in this many instead of failing outright.
	ExposeOpsSlipOdds = 10
	// ExposeOpsShieldDays is how long an Expose Enemy Ops shield lasts. BRE
	// writes `now + 1.0` into a Real48 slot (BRE.OVR 0x1701B, and the string
	// "Bribed Agent will expose enemy operations for 24 Hours"); IB tracks whole
	// days, so one day is the same span.
	ExposeOpsShieldDays = 1
	// CovertBribeOffenseMultiplier: an attacker holding a bribed agent inside the
	// target doubles its own side of the roll (BRE.OVR 0x4BA48, at +0x165 — the
	// flag is read from the ATTACKER's record, indexed by the target).
	CovertBribeOffenseMultiplier = 2
	// An ally's agents count for less on the attacking side than the defending
	// one. The two shares live in the same BRE function and are not equal.
	CovertAllyOffensePct = 40
	CovertAllyDefensePct = 50
)

// Each covert operation divides the attacker's own pool by its own difficulty
// figure, so a harder op lands less often against the same agents. BINARY-VERIFIED
// from the local resolver (BRE.OVR 0x04BE9F), which passes the divisor to the
// roll at its seven call sites; Send Spy and Spy on Relations come through
// report_spy_result (BRE.OVR 0x016D67) instead and pass 1. With no treaties in
// play, a divisor of 1 lands 55% of the time, 2 lands 40%, and 3 lands 32.5%.
const (
	CovertDifficultySendSpy            = 1
	CovertDifficultyStirRevolts        = 1
	CovertDifficultySetUp              = 1
	CovertDifficultySupportDissensions = 1
	CovertDifficultySpyOnRelations     = 1
	CovertDifficultyDemoralizeForces   = 2
	CovertDifficultyBombEnemyTargets   = 2
	CovertDifficultyBribery            = 3
)

// Covert Operations gold costs, charged per op on top of the agent risk.
// BINARY-VERIFIED (#143): the nine dwords BRE's covert menu reads from
// DS:0x63E are initialized data in BRE.EXE at file offset 0x14EDE, not a table
// built at run time. Nothing in either binary writes them, the charge is a bare
// 32-bit sub/sbb with no multiply, and the covert overlay unit never loads the
// config record — so the sysop's cost presets do NOT scale them. Confirmed
// live the same day: a game with Maintenance, Trade Deal, Region, Attack and
// Terrorist Costs all set to High drew the identical nine figures, and a Send
// Spy moved gold by exactly 5,000.
//
// Bomb Enemy Targets is one 100k menu entry in BRE, and IB now charges that
// once for the single op rather than per variant of a submenu it no longer has.
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

// Support Dissensions. BINARY-VERIFIED from the local resolver (BRE.OVR
// 0x04C178): the share of the victim's Troopers that flees is
// Random(10) + 10 - Random(10) percent, so 1-19 with a mean of 10, truncated.
const (
	DissensionsPctBase   = 10
	DissensionsPctSpread = 10
)

// Bomb Enemy Targets, the LOCAL covert op. BINARY-VERIFIED from the local
// resolver (BRE.OVR 0x04C37D onward): a successful strike rolls Random(6)+1 to
// pick ONE of six holdings and destroys Trunc(holding * percent / 100) of it,
// where each holding has its own percentage band. The player chooses nothing —
// the eight-item bombing table BRE also ships is read only by the
// interplanetary Special Operations menu.
const (
	BombTargetPickCount = 6

	BombTargetPeoplePctBase, BombTargetPeoplePctSpread   = 5, 10  // 5-14%  (BRE.OVR 0x04C39E)
	BombTargetTrooperPctBase, BombTargetTrooperPctSpread = 5, 5   // 5-9%   (0x04C427)
	BombTargetAgentPctBase, BombTargetAgentPctSpread     = 5, 5   // 5-9%   (0x04C4B0)
	BombTargetTankPctBase, BombTargetTankPctSpread       = 5, 3   // 5-7%   (0x04C539)
	BombTargetJetPctBase, BombTargetJetPctSpread         = 5, 3   // 5-7%   (0x04C5C2)
	BombTargetFoodPctBase, BombTargetFoodPctSpread       = 20, 70 // 20-89% (0x04C64B)
)

// Per-turn price WALK (#30), BINARY-VERIFIED from BRE.OVR 0x12633. Each empire
// stores its own price for each of the six military units and steps it once per
// turn in PlayTurn (World.stepPrices):
//
//	up, down := 1, 1
//	if price < mid  { down = Random(PriceWalkDampMax) + 1 }
//	if price > mid  { up   = Random(PriceWalkDampMax) + 1 }
//	price += Random(step) / up
//	price -= Random(step) / down
//	if price < lo   { price = lo + Random(PriceFloorJitter) }
//	if price > hi   { price = hi - Random(PriceCeilJitter) }
//
// mid is the band's midpoint. Whichever side of it a price sits on, the move that
// would carry the price further out is divided by 1..3 while the move back is
// taken in full, so the walk is pulled towards the centre of a band it is free to
// roam. Captured prices sit near mid for exactly this reason.
//
// The stored price is what the Spending menu shows AND what a buy/sell charges
// within the turn (shown == charged); it persists across days via the save. Steps
// are deterministic (keyed per empire+turn like riversFish) so play is
// reproducible and concurrency-safe. Regions do NOT walk — their price is
// holdings-only (917+owned×33), which BRE held exact every turn. Neither do covert
// agents, which ratchet with lifetime turns instead (see AgentPriceBase).
const (
	PriceWalkDampMax = 3
	PriceFloorJitter = 60
	PriceCeilJitter  = 100
)

// BombMarketLossPct is the share (percent) of a target's listed goods and pending
// market proceeds destroyed by a successful Bomb Trading Market covert op (#17).
const BombMarketLossPct = 25

// Bomb Trade Routes. BINARY-VERIFIED: BRE.OVR 0x051077, the routine
// `resolve_received_bombing` (0x04a09a) runs for a received op type 3. Three
// rolls decide the damage — a `random(3)` at the top of the caller that voids
// the whole strike two times in three, a `random(3)` per deal that lets one deal
// in three escape, and `trunc(qty x (random(5)+5) / 100)` on each of the deal's
// goods quantities, which leaves 5-9% and destroys the rest.
const (
	BombRoutesLandOdds      = 3 // 1-in-this the strike lands at all
	BombRoutesDealHitOdds   = 3 // 1-in-this a given deal is hit; the rest escape
	BombRoutesKeptPctMin    = 5 // a hit deal keeps this percent of each good...
	BombRoutesKeptPctSpread = 5 // ...plus random(this), so 5-9% survives
)

// MarketCommissionPct is the cut (percent) the general Trading Market takes from
// a seller's proceeds at day-end settlement (#17). It is ZERO, and that is now
// measured rather than assumed (#43, 2026-07-30).
//
// The experiment the ticket asked for, run at last: two empires driven past the
// twenty turns of new-realm protection that gate trading, seller with "Deposit
// gold at End of Turn" OFF so its bank could only move from the sale. Selby
// listed 200 troopers at 1,000; Buyar bought all 200.
//
//	buyer  1,014,956 -> 814,956 gold — paid exactly 200,000, no markup
//	seller bank 0 -> 201,250 after the day rolled
//
// The 1,250 is bank interest, not a bonus: the balance then grew to 205,048 with
// no further deposit, which is 201,250 x 1.00625^3 exactly — 0.625% per turn,
// i.e. the config's 5.0% daily rate over 8 turns per day, compounding. So the
// deposit itself was 200,000 to the coin. A commission could only ever make the
// seller's side SMALLER, so full pass-through is confirmed from both ends.
//
// Corroborating: breins.txt describes selling "at any price you choose" with no
// mention of a cut, and the market code region carries no percentage arithmetic.
const MarketCommissionPct = 0
