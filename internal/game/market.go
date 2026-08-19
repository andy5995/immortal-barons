package game

import "errors"

var (
	ErrBadGood    = errors.New("That is not traded on the market.")
	ErrOwnListing = errors.New("You cannot buy from your own listing.")
	ErrNoListing  = errors.New("That empire has none of those for sale.")
	// New Realm Protection stops a realm trading, not only fighting: BRE's own
	// reset help defines the setting as the turns for which a new empire "is
	// unable to attack, trade, and be attacked". Every path that moves goods
	// between realms checks this.
	ErrInProtection  = errors.New("You cannot trade while under new realm protection.")
	ErrTheyProtected = errors.New("That realm is under new realm protection.")
	// See MarketAccessTreaties: buying from a listing needs one of the four
	// non-trade pacts. IB's own rule, not BRE's.
	ErrNoMarketPact = errors.New("You need a pact with that realm to buy from its listing.")
	// BRE refuses a trade deal between realms with no relation at all.
	ErrNoRelations = errors.New("You need a pact with that realm to send it a trade deal.")
)

// MarketListing is one empire's offer of one good on the general market. The
// listed Qty is escrowed — it has already left the seller's inventory (see
// SetMarketListing), so it is held here until bought or delisted.
type MarketListing struct {
	// Realm is the seller's realm NAME, which is what identifies a market
	// position — not the owner handle this used to key on. A handle cannot tell
	// two computer barons apart: newEmpire gives every one of them the empty
	// handle, and that emptiness is also what MARKS a baron as AI (turn.go,
	// game.go, ibbs.go, dupe.go), so it cannot be made unique without moving the
	// marker. Sharing one key made the pool share one position: each baron's
	// listing overwrote the last, and the goods the overwritten row held had
	// already left its owner's inventory, so they ceased to exist. Realm names
	// are unique (RealmNameTaken guards onboarding) and every baron has one,
	// which is why w.Treaties keys on the name too.
	Realm string
	Good  string
	Qty   int
	Price int // gold per unit, seller-set
}

// marketListing returns the pointer to realm's listing of good (so callers can
// mutate Qty/Price in place), or nil if none exists.
func (w *World) marketListing(realm, good string) *MarketListing {
	for i := range w.Market {
		if w.Market[i].Realm == realm && w.Market[i].Good == good {
			return &w.Market[i]
		}
	}
	return nil
}

// MarketForSale is the quantity realm currently has listed for good.
func (w *World) MarketForSale(realm, good string) int {
	if l := w.marketListing(realm, good); l != nil {
		return l.Qty
	}
	return 0
}

// MarketPrice is realm's set price for good (0 if not listed).
func (w *World) MarketPrice(realm, good string) int {
	if l := w.marketListing(realm, good); l != nil {
		return l.Price
	}
	return 0
}

// visibleOnMarketTo reports whether viewer may see seller's listings at all. In
// a single-board game a realm sees only what it could actually buy: a listing it
// has no pact for is not a missed opportunity, it is noise, and showing the
// quantities would leak what a rival holds. A league board shows the whole
// market, the same place BuyFromMarket stops gating.
func (w *World) visibleOnMarketTo(viewer *Empire, seller string) bool {
	if w.Config.IBBS || seller == viewer.Name {
		return true
	}
	s := w.FindByName(seller)
	return s != nil && w.CanBuyOnMarketFrom(viewer, s)
}

// MarketTotalForSale is the pool of good on the market as viewer sees it: the
// whole planet's, including their own listing, minus the realms a single-board
// game hides from them. It sits beside "your own For Sale" on the market screen,
// so it must keep counting the viewer's goods.
func (w *World) MarketTotalForSale(good string, viewer *Empire) int {
	total := 0
	for i := range w.Market {
		l := &w.Market[i]
		if l.Good == good && w.visibleOnMarketTo(viewer, l.Realm) {
			total += l.Qty
		}
	}
	return total
}

