package game

// balance_costs.go — gold costs, covert ops, food, morale and population.
// Split out of balance.go; the provenance rules in that file's header
// apply here too, and each section below carries its own.

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
	// FoodSpoilFloor: a food stock at or below this spoils nothing at all. BINARY-
	// VERIFIED — the decay block in BRE's end-of-turn routine (BRE.OVR 0xd8ef)
	// sums the stored and market-listed food and skips the whole step unless the
	// total exceeds 1,000, which is why a fresh realm's starting food never
	// rots. IB previously spoiled every stock down to the last unit; the earlier
	// captures that "showed no floor" never ran the stock this low.
	FoodSpoilFloor = 1000
	// Food-shortfall penalties. BINARY-VERIFIED against BRE's food-allocation
	// routine (BRE.OVR 0x38104-0x381E5 for the people, 0x382E9-0x3832C for the
	// army), which bills the two needs SEPARATELY and scores each on the same
	// +1-on-both-sides ratio the crown tax uses:
	//
	//	r        = (given + 1) / (need + 1)
	//	penalty  = trunc((1 − r) × StarvationPenaltyScale)
	//
	// The people's shortfall is charged to popular support and the army's to
	// military morale, each filed as a pending penalty and applied at rollover.
	// A people's ratio under CivilWarThresholdPct additionally lights the civil
	// war (see CivilWarSeverityScale).
	//
	// BRE has NO starvation emigration: nobody leaves for want of food, and the
	// three penalty bytes are the whole of it. IB's own 70/80/10 reconstruction —
	// which also drove people out — is gone.
	StarvationPenaltyScale = 40 // binary: penalty points at a total shortfall
	// Below this percentage of the people's food need, the realm falls into civil
	// war at round((1 − r) × CivilWarSeverityScale) percent severity.
	// BINARY-VERIFIED (BRE.OVR 0x38196 holds 0.65, 0x381E5 holds 30).
	FoodCivilWarThresholdPct = 65
	CivilWarSeverityScale    = 30
)

