package game

import (
	"errors"
	"fmt"
	"math"

	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// ErrInvestDateFull refuses an investment whose return would push the returns
// maturing on its date past MaxReturnsPerDate.
var ErrInvestDateFull = errors.New("That is more than may be invested for that date.")

// clampInvestDays holds a term to [MinInvestDays, MaxInvestDays].
func clampInvestDays(days int) int { return min(max(days, MinInvestDays), MaxInvestDays) }

// Invest locks `amount` gold for `days` days (clamped to [MinInvestDays,
// MaxInvestDays]) and records a maturing return at the current InvestRate.
// Returns the expected return, or an error if the amount is unaffordable or
// more than MaxInvestPrincipal allows for that date.
func (w *World) Invest(e *Empire, amount int64, days int) (int64, error) {
	days = clampInvestDays(days)
	if amount <= 0 {
		return 0, nil
	}
	if amount > w.MaxInvestPrincipal(e, days) {
		return 0, ErrInvestDateFull
	}
	if e.Gold < amount {
		return 0, ErrCantAfford
	}
	ret := ExpectedReturn(amount, w.InvestRate, days)
	e.Gold -= amount
	e.Investments = append(e.Investments, Investment{Amount: amount, Return: ret, MaturesDay: w.GameDay + days})
	return ret, nil
}

// MaxInvestPrincipal is the most gold e may invest now for `days` days: the
// principal whose return fills what is left below MaxReturnsPerDate once the
// returns already maturing on that date are counted, truncated. A full date
// gives 0. The ceiling is on the date, not on one investment, as in BRE —
// matched to the gold against two captured prompts (bank_test.go).
func (w *World) MaxInvestPrincipal(e *Empire, days int) int64 {
	days = clampInvestDays(days)
	due := w.GameDay + days
	room := MaxReturnsPerDate
	for _, inv := range e.Investments {
		if inv.MaturesDay == due {
			room -= inv.Return
		}
	}
	if room <= 0 {
		return 0
	}
	return int64(float64(room) / investGrowth(w.InvestRate, days))
}

// investGrowth is what one gold invested for `days` days at `rate` tenths of a
// percent per day, compounded daily, grows to.
func investGrowth(rate, days int) float64 {
	return math.Pow(1+float64(rate)/1000, float64(days))
}

// ExpectedReturn is the total payout (principal + interest) for investing
// `amount` for `days` days at `rate` TENTHS of a percent per day, COMPOUNDED
// daily — matching BRE (live-verified: 1000 for 2 days at 5%/day returns
// 1102 = 1000·1.05²). BRE computes this in floating point ("TP reals") and
// truncates once at the end, so IB does the same (int-iterative truncation would
// drift low over a long term).
func ExpectedReturn(amount int64, rate, days int) int64 {
	v := float64(amount) * investGrowth(rate, days)
	if v > float64(MoneyCapMax) {
		return MoneyCapMax
	}
	return int64(v)
}

// PctTenths renders a tenths-of-a-percent figure for display: 84 -> "8.4",
// 1594 -> "159.4". The bank's rates — savings, investing and loan — are all held
// in tenths, so they all print through this.
func PctTenths(t int) string { return fmt.Sprintf("%d.%d", t/10, t%10) }

// matureInvestments opens a new day's investment payout for e. BINARY-VERIFIED
// (run_daily_maintenance, BRE.OVR 0x8bbd-0x8f89): what yesterday's turns left
// unpaid goes into the hold, cut to whole thousands; the returns maturing
// today become InvestDue; the hold is poured back into InvestDue up to the
// per-date limit; and InvestShare, the part each turn pays, is InvestDue over
// the turns per day. Nothing is paid here — collectInvestShare pays it a turn
// at a time, so a day's turns left unplayed carry their shares into the hold.
func (w *World) matureInvestments(e *Empire) {
	e.InvestHeld += e.InvestDue / 1000 * 1000
	e.InvestDue = 0
	var remaining []Investment
	for _, inv := range e.Investments {
		if w.GameDay >= inv.MaturesDay {
			e.InvestDue += inv.Return
		} else {
			remaining = append(remaining, inv)
		}
	}
	e.Investments = remaining
	if room := MaxReturnsPerDate - e.InvestDue; room > 0 && e.InvestHeld > 0 {
		pour := min(e.InvestHeld, room)
		e.InvestDue += pour
		e.InvestHeld -= pour
	}
	e.InvestShare = e.InvestDue / int64(max(w.Config.TurnsPerDay, 1))
}

// collectInvestShare pays one turn's share of the day's matured returns into
// gold in hand (process_economic_production, BRE.OVR 0x34b83). Returns what
// reached gold in hand, which the money cap may have trimmed.
func (w *World) collectInvestShare(e *Empire) int64 {
	share := min(e.InvestShare, e.InvestDue)
	if share <= 0 {
		return 0
	}
	e.InvestDue -= share
	before := e.Gold
	w.creditGold(e, share, "a matured investment")
	return e.Gold - before
}

// steadyInvestRate is the fixed daily rate the league's Standard Investment Rate
// knob sets, clamped to the knob's own range. The knob is already in tenths of
// a percent per day — BRE words it as the return over ten days, which is the
// same figure.
func (w *World) steadyInvestRate() int {
	return min(max(w.Config.StdInvestRate, MinStdInvestRate), MaxStdInvestRate)
}

// investRateStep is the day's move in the investment rate, in tenths of a
// percent, for the returns due today averaged over the living realms in whole
// millions (InvestRateSteps). Past the last bracket it keeps the last step;
// BRE cannot get there, since no date may carry more than two billion.
func investRateStep(avgMillions int64) int {
	for _, s := range InvestRateSteps {
		if avgMillions <= s.UpToMillions {
			return s.Tenths
		}
	}
	return InvestRateSteps[len(InvestRateSteps)-1].Tenths
}

// adjustInvestRate moves the floating rate once a day. BINARY-VERIFIED
// (run_daily_maintenance, BRE.OVR 0x9008-0x92ab): each living realm's returns
// due today are cut to whole millions and averaged, the average picks a step
// (investRateStep), and two rails override it — below half the Standard rate
// the bank raises the rate InvestRateRailTenths, above one and a half times it
// the bank lowers it as much. No random step and no hard band: only a rate
// below zero is held at zero. With Steady Investment Rate on, the rate is
// pinned to the Standard rate and never floats.
func (w *World) adjustInvestRate() {
	before := w.InvestRate
	if w.Config.SteadyInvest {
		w.InvestRate = w.steadyInvestRate()
		w.postInvestRateNews(before)
		return
	}
	var millions, realms int64
	for _, e := range w.Empires {
		if e.Alive {
			millions += e.InvestDue / 1_000_000
			realms++
		}
	}
	step := investRateStep(millions / max(realms, 1))
	std := w.steadyInvestRate() // the knob held to its range, as the steady path holds it
	switch {
	case std/2 > w.InvestRate:
		step = InvestRateRailTenths
	case 2*w.InvestRate > 3*std: // rate > 1.5 x std, exact
		step = -InvestRateRailTenths
	}
	w.InvestRate = max(w.InvestRate+step, 0)
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

// CollectBankPayments settles the bank's per-turn business at the start of a
// turn, after the turn's income is in hand, which is where BRE does it
// (process_economic_production, after the income lines): a share of the day's
// matured investments is paid, then the day's loan installment is taken from
// gold. Both amounts are kept on the empire for the turn's income report.
func (w *World) CollectBankPayments(e *Empire) {
	e.InvestPaid = w.collectInvestShare(e)
	e.LoanPaid = w.collectLoanInstallment(e)
}
