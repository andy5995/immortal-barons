package game

import (
	"errors"
	"fmt"
	"time"
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
	// Expires is when the deal lapses unanswered — the span it was sent for,
	// counted from the moment it was sent. A deal saved before deals expired
	// carries the zero time and stands forever, as it did when it was written.
	Expires time.Time `json:",omitempty"`
}

// clampTradeDealDays holds a requested span to the two-day minimum. There is no
// maximum: BRE checks only the floor, and the per-day cost is what bounds a long
// deal (see TradeDealMinDays).
func clampTradeDealDays(days int) int {
	if days < TradeDealMinDays {
		return TradeDealMinDays
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
	to.TradeDeals = append(to.TradeDeals, TradeDeal{
		From:    from.Name,
		Send:    send,
		Demand:  demand,
		Expires: time.Now().AddDate(0, 0, clampTradeDealDays(days)),
	})
	// The offer mails nothing: the recipient meets it at turn start, where the
	// baskets and the accept prompt are. Same reason a treaty proposal stopped
	// mailing (a1b309f) — a generated line telling them what the screen is
	// already asking.
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
	notifyTrader(from, to, "accepted")
	return nil
}

// notifyTrader files the sender's answer on the proposer's recap: BRE holds both
// " accepted your trade deal." and " rejected your trade deal."
// (process_trade_offer, BRE.OVR 0x24D6B), each written to the other realm's
// record rather than mailed, so the answer reaches them whenever they next play.
// Same shape as notifyProposer for treaties.
func notifyTrader(from, to *Empire, verb string) {
	from.addEvent(fmt.Sprintf("%s %s your trade deal.", to.Name, verb))
}

// DeclineTradeDeal drops a pending deal. The escrow is NOT returned: acceptance
// is the only outcome in the original that moves the offered goods anywhere.
// process_trade_offer answers a rejection by filing the notice and zeroing the
// 0x97-byte record (`clear_trade_offer_record` at 0xDC4), and the whole of its
// goods-moving code sits in the accept branch — nothing credits the sender back.
// Sending is therefore a real bet on the answer, and IB used to return the goods.
func (w *World) DeclineTradeDeal(to *Empire, fromName string) bool {
	i := findDeal(to, fromName)
	if i < 0 {
		return false
	}
	if from := w.FindByName(fromName); from != nil {
		notifyTrader(from, to, "rejected")
	}
	to.removeDeal(i)
	return true
}

// ExpireTradeDeals drops every pending deal whose span has run out and tells the
// realm that sent it. The escrow goes with it — see DeclineTradeDeal.
//
// BRE sweeps lazily, inside the turn-start routine that puts pending deals to a
// player (process_trade_offer 0x24E5): whoever plays next is who clears the
// stale ones, so a deal outlives its span until someone takes a turn. IB does
// the same rather than expiring them in daily maintenance.
func (w *World) ExpireTradeDeals(now time.Time) {
	for _, e := range w.Empires {
		kept := e.TradeDeals[:0]
		for _, d := range e.TradeDeals {
			if !d.Expires.IsZero() && now.After(d.Expires) {
				if from := w.FindByName(d.From); from != nil {
					from.addEvent(fmt.Sprintf("%s never answered your trade deal, and the goods you sent it with are lost.", e.Name))
				}
				continue
			}
			kept = append(kept, d)
		}
		e.TradeDeals = kept
	}
}

// forfeitPendingDeals tells each realm holding a deal on e that its trade fleet
// found nobody there. Called when e leaves the world — eliminated, abdicated or
// reaped as idle — where the goods would otherwise vanish with no word at all
// (#174). They are forfeit, as they are on a rejection and on an expiry.
func (w *World) forfeitPendingDeals(e *Empire) {
	for _, d := range e.TradeDeals {
		if from := w.FindByName(d.From); from != nil && from != e {
			from.addEvent(fmt.Sprintf("Your trade fleet could not find %s, and the goods you sent it with are lost.", e.Name))
		}
	}
	e.TradeDeals = nil
}
