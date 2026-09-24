package game

import "math"

// Loan is a term-based Cash Relief loan (#40): Principal gold was borrowed and
// Owed gold (Principal compounded at the term's daily rate) is due on DueDay. It
// mirrors Investment. On DueDay Owed moves into Empire.Debt, which the bank then
// collects a turn at a time (matureLoans).
type Loan struct {
	Principal int64
	Owed      int64
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
func LoanTotalOwed(amount int64, days int) int64 {
	v := float64(amount) * loanFactor(days)
	if v > float64(MoneyCapMax) {
		return MoneyCapMax
	}
	return int64(v)
}

// LoanOverallTenths is the total interest over the whole term in tenths of a
// percent (compound), rounded, for the "totalling Y% overall" line: 2d→175
// (17.5%), 5d→539 (53.9%), 10d→1594 (159.4%).
func LoanOverallTenths(days int) int {
	return int(math.Round((loanFactor(days) - 1) * 1000))
}

// LoansOwed is the total an empire owes across its active term loans.
func (e *Empire) LoansOwed() int64 {
	var total int64
	for _, l := range e.Loans {
		total += l.Owed
	}
	return total
}

// LoanCeiling is the most gold e may borrow over a `days` term — BRE's "We will
// provide up to N gold." The term is an argument because the bank sizes what you
// will OWE at maturity, not what you take now: the headroom is divided by the
// same compound factor the debt will grow by, so asking for ten days offers less
// than asking for one. See balance.go for the binary provenance.
func (w *World) LoanCeiling(e *Empire, days int) int64 {
	nw := int64(w.NetWorth(e))
	if nw > LoanCeilingNetWorthCap {
		nw = LoanCeilingNetWorthCap
	}
	base := nw*LoanCeilingMultiple - e.LoansOwed() - e.Debt
	// BRE clamps against a flat 2,000,000,000 less the realm's outstanding
	// balance. IB clamps against the sysop's money cap instead, which is the same
	// figure by default and stays right when they raise it.
	if cap := w.MoneyCap(); base > cap {
		base = cap
	}
	if base <= 0 {
		return 0
	}
	return int64(float64(base) / loanFactor(days))
}

// TakeLoan borrows `amount` for `days` days (clamped to [LoanMinDays,
// LoanMaxDays]), crediting the gold now and recording the compounded amount due
// on DueDay. Returns the new loan, or ErrCantAfford if `amount` exceeds the
// current ceiling.
func (w *World) TakeLoan(e *Empire, amount int64, days int) (Loan, error) {
	if days < LoanMinDays {
		days = LoanMinDays
	}
	if days > LoanMaxDays {
		days = LoanMaxDays
	}
	if amount <= 0 {
		return Loan{}, nil
	}
	if amount > w.LoanCeiling(e, days) {
		return Loan{}, ErrCantAfford
	}
	l := Loan{Principal: amount, Owed: LoanTotalOwed(amount, days), DueDay: w.GameDay + days}
	w.creditGold(e, amount, "a loan")
	e.Loans = append(e.Loans, l)
	return l, nil
}

// matureLoans moves e's loans that have reached their due day into Debt, the
// amount the bank is collecting now, after growing whatever was left unpaid
// from the day before. BINARY-VERIFIED (run_daily_maintenance, BRE.OVR
// 0x8e0c-0x8fe1): BRE keeps loans in day slots, grows slot 0 — today's unpaid
// remainder — by loanOverdueTenths, then folds tomorrow's slot into it. There
// is no default, no penalty and no loss of support: an unpaid loan simply keeps
// growing and keeps being collected. The installment for the new day is set
// here as well.
func (w *World) matureLoans(e *Empire) {
	if e.Debt > 0 {
		e.Debt = w.growOverdue(e.Debt)
	}
	var remaining []Loan
	for _, l := range e.Loans {
		if w.GameDay < l.DueDay {
			remaining = append(remaining, l)
			continue
		}
		e.Debt = min(e.Debt+l.Owed, w.MoneyCap())
	}
	e.Loans = remaining
	e.LoanInstallment = max(e.Debt/int64(max(w.Config.TurnsPerDay, 1)), LoanMinInstallment)
}

// loanOverdueTenths is the daily growth of an unpaid loan, in tenths of a
// percent: the higher of the investment and savings rates plus
// LoanOverdueExtraTenths (11.0% a day on a board at 5.0%).
func (w *World) loanOverdueTenths() int {
	return max(w.InvestRate, w.Config.InterestRate) + LoanOverdueExtraTenths
}

// growOverdue returns an unpaid loan balance after one day's growth, held at the
// money cap. BRE multiplies in Turbo Pascal reals and truncates, so it can land
// one gold lower than this where the exact product is a whole number (its
// 1.11 is not exact in binary). The product is split around the thousand so it
// cannot overflow int64. BRE has no ceiling to copy: its Trunc raises a runtime
// error past 2^31, so IB holds the balance at the money cap, as it holds gold.
func (w *World) growOverdue(debt int64) int64 {
	f := int64(1000 + w.loanOverdueTenths())
	grown := debt/1000*f + debt%1000*f/1000
	return min(grown, w.MoneyCap())
}

// collectLoanInstallment takes one turn's loan payment from gold in hand: the
// installment set at maintenance, no more than is owed and no more than the
// realm holds. BINARY-VERIFIED (process_economic_production, BRE.OVR 0x34ca3):
// the bank never reaches into savings, and a realm with no gold simply pays
// nothing that turn. Returns the amount paid.
func (w *World) collectLoanInstallment(e *Empire) int64 {
	if e.Debt <= 0 || e.LoanInstallment <= 0 {
		return 0
	}
	pay := min(e.Debt, e.LoanInstallment, e.Gold)
	if pay <= 0 {
		return 0
	}
	e.Gold -= pay
	e.Debt -= pay
	return pay
}
