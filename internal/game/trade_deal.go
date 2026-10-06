package game

import (
	"errors"
	"fmt"
	"time"
)

// ErrTradeSenderGone is returned when a trade deal's proposer no longer exists
// by the time the recipient accepts: removed by a maintenance sweep, a sysop or
// a reset.
var ErrTradeSenderGone = errors.New("The empire that sent this deal is gone.")

// ErrTradeNeedsCarrier is returned when the sender lacks the carriers to
// transport the deal. How many that is depends on the cargo — see
// TradeDealCarriers.
var ErrTradeNeedsCarrier = errors.New("You do not have enough carriers to send this deal.")

// FreeCarriers is how many of e's carriers can transport basket b: those held,
// less any b is itself shipping, which cannot carry themselves. The original
// checks the same (send_trade_offer 0x046d). Every check of a deal's transport,
// local or interplanetary, the screen's warning included, goes through here.
func FreeCarriers(e *Empire, b TradeBasket) int { return max(e.Carriers-b.Carriers, 0) }

// CanCarry reports whether e has the free carriers basket b needs.
func CanCarry(e *Empire, b TradeBasket) bool { return FreeCarriers(e, b) >= TradeDealCarriers(b) }

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
	w.creditGold(e, int64(b.Gold), say("a trade deal"))
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
// while the offer is pending; a decline destroys them (see DeclineTradeDeal).
type TradeDeal struct {
	From   string
	Send   TradeBasket // goods the sender gives the recipient
	Demand TradeBasket // goods the sender wants back from the recipient
	// Expires is when the deal lapses unanswered — the span it was sent for,
	// counted from the moment it was sent. A deal saved before deals expired
	// carries the zero time and stands forever, as it did when it was written.
	Expires time.Time `json:",omitempty"`
	// ArrivesOnTurn is the sender's own turn of the day, counted from 1, at the
	// moment the deal was sent; the recipient does not meet it until they reach
	// that turn of their own day. So a deal sent on your third turn is waiting
	// from their third turn onward — a trade fleet cannot reach someone earlier
	// in their day than it left yours. The gate holds on the day of sending
	// only: daily maintenance clears it (releaseTradeDeals).
	//
	// Zero means no gate, and it has to: a deal saved before this was recorded
	// has no such key, and it must go on arriving at once rather than being
	// held forever behind a turn it was never stamped with.
	ArrivesOnTurn int `json:",omitempty"`
	// Carriers is the transport the deal took off its sender, kept as it was
	// at sending: Bomb Trade Routes cuts Send in transit and leaves this alone,
	// as the original's record keeps the count apart from the goods (+0x52).
	// Read it through transport.
	Carriers int `json:",omitempty"`
}

// releaseTradeDeals lifts the arrival gate from every pending deal at the turn
// of the day, dead realms' included. BINARY-VERIFIED: daily maintenance's pass
// over the offer list (pack_trade_offer_list, BRE.OVR 0x050e1a) sets each
// surviving record's +0x60 stamp to 0xFF, the no-gate value, so a deal waits for
// the recipient's matching turn on the day it was sent and on no later day.
func (w *World) releaseTradeDeals() {
	for _, e := range w.Empires {
		for i := range e.TradeDeals {
			e.TradeDeals[i].ArrivesOnTurn = 0
		}
	}
}

// TurnOfDay is the turn an empire is currently on, counted from 1. It is what
// BRE prints when a deal is sent, and both halves of the arrival gate are
// expressed in it so the comparison does not depend on turns-per-day: the
// setting cancels out of both sides.
func (w *World) TurnOfDay(e *Empire) int {
	if n := w.Config.TurnsPerDay - e.TurnsLeft + 1; n > 1 {
		return n
	}
	return 1
}

// TradeDealArrived reports whether a pending deal has reached `to` yet. A deal
// that has not is left pending — never expired early, never delivered — exactly
// as BRE leaves it (`process_trade_offer`, BRE.OVR 0x24D6B unit offset 0x05B1).
func (w *World) TradeDealArrived(d TradeDeal, to *Empire) bool {
	return d.ArrivesOnTurn == 0 || w.TurnOfDay(to) >= d.ArrivesOnTurn
}

