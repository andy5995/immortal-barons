package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// money wraps a bank action that moves a gold amount, offering max as the
// largest sensible value for that action (e.g. Withdraw's max is p.Bank).
func money(label string, max func(*game.Empire) int, apply func(*game.World, *game.Empire, int) error) Action {
	return func(s session.Session, w *ctx) Result {
		p := w.Player()
		n := promptSuggested(s, label+" how much gold?", 0, max(p))
		if n <= 0 {
			return Stay
		}
		var err error
		w.With(func() {
			// apply (Deposit/Withdraw/Loan/Repay) re-checks the balance/debt and
			// the MoneyCap against the reloaded empire, so a concurrent node can't
			// let two sessions withdraw the same funds or overdraw.
			p := w.Player()
			if p == nil {
				err = errRealmChanged
				return
			}
			err = apply(w.World, p, n)
		})
		if err != nil {
			fail(s, err)
		}
		// On success there's no confirmation line or pause: the Bank menu redraws
		// with the updated Gold/Bank in its status footer (like the Spending buys).
		return Stay
	}
}

// investFunds prompts for a term (days) and amount, shows the expected
// return, and locks the gold via w.Invest.
func investFunds(s session.Session, w *ctx) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n"+tr(s, "Current investment rate: %d%% per day.")+"\n", w.InvestRate)
	days := promptInt(s, "Invest for how many days?")
	if days < game.MinInvestDays {
		days = game.MinInvestDays
	}
	amount := promptSuggested(s, "How much to invest?", 0, p.Gold)
	if amount <= 0 {
		return Stay
	}
	expected := game.ExpectedReturn(amount, w.InvestRate, days)
	fmt.Fprintf(s, "\n  "+tr(s, "Expected return: ~%d")+"\n", expected)
	var ret, matureDay int
	var err error
	w.With(func() {
		p := w.Player()
		if p == nil {
			err = errRealmChanged
			return
		}
		ret, err = w.World.Invest(p, amount, days) // re-checks affordability atomically
		matureDay = w.GameDay + days
	})
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "Invested %d for %d days; ~%d returns on day %d.", amount, days, ret, matureDay)
	}
	return Stay
}

// listInvestments shows the player's pending investments and any debt.
func listInvestments(s session.Session, w *ctx) Result {
	p := w.Player()
	var investments []game.Investment
	var debt int
	w.With(func() {
		investments = append([]game.Investment(nil), p.Investments...)
		debt = p.Debt
	})
	if len(investments) == 0 && debt == 0 {
		fmt.Fprintf(s, "\n%s\n", tr(s, "You have no active investments or loans."))
		pause(s)
		return Stay
	}
	if len(investments) > 0 {
		fmt.Fprintf(s, "\n  %s\n", tr(s, "Amount      Return    Matures Day"))
		for _, inv := range investments {
			fmt.Fprintf(s, "  %-10d  %-8d  %d\n", inv.Amount, inv.Return, inv.MaturesDay)
		}
	}
	if debt > 0 {
		fmt.Fprintf(s, "\n  "+tr(s, "Debt owed: %d")+"\n", debt)
	}
	pause(s)
	return Stay
}

// bankRates shows the current savings and investment rates.
func bankRates(s session.Session, w *ctx) Result {
	fmt.Fprintf(s, "\n  "+tr(s, "Savings interest: ~1%% per game day.")+"\n  "+tr(s, "Investment rate: %d%% per day.")+"\n", w.InvestRate)
	pause(s)
	return Stay
}
