package menu

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// tradeGoods is the nine tradeable goods, keyed 1-9 as on BRE's trade-deal
// screen (Regions and HQ are not tradeable). field points into a basket; owned
// reads the amount an empire holds.
var tradeGoods = []struct {
	key   byte
	name  string
	field func(*game.TradeBasket) *int
	owned func(*game.Empire) int
}{
	{'1', "Troopers", func(b *game.TradeBasket) *int { return &b.Troopers }, func(e *game.Empire) int { return e.Troopers }},
	{'2', "Jets", func(b *game.TradeBasket) *int { return &b.Jets }, func(e *game.Empire) int { return e.Jets }},
	{'3', "Turrets", func(b *game.TradeBasket) *int { return &b.Turrets }, func(e *game.Empire) int { return e.Turrets }},
	{'4', "Bombers", func(b *game.TradeBasket) *int { return &b.Bombers }, func(e *game.Empire) int { return e.Bombers }},
	{'5', "Food", func(b *game.TradeBasket) *int { return &b.Food }, func(e *game.Empire) int { return e.Food }},
	// A trade deal moves at most MaxCountField gold, so the basket holds the
	// amount in count width even though a treasury does not.
	{'6', "Gold", func(b *game.TradeBasket) *int { return &b.Gold }, func(e *game.Empire) int { return int(min(e.Gold, game.MaxCountField)) }},
	{'7', "Agents", func(b *game.TradeBasket) *int { return &b.Agents }, func(e *game.Empire) int { return e.Agents }},
	{'8', "Tanks", func(b *game.TradeBasket) *int { return &b.Tanks }, func(e *game.Empire) int { return e.Tanks }},
	{'9', "Carriers", func(b *game.TradeBasket) *int { return &b.Carriers }, func(e *game.Empire) int { return e.Carriers }},
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
		var g *struct {
			key   byte
			name  string
			field func(*game.TradeBasket) *int
			owned func(*game.Empire) int
		}
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
	to, _ := pickRecipient(s, w, pickOpts{prompt: "Trade with:"})
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
	// BRE: a deal is sent for 2-5 days at a per-day gold fee and consumes a carrier.
	fmt.Fprintf(s, "\n%s"+tr(s, "Sending costs %s gold per day; it needs one carrier.")+"%s\n",
		ansi.Dim, comma(game.TradeDealGoldPerDay), ansi.Reset)
	days := promptSuggested(s, "How many days to send it for?", game.TradeDealMinDays, game.TradeDealMaxDays)
	if days < game.TradeDealMinDays {
		days = game.TradeDealMinDays
	}
	if days > game.TradeDealMaxDays {
		days = game.TradeDealMaxDays
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
		ok(s, "Trade deal sent to %s for %d days (%s gold).", toName, days, comma(game.TradeDealCost(days)))
	}
	return Stay
}

// reviewTradeDeals surfaces each pending trade deal to the player at turn start
// (mirroring reviewTreatyOffers): what they'd receive vs give, with an Accept?
// prompt. Accepting completes the barter; declining returns the sender's escrow.
func reviewTradeDeals(s session.Session, w *ctx) {
	var deals []game.TradeDeal
	withPlayer(w, func(p *game.Empire) {
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
