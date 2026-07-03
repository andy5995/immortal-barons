package game

import "fmt"

// SendGold transfers `amount` gold from `from` to `to`, capped at the money
// limit, and mails the recipient a notice. Returns ErrCantAfford if the
// sender lacks the funds.
func (w *World) SendGold(from, to *Empire, amount int) error {
	if amount <= 0 {
		return nil
	}
	if from.Gold < amount {
		return ErrCantAfford
	}
	from.Gold -= amount
	to.Gold += amount
	if to.Gold > MoneyCap {
		to.Gold = MoneyCap
	}
	to.Mail = append(to.Mail, fmt.Sprintf("%s sent you %d gold in a trade deal.", from.Name, amount))
	return nil
}
