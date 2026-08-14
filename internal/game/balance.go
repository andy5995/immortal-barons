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

// --- The Queen Royale's tax refund (#93) ---
//
// The crown tax is not destroyed. Every gold actually paid is banked in a
// planet-wide pool, and the Queen hands a share of that pool back to each realm
// at the start of its first session of a game day. The refund is deterministic:
// the routine holds no random draw, so a realm that logs in gets it.
//
// BINARY-VERIFIED (BRE.OVR 0x18280, called from BRE.EXE 0x61dd):
//
//	rate    = QueenRefundRate, or QueenRefundHighRate once pool > QueenRefundHighPool
//	payout  = trunc(pool * rate)          capped at QueenRefundCap while protected
//	pool   -= payout
//
// The cap is gated on the realm still being under New Realm Protection, so a
// newcomer joining a mature planet cannot open with a many-million-gold windfall
// while an established realm takes the full share. Because the pool is read
// fresh each time, the first baron to play on a given day takes the largest cut
// and everyone after them draws on what is left.
//
// IB pays exactly QueenRefundCap where BRE often pays 999,999: the original caps
// by substituting cap/pool for the rate and multiplying back, and the round trip
// through a six-byte real loses the last unit. That is an artifact of its float
// format, not a rule.
const (
	QueenRefundPoolSeed = 100_000     // the pool a fresh game starts with
	QueenRefundRate     = 2           // percent of the pool paid out
	QueenRefundHighRate = 7           // percent once the pool is over QueenRefundHighPool
	QueenRefundHighPool = 100_000_000 // the threshold that selects the higher rate
	QueenRefundCap      = 1_000_000   // ceiling while the realm is still protected
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

// --- Unit / land / food prices ---
//
// The six military units are BINARY-VERIFIED. BRE gives every empire its own
// stored price per unit and walks it one step per turn inside a fixed band. The
// three tables below are three arrays of six words in BRE.EXE's data segment
// (DS:0x508 step, DS:0x514 low, DS:0x520 high), read by the walk at BRE.OVR
// 0x12633. Sell price is buy/3 (see sellUnit), except agents (SellAgentPrice).
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
	// SellAgentPrice: agents sell at a flat 100 in BRE, not buy/3 like other units.
	// Binary-verified — the Sell menu's agent column is the literal 100 pushed at
	// BRE.OVR 0x16AEB, and every sell capture shows it.
	SellAgentPrice = 100
)

// Covert Operations gold costs, charged per op on top of the agent risk
// (live-sampled from BRE's Covert Operations menu at the DEFAULT/medium game
// setup, 2026-07-21 — other BRE setups scale these; tune here as needed). Bomb
// Enemy Targets is one 100k menu entry in BRE; IB splits it into a submenu and
// charges each variant CostBombEnemyTargets.
// The covert roll, BINARY-VERIFIED from BRE.OVR 0x4BA48 (the roll) and 0x4CAB7
// (the agent pools). docs/mechanics-reference.md carries the full routine and the
// evidence, including the defect IB now reproduces for Send Spy.
const (
	// CovertAutoSuccessOdds: one roll in this many succeeds before agents are
	// weighed at all.
	CovertAutoSuccessOdds = 10
	// CovertBribeSlipOdds: an attacker whose agent has been bribed still slips
	// through one time in this many instead of failing outright.
	CovertBribeSlipOdds = 10
	// SpyDifficulty divides the attacker's pool. BRE passes 1 for Send Spy, which
	// is the only op reached through this roll; a harder op would pass more.
	SpyDifficulty = 1
	// An ally's agents count for less on the attacking side than the defending
	// one. The two shares live in the same BRE function and are not equal.
	CovertAllyOffensePct = 40
	CovertAllyDefensePct = 50
)

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

