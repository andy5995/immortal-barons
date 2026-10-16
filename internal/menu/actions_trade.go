package menu

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// tradeGoods is the nine tradeable goods, keyed 1-9 as on BRE's trade-deal
// screen (Regions and HQ are not tradeable). Each row is a key and the
// canonical table's row for the good (#134); gold has no table row, since it is
// not a holding anyone counts by the unit, so it carries its own accessors.
type tradeGood struct {
	key   byte
	name  string
	field func(*game.TradeBasket) *int
	owned func(*game.Empire) int
}

func tradeRow(key byte, g *game.Good) tradeGood {
	return tradeGood{key: key, name: g.Plural, field: g.Basket,
		owned: func(e *game.Empire) int { return *g.Count(e) }}
}

var tradeGoods = []tradeGood{
	tradeRow('1', game.Trooper),
	tradeRow('2', game.Jet),
	tradeRow('3', game.Turret),
	tradeRow('4', game.Bomber),
	tradeRow('5', game.Food),
	// A trade deal moves at most MaxCountField gold, so the basket holds the
	// amount in count width even though a treasury does not.
	{key: '6', name: "Gold",
		field: func(b *game.TradeBasket) *int { return &b.Gold },
		owned: func(e *game.Empire) int { return int(min(e.Gold, game.MaxCountField)) }},
	tradeRow('7', game.Agent),
	tradeRow('8', game.Tank),
	tradeRow('9', game.Carrier),
}

// basketSummary renders a basket as "5,000 Gold and 100 Tanks", or "nothing".
func basketSummary(s session.Session, b game.TradeBasket) string {
	return b.Describe(sessionLang(s))
}

// TradeRequestMax bounds what one basket line may ask for. BRE uses 0xFFFFFFFF
// as "no ceiling" on the Request side; IB keeps a real one so the figure stays
// inside an int on a 32-bit door build.
const TradeRequestMax = 1 << 30

// The basket table's widths. Good is padded to 11, or to the caller's longest
// name for a good, and each figure column holds two billion with its separators
// ("2,000,000,000"), the most one good can hold.
const (
	tradeNameWidth   = 11
	tradeFigureWidth = 13
	tradeTitleFill   = 3 // the least rule drawn either side of the title
)

// shortOfCarriers refuses a basket the player has too few free carriers to ship,
// right after it is filled, so they can go back and send less instead of
// answering every later question first.
func shortOfCarriers(s session.Session, w *ctx, b game.TradeBasket) bool {
	short := false
	w.Read(func() {
		if p := w.Player(); p != nil {
			short = !game.CanCarry(p, b)
		}
	})
	if short {
		fail(s, game.ErrTradeNeedsCarrier)
	}
	return short
}

