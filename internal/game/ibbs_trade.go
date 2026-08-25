package game

import (
	"errors"
	"fmt"

	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// Interplanetary trading — IB's own, not the original's. BRE never let a baron
// touch another planet's market; this is what the feature looks like if the
// original had grown one.
//
// The shape follows from the transport rather than from taste. A buyer is
// looking at a market snapshot that arrived with the last packet, so by the time
// their order lands the listing may be gone, smaller, or repriced. An order is
// therefore a BID, not a purchase: the gold leaves the buyer's hands when the
// bid is sent, the far board fills it only if the listing still matches what the
// buyer was quoted, and either the goods or the gold comes home on the next
// packet. Nothing is created or destroyed in between — at every instant the
// value is in exactly one of three places (the buyer's escrow, the seller's
// proceeds, the returning packet), which is what makes a lost packet recoverable
// rather than a hole.

var (
	ErrIPTradingOff   = errors.New("Trading with other planets is turned off in this league.")
	ErrNotAllied      = errors.New("You may only trade with a planet your Coordinator has marked Allied.")
	ErrOwnPlanet      = errors.New("That is your own planet's market.")
	ErrBadTradeAmount = errors.New("Enter how many units to bid for.")
	ErrBadTradePrice  = errors.New("That listing's price is not valid.")
)

// RemoteListing is one row of another planet's market as this board last heard
// it. It is a SNAPSHOT and is expected to go stale — that staleness is the whole
// reason a bid can fail.
type RemoteListing struct {
	Realm string // the seller's realm on that planet
	Good  string
	Qty   int
	Price int // gold per unit
}

// IPTradeBid is a buy order travelling to another planet. Price is what the
// buyer was quoted, and the far board fills the bid only at that price — a
// seller who has since raised it is not made to honour the old one, and a buyer
// who would have paid more is not charged it.
type IPTradeBid struct {
	ID         int
	FromBoard  string
	FromOwner  string // the buyer's handle, so the answer reaches them
	FromEmpire string // and their realm name, for the seller's event
	Seller     string // the realm on the target planet whose listing this is
	Good       string
	Qty        int
	Price      int
}

// IPTradeFill is a bid's answer coming home: the goods if it filled, the gold
// back if it did not. Reason carries the seller-side wording so the buyer is
// told WHY rather than just handed their money back.
type IPTradeFill struct {
	ID     int
	Filled bool
	Good   string
	Qty    int
	Gold   int64  // the refund when Filled is false
	Reason string `json:",omitempty"`
}

// TradeBidCost is what a bid escrows: the quoted price for every unit asked for.
func TradeBidCost(qty, price int) int64 { return int64(qty) * int64(price) }

// SendTradeBid escrows the gold for a bid and queues it for the target planet.
// The bid is only offered to a planet the Coordinator has marked Allied — this
// is a favour between allies, not an open exchange.
func (w *World) SendTradeBid(e *Empire, targetBoard, seller, good string, qty, price int) (int, error) {
	if e.Protection > 0 {
		return 0, ErrInProtection
	}
	if !w.Config.IPTrading {
		return 0, ErrIPTradingOff
	}
	if targetBoard == "" || targetBoard == w.Config.BoardID {
		return 0, ErrOwnPlanet
	}
	if w.PlanetRelationWith(targetBoard) != PlanetAllied {
		return 0, ErrNotAllied
	}
	if qty <= 0 {
		return 0, ErrBadTradeAmount
	}
	// A price of 0 is a real listing — a giveaway between allies — and the local
	// market has always allowed one. Only a negative price is rejected, and that
	// can only arrive from a malformed snapshot packet, never from a seller here.
	if price < 0 {
		return 0, ErrBadTradePrice
	}
	cost := TradeBidCost(qty, price)
	if e.Gold < cost {
		return 0, ErrCantAfford
	}
	e.Gold -= cost
	w.NextAttackID++
	id := w.NextAttackID
	w.enqueueTradeBid(targetBoard, IPTradeBid{
		ID: id, FromBoard: w.Config.BoardID, FromOwner: e.Owner, FromEmpire: e.Name,
		Seller: seller, Good: good, Qty: qty, Price: price,
	})
	// The bid waits in the same list a strike does, so the lost-packet timer
	// (ReturnLostForces) covers it without a second mechanism.
	w.InFlight = append(w.InFlight, InFlightStrike{
		ID: id, Kind: "trade", TargetBoard: targetBoard, TargetEmpire: seller,
		LaunchedDay: w.GameDay, Owner: e.Owner,
		Gold: cost, Good: good, Qty: qty, Price: price,
	})
	return id, nil
}

// resolveRemoteTradeBid fills a bid against this board's market, or refuses it.
// It fills only what the buyer was quoted and only at that price: a listing that
// has shrunk fills partially, one that has repriced or gone fills not at all.
func (w *World) resolveRemoteTradeBid(b IPTradeBid) IPTradeFill {
	fill := IPTradeFill{ID: b.ID}
	// Through the former name too: a seller who renamed while the bid crossed
	// still holds the listing it was placed against, under its new key.
	seller := w.FindByNameOrFormer(b.Seller)
	realm := b.Seller
	if seller != nil {
		realm = seller.Name
	}
	have := w.MarketForSale(realm, b.Good)
	switch {
	// The standing is judged HERE and NOW, not as the buyer saw it when they
	// bid. An alliance broken while the bid was in transit closes this market to
	// them, the same way New Realm Protection is checked on an arriving strike
	// rather than at launch — a packet crossing takes days, and what a board owes
	// a stranger is decided by where things stand when the order lands.
	case !w.Config.IPTrading:
		fill.Reason = fmt.Sprintf("%s no longer trades with other planets.", w.Config.BoardID)
	case w.PlanetRelationWith(b.FromBoard) != PlanetAllied:
		fill.Reason = fmt.Sprintf("Our alliance with %s has ended; the market is closed to you.", b.FromBoard)
	case seller == nil || !seller.Alive:
		fill.Reason = fmt.Sprintf("%s is no longer on our planet.", b.Seller)
	case have <= 0:
		fill.Reason = fmt.Sprintf("%s no longer offers %s.", b.Seller, b.Good)
	case w.MarketPrice(realm, b.Good) != b.Price:
		fill.Reason = fmt.Sprintf("%s has repriced its %s since you were quoted.", b.Seller, b.Good)
	}
	if fill.Reason != "" {
		fill.Gold = TradeBidCost(b.Qty, b.Price)
		return fill
	}
	// A shrunken listing sells what it still has; the rest of the gold goes back.
	qty := min(b.Qty, have)
	w.takeFromMarket(realm, b.Good, qty)
	paid := TradeBidCost(qty, b.Price)
	if w.MarketProceeds == nil {
		w.MarketProceeds = map[string]int64{}
	}
	w.MarketProceeds[realm] += paid
	// The seller is told privately and the planet is not: a market sale is
	// business between two realms, and putting every one on the news both fills
	// the paper on a trading planet and tells every rival what a realm is
	// stocking up on.
	seller.addEvent(fmt.Sprintf("%s of %s bought %s %s from your market for %s gold.",
		b.FromEmpire, b.FromBoard, numfmt.Comma(int64(qty)), b.Good, numfmt.Comma(paid)))
	fill.Filled = true
	fill.Good, fill.Qty = b.Good, qty
	fill.Gold = TradeBidCost(b.Qty-qty, b.Price) // the unfilled remainder
	if qty < b.Qty {
		fill.Reason = fmt.Sprintf("%s had only %s left.", b.Seller, numfmt.Comma(int64(qty)))
	}
	return fill
}

// applyTradeFill hands a returning bid's goods and any refund to the buyer.
func (w *World) applyTradeFill(f IPTradeFill) {
	sent, waiting := w.takeInFlight(f.ID)
	if !waiting {
		// Already refunded by the lost-packet timer, or a duplicate packet.
		// Crediting now would pay twice, which is the same trap the returning
		// attack path guards (applyAttackResult).
		return
	}
	e := w.FindByOwner(sent.Owner)
	if e == nil {
		return
	}
	if f.Filled && f.Qty > 0 {
		w.giveGood(e, f.Good, f.Qty)
		e.addEvent(fmt.Sprintf("Your bid on %s came home with %s %s.",
			sent.TargetBoard, numfmt.Comma(int64(f.Qty)), f.Good))
	}
	if f.Gold > 0 {
		w.creditGold(e, f.Gold, "an unfilled interplanetary bid")
	}
	if f.Reason != "" {
		e.addEvent(f.Reason)
	}
	if !f.Filled {
		e.addEvent(fmt.Sprintf("Your bid on %s was not filled; the gold is back in your treasury.", sent.TargetBoard))
	}
}

// ExportMarket is this board's market as the other planets should see it, for
// the snapshot that rides the scores packet. Only living human realms are
// listed: a computer baron's stock is not on offer across the league.
func (w *World) ExportMarket() []RemoteListing {
	var out []RemoteListing
	for _, l := range w.Market {
		if l.Qty <= 0 {
			continue
		}
		if e := w.FindByName(l.Realm); e == nil || !e.Alive || e.Owner == "" {
			continue
		}
		out = append(out, RemoteListing{Realm: l.Realm, Good: l.Good, Qty: l.Qty, Price: l.Price})
	}
	return out
}

// RemoteMarket is the last-known market of board, or nil when this board has
// heard nothing from it.
func (w *World) RemoteMarket(board string) []RemoteListing {
	for i := range w.RemoteBoards {
		if w.RemoteBoards[i].BoardID == board {
			return w.RemoteBoards[i].Market
		}
	}
	return nil
}

// AlliedMarketBoards are the planets a baron here may bid on: those the
// Coordinator has marked Allied, in roster order, whether or not this board has
// a market snapshot for them yet (an ally that has sent none is still an ally,
// and saying so beats hiding it).
func (w *World) AlliedMarketBoards() []string {
	var out []string
	for _, b := range w.RemoteBoards {
		if b.BoardID != w.Config.BoardID && w.PlanetRelationWith(b.BoardID) == PlanetAllied {
			out = append(out, b.BoardID)
		}
	}
	return out
}