// The two maintenance shortfalls are scored the same way as the food and crown-
// tax ones — trunc((1 − (paid+1)/(due+1)) × scale) — but land on different
// stats. BINARY-VERIFIED: the armed-forces branch at BRE.OVR 0x2F077 scales by
// 40 against MORALE, the region branch at 0x2F196 by 50 against SUPPORT.
// IB previously docked a flat fraction of the shortfall percentage instead
// (support/5 and morale/4 on forces, support/10 on regions).
//
// Region maintenance ALSO lights a civil war, and on a far easier trigger than
// famine does: anything under RegionCivilWarThresholdPct of what is due
// (BRE.OVR 0x2F23C holds 0.9), at the same round((1 − r) × 30) severity.
const (
	ForcesShortfallMoraleScale  = 40 // binary
	RegionShortfallSupportScale = 50 // binary
	RegionCivilWarThresholdPct  = 90 // binary
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

	// --- Military morale ---
	//
	// The pay-to-boost-morale prompt is the same shape as the support one, but
	// priced off the ARMY rather than the population — a realm with no soldiers
	// buys morale for the flat fee. BINARY-VERIFIED (BRE.OVR 0x2F6BA for the
	// deficit cap, 0x2F6CA for the cost, 0x2F82C for the ceiling, 0x2F91E for the
	// award):
	//
	//	deficit = min(100 − Morale, MaxMoraleBoostPerTurn)
	//	cost    = deficit × Σ(units × weight) + MoraleBoostFlat
	//	points  = deficit × (given + 1) / (cost + 1)
	//
	// Weights are held in hundredths because the original computes them as
	// Real48 fractions (0.10, 0.05, 0.10, 0.15). Bombers and Carriers are NOT
	// priced — the original loads four unit counts and stops. Units escrowed on
	// the Trading Market are counted, exactly as they are for maintenance.
	// IB's earlier placeholder charged a flat 100 gold per point up to 20 points.
	MaxMoraleBoostPerTurn = 15  // binary
	MoraleBoostFlat       = 500 // binary
	MoraleBoostMaxPct     = 150 // binary: most a baron may pay, as a % of the request
	MoraleBoostWeightUnit = 100 // the weights below are hundredths of a gold
	MoraleBoostTrooper    = 10  // binary: 0.10
	MoraleBoostJet        = 5   // binary: 0.05
	MoraleBoostTurret     = 10  // binary: 0.10
	MoraleBoostTank       = 15  // binary: 0.15

	// Desertion. BINARY-VERIFIED (BRE.OVR 0xC1F9-0xC2D5): the per-turn desertion
	// RATE is drawn from a band chosen by morale, then each of Troopers, Jets and
	// Tanks independently deserts that percentage with probability
	// (MoraleDesertTypeOdds−1)/MoraleDesertTypeOdds. Turrets, Bombers and Carriers
	// never desert. A realm at MoraleDesertBandTop or above loses nobody.
	//
	//	morale 0-9   → 22 + Random(7) − Random(17)
	//	morale 10-19 → 17 + Random(5) − Random(12)
	//	morale 20-29 → 10 + Random(3) − Random(8)
	//	morale 30-39 →  5 + Random(2) − Random(5)
	//
	// so the rate can come out zero or negative in the milder bands, which is how
	// a realm at 35 morale often loses nothing at all. IB's placeholder was a flat
	// 5% of four unit types below morale 30.
	MoraleDesertBandTop   = 40 // morale at/above which nothing deserts
	MoraleDesertBandWidth = 10 // each band spans this many morale points
	MoraleDesertTypeOdds  = 4  // one face in this many spares a unit type
	MoraleDesertPerCent   = 100
	// The bands themselves are MoraleDesertBands, below.
	// There is NO free morale recovery in the original: nothing anywhere in the
	// binary adds to morale except the boost the baron pays for. IB used to drift
	// morale back toward 100 by 4 points a turn, which quietly hid the mechanic.

	// --- Civil war (BRE.OVR 0xC59A) ---
	//
	// Severity is a percentage, filed by a food shortfall (see
	// CivilWarSeverityScale). When it fires, the realm loses that percentage of
	// every military unit type — held AND escrowed on the market — and of its
	// regions, and popular support is divided by CivilWarSupportDivisor.
	CivilWarSupportDivisor = 2   // binary: support is halved
	CivilWarPerCent        = 100 // severity is a percentage of each holding

	// Popular support below LowSupportNewsCeil puts a riot in the planet news with
	// probability 1/LowSupportNewsOdds a turn — separate from the tax riot, which
	// costs support and people. BINARY-VERIFIED (BRE.OVR 0xD5AD).
	LowSupportNewsCeil = 35
	LowSupportNewsOdds = 20

	// No covert operation pushes its victim below CovertStatFloor popular support
	// or military morale. BINARY-VERIFIED (BRE.OVR 0x4C02F for support, 0x4C2E0
	// for morale) — both floors sit in the resolver that runs every queued covert
	// op, so the floor is not the computer's alone.
	CovertStatFloor = 5

	// Stir Revolts and Demoralize Forces dock POINTS off the victim's stat, then
	// hold it at CovertStatFloor. BINARY-VERIFIED from the local resolver
	// (BRE.OVR 0x04C00C support, 0x04C2BD morale): support loses Random(4)+5,
	// morale Random(5)+5. The scaling forms IB used before (x6/7, x11/13) belong
	// to the inter-BBS packet resolver, which is a different op enumeration.
	StirRevoltsLossBase, StirRevoltsLossSpread = 5, 4 // support -= 5..8
	DemoralizeLossBase, DemoralizeLossSpread   = 5, 5 // morale -= 5..9
	// The two WMD strikes cut both stats too; their constants live with the rest
	// of each missile's figures further down (ChemMoraleKeepNum, BioMoraleDivisor,
	// StrikeSupportKeepNum).

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
	// SpyGuy — the interplanetary watcher on the InterPlanetary Special
	// Operations menu. BINARY-VERIFIED (BRE.OVR run_bombing_operations_menu
	// +0x16cb..+0x194f): the sending planet sums total_regions over every realm
	// it holds and multiplies by 300 for a PER-DAY price ("A SpyGuy will cost N
	// gold per day."), then offers a stay of up to SpyGuyMaxDays or as many days
	// as the caller's gold buys, whichever is smaller, defaulting to
	// SpyGuyDefaultDays. The 15-day ceiling is corroborated by whatsnew.doc:
	// "Extended SpyGuy to function for up to 15 days (previously, max was 10)".
	SpyGuyGoldPerRegion = 300
	SpyGuyMaxDays       = 15
	SpyGuyDefaultDays   = 3

	NukeCostPerRegion = 3_543
	NukeWastePct      = 7 // centre of the band
	NukeWasteJitter   = 3 // each of the two Random(3) draws
	// StrikeCostCap is the arms dealer's ceiling on a missile, shared by all
	// three missiles. BINARY-VERIFIED: each routine hands the same literal
	// 50,000,000 to the resident min helper before it quotes a price (BRE.OVR
	// +0x22ae nuclear, +0x266c chemical, +0x2b6e biological), so this is one
	// fact rather than three.
	StrikeCostCap = 50_000_000
	// A successful strike also pays the attacker a FLAT NukeScoreAward of Score —
	// re-read 2026-08-23 and corrected: the routine loads the figure as an
	// immediate and adds it, with no Random call anywhere near it. The earlier
	// note here said Random(900) and IB rolled one, which halved the award on
	// average. The pirate raid IS rolled (Random(300) + 100) — that is where the
	// shape came from. It adds to empire field +0x286, the field the scores table
	// prints in its Score column and every aggressive action credits. Siblings:
	// chemical 700, biological 400 (both flat).
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
	// How funding converts to strength, BINARY-VERIFIED (BRE.EXE resident
	// 056d:1139, the routine every reader of the percentage calls):
	//
	//	strength% = trunc(sqrt(funding / (SDIStrengthLandDivisor * (regions + 1))))
	//
	// clamped to 0..SDIMax. The original stores the program in whole thousands
	// and computes `thousands * 1000 / (regions+1) / 10` before the square root,
	// which is the same figure. The curve reproduces all sixteen captured screens
	// exactly at that game's 8,321 regions, so land is a divisor of the shield,
	// not just of its cost: doubling your realm halves the funding-per-region and
	// costs you ~29% of the percentage.
	SDIStrengthLandDivisor = 10
	SDIMax                 = 100 // the original's own clamp on the percentage
	// What the shield actually does, BINARY-VERIFIED. Seven call instructions in
	// four routines read the percentage, and that list is its whole reach: the two
	// screens that print it, an arriving interplanetary attack (BRE.OVR
	// ovr_03f4a0 +0xed6 for the planet-wide average, +0x10aa for the named
	// defender), and an arriving S3-Sabre bombing op (ovr_0450a9 +0x481). Nothing
	// local consults it — neither a neighbour's attack nor a nuclear, chemical or
	// biological missile — so the three effects below are all of it, and they are
	// the three the original's own instructions name.
	//
	// Against an arriving strike the reduction is linear in the percentage, and
	// the published "up to 30% / 20%" figures are what it reaches at SDI 100:
	// jets fight at `1 - SDI*0.3/100` and bombers at `1 - SDI*0.2/100`.
	SDIJetReductionPct    = 30
	SDIBomberReductionPct = 20
	// The missile ceiling works differently: the S3-Sabre is INTERCEPTED on a
	// roll — `Random(100) <= SDI * 0.5` — so a full program stops about half the
	// missiles aimed at it rather than halving each one's damage. The comparison
	// is inclusive in the original, so a realm with no program still intercepts
	// on a zero roll (1%); that is faithful, not a rounding artifact.
	SDIMissileInterceptPct = 50
	// TerrorUnitLossDenom is what a terror op takes when the packet does not say
	// WHICH operation it was — a strike written by a board old enough to predate
	// the per-op dispatch. It removes 1/N of one random unit type per agent, the
	// blanket effect every op used to get. See TerrorOpLosses for the real ones.
	TerrorUnitLossDenom = 7
	// The two morale/support operations scale the stat rather than taking a
	// percentage band off it. BINARY-VERIFIED (resolve_received_covert_operation,
	// BRE.OVR 0x04a96b): Demoralize Forces multiplies military morale by 6/7 at
	// 0x5E9, and Spread Propaganda multiplies popular support by 11/13 at 0x7B9,
	// each per landed agent.
	TerrorMoraleKeepNumerator    = 6
	TerrorMoraleKeepDenominator  = 7
	TerrorSupportKeepNumerator   = 11
	TerrorSupportKeepDenominator = 13
	// Whether one agent gets through is BRE's own two-coin-then-odds roll,
	// BINARY-VERIFIED in calculate_combat_odds (BRE.OVR 0x04a7a9): one roll in
	// TerrorAutoLandOdds lands whatever the pools say, then one in
	// TerrorAutoFoilOdds fails the same way, and only then are the two covert
	// pools weighed. The weighing takes the ROOT of each side and gives the
	// defender half again its own — success on `Random(a + d*3/2) < a`, where
	// `a` and `d` are the rounded roots — so a realm needs four times the agents
	// to double its edge. Both sides are halved until the total fits
	// TerrorOddsCeiling, which is the original guarding its 16-bit Random.
	TerrorAutoLandOdds       = 100
	TerrorAutoFoilOdds       = 20
	TerrorDefenseWeightNum   = 3
	TerrorDefenseWeightDenom = 2
	TerrorOddsCeiling        = 30_000
	// Sabotage HQ takes a flat fifteen points off the target's HeadQuarters
	// progress per landed agent — the one operation with no roll in it (0x87C).
	TerrorHQSabotagePoints = 15
	// ScorePerTurn is the Score a played turn earns. BINARY-VERIFIED as a flat
	// literal: `run_player_turn` (BRE.EXE 0x03a4f) does
	//
	//	add di,0x286        { &Score }
	//	push 0x00D5         { 213 }
	//	call add_i32_indirect
	//
	// so the award is size-independent and hardcoded, and the long-open question
	// of what the original computed it FROM is answered: nothing, it is a
	// constant. IB derived it from the starting net worth until 2026-08-23, which
	// gave the same 213 but would have drifted the moment the starting setup was
	// retuned.
	//
	// Two earlier notes here were wrong and are worth naming, because both look
	// like proofs. "213 appears nowhere in either binary" came from searching for
	// a 32-bit 213; the instruction carries a 16-bit immediate. And "Score is a
	// Real48 at record +0x28A, written from two sites, neither per-turn" was the
	// wrong FIELD — +0x28A is the last-played timestamp `run_door_session` stamps
	// from the clock global. Score is a 32-bit integer at +0x286, the field
	// show_scores prints.
	ScorePerTurn = 213
	// Riots and food spoilage do NOT affect Score — Score is the cumulative earned
	// metric, and BRE leaves it untouched by economy events (Andy's call, reversing
	// IB's earlier per-event dings).
	// Combat score, BINARY-VERIFIED (`resolve_regular_attack`, BRE.OVR 0x0102b0
	// and 0x010405): a won attack pays PER REGION TAKEN, and the two award sites
	// are told apart by the report each one follows — the higher rate sits under
	// "You have crushed the enemy completely!", the lower under "You won the
	// battle!".
	//
	// Nothing else moves Score in a battle. A repelled attack pays the defender
	// nothing and costs the attacker nothing: across both binaries the ONLY
	// writes to the Score field are on these paths and the WMD/pirate/per-turn
	// ones, and not one of them writes a realm other than the acting player's.
	// IB's earlier model — an award scaled by casualties, a defender's bonus and
	// a loser's penalty — was invented, and is gone.
	CombatScoreWinPerRegion   = 82  // ordinary win, per region captured
	CombatScoreCrushPerRegion = 192 // total conquest, per region captured
	// AllyDefenseContribPct is the share of a Full Defense Alliance partner's
	// MOBILE forces (troopers + tanks — not turrets/jets/bombers/carriers) that it
	// sends to reinforce an ally under attack. BRE-verified live (2026-07): an ally
	// sent exactly 30% of its troopers and tanks to the defender's aid.
	//
	// NOT agents. The Alliance Strength screen shows an agents column, but that
	// column belongs to a Terrorist Prevention pact at CovertAllyDefensePct — a
	// separate share set by the same routine (BRE.OVR 0x01177a, send_defensive_aid,
	// which writes 0x1e against relation 7 and 0x32 against relation 4). An
	// alliance lends no agents at all.
	//
	// See docs/mechanics-reference.md "Diplomacy" and the bre-binary-verified-math
	// memory. The sent detachment adds to the defender's battle power and takes the
	// same casualty rate as the defender.
	AllyDefenseContribPct = 30
	// A won pirate raid pays Random(PirateScoreRoll) + PirateScoreBase Score.
	// BINARY-VERIFIED (`launch_pirate_raid`, BRE.OVR 0x037004): the one Score site
	// in the routine, so a failed raid costs nothing. This is the only rolled
	// award in the game — the WMD ones are flat.
	PirateScoreBase = 100
	PirateScoreRoll = 300
	// GroupAttackLossPct is the share of a committed force lost in the strike;
	// the rest returns. 15% matches attack.hlp's normal-attack losses.
	GroupAttackLossPct = 15
)