// clampTradeDealDays holds a requested span to TradeDealMinDays..TradeDealMaxDays,
// the bounds create_trade_offer's days prompt accepts.
func clampTradeDealDays(days int) int {
	return min(max(days, TradeDealMinDays), TradeDealMaxDays)
}

// TradeDealGoldPerDayBetween is the per-day transit cost `from` pays to send
// `send` to `to`: the cargo-weighted TradeOfferCost of the basket, which already
// carries the sysop's Trade Deal Costs ladder, cut to a third by a standing
// Protective Trade agreement (ProtectiveTradeCostDivisor). BINARY-VERIFIED:
// create_trade_offer calls calculate_trade_offer_cost (BRE.OVR 0x268a1) and
// divides its result when the pact is in force, before the days prompt. At
// Trade Deal Costs = None there is nothing for the pact to discount.
func (w *World) TradeDealGoldPerDayBetween(from, to *Empire, send TradeBasket) int64 {
	rate := w.TradeOfferCost(send)
	if w.HasTreaty(from, to, protectiveTrade) {
		return rate / ProtectiveTradeCostDivisor
	}
	return rate
}

// TradeDealCostBetween is what `from` actually pays to send `to` a deal over the
// given span. BRE discounts the PER-DAY rate and then multiplies by the days, so
// the truncation lands on the rate, not on the total.
func (w *World) TradeDealCostBetween(from, to *Empire, send TradeBasket, days int) int64 {
	return int64(clampTradeDealDays(days)) * w.TradeDealGoldPerDayBetween(from, to, send)
}

// SendTradeDeal sends a trade deal from `from` to `to` over `days` days: it
// takes the carriers its cargo needs (they come back on accept), charges the
// per-day gold fee, escrows the Send goods, and records a pending deal on `to`,
// stamped with the turn of the day it left (see TradeDeal.ArrivesOnTurn). Fails
// if both baskets are empty, `from` lacks the offered goods, lacks the transport
// carriers, or can't afford the fee.
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
	cost := w.TradeDealCostBetween(from, to, send, days)
	if !empireHasBasket(from, send) {
		return ErrCantAfford
	}
	// The original sizes the transport to the CARGO rather than charging a flat
	// one per deal (#195).
	if !CanCarry(from, send) {
		return ErrTradeNeedsCarrier
	}
	if from.Gold < int64(send.Gold)+cost {
		return ErrCantAfford
	}
	carriers := TradeDealCarriers(send)
	subBasket(from, send)     // escrow the offered goods
	from.Carriers -= carriers // the transport goes with them
	from.Gold -= cost         // pay the per-day transit fee
	to.TradeDeals = append(to.TradeDeals, TradeDeal{
		From:          from.Name,
		Send:          send,
		Demand:        demand,
		Expires:       timeNow().AddDate(0, 0, clampTradeDealDays(days)),
		ArrivesOnTurn: w.TurnOfDay(from),
		Carriers:      carriers,
	})
	// The offer mails nothing: the recipient meets it at turn start, where the
	// baskets and the accept prompt are. Same reason a treaty proposal stopped
	// mailing (a1b309f) — a generated line telling them what the screen is
	// already asking.
	return nil
}

// transport is the carriers the deal took off its sender. A deal saved before
// the count was kept has none recorded, and gets it worked out from what it
// still carries.
func (d TradeDeal) transport() int {
	if d.Carriers > 0 {
		return d.Carriers
	}
	return TradeDealCarriers(d.Send)
}

// findDeal returns the index of the pending deal on `to` that is `want`, or -1.
// The sender's name alone is not enough: one sender may have several deals
// pending with the same realm, and the first by name can be one still in
// transit that the player was never shown.
func findDeal(to *Empire, want TradeDeal) int {
	for i, d := range to.TradeDeals {
		if d.From == want.From && d.Send == want.Send && d.Demand == want.Demand &&
			d.Expires.Equal(want.Expires) && d.ArrivesOnTurn == want.ArrivesOnTurn {
			return i
		}
	}
	return -1
}

// removeDeal drops the deal at index i from to.TradeDeals.
func (to *Empire) removeDeal(i int) {
	to.TradeDeals = append(to.TradeDeals[:i], to.TradeDeals[i+1:]...)
}

