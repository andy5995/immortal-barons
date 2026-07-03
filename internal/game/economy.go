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

func (w *World) BuyLand(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Land); err != nil {
		return err
	}
	e.Land += n
	return nil
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

// Sell returns land for half its purchase price.
func (w *World) SellLand(e *Empire, n int) error {
	if n <= 0 {
		return nil
	}
	if n > e.Land {
		n = e.Land
	}
	e.Land -= n
	e.Gold += n * w.Prices.Land / 2
	return nil
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
