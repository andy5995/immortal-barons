package game

import (
	"fmt"
	"math"

	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// Invest locks `amount` gold for `days` days (clamped to [MinInvestDays,
// MaxInvestDays]) and records a maturing return at the current InvestRate.
// Returns the expected return, or an error if the amount is unaffordable.
//
// One investment is capped at MaxInvestment however much gold is on hand; the
// menu offers no more than that, and this is the rule a direct caller meets.
// There is no limit on how MANY investments a baron opens.
func (w *World) Invest(e *Empire, amount int64, days int) (int64, error) {
	if days < MinInvestDays {
		days = MinInvestDays
	}
	if days > MaxInvestDays {
		days = MaxInvestDays
	}
	if amount <= 0 {
		return 0, nil
	}
	amount = min(amount, MaxInvestment)
	if e.Gold < amount {
		return 0, ErrCantAfford
	}
	ret := ExpectedReturn(amount, w.InvestRate, days)
	e.Gold -= amount
	e.Investments = append(e.Investments, Investment{Amount: amount, Return: ret, MaturesDay: w.GameDay + days})
	return ret, nil
}

// ExpectedReturn is the total payout (principal + interest) for investing
// `amount` for `days` days at `rate` TENTHS of a percent per day, COMPOUNDED
// daily — matching BRE (live-verified: 1000 for 2 days at 5%/day returns
// 1102 = 1000·1.05²). BRE computes this in floating point ("TP reals") and
// truncates once at the end, so IB does the same (int-iterative truncation would
// drift low over a long term).
func ExpectedReturn(amount int64, rate, days int) int64 {
	v := float64(amount) * math.Pow(1+float64(rate)/1000, float64(days))
	if v > float64(MoneyCapMax) {
		return MoneyCapMax
	}
	return int64(v)
}

// PctTenths renders a tenths-of-a-percent figure for display: 84 -> "8.4",
// 1594 -> "159.4". The bank's rates — savings, investing and loan — are all held
// in tenths, so they all print through this.
func PctTenths(t int) string { return fmt.Sprintf("%d.%d", t/10, t%10) }

// PendingInvested is the total principal an empire has locked in investments.
func (w *World) PendingInvested(e *Empire) int64 {
	var total int64
	for _, inv := range e.Investments {
		total += inv.Amount
	}
	return total
}

// matureInvestments pays out any of e's investments that have reached their
// maturity day, crediting Return to gold (clamped to MoneyCap) and removing
// them. Returns the total paid out this call.
func (w *World) matureInvestments(e *Empire) int64 {
	var paid int64
	var remaining []Investment
	for _, inv := range e.Investments {
		if w.GameDay >= inv.MaturesDay {
			before := e.Gold
			w.creditGold(e, inv.Return, "a matured investment")
			paid += e.Gold - before
		} else {
			remaining = append(remaining, inv)
		}
	}
	e.Investments = remaining
	return paid
}

// Investment-rate drift (v1 tunables, in the same tenths-of-a-percent unit as
// InvestRate). Total gold invested across all empires above the threshold pushes
// the rate down (heavy demand for the bank's gold); below it, the rate drifts
// up. The nudge is BRE's stated half point; the small random drift on top is the
// clone's stand-in for the original's occasional inflation events.
const (
	investRateHeavyThreshold = 5_000_000
	investRateNudgeTenths    = 5
	investRateDriftTenths    = 2
)

// steadyInvestRate is the fixed daily rate the league's Standard Investment Rate
// knob sets, clamped to the engine's [MinInvestRate, MaxInvestRate] band. The
// knob is already in tenths of a percent per day — BRE words it as the return
// over ten days, which is the same figure.
func (w *World) steadyInvestRate() int {
	return min(max(w.Config.StdInvestRate, MinInvestRate), MaxInvestRate)
}

// adjustInvestRate nudges the floating rate: heavy total investing across all
// empires pushes it down, light investing pushes it up, plus a small random
// drift; clamped to [MinInvestRate, MaxInvestRate]. With Steady Investment Rate
// on, the rate is instead pinned to the league's standard rate and never
// floats.
func (w *World) adjustInvestRate() {
	before := w.InvestRate
	if w.Config.SteadyInvest {
		w.InvestRate = w.steadyInvestRate()
		w.postInvestRateNews(before)
		return
	}
	var total int64
	for _, e := range w.Empires {
		if e.Alive {
			total += w.PendingInvested(e)
		}
	}
	if total > investRateHeavyThreshold {
		w.InvestRate -= investRateNudgeTenths
	} else {
		w.InvestRate += investRateNudgeTenths
	}
	w.InvestRate += w.rng.Intn(2*investRateDriftTenths+1) - investRateDriftTenths
	w.InvestRate = min(max(w.InvestRate, MinInvestRate), MaxInvestRate)
	w.postInvestRateNews(before)
}

// creditGold pays n gold into e's gold in hand, holding it at the money cap and
// telling the owner when the ceiling ate part of the payment. `source` names
// where the gold came from, so the recap says WHAT was lost rather than only
// that something was.
//
// Every path that pays gold in goes through this. Silent destruction is what
// made the old hard-coded 2-billion limit read as a bug rather than a rule: a
// baron watched the number stop growing with nothing on screen to explain it.
// Withdraw is the deliberate exception — it draws only what fits and leaves the
// remainder safely in the bank, so nothing is ever lost there to report.
func (w *World) creditGold(e *Empire, n int64, source string) {
	if n <= 0 {
		return
	}
	e.Gold += n
	over := e.Gold - w.MoneyCap()
	if over <= 0 {
		return
	}
	e.Gold = w.MoneyCap()
	e.addEvent(fmt.Sprintf("You cannot hold more than %s gold in hand — %s gold from %s was lost.",
		numfmt.Comma(w.MoneyCap()), numfmt.Comma(over), source))
}