// drawTradeBasket draws a basket the way the menu engine draws a menu (draw):
// the accent's title rule, a white header, white labels with bright-white
// figures, the box sized to what is in it, and the carrier line under the
// closing rule where a menu's status line goes. The Offer side wears the Sell
// menu's accent and adds an Owned column; the Request side wears Spending's.
func drawTradeBasket(s session.Session, p *game.Empire, b game.TradeBasket, title string, limitToOwned bool) {
	col := spendingColor
	if limitToOwned {
		col = sellColor
	}
	kw, nw := keyCellWidth, tradeNameWidth
	if n := utf8.RuneCountInString(tr(s, "Key")); n > kw {
		kw = n
	}
	for _, g := range tradeGoods {
		if n := utf8.RuneCountInString(tr(s, g.name)); n > nw {
			nw = n
		}
	}
	keyPad := strings.Repeat(" ", kw-keyCellWidth)
	key := func(k byte) string {
		return dim(col) + "(" + keyHi(col) + string(k) + dim(col) + ")" + ansi.Reset + keyPad
	}

	var body strings.Builder
	fmt.Fprintf(&body, "%s  %s %s %*s", ansi.FgWhite,
		padLabel(tr(s, "Key"), kw), padLabel(tr(s, "Good"), nw), tradeFigureWidth, tr(s, "In Deal"))
	if limitToOwned {
		fmt.Fprintf(&body, " %*s", tradeFigureWidth, tr(s, "Owned"))
	}
	fmt.Fprintf(&body, "%s\n", ansi.Reset)
	for _, g := range tradeGoods {
		fmt.Fprintf(&body, "  %s %s%s%s %s%*s%s", key(g.key),
			ansi.FgWhite, padLabel(tr(s, g.name), nw), ansi.Reset,
			ansi.FgBrightWhite, tradeFigureWidth, comma(*g.field(&b)), ansi.Reset)
		if limitToOwned {
			fmt.Fprintf(&body, " %s%*s%s", ansi.FgWhite, tradeFigureWidth, comma(g.owned(p)), ansi.Reset)
		}
		body.WriteString("\n")
	}
	fmt.Fprintf(&body, "  %s %s%s%s\n", key('0'), ansi.FgWhite, tr(s, "Done"), ansi.Reset)

	// The title strings end in a colon from when they were a bare line; inside
	// the brackets it would read as punctuation in the wrong place. The box is
	// widened to keep a little rule either side of a title as long as the table,
	// which "[Request — goods you want back]" is.
	label := strings.TrimSuffix(tr(s, title), ":")
	width := utf8.RuneCountInString(label) + 2 + 2*tradeTitleFill
	for _, ln := range strings.Split(body.String(), "\n") {
		width = max(width, visibleLen(ln))
	}
	fmt.Fprintf(s, "\n%s\n%s%s\n", titleRule(col, label, width), body.String(), closingRule(col, width))
	if limitToOwned {
		// The offered goods need carriers to transport them (BRE's "Trade Deal
		// requires N Carriers" line), sized to the CARGO rather than one per
		// deal (#195), so the figure moves as the basket is filled.
		need := game.TradeDealCarriers(b)
		// FREE carriers, not carriers held (game.FreeCarriers): comparing
		// against the total would show no warning and then refuse the deal.
		free := game.FreeCarriers(p, b)
		warn := ""
		if free < need {
			warn = "  " + ansi.FgBrightRed + tr(s, "(not enough carriers)") + ansi.Reset
		}
		line := fmt.Sprintf(tr(s, "This deal needs %s carrier(s); you have %s free."), comma(need), comma(free))
		fmt.Fprintf(s, "%s%s%s%s\n", ansi.FgWhite, hiNumsReset(line, ansi.FgBrightWhite, ansi.FgWhite), ansi.Reset, warn)
	}
	fmt.Fprintf(s, "%s%s%s ", ansi.FgBrightWhite, tr(s, "Choice?"), ansi.Reset)
}

// buildTradeBasket lets the player assemble a basket of goods: pick a good (1-9),
// enter a quantity, repeat, and 0/Enter when done. When limitToOwned, quantities
// are capped at what the player currently holds (the Offer side); the Request
// side is unbounded. Returns the basket and false if the player aborts (no goods
// and immediate quit is allowed — the caller treats an empty result as cancel).
func buildTradeBasket(s session.Session, w *ctx, title string, limitToOwned bool) game.TradeBasket {
	var b game.TradeBasket
	for {
		p := w.Player()
		drawTradeBasket(s, p, b, title, limitToOwned)
		r, err := readKey(s)
		if err != nil {
			return b
		}
		if r == '0' || r == '\r' {
			fmt.Fprint(s, "\n")
			return b
		}
		var g *tradeGood
		for i := range tradeGoods {
			if tradeGoods[i].key == byte(unicode.ToUpper(r)) {
				g = &tradeGoods[i]
				break
			}
		}
		if g == nil {
			continue
		}
		fmt.Fprintf(s, "%c\n", r)
		// The Offer side is bounded by what you hold, and says so — the Owned
		// column beside it is the same figure. The Request side has no honest
		// ceiling to quote, so it quotes none (see promptSuggestedOpen).
		var n int
		label := fmt.Sprintf(tr(s, "How many %s?"), tr(s, g.name))
		if limitToOwned {
			n = promptSuggested(s, label, *g.field(&b), g.owned(p))
		} else {
			n = promptSuggestedOpen(s, label, *g.field(&b), TradeRequestMax)
		}
		if n < 0 {
			n = 0
		}
		if limitToOwned && n > g.owned(p) {
			n = g.owned(p)
		}
		*g.field(&b) = n
	}
}

