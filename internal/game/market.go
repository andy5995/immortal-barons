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
	// LegacyOwner is the owner handle a save written before the name key carried
	// here. EnsureMarket drains it into Realm and clears it; it exists only so
	// such a save keeps its listings.
	LegacyOwner string `json:"Owner,omitempty"`
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
// buyer's own realm, in stable order — the "Choose a Target" browse list.
func (w *World) MarketSellers(good, exclude string) []*MarketListing {
	var out []*MarketListing
	for i := range w.Market {
		l := &w.Market[i]
		if l.Good == good && l.Qty > 0 && l.Realm != exclude {
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
// escrowed food spoils along with granary food (see processEconomy), so listing
// food doesn't dodge spoilage. Returns the amount removed.
func (w *World) spoilListedFood(realm string, n int) int {
	l := w.marketListing(realm, "Food")
	if l == nil || n <= 0 {
		return 0
	}
	if n > l.Qty {
		n = l.Qty
	}
	l.Qty -= n
	if l.Qty == 0 {
		w.removeListing(realm, "Food")
	}
	return n
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

// EnsureMarket migrates a save whose market was keyed by the seller's OWNER
// HANDLE onto the realm-name key (see MarketListing.Realm). A row from such a
// save arrives with Realm empty — a name never is — and its handle in
// LegacyOwner, which is the only thing that tells the two shapes apart.
//
// Where a legacy key resolves to a realm, its listings and unpaid proceeds move
// there intact. Where it does not, they are forfeit, which is what the live game
// already did with them: forgetMarketPosition wipes a departed realm's position
// and settleMarketProceeds pays nobody for it.
func (w *World) EnsureMarket() {
	kept := w.Market[:0]
	for _, l := range w.Market {
		if l.Realm == "" {
			l.Realm = w.legacyMarketRealm(l.LegacyOwner)
		}
		l.LegacyOwner = ""
		if l.Realm == "" {
			continue
		}
		kept = append(kept, l)
	}
	w.Market = kept
	for owner, gross := range w.MarketProceedsByOwner {
		realm := w.legacyMarketRealm(owner)
		if realm == "" {
			continue
		}
		if w.MarketProceeds == nil {
			w.MarketProceeds = map[string]int64{}
		}
		w.MarketProceeds[realm] += gross
	}
	w.MarketProceedsByOwner = nil
}

// legacyMarketRealm names the realm a pre-name-keyed market row belongs to, or
// "" if it belongs to nobody left.
func (w *World) legacyMarketRealm(owner string) string {
	if e := w.FindByOwner(owner); e != nil {
		return e.Name
	}
	// The empty handle was every computer baron's, so a row under it is the one
	// position the whole pool shared and no baron can be shown to own it. It goes
	// to the first living baron — the realm the pre-fix game already handed it to,
	// since that is what FindByOwner("") resolved to when the proceeds settled and
	// what could delist the goods. Handing it over keeps the units and the gold in
	// play; destroying them would take a real baron's stock with the ambiguity.
	if owner == "" {
		if ai := w.AIEmpires(); len(ai) > 0 {
			return ai[0].Name
		}
	}
	return ""
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
