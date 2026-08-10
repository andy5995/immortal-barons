package menu

import (
	"fmt"
	"sort"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// money wraps a bank action that moves a gold amount, offering max as the
// largest sensible value for that action (e.g. Withdraw's max is p.Bank).
func money(label string, max func(*game.Empire) int64, apply func(*game.World, *game.Empire, int64) error) Action {
	return func(s session.Session, w *ctx) Result {
		p := w.Player()
		n := promptSuggested(s, label+" how many gold?", 0, max(p))
		if n <= 0 {
			return Stay
		}
		// apply (Deposit/Withdraw/Loan/Repay) re-checks the balance/debt and the
		// MoneyCap against the reloaded empire, so a concurrent node can't let two
		// sessions withdraw the same funds or overdraw.
		err := w.mutatePlayer(func(p *game.Empire) error {
			return apply(w.World, p, n)
		})
		if err != nil {
			fail(s, err)
		}
		// On success there's no confirmation line or pause: the Bank menu redraws
		// with the updated Gold/Bank in its status footer (like the Spending buys).
		return Stay
	}
}

// cashRelief runs BRE's Cash Relief / Loans flow: settle any overdue debt, then
// borrow. A loan picks a repayment term (days), shows the daily rate and the
// overall interest, offers up to the ceiling, and owes the compounded total on
// its due date. Defaulting at the due date is handled by matureLoans (the
// shortfall rolls into Debt with a penalty).
func cashRelief(s session.Session, w *ctx) Result {
	p := w.Player()
	// Overdue debt (from a defaulted loan) is settled here too — Cash Relief covers
	// both borrowing and paying down what you already owe.
	if p.Debt > 0 {
		fmt.Fprintf(s, "\n%s\n", hiNums(fmt.Sprintf(tr(s, "You owe %s gold in overdue debt (it grows each turn)."), comma(p.Debt))))
		if AskYesNo(s, "Repay some now?", false) {
			if n := promptSuggested(s, "How much to repay?", 0, min(p.Gold, p.Debt)); n > 0 {
				if err := w.mutatePlayer(func(p *game.Empire) error { return w.World.Repay(p, n) }); err != nil {
					fail(s, err)
				}
			}
		}
	}
	days := promptSuggested(s, "When do you wish to pay your loan back [# of Days]?", game.LoanMinDays, game.LoanMaxDays)
	if days < game.LoanMinDays {
		days = game.LoanMinDays
	}
	fmt.Fprintf(s, "\n%s\n", hiNums(fmt.Sprintf(tr(s, "The loan rate will be %s%% per day, totalling %s%% interest overall."),
		tenthsPct(game.LoanRateTenths(days)), tenthsPct(game.LoanOverallTenths(days)))))
	ceiling := w.LoanCeiling(p)
	fmt.Fprintf(s, "%s\n", hiNums(fmt.Sprintf(tr(s, "We will provide up to %s gold."), comma(ceiling))))
	amount := promptSuggested(s, "How many would you like to borrow?", 0, ceiling)
	if amount <= 0 {
		return Stay
	}
	var owed int64
	err := w.mutatePlayer(func(p *game.Empire) error {
		l, e := w.World.TakeLoan(p, amount, days)
		owed = l.Owed
		return e
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n  %s\n", hiNums(fmt.Sprintf(tr(s, "You owe %s gold in %d Days."), comma(owed), days)))
	okNoPause(s, "Note that late payments will incur additional penalties.")
	return Stay
}

// tenthsPct renders a tenths-of-a-percent value as "N.N" (84 -> "8.4").
func tenthsPct(t int) string { return fmt.Sprintf("%d.%d", t/10, t%10) }

// investFunds prompts for a term (days) and amount, shows the expected return,
// and locks the gold via w.Invest.
func investFunds(s session.Session, w *ctx) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%s\n", hiNums(fmt.Sprintf(tr(s, "The current interest returns on investments are %d%%."), w.InvestRate)))
	fmt.Fprintf(s, "%s\n", hiNums(fmt.Sprintf(tr(s, "There is a %d-day minimum on investments."), game.MinInvestDays)))
	days := promptSuggested(s, "How many days would you like to invest for?", game.MinInvestDays, game.MaxInvestDays)
	if days < game.MinInvestDays {
		days = game.MinInvestDays
	}
	amount := promptSuggested(s, "How much would you like to invest?", 0, min(p.Gold, game.MaxInvestment))
	if amount <= 0 {
		return Stay
	}
	expected := game.ExpectedReturn(amount, w.InvestRate, days)
	fmt.Fprintf(s, "\n  %s\n", hiNums(fmt.Sprintf(tr(s, "Returns expected to be approximately %s gold."), comma(expected))))
	if !AskYesNo(s, "Accept?", true) {
		return Stay
	}
	var matureDay int
	err := w.mutatePlayer(func(p *game.Empire) error {
		_, e := w.World.Invest(p, amount, days) // re-checks affordability atomically
		matureDay = w.GameDay + days
		return e
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	okNoPause(s, "Investment will be returned on %s.", w.DateForDay(matureDay))
	return Stay
}

// listInvestments shows the player's pending investments and loans in BRE's
// combined "Date / Investments / Loans Due" table — a row per maturity/due date
// (sorted), the maturing investment total and the loan total owed on that date.
// Any defaulted, open-ended debt is shown separately below (it has no due date).
func listInvestments(s session.Session, w *ctx) Result {
	var invs []game.Investment
	var loans []game.Loan
	var debt int64
	w.With(func() {
		p := w.Player()
		if p == nil {
			return
		}
		invs = append([]game.Investment(nil), p.Investments...)
		loans = append([]game.Loan(nil), p.Loans...)
		debt = p.Debt
	})
	if len(invs) == 0 && len(loans) == 0 && debt == 0 {
		ok(s, "You have no active investments or loans.")
		return Stay
	}
	// Aggregate by day: maturing investment returns and loan amounts due.
	type row struct{ inv, loan int64 }
	byDay := map[int]*row{}
	var days []int
	at := func(day int) *row {
		r, ok := byDay[day]
		if !ok {
			r = &row{}
			byDay[day] = r
			days = append(days, day)
		}
		return r
	}
	for _, inv := range invs {
		at(inv.MaturesDay).inv += inv.Return
	}
	for _, l := range loans {
		at(l.DueDay).loan += l.Owed
	}
	sort.Ints(days)

	fmt.Fprintf(s, "\n  %s%-12s %-20s %s%s\n", ansi.FgBrightCyan, tr(s, "Date"), tr(s, "Investments"), tr(s, "Loans Due"), ansi.Reset)
	dollar := func(n int64) string {
		if n <= 0 {
			return ""
		}
		return "$" + comma(n)
	}
	for _, day := range days {
		r := byDay[day]
		fmt.Fprintf(s, "  %-12s %s%-20s %s%s\n", w.DateForDay(day), ansi.FgBrightWhite, dollar(r.inv), dollar(r.loan), ansi.Reset)
	}
	if debt > 0 {
		fmt.Fprintf(s, "\n  %s\n", hiNums(fmt.Sprintf(tr(s, "Overdue debt: $%s (grows each turn until repaid)"), comma(debt))))
	}
	pause(s)
	return Stay
}

// bankRates shows the daily savings and investing rates BRE-style. Savings is the
// Interest Rate knob read as a daily percent (config/10, e.g. 50 → 5.0%);
// investing is the current floating rate.
func bankRates(s session.Session, w *ctx) Result {
	fmt.Fprintf(s, "\n  %-25s%s%s%%%s\n", tr(s, "Savings Interest Rate:"), ansi.FgBrightYellow, tenthsPct(w.Config.InterestRate), ansi.Reset)
	fmt.Fprintf(s, "  %-25s%s%s%%%s\n", tr(s, "Investing Interest Rate:"), ansi.FgBrightYellow, tenthsPct(w.InvestRate*10), ansi.Reset)
	pause(s)
	return Stay
}
