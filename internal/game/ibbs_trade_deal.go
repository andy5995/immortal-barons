package game

import (
	"errors"
	"fmt"

	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// The interplanetary trade deal (#195). The original keeps this in a routine of
// its own — BRE.OVR 0x024212, whose sole caller is the InterBBS menu — separate
// from the local deal at 0x0260cd, and the two mechanics are not variants of one
// another:
//
//   - It is ONE-WAY. You may demand nothing in return, and the recipient is
//     given no choice: the goods land when the packet is processed. There is no
//     pending offer, no accept, no decline (docs/bre.doc:2088).
//   - It charges a FLAT fee to send, where the local deal charges over a span of
//     days. There is no day span here at all, so nothing arrives "on a turn":
//     it arrives when the packet does.
//   - It needs NO treaty or relation. The local deal refuses a realm you hold no
//     pact with; this one asks nothing of the two realms, and nothing of the two
//     planets' diplomacy either.
//   - It cannot address your OWN planet. The picker refuses it outright, which
//     is what keeps this from being the local mechanic with a longer reach.
//
// IB DIVERGES on one rule, deliberately. The original delivers a deal aimed at a
// realm under New Realm Protection into a routine that checks protection and
// returns — the goods and the fee are gone, and neither side is told anything
// (resolve_received_trade_offer, BRE.OVR 0x043df1). IB refuses the target at the
// picker instead, where the sender can still do something about it, and #214
// made protection visible on exactly those lists. Nothing is destroyed, and
// nothing needs an arrival guard: protection only ever counts DOWN, and delivery
// is keyed by realm name, so a realm that was clear when the deal left cannot be
// protected when it lands.

// ErrTradeDealOwnPlanet is returned when an interplanetary deal is addressed to
// the sender's own planet, which the original's planet picker refuses.
var ErrTradeDealOwnPlanet = errors.New("A trade deal to your own planet is a local deal; send it from the Trading menu.")

// ErrTradeDealProtected is returned when the named target is still under New
// Realm Protection. See the divergence note above.
var ErrTradeDealProtected = errors.New("That realm is under New Realm Protection and cannot receive a deal yet.")

// ErrNotInterBBSGame is returned when an interplanetary action is attempted on a
// stand-alone board, which has no other planet to reach.
var ErrNotInterBBSGame = errors.New("This board is not part of an inter-BBS league.")

// ErrEmptyTradeDeal is the original's own refusal of a deal with nothing in it
// (send_trade_offer 0x0534; docs/whatsnew.doc records it being added).
var ErrEmptyTradeDeal = errors.New("A trade deal must ship something.")

// IPTradeDeal is a shipment of goods travelling to a realm on another planet.
// It carries no price and no demand: what is in the basket is a gift, already
// paid for and already taken off the sender.
type IPTradeDeal struct {
	FromBoard  string
	FromEmpire string
	ToEmpire   string // the realm on ToBoard the goods are for
	Goods      TradeBasket
	When       string
}

// TradeDealCarriers is how many carriers a basket needs to travel, local or
// interplanetary. BINARY-VERIFIED (BRE.OVR ovr_050dfb_entry_0436): the original
// sums each good against its own per-carrier capacity and takes the ceiling of
// the total, so one carrier is only ever enough for a small shipment. Food,
// bombers, agents and carriers ride free — they need no carrier space at all.
//
// IB charged a flat ONE carrier per deal until #195, which made a shipment of a
// million troopers cost the same transport as a single jet; a capture has the
// original asking for twenty (docs/dev/bre-screens.md).
//
// The arithmetic is done in hundred-thousandths of a carrier so it stays in
// integers: every capacity below divides carrierScale exactly. The original adds
// 0.9999 and truncates rather than taking a true ceiling, so a total whose
// fraction lands within the last ten-thousandth comes out one carrier low;
// carrierRoundUp reproduces that exactly rather than rounding it away, because
// the point of a verified constant is to match.
func TradeDealCarriers(b TradeBasket) int {
	var units int64
	for _, g := range MarketGoods {
		if g.CarrierPer > 0 {
			units += int64(*g.Basket(&b)) * (carrierScale / int64(g.CarrierPer))
		}
	}
	units += int64(b.Gold) * (carrierScale / GoldPerCarrier)
	return int(carrierRoundUp(units))
}

// carrierRoundUp turns hundred-thousandths of a carrier into whole carriers the
// way the original does: add all but a ten-thousandth of one and truncate.
func carrierRoundUp(units int64) int64 {
	return (units + carrierScale - carrierScale/10_000) / carrierScale
}

// IPTradeDealCost is the flat gold fee to send a basket to another planet.
// BINARY-VERIFIED (BRE.OVR 0x0513e7, calculate_trade_offer_cost): the nine goods
// are summed against fixed weights, divided by five, and a flat base is added;
// the sysop's Trade Deal Costs ladder then scales the whole figure, and it is
// capped. An empty basket costs nothing — the original tests the weighted sum
// for zero before adding its base, so a deal with nothing in it is free rather
// than costing the base on its own.
//
// The weights are exact decimals here and Real48 approximations in the original
// (0.01 and 0.05 have no binary form), so on a very large basket the two can
// part company by a gold or two. Integers are the right answer for a treasury.
//
// A Protective Trade agreement does NOT discount this. The original applies that
// divisor in the local routine only (create_trade_offer 0x1f5b), which follows:
// the pact is between two realms on one planet, and this deal crosses planets.
func (w *World) IPTradeDealCost(b TradeBasket) int64 {
	var weighted int64
	for _, g := range MarketGoods {
		weighted += int64(*g.Basket(&b)) * int64(g.ShipWeight)
	}
	weighted += int64(b.Gold) * GoldShipWeight
	if weighted == 0 {
		return 0
	}
	cost := weighted/(shipWeightScale*TradeDealCostDivisor) + TradeDealGoldBase
	cost = w.Config.TradeCosts.TradeCostScaled(cost)
	if cost > MoneyCapMax {
		cost = MoneyCapMax
	}
	return cost
}

// SendIPTradeDeal ships goods to a named realm on another planet. It charges the
// flat fee, spends the carriers the basket needs, takes the goods off the sender
// and queues the shipment. The goods are gone from here the moment it returns
// nil: there is no pending offer to withdraw and nothing comes back.
func (w *World) SendIPTradeDeal(from *Empire, toBoard, toEmpire string, goods TradeBasket) error {
	if !w.Config.IBBS {
		return ErrNotInterBBSGame
	}
	if toBoard == "" || toBoard == w.Config.BoardID {
		return ErrTradeDealOwnPlanet
	}
	if from.Protection > 0 {
		return ErrInProtection
	}
	if goods.IsEmpty() {
		return ErrEmptyTradeDeal
	}
	if !empireHasBasket(from, goods) {
		return ErrCantAfford
	}
	// Carriers are spent as transport and are NOT part of the basket's own
	// carrier count if the basket happens to be carrying carriers: the original
	// checks what is held MINUS what is being shipped against what the shipment
	// needs (send_trade_offer 0x046d).
	need := TradeDealCarriers(goods)
	if from.Carriers-goods.Carriers < need {
		return fmt.Errorf("This shipment needs %d carriers to transport and you have %d free.",
			need, max(from.Carriers-goods.Carriers, 0))
	}
	cost := w.IPTradeDealCost(goods)
	// The fee is paid out of gold in hand AFTER the basket's own gold is set
	// aside, so a deal cannot be funded with the gold it is shipping.
	if from.Gold-int64(goods.Gold) < cost {
		return ErrCantAfford
	}
	subBasket(from, goods)
	from.Carriers -= need
	from.Gold -= cost
	w.enqueueIPTradeDeal(toBoard, IPTradeDeal{
		FromBoard:  w.Config.BoardID,
		FromEmpire: from.Name,
		ToEmpire:   toEmpire,
		Goods:      goods,
		When:       timeNow().Format(StampFormat),
	})
	from.addEvent(fmt.Sprintf("You shipped a trade deal to %s of %s, at a cost of %s gold and %d carriers.",
		toEmpire, toBoard, numfmt.Comma(cost), need))
	return nil
}

func (w *World) enqueueIPTradeDeal(board string, d IPTradeDeal) {
	p := w.outboxFor(board)
	p.TradeDeals = append(p.TradeDeals, d)
}

// deliverIPTradeDeal hands an arriving shipment to the realm it names, and says
// nothing to anyone else: the original files a private report for each side and
// writes no news, and no .dat template carries a trade-deal news category.
// A realm that has died since the deal left keeps nothing — there is no return
// path in the original and none here.
func (w *World) deliverIPTradeDeal(d IPTradeDeal) {
	to := w.remoteTarget(d.ToEmpire)
	if to == nil {
		return
	}
	w.addBasket(to, d.Goods)
	to.addEvent(fmt.Sprintf("%s of %s shipped you a trade deal: %s.",
		d.FromEmpire, d.FromBoard, describeBasket(d.Goods)))
}

// describeBasket lists a basket's contents for an event line, in the canonical
// good order and in the original's own tally shape (breTally), so a shipment
// reads the way the raid and battle reports read.
func describeBasket(b TradeBasket) string {
	var parts []string
	if b.Gold > 0 {
		parts = append(parts, fmt.Sprintf("%s Gold", numfmt.Comma(int64(b.Gold))))
	}
	for _, g := range MarketGoods {
		if n := *g.Basket(&b); n > 0 {
			parts = append(parts, fmt.Sprintf("%s %s", numfmt.Comma(int64(n)), g.Plural))
		}
	}
	return breTally(parts)
}
