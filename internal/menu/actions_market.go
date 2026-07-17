package menu

import (
	"fmt"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// marketGoodKeys are the selection keys for the market goods, in MarketGoods
// order (Trooper, Jet, Turret, Bomber, Food, Agent, Tank, Carrier). '6' is
// skipped, matching BRE (Regions/HQ are not tradeable).
var marketGoodKeys = []byte{'1', '2', '3', '4', '5', '7', '8', '9'}

// marketGoodByKey returns the MarketGoods index for a selection key, or -1.
func marketGoodByKey(k byte) int {
	for i, kk := range marketGoodKeys {
		if kk == k {
			return i
		}
	}
	return -1
}

// tradingMarket is the general Trading Market (#17): list your own goods at your
// price, or buy from other empires' listings. Gated on new-realm protection, as
// in BRE. Mirrors BRE's screen: a Your Prices / Owned / For Sale / Total For Sale
// table, then per-good [C]hange setup or [B]uy from Market.
func tradingMarket(s session.Session, w *ctx) Result {
	if p := w.Player(); p == nil || p.Protection > 0 {
		fail(s, fmt.Errorf("Your dominion is still under protection."))
		return Stay
	}
	for {
		printMarketTable(s, w)
		fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, tr(s, "Your Choice?"), ansi.Reset)
		r, err := readKey(s)
		if err != nil {
			return Stay
		}
		if r == '0' || r == '\r' {
			return Stay
		}
		gi := marketGoodByKey(byte(unicode.ToUpper(r)))
		if gi < 0 {
			continue
		}
		fmt.Fprintf(s, "%c\n", r)
		good := game.MarketGoods[gi]
		fmt.Fprintf(s, "%s[C]%s %s %s[B]%s %s ",
			ansi.FgBrightCyan, ansi.Reset, tr(s, "Change your setup, or"),
			ansi.FgBrightCyan, ansi.Reset, tr(s, "Buy from Market:"))
		a, err := readKey(s)
		if err != nil {
			return Stay
		}
		switch unicode.ToUpper(a) {
		case 'C':
			fmt.Fprint(s, "C\n")
			marketChangeSetup(s, w, good)
		case 'B':
			fmt.Fprint(s, "B\n")
			marketBuy(s, w, good)
		default:
			fmt.Fprint(s, "\n")
		}
	}
}

// printMarketTable renders the player's Trading Market table with BRE's columns
// and colors (parens yellow, key bright-yellow, name white, prices bright-white,
// owned bright-cyan, for-sale/total bright-yellow).
func printMarketTable(s session.Session, w *ctx) {
	p := w.Player()
	lang := playerLang(w)
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightWhite, tr(s, "Trading Market"), ansi.Reset)
	fmt.Fprintf(s, "%s%-4s%-16s%12s %10s %13s %16s%s\n", ansi.FgBrightWhite,
		tr(s, "Key"), tr(s, "Name"), tr(s, "Your Prices"), tr(s, "Owned"),
		tr(s, "For Sale"), tr(s, "Total For Sale"), ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightRed, marketRule, ansi.Reset)
	for i, good := range game.MarketGoods {
		owned := 0
		if f := marketFieldValue(p, good); f >= 0 {
			owned = f
		}
		fmt.Fprintf(s, "%s(%s%c%s)%s %s%-14s%s %s%10d%s %s%10d%s %s%13d%s %s%16d%s\n",
			ansi.FgYellow, ansi.FgBrightYellow, marketGoodKeys[i], ansi.FgYellow, ansi.Reset,
			ansi.FgWhite, tr(s, good), ansi.Reset,
			ansi.FgBrightWhite, w.MarketPrice(p.Owner, good), ansi.Reset,
			ansi.FgBrightCyan, owned, ansi.Reset,
			ansi.FgBrightYellow, w.MarketForSale(p.Owner, good), ansi.Reset,
			ansi.FgBrightYellow, w.MarketTotalForSale(good), ansi.Reset)
		_ = lang
	}
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightRed, marketRule, ansi.Reset)
}

