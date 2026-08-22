package menu

import (
	"fmt"
	"strings"
	"time"
	"unicode"

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

// basketSummary renders a basket as "100 Tanks, 5,000 Gold", or "nothing".
func basketSummary(s session.Session, b game.TradeBasket) string {
	var parts []string
	for _, g := range tradeGoods {
		if n := *g.field(&b); n > 0 {
			parts = append(parts, comma(n)+" "+tr(s, g.name))
		}
	}
	if len(parts) == 0 {
		return tr(s, "nothing")
	}
	return strings.Join(parts, ", ")
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
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, title), ansi.Reset)
		// Header + aligned columns matching BRE: Key / Good / In Deal / (Owned, on
		// the Offer side only — the Request side is what you want back).
		if limitToOwned {
			fmt.Fprintf(s, "  %s%-4s %-11s %10s %10s%s\n", ansi.FgBrightCyan, tr(s, "Key"), tr(s, "Good"), tr(s, "In Deal"), tr(s, "Owned"), ansi.Reset)
		} else {
			fmt.Fprintf(s, "  %s%-4s %-11s %10s%s\n", ansi.FgBrightCyan, tr(s, "Key"), tr(s, "Good"), tr(s, "In Deal"), ansi.Reset)
		}
		fmt.Fprintf(s, "  %s%s%s\n", ansi.FgBlue, strings.Repeat("─", 38), ansi.Reset)
		for _, g := range tradeGoods {
			if limitToOwned {
				fmt.Fprintf(s, "  (%s%c%s) %-11s %s%10s%s %s%10s%s\n",
					ansi.FgBrightYellow, g.key, ansi.Reset, tr(s, g.name),
					ansi.FgBrightWhite, comma(*g.field(&b)), ansi.Reset,
					ansi.Dim, comma(g.owned(p)), ansi.Reset)
			} else {
				fmt.Fprintf(s, "  (%s%c%s) %-11s %s%10s%s\n",
					ansi.FgBrightYellow, g.key, ansi.Reset, tr(s, g.name),
					ansi.FgBrightWhite, comma(*g.field(&b)), ansi.Reset)
			}
		}
		fmt.Fprintf(s, "  %s%s%s\n", ansi.FgBlue, strings.Repeat("─", 38), ansi.Reset)
		if limitToOwned {
			// The offered goods need a carrier to transport them (BRE's "Trade Deal
			// requires N Carriers" line). A carrier is consumed when the deal is sent.
			need := 0
			if !b.IsEmpty() {
				need = game.TradeDealCarriers
			}
			warn := ""
			if p.Carriers < need {
				warn = "  " + ansi.FgBrightRed + tr(s, "(not enough carriers)") + ansi.Reset
			}
			fmt.Fprintf(s, "  %s"+tr(s, "This deal needs %d carrier(s); you own %d.")+"%s%s\n",
				ansi.Dim, need, p.Carriers, ansi.Reset, warn)
		}
		fmt.Fprintf(s, "  (%s0%s) %s\n", ansi.FgBrightMagenta, ansi.Reset, tr(s, "Done"))
		fmt.Fprintf(s, "%s%s%s ", ansi.FgBrightWhite, tr(s, "Choice?"), ansi.Reset)
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
		maxAmt := 1 << 30
		if limitToOwned {
			maxAmt = g.owned(p)
		}
		n := promptSuggested(s, fmt.Sprintf(tr(s, "How many %s?"), tr(s, g.name)), *g.field(&b), maxAmt)
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
func sendTradeDeal(s session.Session, w *ctx) Result {
	to := pickRecipient(s, w, pickOpts{prompt: "Trade with:"})
	if to == nil {
		return Stay
	}
	toName := to.Name

	send := buildTradeBasket(s, w, "Offer — goods you send:", true)
	demand := buildTradeBasket(s, w, "Request — goods you want back:", false)
	if send.IsEmpty() && demand.IsEmpty() {
		return Stay
	}

	fmt.Fprintf(s, "\n%s"+tr(s, "You offer %s to %s")+"%s\n", ansi.FgBrightCyan, basketSummary(s, send), toName, ansi.Reset)
	fmt.Fprintf(s, "%s"+tr(s, "in return for %s.")+"%s\n", ansi.FgBrightCyan, basketSummary(s, demand), ansi.Reset)
	if !AskYesNo(s, "Send this trade deal?", true) {
		return Stay
	}
	// BRE: a deal is sent for a span of days at a per-day gold fee and consumes a
	// carrier. A standing Protective Trade agreement cuts the per-day rate. The
	// span is how long the offer stands before it lapses, so the ceiling here is
	// what the sender can pay for — the original has no other limit.
	perDay := int64(game.TradeDealGoldPerDay)
	var purse int64
	w.With(func() {
		if p, recip := w.Player(), findRealm(w, toName); p != nil && recip != nil {
			perDay = w.World.TradeDealGoldPerDayBetween(p, recip)
			purse = p.Gold
		}
	})
	// What is left after the gold being offered is what can pay for the span.
	maxDays := game.TradeDealMinDays
	if left := purse - int64(send.Gold); perDay > 0 && left > 0 {
		if affordable := int(left / perDay); affordable > maxDays {
			maxDays = affordable
		}
	}
	fmt.Fprintf(s, "\n%s"+tr(s, "Sending costs %s gold per day; it needs one carrier.")+"%s\n",
		ansi.Dim, comma(perDay), ansi.Reset)
	fmt.Fprintf(s, "%s"+tr(s, "The deal stands until the span runs out; unanswered, the goods are lost.")+"%s\n",
		ansi.Dim, ansi.Reset)
	suggested := game.TradeDealDefaultDays
	if suggested > maxDays {
		suggested = maxDays
	}
	days := promptSuggested(s, "How many days to send it for?", suggested, maxDays)
	if days < game.TradeDealMinDays {
		days = game.TradeDealMinDays
	}

	err := w.mutatePlayer(func(p *game.Empire) error {
		recip := findRealm(w, toName)
		if recip == nil || recip == p {
			return errTargetGone
		}
		return w.World.SendTradeDeal(p, recip, send, demand, days)
	})
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "Trade deal sent to %s for %d days (%s gold).", toName, days, comma(int64(days)*perDay))
	}
	return Stay
}