// --- S3-Sabre: the dial, and what each setting hits (BINARY-VERIFIED) ---
//
// The dial IS the weapon's aim, not the bluff IB took it for until 2026-08-31.
// The arriving strike (BRE.OVR resolve_received_sabre_strike, 0x04546e) puts the
// dial through a mapper at ovr_0450a9 +0xf9 whose whole body is a table from the
// dial's range onto the seven `^SABREHIT` lines of game/ipreport.dat:
//
//	0, 1  -> 1  Intelligence Headquarters
//	2, 3  -> 2  residential zones
//	4     -> 3  military bases
//	5, 6  -> 4  airbases
//	7, 8  -> 5  regions incinerated
//	9, 10 -> 6  food supply
//	11    -> 7  regions DEVELOPED for the target
//
// Two rolls sit on top of it, which is what made the weapon read as random from
// the player's seat and is why the original's own manual never explained the
// numbers:
//
//   - the dial is jittered by Random(2) - Random(2) before the mapper, then taken
//     mod 11 (+0x6f7..+0x767) — so -1 at 1/4, unchanged at 1/2, +1 at 1/4;
//   - one launch in ten (Random(10) == 0 at +0x6d1) discards the dial outright
//     and takes Random(11) instead (+0x6e1).
//
// So dial 4 lands on military bases half the time, while dial 5 lands on
// airbases three quarters of the time — 5 and 6 both map there. That asymmetry
// is the shape of the advice experienced players give, which is the corroboration
// that made this worth re-reading.
//
// Setting 11 is unreachable through this path: the input is taken mod 11, so the
// mapper's last row never fires from a dial. Effect 7 belongs to another caller.
const (
	SabreDialMin = 0
	SabreDialMax = 10

	// SabreDialJitterSides is the size of each of the two Random() draws whose
	// difference nudges the dial (binary: Random(2) twice).
	SabreDialJitterSides = 2
	// SabreDialWrap is the modulus applied after the jitter (binary: 11).
	SabreDialWrap = 11
	// SabreWildOdds: one launch in this many ignores the dial entirely.
	SabreWildOdds = 10
)

// SabreEffect is one of the seven outcomes the mapper selects.
type SabreEffect int

const (
	SabreHitHQ SabreEffect = iota + 1
	SabreHitPeople
	SabreHitMilitaryBases
	SabreHitAirbases
	SabreHitRegions
	SabreHitFood
	SabreDevelopRegions
)

// sabreDialTable is the mapper's table, indexed by the (jittered, wrapped) dial.
// Index 11 is the mapper's last row, unreachable from a dial but kept so the
// table is the binary's table rather than a truncation of it.
var sabreDialTable = [12]SabreEffect{
	SabreHitHQ, SabreHitHQ,
	SabreHitPeople, SabreHitPeople,
	SabreHitMilitaryBases,
	SabreHitAirbases, SabreHitAirbases,
	SabreHitRegions, SabreHitRegions,
	SabreHitFood, SabreHitFood,
	SabreDevelopRegions,
}

// SabreConstantDial is the dial a board fires on under Constant handling. IB's
// own choice — the original's handling modes are a per-install setting and it
// does not say what "constant" fires. The middle of the range keeps the mode
// from being strictly better or worse than dialling by hand.
const SabreConstantDial = 5
