package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// actions_iptradedeal.go — the InterPlanetary Operations menu's Send Trade Deal.
//
// This item ran the LOCAL trade action until #195, so the interplanetary menu
// offered a deal to a neighbour on the sender's own planet. The two are separate
// routines in the original with different economics, and only this one is
// reachable from that menu (BRE.OVR 0x024212, sole caller run_interbbs_menu).
// See game.SendIPTradeDeal for what the mechanic is and where IB diverges.
//
// The item stays on '3'. A live capture of the original's interplanetary menu
// has Send Trade Deal there and no trading submenu on that screen at all; IB's
// Trading submenu is IB's own and covers markets and bids, a different mechanic
// the sysop can switch off. Moving the item under it would take the deal away
// with that switch and shift every hotkey below it.

// protectedNoDeal is the refusal for a realm the last scores packet had under
// New Realm Protection. Takes the realm name.
const protectedNoDeal = "%s is under New Realm Protection and cannot receive a deal yet."

// sendIPTradeDeal runs the original's order: the planet, then a realm on it,
// then the goods, then the price and the confirm.
func sendIPTradeDeal(s session.Session, w *ctx) Result {
	if !w.Config.IBBS {
		fail(s, game.ErrNotInterBBSGame)
		return Stay
	}
	board, baron := pickIPDealTarget(s, w)
	if board == "" || baron == "" {
		return Stay
	}
	// One basket, not two: an interplanetary deal may demand nothing in return
	// (docs/bre.doc:2088), so there is no Request half to fill in.
	goods := buildTradeBasket(s, w, "Goods you ship:", true)
	if goods.IsEmpty() {
		return Stay
	}
	var cost int64
	var carriers, held int
	w.With(func() {
		cost = w.World.IPTradeDealCost(goods)
		carriers = game.TradeDealCarriers(goods)
		if p := w.Player(); p != nil {
			held = p.Carriers
		}
	})
	fmt.Fprintf(s, "\n%s"+tr(s, "You ship %s to %s of %s.")+"%s\n",
		ansi.FgBrightCyan, basketSummary(s, goods), baron, board, ansi.Reset)
	// The original prices the shipment before asking, and the carrier line is
	// its own ("Trade Deal requires N Carriers"). Both figures move with the
	// cargo, so a player who cannot afford one can go back and ship less.
	fmt.Fprintf(s, "%s"+tr(s, "Sending costs %s gold and %d carriers; you own %d.")+"%s\n",
		ansi.Dim, comma(cost), carriers, held, ansi.Reset)
	// Say plainly that this is one-way. It is the difference from the local deal
	// that a player is most likely to be caught by: nothing comes back, and the
	// other side is not asked.
	fmt.Fprintf(s, "%s%s%s\n", ansi.Dim,
		tr(s, "The goods are a gift: the other realm cannot refuse them and sends nothing back."), ansi.Reset)
	if !AskYesNo(s, "Send this trade deal?", true) {
		return Stay
	}
	// Re-resolved under the lock: the basket was priced from a snapshot, and
	// another node may have spent the gold or the carriers since.
	err := w.mutatePlayer(func(p *game.Empire) error {
		return w.World.SendIPTradeDeal(p, board, baron, goods)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Your shipment is on its way to %s of %s.", baron, board)
	return Stay
}

// pickIPDealTarget asks for a planet and then a realm on it, through the same
// walk the war menus use, so a protected realm carries the same `(P)` flag it
// carries everywhere else (#214) and the list cannot drift out of step with
// theirs. Protection is a bar here — Andy's call, and a divergence from the
// original, which lets the deal go and destroys it on arrival (see
// game.SendIPTradeDeal).
func pickIPDealTarget(s session.Session, w *ctx) (board, baron string) {
	return pickRemoteBaronOn(s, w, "Ship to which planet?", "Ship to which baron?", protectedNoDeal)
}
