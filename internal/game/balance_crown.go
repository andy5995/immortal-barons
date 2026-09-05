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
// screen, which is BRE's behaviour and not an oversight here. The offer is
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
