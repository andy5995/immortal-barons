package game

// balance_networth.go — net-worth weights.
// Split out of balance.go; the provenance rules in that file's header
// apply here too, and each section below carries its own.

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

// Cash Relief loans (#40) — term-based borrowing: the daily rate rises with the
// term and compounds daily.
//
// The ceiling is BINARY-VERIFIED from `run_bank` (BRE.OVR 0x38648-0x38973).
// It had been an IB reconstruction, and a wrong one — the gathered points
// "would not fit" because the shape is a DISCOUNT, not a multiple:
//
//	base    = LoanCeilingMultiple x min(netWorth, LoanCeilingNetWorthCap)
//	base   -= everything already owed
//	ceiling = trunc( base / (1 + dailyRate)^days )
//
// So a longer term lowers what may be borrowed, because the bank is sizing what
// you will OWE at maturity rather than what you take now. Nothing in the earlier
// sampling could have recovered that without varying the term.
//
// The daily rate's per-term slope is verified live (2d->8.4, 5d->9.0,
// 10d->10.0 %/day, matching three loans). Its BASE is the one part still
// diverging: BRE computes `max(investRate, savingsRate) + 30` tenths rather
// than a constant, so a board whose sysop moved either rate gets a different
// loan rate. IB's 80 is that expression on a board sitting at 5.0%. See
// docs/mechanics-reference.md.
const (
	LoanMinDays          = 1 // BRE prompt "(1; 10)"
	LoanMaxDays          = 10
	LoanBaseRateTenths   = 80 // daily rate at term 0, in tenths of a % (8.0%/day) — see the note above
	LoanRatePerDayTenths = 2  // +0.2%/day of daily rate per term-day (2d→8.4, 5d→9.0, 10d→10.0 — verified)
	// BINARY-VERIFIED: net worth is capped before the multiply, so the richest
	// realm in the game borrows against the same headroom as a merely rich one.
	LoanCeilingMultiple    = 10         // × net worth, before the term discount
	LoanCeilingNetWorthCap = 10_000_000 // net worth counts no further than this
	LoanDefaultPenaltyPct  = 25         // an unpaid loan at its due date rolls into Debt grown by this % (IB's late-payment penalty)
	LoanDefaultSupportDrop = 10         // popular-support points lost when a loan defaults
)