// sendTradeDeal is BRE's Trading -> "Trading" (Send Trade Deal): pick a target,
// build an Offer basket (goods you send) and a Request basket (goods you want
// back), confirm, and send. The recipient sees it on their turn (reviewTradeDeals)
// and accepts or declines. The offered goods are escrowed until then.
//
// Both New Realm Protection refusals come where the original's create_trade_offer
// puts them, in its words: the sender's own shield before the picker, the
// target's straight after it, so neither is found out only after both baskets
// have been filled.
func sendTradeDeal(s session.Session, w *ctx) Result {
	if refuseInProtection(s, w, "Your dominion is still under protection.") {
		return Stay
	}
	to := pickRecipient(s, w, pickOpts{prompt: "Trade with:"})
	if to == nil {
		return Stay
	}
	var shielded bool
	w.Read(func() {
		if r := findRealm(w, to.Name); r != nil {
			shielded = r.Protection > 0
		}
	})
	if shielded {
		fail(s, game.ErrTheyProtected)
		return Stay
	}
	toName := to.Name

	send := buildTradeBasket(s, w, "Offer — goods you send:", true)
	if shortOfCarriers(s, w, send) {
		return Stay
	}
	demand := buildTradeBasket(s, w, "Request — goods you want back:", false)
	if send.IsEmpty() && demand.IsEmpty() {
		return Stay
	}

	fmt.Fprintf(s, "\n%s"+tr(s, "You offer %s to %s")+"%s\n", ansi.FgBrightCyan, basketSummary(s, send), toName, ansi.Reset)
	fmt.Fprintf(s, "%s"+tr(s, "in return for %s.")+"%s\n", ansi.FgBrightCyan, basketSummary(s, demand), ansi.Reset)
	fmt.Fprintf(s, "\n"+tr(s, "Trade Deal requires %d Carriers.")+"\n", game.TradeDealCarriers(send))
	if !askYesNoHere(s, "Send Trade Deal?", true) {
		return Stay
	}
	days, perDay := tradeDealSpan(s, w, toName, send)
	if days == 0 {
		return Stay
	}

	// The gold being offered and the span's fee are both paid on sending, so the
	// bank is offered against the pair of them.
	if !affordOrBank(s, w, int64(send.Gold)+int64(days)*perDay, game.ErrCantAfford) {
		return Stay
	}

	// Read inside the mutation, so the turn reported to the sender is the very
	// one stamped on the deal even if sending ever comes to cost a turn.
	var arrives int
	err := w.mutatePlayer(func(p *game.Empire) error {
		recip := findRealm(w, toName)
		if recip == nil || recip == p {
			return errTargetGone
		}
		arrives = w.World.TurnOfDay(p)
		return w.World.SendTradeDeal(p, recip, send, demand, days)
	})
	if err != nil {
		fail(s, err)
	} else {
		okNoPause(s, "Trade deal sent to %s for %d days (%s gold).", toName, days, comma(int64(days)*perDay))
		// The sender is told when it lands, as BRE tells them: a trade fleet
		// cannot reach the other realm earlier in their day than it left ours.
		ok(s, "It reaches them no sooner than turn %d of their day.", arrives)
	}
	return Stay
}

// tradeDealSpan asks how many days a deal to toName stands and returns the span
// with its per-day fee, or a span of 0 when none can be sent. The span is also
// the offer's lifetime. BRE's create_trade_offer (BRE.OVR 0x268a1-0x26b4a):
// the per-day fee is the cargo-weighted cost of what is sent, cut by a
// Protective Trade agreement; the span runs from TradeDealMinDays to
// TradeDealMaxDays, or to the days the gold left after the offered gold can pay
// for, and too few for the minimum refuses the deal. A basket that sends nothing
// costs nothing and skips the fee lines. BRE counts gold in hand only; IB adds
// the bank, so affordOrBank's withdrawal offer can still cover the fee.
func tradeDealSpan(s session.Session, w *ctx, toName string, send game.TradeBasket) (int, int64) {
	var perDay, purse int64
	w.Read(func() {
		if p, recip := w.Player(), findRealm(w, toName); p != nil && recip != nil {
			perDay = w.World.TradeDealGoldPerDayBetween(p, recip, send)
			purse = p.Gold + p.Bank
		}
	})
	maxDays := game.TradeDealMaxDays
	if perDay > 0 {
		fmt.Fprintf(s, "\n"+tr(s, "This trade deal will cost %s gold per day to send.")+"\n", comma(perDay))
		fmt.Fprintf(s, tr(s, "Trade deals must be sent for a minimum of %d days.")+"\n", game.TradeDealMinDays)
		if affordable := (purse - int64(send.Gold)) / perDay; affordable < int64(maxDays) {
			maxDays = int(max(affordable, 0))
		}
		if maxDays < game.TradeDealMinDays {
			fail(s, game.ErrCantAfford)
			return 0, perDay
		}
	}
	days := promptSuggested(s, "How many days would you like to send this deal for?", game.TradeDealMinDays, maxDays)
	// A span below the minimum is refused and the deal is not sent, as BRE's
	// create_trade_offer does (BRE.OVR +0x21bb: under 2, its refusal, then
	// return).
	if days < game.TradeDealMinDays {
		fail(s, fmt.Errorf(tr(s, "Sorry, trade deals must be sent for at least %d days."), game.TradeDealMinDays))
		return 0, perDay
	}
	return days, perDay
}