// reviewTradeDeals surfaces each pending trade deal to the player at turn start
// (mirroring reviewTreatyOffers): what they'd receive vs give, with an Accept?
// prompt. Accepting completes the barter; declining returns the sender's escrow.
func reviewTradeDeals(s session.Session, w *ctx) {
	var deals []game.TradeDeal
	withPlayer(w, func(p *game.Empire) {
		// Deals lapse here rather than in daily maintenance, which is where the
		// original sweeps them too: whoever takes the next turn clears the stale
		// ones (see World.ExpireTradeDeals).
		w.World.ExpireTradeDeals(time.Now())
		deals = append([]game.TradeDeal(nil), p.TradeDeals...)
	})
	for _, d := range deals {
		fmt.Fprintf(s, "\n%s"+tr(s, "%s offers you a trade deal.")+"%s\n", ansi.FgBrightCyan, d.From, ansi.Reset)
		fmt.Fprintf(s, "  "+tr(s, "You receive: %s")+"\n", basketSummary(s, d.Send))
		fmt.Fprintf(s, "  "+tr(s, "You give:    %s")+"\n", basketSummary(s, d.Demand))
		// BRE's three-way: Yes accepts, No declines, Ignore leaves it pending.
		fmt.Fprintf(s, "  %s%s%s ", ansi.FgBrightWhite, tr(s, "Accept? [Y]es, [N]o, [I]gnore for now"), ansi.Reset)
		r, err := readKey(s)
		if err != nil {
			return
		}
		fmt.Fprint(s, "\n")
		switch unicode.ToUpper(r) {
		case 'Y':
			var aerr error
			withPlayer(w, func(p *game.Empire) { aerr = w.World.AcceptTradeDeal(p, d.From) })
			if aerr != nil {
				fail(s, aerr)
			} else {
				ok(s, "Trade deal accepted.")
			}
		case 'N':
			withPlayer(w, func(p *game.Empire) { w.World.DeclineTradeDeal(p, d.From) })
		default:
			// Ignore for now: leave the deal pending for a later turn.
		}
	}
}
