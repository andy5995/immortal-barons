package game

// balance_combat.go — combat: the interplanetary attack variants and the cost levels.
// Split out of balance.go; the provenance rules in that file's header
// apply here too, and each section below carries its own.

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
//
// The STRENGTH multipliers are the one place the help and the code disagree, and
// the code wins. BRE's own invasion resolver (BRE.OVR 0x4055a-0x405a8, unit
// ovr_03f4a0 +0x10ef..+0x1184) switches on the arriving attack's type byte and
// scales the incoming force by a Real48 constant: Quick loads 1.2, Extended
// loads 0.85, Normal is left alone. attack.hlp advertises the quick strike at
// "110%" — the binary applies 120%.
//
// What settles WHICH case is which — and so that the 1.2 really is the quick
// strike's — is that the same switch loads a SECOND Real48 per case, the
// survival fraction the battle breaks off at: 0.92, 0.85 and 0.80. Those are
// losses of 8%, 15% and 20%, which is exactly what attack.hlp publishes for
// quick, normal and extended. The help file is therefore right about three of
// the four figures in that paragraph and wrong about the fourth, rather than
// the case mapping being misread. Re-derived independently after the first
// reading flagged itself as uncertain; do not "correct" 120 back to 110.
const (
	// A quick strike buys surprise: it hits harder but takes far less land, and
	// the disorganised battle breaks off early.
	QuickStrikeStrengthPct = 120 // binary: Real48 1.2 (attack.hlp says 110%)
	QuickStrikeCapturePct  = 50
	QuickStrikeLossPct     = 8

	// The normal attack is the baseline the other two are quoted against.
	NormalAttackStrengthPct = 100
	NormalAttackCapturePct  = 100
	NormalAttackLossPct     = GroupAttackLossPct

	// An extended battle grinds: fatigue costs strength, but it takes the most
	// land and both armies absorb the heaviest losses before retreating.
	ExtendedBattleStrengthPct = 85 // binary: Real48 0.85
	ExtendedBattleCapturePct  = 125
	ExtendedBattleLossPct     = 20

	// GroupAttackHoursMin/Max bound the delay before a group attack leaves.
	// BINARY-VERIFIED: BRE.OVR 0x2c38a/0x2c391 (unit ovr_02b783 +0x0c07/+0x0c0e)
	// push 12 and 120 as the prompt's bounds and +0x0c4b/+0x0c4f re-check the
	// answer against both. The delay is HOURS, not days — +0x0c88 divides it by
	// 24 and +0x0c99 adds the quotient to the clock, so what BRE stores is an
	// instant the force leaves at, not a day number (#124).
	GroupAttackHoursMin = 12
	GroupAttackHoursMax = 120

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

	// The INTERPLANETARY resolver keeps its own ladder and only the top rung
	// differs: 0 / 5 / 10 / 15 against the local 0 / 5 / 10 / 25. Binary, from
	// resolve_received_invasion +0x12a4 (Real48 15 at the High branch).
	InterplanetaryCaptureHighPct = 15

	// And its own floor, which is NOT the local resolver's 15. Binary:
	// resolve_received_invasion +0x18a8 pushes 10 into max_i32 against the
	// computed share.
	InterplanetaryCaptureFloor = 10

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
	// The binary subtracts a flat 1 after the multiply — but in ITS units, where
	// a trooper is worth 0.5 and a turret 1. IB values everything at twice that,
	// so the same flat term is 2 here.
	//
	// This is NOT a scaling detail that cancels. The survival factor is a ratio
	// and does cancel; a flat subtraction does not, and halving the scale doubles
	// how much of a side each hit removes. Getting it wrong is invisible in a
	// large battle and decides a small one: a captured 3-jets-against-112-turrets
	// fight cost the defender 2 turrets, which a flat 1 at IB's scale renders as
	// 1 (TestCapturedJetsVersusTurrets).
	BattleRoundFlatLoss = 2
	BattleUpsetPct      = 5 // binary: Real48 0.05, the consolation roll
)

// The interplanetary battle runs the same attrition, with two differences read
// from its own resolver (BRE.OVR 0x03f4a0 +0x0647, reached from
// resolve_received_invasion at 0x040012).
//
// It has NO upset roll: each side is hit on its opponent's share of the two
// strengths and on nothing else (the hits at +0x0718 and +0x0802 follow a single
// compare each, where the local resolver tests twice).
//
// And it fights a SECOND battle in the same loop, in the air. The defender's
// jets are ground down on their own roll (+0x0744) against the attacking force's
// bombers weighted by RemoteJetBomberWeight, and the fraction that survives is
// applied to jets ALONE — the caller multiplies the ground fraction into
// troopers, turrets and tanks (+0x76, +0x82, +0x86) and the air fraction into
// jets (+0x7e). So a strike carrying no bombers costs the defender no jets
// however hard it presses, and bombers cost the defender nothing but jets.
const (
	// binary: Real48 5.0, multiplying the bomber count in the air roll's
	// denominator (BRE.OVR +0x074f).
	RemoteJetBomberWeight = 5
)

// --- Attack Costs / Terrorist Costs levels (BINARY-VERIFIED) ---
//
// The sysop's two cost Levels scale what an interplanetary strike and a
// terrorist op charge. They do NOT use the even 0/50/100/200 ladder the other
// Level knobs were once all read through: BRE keeps its own spread for these
// two, and High TRIPLES the price while Low cuts it to a fifth.
//
// Read from BRE.OVR two independent ways, agreeing exactly. The attack site
// (0x2bbc2, config byte 0x182) branches on the level and then divides the price
// by Real48 5.0 or multiplies it by 3.0; the terrorist pricing routine
// (0x2ad9f, config byte 0x184) states the same spread as literal percents —
// 100, 0, 20, 300 — and a sibling site (0x2ad1a) repeats the ÷5 / ×3 form on
// longints. BRE's own encoding of the byte is Medium 0, None 1, Low 2, High 3,
// which is what ties each figure to its level.
//
// Fidelity contract, not playtest knobs.
const (
	CostLevelNonePct   = 0
	CostLevelLowPct    = 20
	CostLevelMediumPct = 100
	CostLevelHighPct   = 300

	// AttackCostCap is the ceiling BRE clamps an interplanetary strike's gold
	// price to before quoting it (BRE.OVR 0x2bc45, a Real48 2e8 compared against
	// the computed price and substituted for it when the price is larger).
	AttackCostCap int64 = 200_000_000

	// TerrorOpGoldPerRegion is the minimum per-region cost of a terrorist op
	// (64 gold), confirmed against four captured menu prices. The full
	// BINARY-VERIFIED formula (docs/mechanics-reference.md) is:
	//
	//	capped := clamp(terrorOpsToday, 1, 100)
	//	cost   := (capped + TerrorOpGoldPerRegion - 1) * totalRegions * configMult
	//
	// For opsToday ≤ 1 this yields TerrorOpGoldPerRegion per region; each
	// additional op that day raises it by 1, up to 163 at the cap.
	TerrorOpGoldPerRegion int64 = 64
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