// MarketSellers returns the live listings for good (Qty > 0) that viewer may
// see, excluding their own realm, in stable order — the "Choose a Target"
// browse list.
func (w *World) MarketSellers(good string, viewer *Empire) []*MarketListing {
	var out []*MarketListing
	for i := range w.Market {
		l := &w.Market[i]
		if l.Good == good && l.Qty > 0 && l.Realm != viewer.Name && w.visibleOnMarketTo(viewer, l.Realm) {
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
	if e.Protection > 0 {
		return ErrInProtection
	}
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
	l := w.marketListing(e.Name, good)
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
		w.removeListing(e.Name, good)
	case newQty > 0 && l == nil:
		w.Market = append(w.Market, MarketListing{Realm: e.Name, Good: good, Qty: newQty, Price: price})
	case l != nil:
		l.Qty, l.Price = newQty, price
	}
	return nil
}

func (w *World) removeListing(realm, good string) {
	for i := range w.Market {
		if w.Market[i].Realm == realm && w.Market[i].Good == good {
			w.Market = append(w.Market[:i], w.Market[i+1:]...)
			return
		}
	}
}

// BuyFromMarket buys n of good from seller's listing (named by realm): the buyer
// pays gold now, receives the goods, and the seller's proceeds accrue to be
// deposited at daily maintenance (see settleMarketProceeds). n is clamped to the
// listing and to what the buyer can afford. Buying your own listing is refused
// (as in BRE).
func (w *World) BuyFromMarket(buyer *Empire, seller, good string, n int) error {
	if buyer.Protection > 0 {
		return ErrInProtection
	}
	bf := marketField(buyer, good)
	if bf == nil {
		return ErrBadGood
	}
	if seller == buyer.Name {
		return ErrOwnListing
	}
	l := w.marketListing(seller, good)
	if l == nil || l.Qty <= 0 {
		return ErrNoListing
	}
	// Single-board games only. In a league the whole planet is one team, so an
	// open listing already reaches only teammates and the gate would buy nothing
	// while getting in the way of the trading a league runs on. The interplanetary
	// market is a separate path (resolveRemoteTradeBid) with its own alliance
	// check, and never comes through here.
	if !w.Config.IBBS {
		if s := w.FindByName(seller); s == nil || !w.CanBuyOnMarketFrom(buyer, s) {
			return ErrNoMarketPact
		}
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
	w.MarketProceeds[seller] += cost
	if l.Qty == 0 {
		w.removeListing(seller, good)
	}
	return nil
}

// spoilListedFood removes up to n units of food from realm's market listing —
// escrowed food spoils along with the food in hand (see processEconomy), so listing
// food doesn't dodge spoilage. Returns the amount removed.
func (w *World) spoilListedFood(realm string, n int) int {
	return w.takeFromMarket(realm, "Food", n)
}

// takeFromMarket removes up to n units of good from realm's listing, dropping
// the listing when it empties, and reports how many it actually took. The goods
// are simply gone as far as the market is concerned — where they end up is the
// caller's business: spoilage destroys them, an interplanetary bid ships them.
func (w *World) takeFromMarket(realm, good string, n int) int {
	l := w.marketListing(realm, good)
	if l == nil || n <= 0 {
		return 0
	}
	if n > l.Qty {
		n = l.Qty
	}
	l.Qty -= n
	if l.Qty == 0 {
		w.removeListing(realm, good)
	}
	return n
}

// giveGood adds n units of a market good to e's inventory. Unknown goods are
// ignored rather than silently landing somewhere wrong.
func (w *World) giveGood(e *Empire, good string, n int) {
	if f := marketField(e, good); f != nil && n > 0 {
		*f += n
	}
}

// bombMarketPosition destroys pct% of d's listed goods (per listing) and pct% of
// its pending sale proceeds, returning the totals wiped. BRE's Bomb Enemy Trade
// Market "destroys a portion of all goods stored in an opposing planet's trading
// market" (community strategy guide).
func (w *World) bombMarketPosition(d *Empire, pct int) (goods int, proceeds int64) {
	for i := range w.Market {
		if w.Market[i].Realm == d.Name {
			loss := w.Market[i].Qty * pct / 100
			w.Market[i].Qty -= loss
			goods += loss
		}
	}
	if w.MarketProceeds != nil {
		loss := pctOf(w.MarketProceeds[d.Name], pct)
		w.MarketProceeds[d.Name] -= loss
		proceeds += loss
	}
	return goods, proceeds
}

// forgetMarketPosition destroys what the market still holds for a departed
// realm: the goods escrowed in its listings and the sale gold not yet paid out.
// Called from dropEmpires for every removed realm — see the reasoning there for
// why a leftover position is not merely untidy.
//
// Both are destroyed rather than paid to anyone or left standing as unowned
// stock, because deletion in BRE is a full wipe: it clears the departing
// player's trade offers along with their messages and reports, and leaves them
// nothing. Leaving the listings would be worse than untidy — a successor holding
// that key can delist them straight into its own inventory, and any sale off them
// accrues to the key and settles into its pocket.
//
// This took an empty-handle guard while the market was keyed by the owner
// handle, because every computer baron carried the empty one and a wipe on it
// would have taken the whole living pool's listings with it. The realm name is
// per-baron, so the guard is gone and a dead baron's listing no longer outlives
// it.
func (w *World) forgetMarketPosition(realm string) {
	kept := w.Market[:0]
	for _, l := range w.Market {
		if l.Realm == realm {
			continue
		}
		kept = append(kept, l)
	}
	w.Market = kept
	delete(w.MarketProceeds, realm)
}

// EnsureMarket drops any listing that names no realm. Nothing migrates a market
// written before the realm-name key — no released version's market had been
// traded on, so there is nothing to carry over — but such a row would still LOAD,
// with Realm empty where its old Owner key used to be. That is worse than losing
// it: MarketTotalForSale counts every row whatever it is keyed on, so the goods
// would sit on the shelf, be purchasable from a seller who does not exist, and
// pay their gold into a realm nobody can be.
func (w *World) EnsureMarket() {
	kept := w.Market[:0]
	for _, l := range w.Market {
		if l.Realm != "" {
			kept = append(kept, l)
		}
	}
	w.Market = kept
}

// settleMarketProceeds deposits each seller's accrued market proceeds (minus the
// market's commission) and clears the pool. Called once per day from
// DailyMaintenance — BRE settles at day rollover ("Depositing trading market
// money", a sysop-maintenance step, verified live). A seller whose empire is gone
// simply forfeits its proceeds.
func (w *World) settleMarketProceeds() {
	for realm, gross := range w.MarketProceeds {
		e := w.FindByName(realm)
		if e == nil {
			continue
		}
		w.creditGold(e, pctOf(gross, 100-MarketCommissionPct), "the trading market")
	}
	w.MarketProceeds = nil
}