// AcceptTradeDeal completes the pending deal `want`: the recipient `to`
// receives the escrowed Send goods and pays the Demand goods to the (re-resolved)
// sender. Fails if `to` can't cover the Demand, or the sender has vanished (its
// escrow is then forfeit — the deal is dropped by the caller path). No-op-safe:
// returns an error if there is no such pending deal.
func (w *World) AcceptTradeDeal(to *Empire, want TradeDeal) error {
	i := findDeal(to, want)
	if i < 0 {
		return fmt.Errorf("That trade deal is no longer available.")
	}
	d := to.TradeDeals[i]
	if !empireHasBasket(to, d.Demand) {
		return ErrCantAfford
	}
	from := w.FindByName(d.From)
	if from == nil {
		// Sender gone: drop the deal; the escrow is forfeit.
		to.removeDeal(i)
		return ErrTradeSenderGone
	}
	w.addBasket(to, d.Send)     // deliver the offered goods (escrow released to recipient)
	subBasket(to, d.Demand)     // recipient pays the demand
	w.addBasket(from, d.Demand) // sender receives the demand
	// The transport comes home with a completed deal: the accept branch credits
	// the record's carrier count back to the sender (process_trade_offer,
	// BRE.OVR 0x02563a), and a rejection or expiry credits nothing.
	from.Carriers += d.transport()
	to.removeDeal(i)
	notifyTrader(from, to, msgid("{who} accepted your trade deal."))
	return nil
}

// notifyTrader files the sender's answer on the proposer's recap: BRE holds both
// " accepted your trade deal." and " rejected your trade deal."
// (process_trade_offer, BRE.OVR 0x24D6B), each written to the other realm's
// record rather than mailed, so the answer reaches them whenever they next play.
// Same shape as notifyProposer for treaties.
func notifyTrader(from, to *Empire, answer string) {
	from.addEvent(say(answer, "who", to.Name))
}

// DeclineTradeDeal drops a pending deal. The escrow is NOT returned: acceptance
// is the only outcome in the original that moves the offered goods anywhere.
// process_trade_offer answers a rejection by filing the notice and zeroing the
// 0x97-byte record (`clear_trade_offer_record` at 0xDC4), and the whole of its
// goods-moving code sits in the accept branch — nothing credits the sender back.
// Sending is therefore a real bet on the answer, and IB used to return the goods.
func (w *World) DeclineTradeDeal(to *Empire, want TradeDeal) bool {
	i := findDeal(to, want)
	if i < 0 {
		return false
	}
	if from := w.FindByName(want.From); from != nil {
		notifyTrader(from, to, msgid("{who} rejected your trade deal."))
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
					from.addEvent(say("{who} never answered your trade deal, and the goods you sent it with are lost.", "who", e.Name))
				}
				continue
			}
			kept = append(kept, d)
		}
		e.TradeDeals = kept
	}
}

// returnPendingDeals hands each escrowed shipment back to the realm that sent
// it, and says so. Called when e leaves the world — eliminated, abdicated or
// reaped as idle.
//
// This is the one forfeit case with nobody to blame for it (#248). A rejection
// and an expiry both destroy the escrow, and deliberately: the target answered,
// or could have. A target that is GONE was never offered the choice, so the
// sender loses goods over something no player did. IB returns them instead.
//
// The original settles none of this — nothing on its elimination path clears a
// trade record (`clear_trade_offer_record` is reached only from
// `create_trade_offer` and `process_trade_offer`), so a departing realm takes
// any pending offer with it silently. IB already diverged by telling the sender
// at all; this carries the goods with the notice, and the carriers that
// shipped them, as an accepted deal would.
//
// addBasket is what returns them, so gold lands under the money cap and files
// its own loss event if the realm is already at it.
func (w *World) returnPendingDeals(e *Empire) {
	for _, d := range e.TradeDeals {
		if from := w.FindByName(d.From); from != nil && from != e {
			w.addBasket(from, d.Send)
			from.Carriers += d.transport()
			from.addEvent(say("Your trade fleet could not find {who}, and has brought the goods home.", "who", e.Name))
		}
	}
	e.TradeDeals = nil
}
