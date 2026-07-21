package game

import (
	"errors"
	"fmt"
)

// ErrTradeSenderGone is returned when a trade deal's proposer no longer exists
// by the time the recipient accepts (eliminated by another node mid-turn).
var ErrTradeSenderGone = errors.New("The empire that sent this deal is gone.")

// FindByName returns the empire whose realm name equals name, alive or dead, or
// nil. Realm names are unique (RealmNameTaken guards onboarding), so this is
// unambiguous.
func (w *World) FindByName(name string) *Empire {
	for _, e := range w.Empires {
		if e.Name == name {
			return e
		}
	}
	return nil
}

// TradeBasket is a bundle of tradeable goods — BRE's nine trade-deal goods
// (Regions and HQ are not tradeable). A zero field means none of that good.
type TradeBasket struct {
	Troopers int
	Jets     int
	Turrets  int
	Bombers  int
	Food     int
	Gold     int
	Agents   int
	Tanks    int
	Carriers int
}

// IsEmpty reports whether the basket holds nothing.
func (b TradeBasket) IsEmpty() bool { return b == TradeBasket{} }

// goodPtrs returns pointers to an empire's fields for each basket good, in a
// fixed order, so add/subtract/ownership can loop uniformly.
func goodPtrs(e *Empire) [9]*int {
	return [9]*int{&e.Troopers, &e.Jets, &e.Turrets, &e.Bombers, &e.Food, &e.Gold, &e.Agents, &e.Tanks, &e.Carriers}
}

// basketVals returns a basket's amounts in the same fixed order as goodPtrs.
func basketVals(b TradeBasket) [9]int {
	return [9]int{b.Troopers, b.Jets, b.Turrets, b.Bombers, b.Food, b.Gold, b.Agents, b.Tanks, b.Carriers}
}

// empireHasBasket reports whether e owns at least everything in b.
func empireHasBasket(e *Empire, b TradeBasket) bool {
	ptrs, vals := goodPtrs(e), basketVals(b)
	for i := range ptrs {
		if *ptrs[i] < vals[i] {
			return false
		}
	}
	return true
}

// addBasket / subBasket move a basket's goods onto / off of e. Gold is clamped
// to MoneyCap on the way in (excess is lost, as when any gold overflows the cap).
func addBasket(e *Empire, b TradeBasket) {
	ptrs, vals := goodPtrs(e), basketVals(b)
	for i := range ptrs {
		*ptrs[i] += vals[i]
	}
	if e.Gold > MoneyCap {
		e.Gold = MoneyCap
	}
}

func subBasket(e *Empire, b TradeBasket) {
	ptrs, vals := goodPtrs(e), basketVals(b)
	for i := range ptrs {
		*ptrs[i] -= vals[i]
	}
}

// TradeDeal is a pending barter offer recorded on the target empire: the sender
// gives Send and wants Demand in return. The Send goods are escrowed off the
// sender when the deal is sent (see SendTradeDeal), so they can't be double-spent
// while the offer is pending; a decline returns them.
type TradeDeal struct {
	From   string
	Send   TradeBasket // goods the sender gives the recipient
	Demand TradeBasket // goods the sender wants back from the recipient
}

// SendTradeDeal escrows the Send goods off `from` and records a pending deal on
// `to`. Fails if `from` doesn't own the Send goods, or both baskets are empty.
func (w *World) SendTradeDeal(from, to *Empire, send, demand TradeBasket) error {
	if send.IsEmpty() && demand.IsEmpty() {
		return fmt.Errorf("A trade deal must offer or request something.")
	}
	if !empireHasBasket(from, send) {
		return ErrCantAfford
	}
	subBasket(from, send) // escrow
	to.TradeDeals = append(to.TradeDeals, TradeDeal{From: from.Name, Send: send, Demand: demand})
	to.Mail = append(to.Mail, fmt.Sprintf("%s sent you a trade deal (respond in the Trading menu).", from.Name))
	return nil
}

// findDeal returns the index of the first pending deal on `to` from fromName, or -1.
func findDeal(to *Empire, fromName string) int {
	for i, d := range to.TradeDeals {
		if d.From == fromName {
			return i
		}
	}
	return -1
}

// removeDeal drops the deal at index i from to.TradeDeals.
func (to *Empire) removeDeal(i int) {
	to.TradeDeals = append(to.TradeDeals[:i], to.TradeDeals[i+1:]...)
}

// AcceptTradeDeal completes a pending deal from fromName: the recipient `to`
// receives the escrowed Send goods and pays the Demand goods to the (re-resolved)
// sender. Fails if `to` can't cover the Demand, or the sender has vanished (its
// escrow is then forfeit — the deal is dropped by the caller path). No-op-safe:
// returns an error if there is no such pending deal.
func (w *World) AcceptTradeDeal(to *Empire, fromName string) error {
	i := findDeal(to, fromName)
	if i < 0 {
		return fmt.Errorf("That trade deal is no longer available.")
	}
	d := to.TradeDeals[i]
	if !empireHasBasket(to, d.Demand) {
		return ErrCantAfford
	}
	from := w.FindByName(fromName)
	if from == nil {
		// Sender gone: drop the deal; the escrow is forfeit.
		to.removeDeal(i)
		return ErrTradeSenderGone
	}
	addBasket(to, d.Send)     // deliver the offered goods (escrow released to recipient)
	subBasket(to, d.Demand)   // recipient pays the demand
	addBasket(from, d.Demand) // sender receives the demand
	to.removeDeal(i)
	from.Mail = append(from.Mail, fmt.Sprintf("%s accepted your trade deal.", to.Name))
	return nil
}

// DeclineTradeDeal drops a pending deal and returns the escrowed Send goods to
// the (re-resolved) sender. Returns false if there was no such deal.
func (w *World) DeclineTradeDeal(to *Empire, fromName string) bool {
	i := findDeal(to, fromName)
	if i < 0 {
		return false
	}
	d := to.TradeDeals[i]
	if from := w.FindByName(fromName); from != nil {
		addBasket(from, d.Send) // return the escrow
	}
	to.removeDeal(i)
	return true
}
