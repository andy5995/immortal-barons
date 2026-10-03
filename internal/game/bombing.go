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
// rolled once, and reports the units lost and the share rolled (see
// BombFoodMarketLossPctMin).
func (w *World) bombFoodMarketEffect() (lost, pct int) {
	pct = w.bombFoodMarketLossPct()
	lost = foodMarketLoss(w.FoodMarketSupply, pct)
	w.FoodMarketSupply -= lost
	return lost, pct
}

// foodMarketLoss is Trunc(supply x (pct / 100)) in the original's Real48, the
// food a run destroys. BINARY-VERIFIED (resolve_received_bombing +0x152..+0x187:
// RFloat(pct) / 100.0, times RFloat(supply), RTrunc). The rounded pct / 100 can
// put a product that should land on a whole number just under it, so this is
// one lower than integer math there (5,500 at 73% loses 4,014, not 4,015).
func foodMarketLoss(supply, pct int) int {
	x := r48Div(r48Int(int64(pct)), r48Int(100))
	return int(r48Trunc(r48Mul(r48Int(int64(supply)), x)))
}

// marketKept is Trunc(qty x (100 - pct) / 100), what a bombed listing keeps.
func marketKept(qty, pct int) int { return pctOf(qty, 100-pct) }

// undermineKept is Round((x / 100) x (100 - pct)) in the original's Real48,
// halves away from zero, what an undermined investment keeps. BINARY-VERIFIED
// (resolve_received_bombing +0x33a..+0x391: RFloat(value) / 100.0, times
// RFloat(100 - pct), RRound). The rounded x / 100 can put an exact half just
// under it, so this is one lower than integer math there (2,130 at 5% keeps
// 2,023, not 2,024).
func undermineKept(x int64, pct int) int64 {
	v := r48Mul(r48Div(r48Int(x), r48Int(100)), r48Int(int64(100-pct)))
	return r48Round(v)
}

// bombFoodMarketLossPct is the one 20-99% share a Bomb Food Market run burns.
func (w *World) bombFoodMarketLossPct() int {
	return BombFoodMarketLossPctMin + w.rng.Intn(BombFoodMarketLossPctSpread)
}

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
// UndermineReachDays from maturity to undermineKept, Round((x / 100) x
// (100 - pct)) in Real48, principal and return alike, and reports the principal
// lost. The original keeps one figure
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

// bombRoutesEffect damages the goods riding in the planet's pending trade deals
// and reports how many deals it hit.
//
// BINARY-VERIFIED against BRE.OVR 0x051077, which walks the pending deals and,
// for each, rolls a `random(3)` that lets one deal in three escape, skips a deal
// whose own two parties hold Protective Trade, and otherwise takes the run's
// one percentage off each of the nine goods in the deal's SEND basket. The
// percentage is drawn once by the caller (resolve_received_bombing +0x2bd) and
// shared by every deal. What the deal demands back is not in transit and is
// not touched. Nothing is refunded and no per-deal message is filed.
//
// The Protective Trade guard is a property of the DEAL, not of the attacker: it
// reads the relation between the deal's sender and recipient (record fields +8
// and +9), never the firing realm's own. So a deal between a pair who hold the
// pact survives a strike from anyone, and holding it with the victim buys the
// attacker nothing.
func (w *World) bombRoutesEffect() (hit int) {
	pct := BombRoutesLossPctMin + w.rng.Intn(BombRoutesLossPctSpread)
	// Every realm still on the roster, dead ones too: the walker reads every
	// trade-offer record with no alive test (ovr_050dfb +0x214..+0x32f), and a
	// dead realm's pending deals go home to their senders when its husk is
	// removed (returnPendingDeals), so what a strike takes off them is real.
	for _, to := range w.Empires {
		for i := range to.TradeDeals {
			deal := &to.TradeDeals[i]
			from := w.FindByName(deal.From)
			// Random(3) = 0 spares the deal (BRE.OVR ovr_050dfb +0x246..+0x256);
			// IB had the test inverted until 2026-09-24 and hit one deal in three.
			if w.rng.Intn(BombRoutesDealEscapeOdds) == 0 {
				continue
			}
			if from != nil && w.HasTreaty(from, to, protectiveTrade) {
				continue
			}
			bombDealBasket(&deal.Send, pct)
			hit++
		}
	}
	return hit
}

// bombDealBasket takes pct percent off every good in b, gold included:
// each loses Trunc(pct x (qty / 100)), as the original subtracts it.
//
// The original computes that in Real48 (ovr_050dfb +0x2a8..+0x2e2: RFloat(qty)
// / 100.0, times RFloat(pct), RTrunc), the same inexact divide the food market
// takes. Here it never changes the answer: at pct 5-9, integer math matches
// the Real48 port for every quantity to 2^31 - 1, the original's ceiling, so
// this stays pctOf.
func bombDealBasket(b *TradeBasket, pct int) {
	for _, p := range basketPtrs(b) {
		*p -= pctOf(*p, pct)
	}
	b.Gold -= pctOf(b.Gold, pct)
}
