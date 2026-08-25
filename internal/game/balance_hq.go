package game

import "math"

// balance_hq.go — the HeadQuarters.
// Split out of balance.go; the provenance rules in that file's header
// apply here too, and each section below carries its own.

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

// An ARRIVING interplanetary strike is valued by a different builder, with its
// own two constants. Both are BINARY-VERIFIED, and the pair of routines is why
// this looks like a contradiction of the block above:
//
//	local          (resolve_regular_attack, BRE.OVR +0x0adb)  HQ/200 + 1.75
//	interplanetary (resolve_received_invasion,        +0x0f9d) HQ/100 + 1.5
//
// Doubled into trooper units those are (350 + HQ)/100 and (300 + 2*HQ)/100. They
// cross at HQ 50 and diverge either side of it, so a realm with no HeadQuarters
// defends an invasion WORSE than a neighbour's attack, and a finished one better.
//
// This is the reading the block above records as rejected — "300 rising to 500".
// It was not a misread, it was the other resolver's constant, and IB applied the
// local pair to both battles until 2026-08-24.
const (
	RemoteTankStrengthPctBase  = 300 // binary: Real48 1.5, doubled
	RemoteTankStrengthPctPerHQ = 2   // binary: HQ/100, doubled
)

// remoteTankStrength values n tanks in troopers for an arriving interplanetary
// strike, given HQ completion percent.
// Widened before the multiply: a realm can hold tens of millions of tanks, and
// `n * 500` passes 2^31 on the 32-bit door builds this project supports.
func remoteTankStrength(n, hq int) int {
	wide := int64(n) * int64(RemoteTankStrengthPctBase+RemoteTankStrengthPctPerHQ*hq) / 100
	return int(min(wide, math.MaxInt32))
}

// tankStrength values n tanks in troopers, given HQ completion percent.
func tankStrength(n, hq int) int {
	wide := int64(n) * int64(TankStrengthPctBase+TankStrengthPctPerHQ*hq) / 100
	return int(min(wide, math.MaxInt32))
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
