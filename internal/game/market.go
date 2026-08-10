package game

import "errors"

var (
	ErrBadGood    = errors.New("That is not traded on the market.")
	ErrOwnListing = errors.New("You cannot buy from your own listing.")
	ErrNoListing  = errors.New("That empire has none of those for sale.")
)

// MarketGoods are the goods tradeable on the general Trading Market: military
// units plus food and agents (BRE-verified live, 2026-07-15). Regions and
// HeadQuarters are not tradeable. The order matches BRE's Trading Market screen.
var MarketGoods = []string{"Trooper", "Jet", "Turret", "Bomber", "Food", "Agent", "Tank", "Carrier"}

// marketField returns a pointer to e's inventory count for a market good, or nil
// if the good is not tradeable.
func marketField(e *Empire, good string) *int {
	switch good {
	case "Trooper":
		return &e.Troopers
	case "Jet":
		return &e.Jets
	case "Turret":
		return &e.Turrets
	case "Bomber":
		return &e.Bombers
	case "Food":
		return &e.Food
	case "Agent":
		return &e.Agents
	case "Tank":
		return &e.Tanks
	case "Carrier":
		return &e.Carriers
	}
	return nil
}

// MarketListing is one empire's offer of one good on the general market. The
// listed Qty is escrowed — it has already left the owner's inventory (see
// SetMarketListing), so it is held here until bought or delisted.
type MarketListing struct {
	Owner string // empire Owner handle
	Good  string
	Qty   int
	Price int // gold per unit, seller-set
}

// marketListing returns the pointer to owner's listing of good (so callers can
// mutate Qty/Price in place), or nil if none exists.
func (w *World) marketListing(owner, good string) *MarketListing {
	for i := range w.Market {
		if w.Market[i].Owner == owner && w.Market[i].Good == good {
			return &w.Market[i]
		}
	}
	return nil
}

// MarketForSale is the quantity owner currently has listed for good.
func (w *World) MarketForSale(owner, good string) int {
	if l := w.marketListing(owner, good); l != nil {
		return l.Qty
	}
	return 0
}

// MarketPrice is owner's set price for good (0 if not listed).
func (w *World) MarketPrice(owner, good string) int {
	if l := w.marketListing(owner, good); l != nil {
		return l.Price
	}
	return 0
}

// MarketTotalForSale is the planet-wide pool of good on the market (all empires).
func (w *World) MarketTotalForSale(good string) int {
	total := 0
	for i := range w.Market {
		if w.Market[i].Good == good {
			total += w.Market[i].Qty
		}
	}
	return total
}

// MarketSellers returns the live listings for good (Qty > 0), excluding the
// buyer's own, in stable order — the "Choose a Target" browse list.
func (w *World) MarketSellers(good, exclude string) []*MarketListing {
	var out []*MarketListing
	for i := range w.Market {
		l := &w.Market[i]
		if l.Good == good && l.Qty > 0 && l.Owner != exclude {
			out = append(out, l)
		}
	}
	return out
}

// SetMarketListing sets e's For Sale quantity and price for good. The goods are
// escrowed: e's inventory and its listed quantity trade off so that
// inventory + listed is conserved. newQty is the new total For Sale; it is
// clamped to what e can back (current inventory + already-listed). A newQty of 0
// removes the listing and returns the goods to inventory.
func (w *World) SetMarketListing(e *Empire, good string, newQty, price int) error {
	f := marketField(e, good)
	if f == nil {
		return ErrBadGood
	}
	if newQty < 0 {
		newQty = 0
	}
	if price < 0 {
		price = 0
	}
	l := w.marketListing(e.Owner, good)
	cur := 0
	if l != nil {
		cur = l.Qty
	}
	if max := *f + cur; newQty > max {
		newQty = max
	}
	*f += cur - newQty // move goods between inventory and escrow
	switch {
	case newQty == 0 && l != nil:
		w.removeListing(e.Owner, good)
	case newQty > 0 && l == nil:
		w.Market = append(w.Market, MarketListing{Owner: e.Owner, Good: good, Qty: newQty, Price: price})
	case l != nil:
		l.Qty, l.Price = newQty, price
	}
	return nil
}

func (w *World) removeListing(owner, good string) {
	for i := range w.Market {
		if w.Market[i].Owner == owner && w.Market[i].Good == good {
			w.Market = append(w.Market[:i], w.Market[i+1:]...)
			return
		}
	}
}

// BuyFromMarket buys n of good from sellerOwner's listing: the buyer pays gold
// now, receives the goods, and the seller's proceeds accrue to be deposited at
// daily maintenance (see settleMarketProceeds). n is clamped to the listing and
// to what the buyer can afford. Buying your own listing is refused (as in BRE).
func (w *World) BuyFromMarket(buyer *Empire, sellerOwner, good string, n int) error {
	bf := marketField(buyer, good)
	if bf == nil {
		return ErrBadGood
	}
	if sellerOwner == buyer.Owner {
		return ErrOwnListing
	}
	l := w.marketListing(sellerOwner, good)
	if l == nil || l.Qty <= 0 {
		return ErrNoListing
	}
	if n <= 0 {
		return nil
	}
	if n > l.Qty {
		n = l.Qty
	}
	cost := goldCost(n, l.Price)
	if buyer.Gold < cost {
		return ErrCantAfford
	}
	buyer.Gold -= cost
	*bf += n
	l.Qty -= n
	if w.MarketProceeds == nil {
		w.MarketProceeds = map[string]int64{}
	}
	w.MarketProceeds[sellerOwner] += cost
	if l.Qty == 0 {
		w.removeListing(sellerOwner, good)
	}
	return nil
}

// spoilListedFood removes up to n units of food from owner's market listing —
// escrowed food spoils along with granary food (see processEconomy), so listing
// food doesn't dodge spoilage. Returns the amount removed.
func (w *World) spoilListedFood(owner string, n int) int {
	l := w.marketListing(owner, "Food")
	if l == nil || n <= 0 {
		return 0
	}
	if n > l.Qty {
		n = l.Qty
	}
	l.Qty -= n
	if l.Qty == 0 {
		w.removeListing(owner, "Food")
	}
	return n
}

// bombMarketPosition destroys pct% of d's listed goods (per listing) and pct% of
// its pending sale proceeds, returning the totals wiped. BRE's Bomb Enemy Trade
// Market "destroys a portion of all goods stored in an opposing planet's trading
// market" (community strategy guide).
func (w *World) bombMarketPosition(d *Empire, pct int) (goods int, proceeds int64) {
	for i := range w.Market {
		if w.Market[i].Owner == d.Owner {
			loss := w.Market[i].Qty * pct / 100
			w.Market[i].Qty -= loss
			goods += loss
		}
	}
	if w.MarketProceeds != nil {
		loss := pctOf(w.MarketProceeds[d.Owner], pct)
		w.MarketProceeds[d.Owner] -= loss
		proceeds += loss
	}
	return goods, proceeds
}

// settleMarketProceeds deposits each seller's accrued market proceeds (minus the
// market's commission) and clears the pool. Called once per day from
// DailyMaintenance — BRE settles at day rollover ("Depositing trading market
// money", a sysop-maintenance step, verified live). A seller whose empire is gone
// simply forfeits its proceeds.
func (w *World) settleMarketProceeds() {
	for owner, gross := range w.MarketProceeds {
		e := w.FindByOwner(owner)
		if e == nil {
			continue
		}
		w.creditGold(e, pctOf(gross, 100-MarketCommissionPct), "the trading market")
	}
	w.MarketProceeds = nil
}
