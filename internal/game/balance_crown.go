package game

// balance_crown.go — the Queen Royale's tax refund and lottery.
// Split out of balance.go; the provenance rules in that file's header
// apply here too, and each section below carries its own.

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

// The Planetary Master's daily share of the same purse.
//
// BINARY-VERIFIED (BRE.OVR 0x007aeb, update_planet_title, called only from
// run_daily_maintenance): the title is settled once a day by net worth, and the
// holder is paid pool/100, which is then subtracted from the pool. Uncapped and
// ungated — the protection cap above applies to the refund only.
const MasterAwardPct = 1 // percent of the Queen's purse, paid daily to the Master

// --- The Queen's lottery ---
//
// A ticket is offered once a game day, in the same first-play event block as
// the tax refund above and immediately after it. The player picks six letters,
// six are drawn, and the prize is paid by how many of the six drawn letters the
// ticket covers.
//
// BINARY-VERIFIED (BRE.OVR 0x018610, run_lottery, called from BRE.EXE 0x038a2):
// the ticket price, the alphabet, the six-letter ticket, and every prize below
// are literals in that routine. The 6-letter prize is 0x00989680 = 10,000,000 —
// a hundred million is a figure that circulates among players and is not in the
// binary.
//
// The price is charged the moment the offer is accepted and is never named on
// screen, which is BRE's behavior and not an oversight here. The offer is
// withheld entirely from a realm that cannot pay it.
const (
	LotteryTicketPrice = 5_000 // charged on "yes", never displayed
	LotteryLetters     = 6     // letters on a ticket, and letters drawn
	LotteryAlphabet    = 26    // 'A'..'Z', uppercase only
)

// LotteryPrizes is the payout by number of matched letters, indexed 0..6.
// Binary-verified alongside the constants above; all seven are golden figures,
// not playtest knobs.
var LotteryPrizes = [LotteryLetters + 1]int64{
	0,
	2_500,
	10_000,
	500_000,
	1_000_000,
	4_000_000,
	10_000_000,
}

// The crown purse's OTHER outlet: an occasional planet-wide handout.
//
// BINARY-VERIFIED (`write_economic_policy_news`, BRE.OVR 0x04F082), tail-called
// from `process_end_of_turn` behind `Random(100) == 0` — about one player turn
// in a hundred. It then rolls `Random(6)` and runs one of six crown events; two
// of the six pay out of the purse and are the ones below. (The other four are
// the investment-rate nudges and the crown-tax random walk; IB's investment
// drift already stands in for the first pair, and the tax walk is a separate
// question because IB holds that rate as a whole-percent sysop knob.)
//
// Both are shares of the WHOLE purse rather than per-realm allowances, so a
// large league drains it faster than a small one — that asymmetry is the
// original's, not an oversight.
const (
	// CrownEventChancePct is how often any crown event fires, per turn.
	CrownEventChancePct = 1
	// CrownEventKinds is the number of crown events rolled between (Random(6)).
	CrownEventKinds = 6
	// CrownGoldRoll and CrownTrooperRoll are which of those rolls pay out.
	CrownGoldRoll    = 5
	CrownTrooperRoll = 0

	// CrownGoldDivisor: every living realm is paid trunc(purse/50), and the
	// purse is charged that once PER REALM (`mov cx,0x32` at +0x198d, the share
	// computed once before the loop).
	CrownGoldDivisor = 50

	// The trooper handout buys troopers out of the purse at CrownTrooperPrice
	// gold each: the share is trunc(trunc(purse/1000)/livingRealms), so the
	// division by 1000 IS the price (`mov cx,0x3e8` at +0x1683, then a second
	// divide by the realm count at +0x168f). Capped at CrownTrooperMax
	// (`0x61a8` at +0x16a2) and skipped entirely when it comes out at zero.
	CrownTrooperPrice = 1000
	CrownTrooperMax   = 25_000
)