// reviewTradeDeals surfaces each pending trade deal to the player at turn start
// (mirroring reviewTreatyOffers): what they'd receive vs give, with an Accept?
// prompt. Accepting completes the barter; declining sends the escrow home.
func reviewTradeDeals(s session.Session, w *ctx) {
	var deals []game.TradeDeal
	withPlayer(w, func(p *game.Empire) {
		// Deals lapse here rather than in daily maintenance, which is where the
		// original sweeps them too: whoever takes the next turn clears the stale
		// ones (see World.ExpireTradeDeals).
		w.World.ExpireTradeDeals(game.Now())
		// A deal still in transit is skipped, not removed: it will be waiting
		// on the turn it was sent for, today and on any later day it survives
		// to (see TradeDeal.ArrivesOnTurn). ExpireTradeDeals stays the only
		// thing that ever takes one off the list. dealTurns brings the review back
		// at the head of the turn it lands on (runTurn).
		w.dealTurns = map[int]bool{}
		w.dealsSeen = map[dealKey]bool{}
		for _, d := range p.TradeDeals {
			w.dealsSeen[keyOfDeal(d)] = true
			if w.World.TradeDealArrived(d, p) {
				deals = append(deals, d)
				continue
			}
			w.dealTurns[d.ArrivesOnTurn] = true
		}
	})
	for _, d := range deals {
		fmt.Fprintf(s, "\n%s"+tr(s, "%s offers you a trade deal.")+"%s\n", ansi.FgBrightCyan, d.From, ansi.Reset)
		fmt.Fprintf(s, "  "+tr(s, "You receive: %s")+"\n", basketSummary(s, d.Send))
		fmt.Fprintf(s, "  "+tr(s, "You give:    %s")+"\n", basketSummary(s, d.Demand))
		// BRE's three-way: Yes accepts, No declines, Ignore leaves it pending.
		fmt.Fprintf(s, "  %s%s%s ", ansi.FgBrightWhite, tr(s, "Accept? [Y]es, [N]o, [I]gnore for now"), ansi.Reset)
		// Only the three offered keys answer: a player holding a key down to
		// race through their turns must not answer a deal with it.
		r := readKeys(s, tradeDealOpts, 0, 0)
		if r == 0 {
			return
		}
		fmt.Fprint(s, "\n")
		switch r {
		case 'Y':
			var aerr error
			withPlayer(w, func(p *game.Empire) { aerr = w.World.AcceptTradeDeal(p, d) })
			if aerr != nil {
				fail(s, aerr)
			} else {
				ok(s, "Trade deal accepted.")
			}
		case 'N':
			withPlayer(w, func(p *game.Empire) { w.World.DeclineTradeDeal(p, d) })
		default:
			// Ignore for now: leave the deal pending for a later turn.
		}
	}
}

// dealKey identifies a pending trade deal across reloads: who sent it and what
// it carries. Not the whole TradeDeal, whose ArrivesOnTurn daily maintenance
// clears and whose Expires can come back from a reload in another zone, and
// either would make an ignored deal look new.
type dealKey struct {
	From         string
	Send, Demand game.TradeBasket
}

func keyOfDeal(d game.TradeDeal) dealKey { return dealKey{d.From, d.Send, d.Demand} }

// tradeDealOpts are the keys the trade-deal prompt accepts. The prompt is drawn
// by hand, so only Key is set.
var tradeDealOpts = []keyOpt{{Key: 'Y'}, {Key: 'N'}, {Key: 'I'}}
