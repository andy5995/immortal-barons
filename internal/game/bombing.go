package game

// bombing.go — the target-side effects a bombing op has, shared by the local
// Bomb Enemy Targets covert op and its InterPlanetary terror counterparts (#49).
//
// They live apart from covert.go because they are not the covert menu's alone:
// ibbs_special.go runs them for a strike that arrived in a packet, with no
// agent and no covert roll involved, and diplomacy.go consults the trade-route
// one. What stays in covert.go is the covert OP itself — BombEnemyTargets,
// resolveBombEnemyTargets and the six-slot target table they roll on — which is
// a Covert Operations menu item and nothing else.
//
// Each function here is the part of an op that touches the TARGET only: no
// attacker, no success roll, no fee. That is exactly the part a board can run
// for a strike it received, and keeping one copy is what stops the two menus
// drifting apart when a number is tuned.

// The effects the local Bomb Enemy Targets ops and their interplanetary
// counterparts share (#49). Each one is the part of an op that touches the
// TARGET only — no attacker, no success roll, no fee — which is exactly the
// part a board can run for a strike that arrived in a packet. Keeping them here
// rather than duplicating the arithmetic is what stops the two menus drifting
// apart when a number is tuned.

// bombFoodEffect burns half of d's food reserve and reports what was lost.
func bombFoodEffect(d *Empire) int {
	lost := d.Food / 2
	d.Food -= lost
	return lost
}

// bombRoutesLands reports whether a trade-route strike comes to anything at
// all. BINARY-VERIFIED (BRE.OVR 0x04a09a): the original rolls this once at the
// top of the routine that resolves a received bombing op, before it looks at a
// single deal, and two strikes in three end there.
func (w *World) bombRoutesLands() bool {
	return w.rng.Intn(BombRoutesLandOdds) == 0
}

// bombRoutesEffect wrecks the goods riding in pending trade deals and reports
// how many deals it hit: every deal on the planet when only is nil, otherwise
// every deal `only` is a party to, whichever side of it that realm is on.
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
//
// Scope is IB's own call, not BRE's. The original's Bomb Trade Routes is
// interplanetary and planet-wide, so it never had to say which deals a strike
// against ONE realm reaches, and its local Covert item 7 is a different op
// entirely. IB takes both sides of a deal — a realm's trade routes run in both
// directions, and counting only inbound deals would let a realm dodge the op by
// never accepting one. The local covert op and its interplanetary counterpart
// call this same helper, so neither menu can become the cheaper way to do it.
func (w *World) bombRoutesEffect(only *Empire) (hit int) {
	for _, to := range w.Empires {
		if !to.Alive {
			continue
		}
		for i := range to.TradeDeals {
			deal := &to.TradeDeals[i]
			from := w.FindByName(deal.From)
			if only != nil && to != only && from != only {
				continue
			}
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