// --- HeadQuarters (BRE-verified: BRE.OVR 0xD010, 0x12C8C, 0x40241/0x4043D) ---
//
// A tank's strength in hundredths of a trooper, rising with HQ completion:
// 350 at HQ 0%, 400 at 50%, 450 at 100%. BINARY-VERIFIED from the regular
// attack routine (BRE.OVR 0xF2E4), which builds each side's strength as
//
//	troopers x 0.5 + (jets | turrets) x 1 + tanks x (1.75 + HQ/200)
//
// against a trooper's 0.5, so in whole troopers that is 1 : 2 : 3.5..4.5. Only
// the ratios matter, since both sides are scaled the same way. breins.txt calls
// a tank "about the equivalent of four Troopers", which is the HQ-50 value —
// the manual and the binary agree once the HQ term is read.
//
// An earlier reading had the base a half-trooper low and the HQ term twice too
// steep (300 rising to 500), which undervalued tanks in a realm with no
// HeadQuarters and overvalued them in one with a finished HeadQuarters.
const (
	TankStrengthPctBase  = 350 // binary
	TankStrengthPctPerHQ = 1   // binary: HQ/200 doubled to trooper units
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

	// What a lost defence costs the HeadQuarters: Random(3)+5 points off, then
	// clamped at zero. BINARY-VERIFIED (BRE.OVR 0xFFA2, subtracted via the
	// sub32 helper 0c03:0fe3). So a realm under repeated attack loses the tank
	// bonus it spent 20 turns building, and a HeadQuarters is not permanent.
	HQBattleLossMin    = 5 // binary
	HQBattleLossJitter = 3 // binary: Random(3) added to the minimum
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
// score-to-turns derivation used to check them, not from the formula. Re-checked
// on a third, unrelated game (5/5), which included a brand-new realm priced at
// exactly 5,000 — the base with a zero draw.
//
// Note the price does NOT rise every single turn: the trend is +75 but the draw
// spans 300, so consecutive turns can dip. It climbs unmistakably over many
// turns, which is what makes building early worth it.
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

// Covert agents price the same way as a HeadQuarters, off the same counter.
// BRE.OVR 0x1293E reads the empire's lifetime turn count and computes
// AgentPriceBase + AgentPricePerTurn×TurnsPlayed + Random(AgentPriceJitter), with
// no cap. Agents therefore take no part in the unit walk: their price ratchets for
// as long as the realm keeps playing and never comes back down, so a covert
// programme started late costs 20 gold a turn more than one started early, for
// every agent, forever.
//
// The 30 consecutive turns in `cap/eots-covert-agents.cap` pin it. Solving each
// turn's agent AND HeadQuarters price for a shared turn count leaves exactly one
// integer feasible per turn, and it advances by one every turn, day boundary
// included. Binary-verified; do not retune.
const (
	AgentPriceBase    = 450
	AgentPricePerTurn = 20
	AgentPriceJitter  = 300
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
	// floor(0.05 × food). BRE-verified by driving the original (2026-07-16):
	// spoilage matched floor(5% × food) to the unit across food stocks from 1.4k
	// to 13.9k. Technology decreases it (via TechFactor).
	FoodSpoilPct = 5
	// FoodSpoilFloor: a granary at or below this spoils nothing at all. BINARY-
	// VERIFIED — the decay block in BRE's end-of-turn routine (BRE.OVR 0xd8ef)
	// sums the stored and market-listed food and skips the whole step unless the
	// total exceeds 1,000, which is why a fresh realm's starting granary never
	// rots. IB previously spoiled every stock down to the last unit; the earlier
	// captures that "showed no floor" never ran the granary this low.
	FoodSpoilFloor = 1000
	// Food-shortfall penalties (IB reconstruction — BRE publishes no rate;
	// calibrated to a live BRE point: ~73% short dropped support ~50 points in one
	// turn). Scaled by how much of the turn's food need went unmet (0–100%): the
	// hungrier the realm, the more popular support falls and the more people leave.
	FoodShortfallSupportDrop   = 70 // support points lost per turn at 100% unfed
	FoodShortfallMoraleDrop    = 80 // morale points lost per turn at 100% unfed (hungry troops demoralize faster than the public, as under pay shortfall)
	FoodShortfallEmigrationPct = 10 // % of population that leaves per turn at 100% unfed
)

// Population / migration. BINARY-VERIFIED against BRE's end-of-turn routine
// (BRE.OVR 0xD08A-0xD3CC), which IB had previously replaced with its own
// logistic tuning. The carrying capacity is
//
//	Σ(regions × weight) × support/90 × 10/max(3, tax) + 50
//
// and each turn the population moves 5-9% of the gap toward it, amplified when
// it is shrinking, jittered, cut again above a punitive tax rate, and held to
// half the realm's size. See popCapacity and processEconomy in turn.go.
//
// BRE and IB COUNT PEOPLE IN DIFFERENT UNITS, so the weights below have to be
// converted rather than used raw. BRE stores population as a 16-bit count of
// MILLIONS ("Population: 101 Million", record +0x62) and reports migration as
// "gained N million people"; IB counts people directly, twenty to BRE's one.
// See PopBREUnitScale.
const (
	// Carrying-capacity weight per region, by type. Urban housing dominates by
	// more than a factor of ten — a realm that wants people buys Urban.
	PopCapCoastal      = 7   // binary
	PopCapRiver        = 10  // binary
	PopCapAgricultural = 8   // binary
	PopCapDesert       = 4   // binary
	PopCapIndustrial   = 9   // binary
	PopCapUrban        = 102 // binary
	PopCapMountain     = 8   // binary
	PopCapTechnology   = 7   // binary
	// PopCapWaste is ZERO, and that is a reading of the binary rather than an
	// omission. BRE's capacity routine (BRE.OVR 0xD08A) loads eight region counts
	// — `+0x96` through `+0xb2`, one per weight above — and stops. The ninth
	// count, Waste at `+0xb6`, is never read: eight loads, seven adds. So ruined
	// land houses nobody, which is what gives a nuclear strike its bite, since
	// the target keeps every region and its upkeep but loses the people they held.
	PopCapWaste = 0 // binary: absent from the capacity sum

	PopCapSupportDivisor = 90 // binary: capacity scales by support/90, so 90% support is neutral
	PopCapTaxNumerator   = 10 // binary: and by 10/tax, so a 10% rate is neutral
	PopCapTaxFloor       = 3  // binary: max(3, tax) — the tax divisor never goes below 3
	PopCapBase           = 50 // binary: a floor every realm gets regardless of holdings

	// PopBREUnitScale converts the weights and base above — all in BRE's unit of
	// one million — into IB's, which counts twenty people to BRE's one. The
	// factor is pinned by the two games starting the SAME realm: BRE's new realm
	// (2 Agricultural, 5 Desert, 5 Mountain, 3 Coastal, 100 troopers, 1000 food,
	// 100% support, 15% tax — IB's starting mix exactly) reads "Population: 100
	// Million", and IB starts it at StartPeople = 2000.
	//
	// Leaving it out is what made a fresh realm bleed people from its first turn:
	// the raw weights put its capacity at 121 against a population of 2000, so
	// migration read a realm sixteen times over-full and drained ~300 a turn,
	// while the same realm in BRE sits just under capacity and grows. The same
	// factor is already baked into TaxGoldPerCapita — BRE's new realm earns 5183
	// gold at 15%, about 345 per BRE unit, which is IB's 17 x 20 — so the two
	// stay consistent only while both carry it.
	PopBREUnitScale = 20

	// Per-turn movement toward the capacity: Random(5)+5 percent of the gap.
	PopMoveMinPct = 5 // binary
	PopMoveJitter = 5 // binary: Random(5) added to the minimum

	// A SHRINKING realm loses people faster the harder it taxes:
	// growth × sqrt(tax) / 2. The sqrt is BINARY-VERIFIED: the routine the
	// overlay calls (fd0:1841) is at BRE.EXE 0x13e81 and is Newton-Raphson for
	// square root — it seeds the guess by halving the real's biased exponent
	// (add cl,0x80 / sar cl,1 / add cl,0x80), sets a 2^-20 tolerance
	// (sub al,0x14), then loops divide-add-halve (the halve being dec al on the
	// exponent byte) until it converges. Ln would carry a ln(2) polynomial and
	// neither halve an exponent nor divide in a loop.
	PopDeclineTaxDivisor = 2.0 // binary

	// Churn either way, so a realm sitting exactly at capacity still moves:
	// + Random(capacity/100) - Random(capacity/300).
	PopChurnUpDivisor   = 100 // binary
	PopChurnDownDivisor = 300 // binary

	// Above this rate people leave on top of everything else.
	PopPunitiveTaxRate = 50 // binary
	PopPunitiveTaxPct  = 25 // binary: a further quarter off the turn's movement

	// No realm grows by more than half its size in one turn.
	PopGrowthCeilingDivisor = 2 // binary
)

const (
	// Agricultural food follows the same Base + [0, Rate) draw the gold regions
	// use — one roll per turn, multiplied by the region count. BINARY-VERIFIED:
	// BRE.OVR 0x33b64 returns 300 raised by the food technology factor, and its
	// caller in the production routine adds Random(5) before multiplying by the
	// Agricultural count. The technology factor lands on the BASE only, which is
	// what six captures showed as per-region figures of exactly 300 … 304.
	FoodAgriBase = 300
	FoodAgriRate = 5
	// Rivers produce BOTH gold and food every turn — a DELIBERATE DIVERGENCE.
	// In BRE a river does hydropower OR fishing each turn, never both: the
	// production routine rolls Random(4) and fishes only on a zero. IB pays the
	// same expected value with none of the swing: RiverFishShare of a river's
	// output is taken as food, the rest as hydropower gold. A player committed to
	// rivers otherwise watches millions of gold appear and vanish turn to turn
	// with no way to plan around it, and an un-plannable food source is close to
	// useless.
	//
	// One constant drives both halves so they cannot drift apart when tuned, so
	// the share IS the original's fishing chance — 1 in 4.
	RiverFishShare = 25 // binary: % of turns a BRE river fishes; IB takes it as a share
	// The fishing haul itself is the same shape as the Agricultural one, from
	// BRE.OVR 0x33ba6 and the same caller: 110 raised by the food technology
	// factor, plus Random(20). IB paid a flat 124, mid-band and untouched by
	// technology.
	RiverFishFood = 110 // binary: food per River region on a fishing turn, before the draw
	RiverFishRate = 20  // binary: width of the draw added to it
	// Food is TWO obligations, each truncated on its own: BRE bills the people
	// and the armed forces from separate routines and prompts for them one after
	// the other. Summing the terms before truncating reads one food high whenever
	// both have a fraction. BINARY-VERIFIED from the food overlay unit's two
	// need routines, BRE.OVR 0x37418 (people) and 0x37459 (armed forces).
	//
	// FoodPerBREPopUnitTenths: the people eat 1.5 food per unit of population,
	// where BRE's unit is one million — PopBREUnitScale of IB's people. Held in
	// tenths so the conversion to IB's counter stays integer.
	FoodPerBREPopUnitTenths = 15
	// The armed-forces routine sums TWELVE terms as one real and truncates once:
	// six unit types, each counted where it is held AND where it sits escrowed on
	// the Trading Market. Troopers carry weight 0.5, every other type 0.01, and
	// the whole sum is then multiplied by 0.01 — so a trooper eats 1 food per 200
	// and everything else 1 per 10,000. Food and agents are not among the twelve,
	// so neither eats. The weights below are that product, in ten-thousandths.
	//
	// So the whole army eats: BRE's own changelog says "All military units now
	// require food to survive" (docs/whatsnew.doc, 0.97). IB charged troopers
	// only, on a measurement that added 1,000 jets and 533 tanks — both of which
	// truncate to zero at 1 per 10,000, so it could not have detected them (#91).
	ForcesFoodWeightScale   = 10_000
	ForcesFoodTrooperWeight = 50 // 1 food per 200 troopers
	ForcesFoodUnitWeight    = 1  // 1 food per 10,000 jets/turrets/bombers/tanks/carriers

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
	// Those live prompts read BRE's population figure, which counts MILLIONS, so
	// SupportBoostPerPerson is per BRE unit and SupportBoostCost converts IB's
	// count back down by PopBREUnitScale before applying it.
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
	// A new realm's production split, one constant per unit type, totalling 100%.
	//
	// DELIBERATE DIVERGENCE FROM BRE, which defaults all six to 15% and lets the
	// remaining 10% fall through to industrial gold. Two changes here: the whole
	// pool goes to units, and jets and carriers are matched to each other.
	// Because a unit's output is pct/cost, a flat split builds one carrier per
	// 12.5 jets while a carrier lifts JetsPerCarrier (100) — eight times the lift
	// the jets can use, spent at the most expensive unit in the table. Carriers
	// therefore take an eighth of the jet share, which 16 and 2 satisfy exactly.
	//
	// The 82% left over cannot divide four ways evenly, so the odd points go to
	// turrets and tanks — the units that are useful the turn they are built,
	// without an escort or a target worth bombing.
	DefaultProdTroopersPct = 20
	DefaultProdJetsPct     = 16
	DefaultProdTurretsPct  = 21
	DefaultProdBombersPct  = 20
	DefaultProdTanksPct    = 21
	DefaultProdCarriersPct = 2
	DefaultProdGoldPct     = 0 // capacity explicitly reserved for gold; the remainder pays gold anyway
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

	// Nuclear strike. BINARY-VERIFIED (BRE.OVR launch_nuclear_attack, unit
	// ovr_00e809 +0x225e): the missile is priced off the TARGET's size, not sold
	// at a flat rate — cost = min(targetRegions * 3543, 50,000,000) — and it
	// ruins `7 + Random(3) - Random(3)` percent of the target's regions, a 5-9%
	// band centred on 7. Nothing in that routine reads the target's SDI or
	// turrets, so a local nuclear strike is not intercepted; SDI's "up to 50% of
	// incoming missiles" (breins.txt) is the interplanetary path.
	NukeCostPerRegion = 3_543
	NukeWastePct      = 7 // centre of the band
	NukeWasteJitter   = 3 // each of the two Random(3) draws
	// StrikeCostCap is the arms dealer's ceiling on a warhead, shared by all
	// three missiles. BINARY-VERIFIED: each routine hands the same literal
	// 50,000,000 to the resident min helper before it quotes a price (BRE.OVR
	// +0x22ae nuclear, +0x266c chemical, +0x2b6e biological), so this is one
	// fact rather than three.
	StrikeCostCap = 50_000_000
	// A successful strike also pays the attacker Random(NukeScoreAward) Score.
	// BINARY-VERIFIED from the same routine, which adds it to empire field
	// +0x286 — the field the scores table prints in its Score column and every
	// aggressive action credits. The sibling awards, for #103: chemical
	// Random(700), biological Random(400), a pirate raid Random(300) + 100, a
	// spy Random(30). The two regular-attack awards multiply a battle figure by
	// 192 and 82 and are gated behind a league config byte, so they need their
	// own pass.
	NukeScoreAward = 900
	// Waste decontamination, BINARY-VERIFIED (BRE.OVR ovr_02e6b2 +0x458 and
	// +0x4b1). A turn may clean min(max(waste/5, 10), waste) regions — 20% of
	// what is ruined, but never fewer than 10 and never more than you hold — and
	// each one costs the current region price divided by twice the empire's FOOD
	// technology factor (the original passes `technology_factor(2.0, slot 0)`,
	// the agricultural pair, not the maintenance one). So a realm with no
	// technology cleans at half the price of new land, and a fully teched one at
	// a quarter.
	WasteDecontamDivisor  = 5  // 20% of the waste pile per turn
	WasteDecontamFloor    = 10 // ...but at least this many
	WasteDecontamPriceDiv = 2  // region price divided by this, before technology

	// Chemical strike. BINARY-VERIFIED (BRE.OVR launch_chemical_attack, unit
	// ovr_00e809 +0x25f0). Like the nuclear missile it is priced off the target
	// and ruins land into waste through the same helper, but it is priced off
	// the target's PEOPLE as well as its land, ruins a third as much of it, and
	// takes a flat bite out of the population, morale and popular support.
	//
	// ChemCostPerPop is per BRE population unit — one million — so a caller
	// converts with PopBREUnitScale before applying it.
	//
	// Nothing in the routine reads the target's SDI, turrets or tanks: its only
	// reads of the target record are the realm name, the region total, the
	// population, morale and support. `whatsnew.doc`'s "Tanks now help defend
	// against incoming Chemical Missiles" predates v0.988 and does not survive
	// into the shipped local routine, the same way the nuclear entry's missile
	// bases do not.
	ChemCostPerPop    = 94
	ChemCostPerRegion = 2_037
	ChemWastePct      = 3  // centre of the band: 3 + Random(3) - Random(3), so 1-5%
	ChemWasteJitter   = 3  // each of the two Random(3) draws
	ChemPopKillPct    = 20 // a flat fifth of the population, with no roll at all
	ChemMoraleKeepNum = 3  // morale  := round(morale  * 3/4)
	ChemMoraleKeepDen = 4
	ChemScoreAward    = 700

	// Biological strike. BINARY-VERIFIED (BRE.OVR launch_biological_attack,
	// unit ovr_00e809 +0x2ac6). It touches no land at all — the routine never
	// calls the region-to-waste helper — and instead kills people and troopers
	// and halves military morale. Its price reads three of the target's figures:
	// troopers, population (in BRE's unit of a million, so PopBREUnitScale
	// applies) and regions.
	//
	// The Score award lands right after the purchase, BEFORE any damage is
	// rolled, so a strike that kills nothing still pays — the same as the
	// nuclear one, by a different route.
	BioCostPerTrooper    = 23
	BioCostPerPop        = 434
	BioCostPerRegion     = 1_237
	BioPopKillPct        = 10 // 10 + Random(4) - Random(2), so a 9-13% band
	BioPopKillJitterUp   = 4
	BioPopKillJitterDown = 2
	BioTroopKillPct      = 15 // 15 + Random(6) - Random(4), so a 12-20% band
	BioTroopKillJitterUp = 6
	BioTroopKillJitterDn = 4
	BioMoraleDivisor     = 2 // morale := morale / 2, truncated (an integer divide, not a real one)
	BioScoreAward        = 400

	// Both the chemical and the biological strike leave popular support at
	// round(support * 2/3). BINARY-VERIFIED from the same two routines, which
	// run the identical real expression on field +0x92.
	StrikeSupportKeepNum = 2
	StrikeSupportKeepDen = 3

	SDIStep = 1_500_000 // gold of total funding per +1% SDI
	// The SDI program's two running figures, CAPTURE-VERIFIED: seventeen
	// consecutive SDI Program screens from a live league game (2026-08-08, see
	// docs/dev/bre-screens.md) fit both exactly, across funding from 0 to just
	// over 7 million.
	//
	// The original bills the upkeep every turn even though the screen calls it
	// yearly, and refills the spending allowance every turn too — a turn is its
	// "year" here.
	SDIMaintPct  = 4       // per-turn upkeep, as a percent of total funding
	SDISpendPct  = 20      // per-turn funding allowance, as a percent of total funding
	SDIMinSpend  = 250_000 // floor under that allowance, so a new program can start
	SDIIncrement = 1_000   // funding is accepted only in whole thousands
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

// --- Individual interplanetary attack variants (BRE-verified) ---
//
// BRE lets an individual strike be sent three ways, each trading attacking
// strength against how much land it takes and how far both armies press before
// retreating. Every figure below is quoted from the original's own in-game help,
// game/attack.hlp, so these are fidelity contract, not playtest knobs. The wire
// codes come from a disassembly of the IBBS attack unit in BRE.OVR, which stores
// Quick=0, Normal=1, Extended=2 in the outbound attack record.
//
// Capture percentages are relative to a normal attack's take, which is how
// attack.hlp states them ("50% of what you would in a normal attack").
const (
	// A quick strike buys surprise: it hits harder but takes far less land, and
	// the disorganised battle breaks off early.
	QuickStrikeStrengthPct = 110
	QuickStrikeCapturePct  = 50
	QuickStrikeLossPct     = 8

	// The normal attack is the baseline the other two are quoted against.
	NormalAttackStrengthPct = 100
	NormalAttackCapturePct  = 100
	NormalAttackLossPct     = GroupAttackLossPct

	// An extended battle grinds: fatigue costs strength, but it takes the most
	// land and both armies absorb the heaviest losses before retreating.
	ExtendedBattleStrengthPct = 85
	ExtendedBattleCapturePct  = 125
	ExtendedBattleLossPct     = 20

	// IndividualAttackGoldPerUnit is what launching an individual interplanetary
	// strike costs, per unit sent. Captured live (2026-08-11): committing 100
	// troopers and nothing else printed "This attack will cost 100 gold." The
	// rate is confirmed for troopers only — whether jets, tanks and bombers cost
	// the same per unit is NOT verified, so they use this rate until a capture
	// says otherwise.
	IndividualAttackGoldPerUnit = 1

	// IndividualAttackReturnsPct is what an individual strike carries off
	// relative to a group attack of the same weight. BRE's own docs state the
	// trade in both places they describe the option: "You get twice as many
	// returns if you send the attack yourself, but you can't attack an entire
	// planet" (game/breins.txt and docs/bre.doc). Going alone is the reward for
	// giving up the whole-planet target and the pooled forces.
	IndividualAttackReturnsPct = 200
)

// The local Regular Attack. BINARY-VERIFIED against BRE's driver
// (BRE.OVR 0xEF90) and the resolver it calls (0xE81F).
//
// Attack Rewards and Attack Damage are TWO INDEPENDENT TABLES, not the generic
// Level.Percent multiplier IB used to apply to a Medium baseline. That
// multiplier (0/50/100/200) happens to land on Low and Medium for both knobs
// and is wrong at both ends: BRE's High takes two and a half times the Medium
// capture, costs one and a half times the Medium force, and its None still
// costs a side a token one percent.
const (
	// Share of the defender's regions a winning attack takes, by Attack Rewards.
	// BRE.OVR 0xFFF9 loads 0.10 as the default, then switches on the config byte
	// at +0x183 and loads a Real48 per level: 0.00, 0.05, 0.10, 0.25.
	AttackCaptureNonePct   = 0
	AttackCaptureLowPct    = 5
	AttackCaptureMediumPct = 10
	AttackCaptureHighPct   = 25

	// Share of its own force a side will lose before it breaks off, by Attack
	// Damage. Same switch shape at BRE.OVR 0xF84F on the byte at +0x181, except
	// the constants are what SURVIVES — 0.99, 0.90, 0.80, 0.70 — so the losses
	// are their complements. attack.hlp's flat "15%" describes the interplanetary
	// variants, not this table.
	AttackRetreatNonePct   = 1
	AttackRetreatLowPct    = 10
	AttackRetreatMediumPct = 20
	AttackRetreatHighPct   = 30
)

// Per-round attrition inside a battle (BRE.OVR 0xE8F4-0xEA46). Each round, each
// side is hit with a probability equal to its OPPONENT's share of the two
// strengths, or failing that on a flat upset chance; a hit multiplies that side
// by BattleRoundSurvival and takes one more point off the top. The loop runs
// until a side reaches its retreat threshold above, and THAT is what makes the
// winner's casualties an outcome rather than a rate: a lopsided attacker is
// almost never the one hit, so it walks away having lost very little, while an
// evenly matched one pays nearly the full retreat share.
const (
	BattleRoundSurvival = 0.99 // binary: Real48 0.99, both sides
	BattleRoundFlatLoss = 1    // binary: the "- 1" after the multiply
	BattleUpsetPct      = 5    // binary: Real48 0.05, the consolation roll
)

// --- Upkeep / maintenance (BRE-verified — live capture, Maintenance Costs
// "Medium") ---
//
// Per-unit maintenance per turn, in TENTHS of a gold (Technology reduces the
// total via TechFactor — see Empire.ForcesUpkeep). These are literal gold, not a
// ratio to scale: a turret-only empire was charged exactly trunc(0.9 × turrets)
// at five army sizes from 49,691 turrets (44,721 gold) to 219,032 (197,128).
//
// EVERY RATE IS LIVE-VERIFIED, from controlled captures that held one unit type
// at a time with no Technology reduction (Maintenance Costs "Medium", tech
// maintenance factor reading 100%), most at two different army sizes:
//
//	100 troopers → 40 gold      50 troopers → 20     ⇒ 0.40
//	 20 jets     → 24            60 jets     → 72    ⇒ 1.20
//	 20 turrets  → 18            60 turrets  → 54    ⇒ 0.90
//	 20 tanks    → 12            40 tanks    → 24    ⇒ 0.60
//	 20 bombers  → 26                                ⇒ 1.30
//	 20 carriers →  2                                ⇒ 0.10
//
// The guide table turns out to be right on every row EXCEPT troopers, where it
// prints 0.60 against a measured 0.40 (seen in three separate games). Trust the
// figures here, not the table.
const (
	MaintTrooperTenths = 4  // measured, 3 games — the guide's 6 is wrong
	MaintJetTenths     = 12 // measured, two army sizes
	MaintTurretTenths  = 9  // measured, two army sizes, plus five earlier
	MaintBomberTenths  = 13 // measured
	MaintTankTenths    = 6  // measured, two army sizes
	MaintCarrierTenths = 1  // measured

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

// --- Net-worth weights (BINARY-VERIFIED) ---
//
// Contribution to net worth per unit / per region, in thousandths of a gold
// (World.NetWorth divides by 1000 for exactness). Read out of BRE's own net-worth
// function, 056d:0F43 (BRE.EXE 0x8F53), on 2026-08-01: every weight below matches
// to the digit. Bombers and carriers are integer multiplies there; the rest are
// Turbo Pascal reals. See docs/mechanics-reference.md for what BRE counts that IB
// does not.
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

// Money ceilings. The money fields are int64 (Empire.Gold and friends), so what
// a realm may hold is a game rule rather than a machine-word limit: a 32-bit
// door build holds the same range as a 64-bit one. The ceiling used to be a
// hard-coded 2 billion because plain int is 32 bits on a 32-bit build, and every
// gold credit past it was silently discarded.
const (
	InterestCap int64 = 1_599_999_999 // bank balance above this earns no more interest

	// What a realm may HOLD, on hand or in the bank, is the sysop's call:
	// Config.MoneyCapBillions, read through World.MoneyCap. The default is BRE's
	// own 2 billion; a league that wants a longer game raises it. These are the
	// bounds of that knob, in whole billions so the Configuration Editor's field
	// stays three digits and fits an int on a 32-bit door.
	//
	// The ceiling is 999 because MoneyCapMax is the widest figure the abbreviated
	// billions display renders in three digits before the point (999.0000B).
	// GoldPerBillion names the unit the cap is set in, so the multiplier is not
	// respelled at each site that converts between the two.
	GoldPerBillion = 1_000_000_000

	MoneyCapMinBillions = 2
	MoneyCapMaxBillions = 999

	// MoneyCapMax is the highest that knob can reach. Pure helpers that project a
	// future figure (ExpectedReturn, LoanTotalOwed) clamp to it as an overflow
	// guard; what a realm actually holds is clamped to the configured cap when
	// the gold lands.
	MoneyCapMax int64 = MoneyCapMaxBillions * GoldPerBillion

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
// goods are escrowed and arrive on the recipient's next turn. (BRE adds a small
// deal-size component on top of the 100,000/day base that IB does not model.)
const (
	TradeDealCarriers   = 1       // carriers consumed to send one deal
	TradeDealGoldPerDay = 100_000 // gold cost per day of transit
	TradeDealMinDays    = 2       // shortest a deal may be sent for
	TradeDealMaxDays    = 5       // longest a deal may be sent for
)

// The Clingy Annihilator, IB's rename of BRE's Gooie Kablooie. It is not a purchase
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
// Everything below the cost is IB's own reconstruction — the build, decay and
// interception numbers were not read from the binary. They follow what the
// original's help describes and are ordinary playtest knobs.
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

	// Flight and decay (reconstructed). The weapon is visible to its target for
	// the whole flight, which is what makes shooting it down possible.
	AnnihilatorFlightDays = 2
	// On arrival it destroys AnnihilatorDamagePct of each realm's land, and a strike
	// that is only partly intact does proportionally less.
	AnnihilatorDamagePct = 10

	// Interception (reconstructed). Only jets can reach it — the original is
	// explicit that nothing else can — and each sortie of AnnihilatorJetsPerPercent
	// jets knocks one percent off it. The jets are spent either way.
	AnnihilatorJetsPerPercent = 250
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
