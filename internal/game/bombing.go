package game

// bombing.go — what an interplanetary bombing run does to the planet it lands
// on: the landing roll every run meets, and the effect of each of the four
// Special Operations that pass it (#49). ibbs_special.go calls them for a run
// that arrived in a packet; diplomacy.go consults the trade-route one.
//
// The original's LOCAL Covert menu shares none of this. Its Bomb Enemy Targets
// is one op with its own six-slot table (covert.go), and the four bombing ops
// here belong to the InterPlanetary Special Operations menu alone.
//
// Each function is the part of a run that touches the TARGET planet only: no
// attacker, no fee. The figures are the original's receiver's, read from
// resolve_received_bombing (BRE.OVR 0x04a09a); see balance_prices.go.

// bombFoodMarketEffect burns a 20-99% share of the planet's food-market supply,
// rolled once, and reports the units lost (see BombFoodMarketLossPctMin).
func (w *World) bombFoodMarketEffect() int {
	lost := foodMarketLoss(w.FoodMarketSupply, BombFoodMarketLossPctMin+w.rng.Intn(BombFoodMarketLossPctSpread))
	w.FoodMarketSupply -= lost
	return lost
}

// foodMarketLoss is Trunc(supply x pct / 100), the food a run destroys.
func foodMarketLoss(supply, pct int) int { return int(int64(supply) * int64(pct) / 100) }

// marketKept is Trunc(qty x (100 - pct) / 100), what a bombed listing keeps.
func marketKept(qty, pct int) int { return int(int64(qty) * int64(100-pct) / 100) }

// undermineKept is Round(x x (100 - pct) / 100), halves away from zero, what an
// undermined investment keeps.
func undermineKept(x int64, pct int) int64 { return (2*x*int64(100-pct) + 100) / 200 }

// bombMarketLossPct is the one 5-9% share a Bomb Trading Market run takes off
// every listing it reaches.
func (w *World) bombMarketLossPct() int {
	return BombMarketLossPctMin + w.rng.Intn(BombMarketLossPctSpread)
}

// undermineLossPct is the one 2-5% share an Undermine Investments run takes off
// every investment it reaches.
func (w *World) undermineLossPct() int {
	return UndermineLossPctMin + w.rng.Intn(UndermineLossPctSpread)
}

// undermineEffect cuts each of d's investments that is at most
// UndermineReachDays from maturity to Round(x x (100 - pct) / 100), principal and
// return alike, and reports the principal lost. The original keeps one figure
// per day left to maturity and cuts the first four; IB keeps each investment
// apart, so it cuts every one in that window.
func (w *World) undermineEffect(d *Empire, pct int) int64 {
	var lost int64
	for i := range d.Investments {
		inv := &d.Investments[i]
		if inv.MaturesDay-w.GameDay > UndermineReachDays {
			continue
		}
		kept := undermineKept(inv.Amount, pct)
		lost += inv.Amount - kept
		inv.Amount = kept
		inv.Return = undermineKept(inv.Return, pct)
	}
	return lost
}

// bombingLands reports whether an arriving bombing run comes to anything at
// all. BINARY-VERIFIED (BRE.OVR 0x04a09a, see BombingLandOdds): the original
// rolls it once at the top of the routine that resolves a received bombing op,
// ahead of the switch on which of the four ops it is, and two runs in three end
// there whichever op was sent.
func (w *World) bombingLands() bool {
	return w.rng.Intn(BombingLandOdds) == 0
}

// bombRoutesEffect wrecks the goods riding in the planet's pending trade deals
// and reports how many deals it hit.
//
// BINARY-VERIFIED against BRE.OVR 0x051077, which walks the pending deals and,
// for each, rolls a `random(3)` that lets one deal in three escape, skips a deal
// whose own two parties hold Protective Trade, and otherwise cuts every one of
// its goods quantities to bombRoutesKeptPct. Nothing is refunded and no
// per-deal message is filed.
//
// The Protective Trade guard is a property of the DEAL, not of the attacker: it
// reads the relation between the deal's sender and recipient (record fields +8
// and +9), never the firing realm's own. So a deal between a pair who hold the
// pact survives a strike from anyone, and holding it with the victim buys the
// attacker nothing.
func (w *World) bombRoutesEffect() (hit int) {
	for _, to := range w.Empires {
		if !to.Alive {
			continue
		}
		for i := range to.TradeDeals {
			deal := &to.TradeDeals[i]
			from := w.FindByName(deal.From)
			if w.rng.Intn(BombRoutesDealHitOdds) != 0 {
				continue
			}
			if from != nil && w.HasTreaty(from, to, protectiveTrade) {
				continue
			}
			w.bombDealBasket(&deal.Send)
			w.bombDealBasket(&deal.Demand)
			hit++
		}
	}
	return hit
}

// bombDealBasket cuts every good in b to the sliver a bombed deal keeps. The
// share is rolled per good, as the original rolls it inside its own loop over
// the deal's quantities.
func (w *World) bombDealBasket(b *TradeBasket) {
	for _, p := range basketPtrs(b) {
		*p = pctOf(*p, w.bombRoutesKeptPct())
	}
	b.Gold = pctOf(b.Gold, w.bombRoutesKeptPct())
}

// bombRoutesKeptPct is the percentage of one good a bombed deal keeps: 5-9%.
func (w *World) bombRoutesKeptPct() int {
	return BombRoutesKeptPctMin + w.rng.Intn(BombRoutesKeptPctSpread)
}
