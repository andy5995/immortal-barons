package game

import (
	"errors"
	"fmt"
)

// ErrTradeSenderGone is returned when a trade deal's proposer no longer exists
// by the time the recipient accepts (eliminated by another node mid-turn).
var ErrTradeSenderGone = errors.New("The empire that sent this deal is gone.")

// ErrTradeNeedsCarrier is returned when the sender lacks a carrier to transport
// the deal (BRE consumes one carrier per deal).
var ErrTradeNeedsCarrier = errors.New("You do not have a carrier to send this deal.")

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

// goodPtrs returns pointers to an empire's fields for each basket good, and
// basketPtrs to a basket's own amounts. Both walk MarketGoods, so index i is
// the same good in each without either restating the set (#134).
// Gold is not among them: it is the one basket good held in money width, so the
// three routines below handle it beside the loop rather than inside it.
func goodPtrs(e *Empire) []*int {
	ptrs := make([]*int, len(MarketGoods))
	for i, g := range MarketGoods {
		ptrs[i] = g.Count(e)
	}
	return ptrs
}

func basketPtrs(b *TradeBasket) []*int {
	ptrs := make([]*int, len(MarketGoods))
	for i, g := range MarketGoods {
		ptrs[i] = g.Basket(b)
	}
	return ptrs
}

// basketVals returns a basket's amounts in the same order as goodPtrs.
func basketVals(b TradeBasket) []int {
	ptrs := basketPtrs(&b)
	vals := make([]int, len(ptrs))
	for i, p := range ptrs {
		vals[i] = *p
	}
	return vals
}

// empireHasBasket reports whether e owns at least everything in b.
func empireHasBasket(e *Empire, b TradeBasket) bool {
	ptrs, vals := goodPtrs(e), basketVals(b)
	for i := range ptrs {
		if *ptrs[i] < vals[i] {
			return false
		}
	}
	return e.Gold >= int64(b.Gold)
}

// addBasket / subBasket move a basket's goods onto / off of e. Gold is clamped
// to the money cap on the way in (excess is lost, as when any gold overflows the
// cap), which is why addBasket needs the World and subBasket does not.
func (w *World) addBasket(e *Empire, b TradeBasket) {
	ptrs, vals := goodPtrs(e), basketVals(b)
	for i := range ptrs {
		*ptrs[i] += vals[i]
	}
	w.creditGold(e, int64(b.Gold), "a trade deal")
}

func subBasket(e *Empire, b TradeBasket) {
	ptrs, vals := goodPtrs(e), basketVals(b)
	for i := range ptrs {
		*ptrs[i] -= vals[i]
	}
	e.Gold -= int64(b.Gold)
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

// clampTradeDealDays holds a requested span to the allowed 2-5 days.
func clampTradeDealDays(days int) int {
	if days < TradeDealMinDays {
		return TradeDealMinDays
	}
	if days > TradeDealMaxDays {
		return TradeDealMaxDays
	}
	return days
}

// TradeDealCost is the undiscounted gold cost to send a deal for the given
// number of days, clamped to the allowed span (BRE: TradeDealGoldPerDay per day,
// 2-5 days). Use TradeDealCostBetween for the price a specific pair pays.
func TradeDealCost(days int) int64 {
	return int64(clampTradeDealDays(days)) * TradeDealGoldPerDay
}

// TradeDealGoldPerDayBetween is the per-day transit cost `from` pays to send a
// deal to `to`. The sysop's Trade Deal Costs setting scales the rate, and a
// standing Protective Trade agreement then puts guards on the route and cuts
// what is left to a third (ProtectiveTradeCostDivisor).
//
// The setting is one of the original's own five cost knobs ("Trade Deal Costs",
// alongside Maintenance Costs, Region Cost Change, Attack Costs and Terrorism
// Costs) and IB has always stored and broadcast it — it just reached nothing,
// which is what #56 is about. Its ladder is its own, read from the binary; see
// Level.TradeCostScaled. The discount divides the SCALED rate, so at Trade Deal
// Costs = None a Protective Trade pact discounts nothing, there being nothing
// to discount.
func (w *World) TradeDealGoldPerDayBetween(from, to *Empire) int64 {
	rate := w.Config.TradeCosts.TradeCostScaled(TradeDealGoldPerDay)
	if w.HasTreaty(from, to, protectiveTrade) {
		return rate / ProtectiveTradeCostDivisor
	}
	return rate
}

// TradeDealCostBetween is what `from` actually pays to send `to` a deal over the
// given span. BRE discounts the PER-DAY rate and then multiplies by the days, so
// the truncation lands on the rate, not on the total.
func (w *World) TradeDealCostBetween(from, to *Empire, days int) int64 {
	return int64(clampTradeDealDays(days)) * w.TradeDealGoldPerDayBetween(from, to)
}

// SendTradeDeal sends a trade deal from `from` to `to` over `days` days: it
// consumes one carrier to transport it, charges the per-day gold fee, escrows the
// Send goods, and records a pending deal on `to` (arrives on the recipient's next
// turn). Fails if both baskets are empty, `from` lacks the offered goods, lacks a
// transport carrier, or can't afford the fee.
func (w *World) SendTradeDeal(from, to *Empire, send, demand TradeBasket, days int) error {
	if from.Protection > 0 {
		return ErrInProtection
	}
	if to.Protection > 0 {
		return ErrTheyProtected
	}
	if !w.HasPact(from, to) {
		return ErrNoRelations
	}
	if send.IsEmpty() && demand.IsEmpty() {
		return fmt.Errorf("A trade deal must offer or request something.")
	}
	cost := w.TradeDealCostBetween(from, to, days)
	if !empireHasBasket(from, send) {
		return ErrCantAfford
	}
	if from.Carriers < send.Carriers+TradeDealCarriers {
		return ErrTradeNeedsCarrier
	}
	if from.Gold < int64(send.Gold)+cost {
		return ErrCantAfford
	}
	subBasket(from, send)              // escrow the offered goods
	from.Carriers -= TradeDealCarriers // the transport carrier is consumed
	from.Gold -= cost                  // pay the per-day transit fee
	to.TradeDeals = append(to.TradeDeals, TradeDeal{From: from.Name, Send: send, Demand: demand})
	w.SendMail(from, to, Message{
		To:   w.EmpireLetter(to),
		Body: "Sent you a trade deal (respond in the Trading menu).",
	})
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
	w.addBasket(to, d.Send)     // deliver the offered goods (escrow released to recipient)
	subBasket(to, d.Demand)     // recipient pays the demand
	w.addBasket(from, d.Demand) // sender receives the demand
	to.removeDeal(i)
	w.SendMail(to, from, Message{
		To:   w.EmpireLetter(from),
		Body: "Accepted your trade deal.",
	})
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
		w.addBasket(from, d.Send) // return the escrow
	}
	to.removeDeal(i)
	return true
}
