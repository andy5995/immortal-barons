package game

import "errors"

var (
	ErrCantAfford = errors.New("You cannot afford that.")
	ErrNoBank     = errors.New("You do not have that much in the bank.")
	ErrNoDebt     = errors.New("You do not owe that much.")
	ErrNoAgents   = errors.New("You have no agents for that operation.")
)

// spend deducts gold for buying n units at unit cost, returning an error
// if the empire can't afford it. n <= 0 is a no-op.
func (e *Empire) spend(n, unit int) error {
	if n <= 0 {
		return nil
	}
	cost := n * unit
	if e.Gold < cost {
		return ErrCantAfford
	}
	e.Gold -= cost
	return nil
}

// LandPriceStep controls how fast land gets more expensive as an empire
// grows (v1 balance knob — tune freely). Each region you own raises the
// next region's price by Prices.Land/LandPriceStep.
const LandPriceStep = 50

// LandPrice is the current gold cost of the next region for empire e.
func (w *World) LandPrice(e *Empire) int {
	return w.Prices.Land + e.Land*w.Prices.Land/LandPriceStep
}

// BuyRegions buys n regions of the type pointed to by field (a pointer into
// e.Regions, e.g. &e.Regions.Coastal), using the same rising-price formula
// as before region types existed. All-or-nothing: either the whole
// purchase is affordable or nothing happens.
func (w *World) BuyRegions(e *Empire, field *int, n int) error {
	if n <= 0 {
		return nil
	}
	total := 0
	for i := 0; i < n; i++ {
		total += w.Prices.Land + (e.Land+i)*w.Prices.Land/LandPriceStep
	}
	if e.Gold < total {
		return ErrCantAfford // must afford the whole purchase
	}
	e.Gold -= total
	*field += n
	e.syncLand()
	return nil
}

// BuyLand is a thin wrapper over BuyRegions that buys Coastal regions, kept
// for callers that don't care about region type.
func (w *World) BuyLand(e *Empire, n int) error {
	return w.BuyRegions(e, &e.Regions.Coastal, n)
}

func (w *World) BuyFood(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Food); err != nil {
		return err
	}
	e.Food += n
	return nil
}

func (w *World) Recruit(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Trooper); err != nil {
		return err
	}
	e.Troopers += n
	return nil
}

func (w *World) BuildJets(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Jet); err != nil {
		return err
	}
	e.Jets += n
	return nil
}

func (w *World) BuildTurrets(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Turret); err != nil {
		return err
	}
	e.Turrets += n
	return nil
}

func (w *World) BuildCarriers(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Carrier); err != nil {
		return err
	}
	e.Carriers += n
	return nil
}

func (w *World) BuildTanks(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Tank); err != nil {
		return err
	}
	e.Tanks += n
	return nil
}

func (w *World) RecruitAgents(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Agent); err != nil {
		return err
	}
	e.Agents += n
	return nil
}

// SellRegions returns regions of the type pointed to by field for half
// their current market price per region. n is clamped to *field.
func (w *World) SellRegions(e *Empire, field *int, n int) error {
	if n <= 0 {
		return nil
	}
	if n > *field {
		n = *field
	}
	for i := 0; i < n; i++ {
		*field--
		e.syncLand()
		e.Gold += w.LandPrice(e) / 2
	}
	return nil
}

// SellLand is a thin wrapper over SellRegions that sells Coastal regions,
// kept for callers that don't care about region type.
func (w *World) SellLand(e *Empire, n int) error {
	return w.SellRegions(e, &e.Regions.Coastal, n)
}

func (w *World) Deposit(e *Empire, n int) error {
	if n <= 0 {
		return nil
	}
	if e.Gold < n {
		return ErrCantAfford
	}
	e.Gold -= n
	e.Bank += n
	return nil
}

func (w *World) Withdraw(e *Empire, n int) error {
	if n <= 0 {
		return nil
	}
	if e.Bank < n {
		return ErrNoBank
	}
	e.Bank -= n
	e.Gold += n
	return nil
}

// Loan borrows gold, added to the debt that accrues interest each turn.
func (w *World) Loan(e *Empire, n int) error {
	if n <= 0 {
		return nil
	}
	e.Gold += n
	e.Debt += n
	return nil
}

func (w *World) Repay(e *Empire, n int) error {
	if n <= 0 {
		return nil
	}
	if n > e.Debt {
		return ErrNoDebt
	}
	if e.Gold < n {
		return ErrCantAfford
	}
	e.Gold -= n
	e.Debt -= n
	return nil
}