const marketRule = "──────────────────────────────────────────────────────────────────────────"

// marketChangeSetup adjusts the player's listing for good: new For Sale quantity
// (max = owned + already listed) then price. The goods are escrowed by
// SetMarketListing.
func marketChangeSetup(s session.Session, w *ctx, good string) {
	p := w.Player()
	max := marketFieldValue(p, good) + w.MarketForSale(p.Owner, good)
	qty := promptSuggested(s, fmt.Sprintf(tr(s, "New amount of %s for sale"), tr(s, good)), w.MarketForSale(p.Owner, good), max)
	price := promptSuggested(s, fmt.Sprintf(tr(s, "Set %s price"), tr(s, good)), w.MarketPrice(p.Owner, good), game.MoneyCap)
	err := w.mutatePlayer(func(pp *game.Empire) error {
		return w.World.SetMarketListing(pp, good, qty, price)
	})
	if err != nil {
		fail(s, err)
	}
}

// marketBuy buys good from another empire's listing: pick a target from the live
// sellers, then a quantity, then transfer (BuyFromMarket credits the seller).
func marketBuy(s session.Session, w *ctx, good string) {
	p := w.Player()
	sellers := w.MarketSellers(good, p.Owner)
	if len(sellers) == 0 {
		fail(s, fmt.Errorf("Nobody is selling %s right now.", tr(s, good)))
		return
	}
	fmt.Fprintf(s, "\n%s%-4s%-16s%12s %10s%s\n", ansi.FgBrightWhite,
		tr(s, "Id"), tr(s, "Empire Name"), tr(s, "For Sale"), tr(s, "Price"), ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgYellow, marketRule, ansi.Reset)
	for i, l := range sellers {
		name := l.Owner
		if e := w.FindByOwner(l.Owner); e != nil {
			name = e.Name
		}
		fmt.Fprintf(s, "%s[%s%c%s]%s %s%-16s%s %s%10d %10d%s\n",
			ansi.FgYellow, ansi.FgBrightWhite, byte('A'+i), ansi.FgYellow, ansi.Reset,
			ansi.FgWhite, name, ansi.Reset, ansi.FgBrightYellow, l.Qty, l.Price, ansi.Reset)
	}
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgYellow, marketRule, ansi.Reset)
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, tr(s, "Choose a Target (0 to cancel)?"), ansi.Reset)
	r, err := readKey(s)
	if err != nil {
		return
	}
	idx := int(unicode.ToUpper(r) - 'A')
	if idx < 0 || idx >= len(sellers) {
		fmt.Fprint(s, "\n")
		return
	}
	fmt.Fprintf(s, "%c\n", unicode.ToUpper(r))
	target := sellers[idx]
	price := target.Price
	max := target.Qty
	if price > 0 {
		if afford := p.Gold / price; afford < max {
			max = afford
		}
	}
	n := promptSuggested(s, fmt.Sprintf(tr(s, "Buy how many %s"), tr(s, good)), 0, max)
	if n <= 0 {
		return
	}
	owner := target.Owner
	buyErr := w.mutatePlayer(func(pp *game.Empire) error {
		return w.World.BuyFromMarket(pp, owner, good, n)
	})
	if buyErr != nil {
		fail(s, buyErr)
		return
	}
	okNoPause(s, "%d %s bought.", n, tr(s, good))
}

// marketFieldValue returns p's inventory count for a market good, or -1 if the
// good is not tradeable (matches game.marketField, exposed for the menu).
func marketFieldValue(p *game.Empire, good string) int {
	switch good {
	case "Trooper":
		return p.Troopers
	case "Jet":
		return p.Jets
	case "Turret":
		return p.Turrets
	case "Bomber":
		return p.Bombers
	case "Food":
		return p.Food
	case "Agent":
		return p.Agents
	case "Tank":
		return p.Tanks
	case "Carrier":
		return p.Carriers
	}
	return -1
}
