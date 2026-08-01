package game

import (
	"fmt"
	"math"
)

// Loan is a term-based Cash Relief loan (#40): Principal gold was borrowed and
// Owed gold (Principal compounded at the term's daily rate) is due on DueDay. It
// mirrors Investment. Unpaid at DueDay, the remainder rolls into open-ended Debt.
type Loan struct {
	Principal int
	Owed      int
	DueDay    int
}

// LoanRateTenths is a loan's daily interest rate in tenths of a percent for a
// term of `days` days. The rate rises with the term (BRE-verified live: 2d→8.4,
// 5d→9.0, 10d→10.0 %/day).
func LoanRateTenths(days int) int {
	return LoanBaseRateTenths + LoanRatePerDayTenths*days
}

// loanFactor is the compound multiplier (1 + daily_rate)^days for a term. BRE
// uses floating point ("TP reals"), so IB matches it — int-iterative truncation
// drifts low over a 10-day term (500@10d would give 1292, not BRE's 1296).
func loanFactor(days int) float64 {
	return math.Pow(1+float64(LoanRateTenths(days))/1000, float64(days))
}

// LoanTotalOwed is the amount to repay for borrowing `amount` for `days` days:
// the daily rate compounded daily, truncated once at the end. Matches BRE
// (verified: 1000@2d=1175, 616@5d=947, 500@10d=1296).
func LoanTotalOwed(amount, days int) int {
	v := float64(amount) * loanFactor(days)
	if v > float64(MoneyCap) {
		return MoneyCap
	}
	return int(v)
}

// LoanOverallTenths is the total interest over the whole term in tenths of a
// percent (compound), rounded, for the "totalling Y% overall" line: 2d→175
// (17.5%), 5d→539 (53.9%), 10d→1594 (159.4%).
func LoanOverallTenths(days int) int {
	return int(math.Round((loanFactor(days) - 1) * 1000))
}

// LoansOwed is the total an empire owes across its active term loans.
func (e *Empire) LoansOwed() int {
	total := 0
	for _, l := range e.Loans {
		total += l.Owed
	}
	return total
}

// LoanCeiling is the most gold e may borrow right now — BRE's "We will provide up
// to N gold." IB reconstruction: a multiple of net worth, less everything already
// owed (active loans + defaulted debt). BRE's exact formula is unverified.
func (w *World) LoanCeiling(e *Empire) int {
	return max(0, w.NetWorth(e)*LoanCeilingMultiple-e.LoansOwed()-e.Debt)
}

// TakeLoan borrows `amount` for `days` days (clamped to [LoanMinDays,
// LoanMaxDays]), crediting the gold now and recording the compounded amount due
// on DueDay. Returns the new loan, or ErrCantAfford if `amount` exceeds the
// current ceiling.
func (w *World) TakeLoan(e *Empire, amount, days int) (Loan, error) {
	if days < LoanMinDays {
		days = LoanMinDays
	}
	if days > LoanMaxDays {
		days = LoanMaxDays
	}
	if amount <= 0 {
		return Loan{}, nil
	}
	if amount > w.LoanCeiling(e) {
		return Loan{}, ErrCantAfford
	}
	l := Loan{Principal: amount, Owed: LoanTotalOwed(amount, days), DueDay: w.GameDay + days}
	e.Gold += amount
	if e.Gold > MoneyCap {
		e.Gold = MoneyCap
	}
	e.Loans = append(e.Loans, l)
	return l, nil
}

// matureLoans settles any of e's loans that have reached their due day — the
// amount owed is taken from gold first, then the bank (mirroring how
// matureInvestments pays out matured investments). A loan that can't be paid in
// full DEFAULTS: the unpaid remainder rolls into open-ended Debt grown by the
// late-payment penalty, popular support takes a hit, and the owner is told.
func (w *World) matureLoans(e *Empire) {
	var remaining []Loan
	for _, l := range e.Loans {
		if w.GameDay < l.DueDay {
			remaining = append(remaining, l)
			continue
		}
		owed := l.Owed
		if pay := min(owed, e.Gold); pay > 0 {
			e.Gold -= pay
			owed -= pay
		}
		if pay := min(owed, e.Bank); pay > 0 {
			e.Bank -= pay
			owed -= pay
		}
		if owed > 0 {
			e.Debt += owed + owed*LoanDefaultPenaltyPct/100
			e.adjustSupport(-LoanDefaultSupportDrop)
			e.addEvent(fmt.Sprintf("A Cash Relief loan came due and you could not repay %d gold — it was added to your debt with a %d%% penalty.", owed, LoanDefaultPenaltyPct))
		}
	}
	e.Loans = remaining
}
