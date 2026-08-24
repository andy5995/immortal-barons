package menu

import (
	"fmt"
	"sort"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// Interplanetary trading (#47) — IB's own. See internal/game/ibbs_trade.go for
// why an order is a bid rather than a purchase.

// ipMarkets is the Markets screen: pick one of the planets the Coordinator has
// marked Allied, then browse what its barons are offering and bid.
//
// Only allied planets are listed. That is the whole rule — a market you can
// reach is a market you have an alliance with — so the screen says so plainly
// when the list is empty rather than showing an empty box.
func ipMarkets(s session.Session, w *ctx) Result {
	var boards []string
	w.With(func() { boards = w.World.AlliedMarketBoards() })
	if len(boards) == 0 {
		ok(s, "No planet is marked Allied, so no market is open to you.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Allied Markets"), ansi.Reset)
	board := pickAddressee(s, w, boards)
	if board == "" {
		return Stay
	}
	browseRemoteMarket(s, w, board)
	return Stay
}

// browseRemoteMarket shows one planet's last-known market and takes a bid.
func browseRemoteMarket(s session.Session, w *ctx, board string) {
	var rows []game.RemoteListing
	var asOf string
	w.With(func() {
		rows = append(rows, w.World.RemoteMarket(board)...)
		for _, b := range w.World.RemoteBoards {
			if b.BoardID == board {
				asOf = b.Date
			}
		}
	})
	if len(rows) == 0 {
		ok(s, "Nothing has been offered on %s, or no news has reached us yet.", board)
		return
	}
	// Stable order so the numbers a player reads are the numbers they type.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Realm != rows[j].Realm {
			return rows[i].Realm < rows[j].Realm
		}
		return rows[i].Good < rows[j].Good
	})

	// This is the local Trading Market seen from another planet, so it is drawn
	// as that screen is drawn — same title weight, same headings ("Empire Name"
	// and "Name" are what the local market and its seller list call these two
	// columns), same inset rule in the dim market accent, same colors per column.
	// A player should recognise it as the market, not meet a second convention.
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightWhite, fmt.Sprintf(tr(s, "%s Trading Market"), game.FitColumn(board, 60)), ansi.Reset)
	if asOf != "" {
		// The date is not decoration: it is how stale the prices are, and staleness
		// is what makes a bid fail.
		fmt.Fprintf(s, "%s%s%s\n", ansi.FgWhite, fmt.Sprintf(tr(s, "As we last heard, %s."), asOf), ansi.Reset)
	}
	fmt.Fprintf(s, "%s%-5s%-20s%-12s%12s %10s%s\n", ansi.FgBrightWhite,
		tr(s, "Key"), tr(s, "Empire Name"), tr(s, "Name"), tr(s, "For Sale"), tr(s, "Price"), ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", dim(marketAccent), remoteMarketRule(), ansi.Reset)
	for i, r := range rows {
		// The number is padded rather than bare: a market with ten or more
		// listings would otherwise step every column right at "(10)".
		fmt.Fprintf(s, "%s(%s%2d%s)%s %s%-20s%-12s%s%s%12s %10s%s\n",
			ansi.FgYellow, ansi.FgBrightYellow, i+1, ansi.FgYellow, ansi.Reset,
			ansi.FgWhite, r.Realm, tr(s, r.Good), ansi.Reset,
			ansi.FgBrightYellow, comma(r.Qty), comma(r.Price), ansi.Reset)
	}
	fmt.Fprintf(s, "%s%s%s\n", dim(marketAccent), remoteMarketRule(), ansi.Reset)

	pick := promptSuggested(s, tr(s, "Bid on which listing?"), 0, len(rows))
	if pick <= 0 {
		return
	}
	r := rows[pick-1]
	var gold int64
	w.With(func() { gold = w.Player().Gold })
	most := r.Qty
	if afford := int(gold / int64(max(r.Price, 1))); afford < most {
		most = afford
	}
	if most <= 0 {
		ok(s, "You cannot afford one unit at that price.")
		return
	}
	qty := promptSuggested(s, tr(s, "Bid for how many?"), 0, most)
	if qty <= 0 {
		return
	}
	cost := game.TradeBidCost(qty, r.Price)
	fmt.Fprintf(s, "\n%s\n", hiNums(fmt.Sprintf(
		tr(s, "Bidding %s gold for %s %s from %s."), comma(cost), comma(qty), tr(s, r.Good), r.Realm)))
	okNoPause(s, "The gold is held until the answer comes back.")
	if !AskYesNo(s, "Send the bid?", true) {
		return
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		_, e := w.World.SendTradeBid(p, board, r.Realm, r.Good, qty, r.Price)
		return e
	})
	if err != nil {
		fail(s, err)
		return
	}
	ok(s, "Your bid is away. If %s no longer offers it at that price, your gold comes back.", r.Realm)
}

// An allied planet's market carries a realm column the local one does not, so it
// is wider than the seller list and narrower than the full market — sized to its
// own content, as every box here is.
const (
	remoteMarketRuleWidth  = 60
	remoteMarketRuleDouble = 12
)

func remoteMarketRule() string { return insetRule(remoteMarketRuleWidth, remoteMarketRuleDouble) }

// ipPendingBids lists the bids this baron has out, so the held gold is never a
// mystery. It reads the same in-flight list the lost-packet timer sweeps.
func ipPendingBids(s session.Session, w *ctx) Result {
	type row struct {
		board, seller, good string
		qty                 int
		gold                int64
	}
	var rows []row
	var total int64
	w.With(func() {
		p := w.Player()
		if p == nil {
			return
		}
		for _, f := range w.World.InFlight {
			if f.Kind == "trade" && f.Owner == p.Owner {
				rows = append(rows, row{f.TargetBoard, f.TargetEmpire, f.Good, f.Qty, f.Gold})
				total += f.Gold
			}
		}
	})
	if len(rows) == 0 {
		ok(s, "You have no bids out.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Bids Awaiting an Answer"), ansi.Reset)
	for _, r := range rows {
		fmt.Fprintf(s, "  %s%s %s%s from %s of %s — %s%s gold held%s\n",
			ansi.FgBrightWhite, comma(r.qty), tr(s, r.good), ansi.Reset, r.seller, r.board,
			ansi.FgBrightYellow, comma(r.gold), ansi.Reset)
	}
	fmt.Fprintf(s, "\n%s\n", hiNums(fmt.Sprintf(tr(s, "%s gold is held against these bids."), comma(total))))
	pause(s)
	return Stay
}

// tradingHeader is the Trading menu's status line: the gold on hand, and what is
// already committed to bids, since the second is invisible everywhere else.
func tradingHeader(w *ctx) string {
	var gold, held int64
	p := w.Player()
	if p == nil {
		return ""
	}
	gold = p.Gold
	for _, f := range w.World.InFlight {
		if f.Kind == "trade" && f.Owner == p.Owner {
			held += f.Gold
		}
	}
	if held == 0 {
		return fmt.Sprintf("You have %s gold.", comma(gold))
	}
	return fmt.Sprintf("You have %s gold, with %s more held against bids.", comma(gold), comma(held))
}
