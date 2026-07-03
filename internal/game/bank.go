package game

// Invest locks `amount` gold for `days` days (>= MinInvestDays) and records a
// maturing return at the current InvestRate (simple interest). Returns the
// expected return, or an error if the amount is unaffordable.
func (w *World) Invest(e *Empire, amount, days int) (int, error) {
	if days < MinInvestDays {
		days = MinInvestDays
	}
	if amount <= 0 {
		return 0, nil
	}
	if e.Gold < amount {
		return 0, ErrCantAfford
	}
	ret := ExpectedReturn(amount, w.InvestRate, days)
	e.Gold -= amount
	e.Investments = append(e.Investments, Investment{Amount: amount, Return: ret, MaturesDay: w.GameDay + days})
	return ret, nil
}

// ExpectedReturn is the total payout for investing `amount` for `days` days
// at investment rate `rate` (percent per day, simple interest).
func ExpectedReturn(amount, rate, days int) int {
	return amount + amount*rate*days/100
}

// PendingInvested is the total principal an empire has locked in investments.
func (w *World) PendingInvested(e *Empire) int {
	total := 0
	for _, inv := range e.Investments {
		total += inv.Amount
	}
	return total
}

// matureInvestments pays out any of e's investments that have reached their
// maturity day, crediting Return to gold (clamped to MoneyCap) and removing
// them. Returns the total paid out this call.
func (w *World) matureInvestments(e *Empire) int {
	paid := 0
	var remaining []Investment
	for _, inv := range e.Investments {
		if w.GameDay >= inv.MaturesDay {
			before := e.Gold
			e.Gold += inv.Return
			if e.Gold > MoneyCap {
				e.Gold = MoneyCap
			}
			paid += e.Gold - before
		} else {
			remaining = append(remaining, inv)
		}
	}
	e.Investments = remaining
	return paid
}

// investRateHeavyThreshold is a v1 tunable: total gold invested across all
// empires above this pushes the rate down (heavy demand for the bank's
// gold); below it, the rate drifts up.
const investRateHeavyThreshold = 5_000_000

// adjustInvestRate nudges the floating rate: heavy total investing across all
// empires pushes it down, light investing pushes it up, plus a small random
// drift; clamped to [MinInvestRate, MaxInvestRate].
func (w *World) adjustInvestRate() {
	total := 0
	for _, e := range w.Empires {
		if e.Alive {
			total += w.PendingInvested(e)
		}
	}
	if total > investRateHeavyThreshold {
		w.InvestRate--
	} else {
		w.InvestRate++
	}
	w.InvestRate += w.rng.Intn(3) - 1 // -1..+1 random drift (inflation flavor)
	if w.InvestRate < MinInvestRate {
		w.InvestRate = MinInvestRate
	}
	if w.InvestRate > MaxInvestRate {
		w.InvestRate = MaxInvestRate
	}
}
