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
	received := amount
	if to.Gold+received > MoneyCap {
		received = MoneyCap - to.Gold
	}
	if received <= 0 {
		return nil // recipient is already at the money cap
	}
	from.Gold -= received
	to.Gold += received
	to.Mail = append(to.Mail, fmt.Sprintf("%s sent you %d gold in a trade deal.", from.Name, received))
	return nil
}
