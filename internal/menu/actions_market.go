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
		// BRE leaves the market on ESC alone, silently redrawing on 0 and Enter
		// (docs/dev/bre-screens.md). IB accepts ESC too, but keeps 0 and Enter
		// working rather than leaving a screen with no visible way out — every
		// other menu here quits on 0. Recorded as a divergence beside the capture.
		if r == '0' || r == '\r' || r == 0x1b {
			return Stay
		}
		gi := marketGoodByKey(byte(unicode.ToUpper(r)))
		if gi < 0 {
			continue
		}
		fmt.Fprintf(s, "%c\n", r)
		good := game.MarketGoods[gi].Singular
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
//
// The column edges come from the capture in docs/dev/bre-screens.md: every
// figure is right-aligned and the header label above it right-aligned to the
// same edge, at columns 33, 45, 59 and 76.
func printMarketTable(s session.Session, w *ctx) {
	p := w.Player()
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightWhite, tr(s, "Trading Market"), ansi.Reset)
	fmt.Fprintf(s, "%s%-4s%-4s%26s%12s%14s%17s%s\n", ansi.FgBrightWhite,
		tr(s, "Key"), tr(s, "Name"), tr(s, "Your Prices"), tr(s, "Owned"),
		tr(s, "For Sale"), tr(s, "Total For Sale"), ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", dim(marketAccent), marketRule(), ansi.Reset)
	for i, g := range game.MarketGoods {
		good := g.Singular
		owned := game.GoodCount(p, good)
		fmt.Fprintf(s, "%s(%s%c%s)%s %s%-11s%s%s%19d%s%s%12d%s%s%14d%s%s%17d%s\n",
			ansi.FgYellow, ansi.FgBrightYellow, marketGoodKeys[i], ansi.FgYellow, ansi.Reset,
			ansi.FgWhite, marketDisplayName(s, good), ansi.Reset,
			ansi.FgBrightWhite, w.MarketPrice(p.Name, good), ansi.Reset,
			ansi.FgBrightCyan, owned, ansi.Reset,
			ansi.FgBrightYellow, w.MarketForSale(p.Name, good), ansi.Reset,
			ansi.FgBrightYellow, w.MarketTotalForSale(good), ansi.Reset)
	}
	fmt.Fprintf(s, "%s%s%s\n", dim(marketAccent), marketRule(), ansi.Reset)
}

// The market's rule is BRE's inset divider, not a bare line: 5 single, 15
// double, then single to 78 (docs/dev/bre-screens.md). It takes the Trading
// menu's accent in dim form, as every rule the menu engine draws does.
const (
	marketRuleWidth  = 78
	marketRuleDouble = 15
	marketAccent     = ansi.FgBrightRed
)

func marketRule() string { return insetRule(marketRuleWidth, marketRuleDouble) }

// The seller list is IB's own screen — BRE's capture does not show one — so it
// takes the same divider at its own narrower content width, per the rule that a
// box is sized to what it holds.
const (
	sellerRuleWidth  = 43
	sellerRuleDouble = 10
)

func sellerRule() string { return insetRule(sellerRuleWidth, sellerRuleDouble) }

// marketChangeSetup adjusts the player's listing for good: new For Sale quantity
// (max = owned + already listed) then price. The goods are escrowed by
// SetMarketListing.
func marketChangeSetup(s session.Session, w *ctx, good string) {
	p := w.Player()
	max := game.GoodCount(p, good) + w.MarketForSale(p.Name, good)
	qty := promptSuggested(s, fmt.Sprintf(tr(s, "New amount of %s for sale"), tr(s, good)), w.MarketForSale(p.Name, good), max)
	// A per-unit price is held in count width, so it is bounded by what one
	// transaction may move rather than by what a treasury may hold.
	price := promptSuggested(s, fmt.Sprintf(tr(s, "Set %s price"), tr(s, good)), w.MarketPrice(p.Name, good), game.MaxCountField)
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
	sellers := w.MarketSellers(good, p.Name)
	if len(sellers) == 0 {
		fail(s, fmt.Errorf("Nobody is selling %s right now.", tr(s, good)))
		return
	}
	fmt.Fprintf(s, "\n%s%-4s%-16s%12s %10s%s\n", ansi.FgBrightWhite,
		tr(s, "Id"), tr(s, "Empire Name"), tr(s, "For Sale"), tr(s, "Price"), ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", dim(marketAccent), sellerRule(), ansi.Reset)
	for i, l := range sellers {
		fmt.Fprintf(s, "%s[%s%c%s]%s %s%s%s %s%10d %10d%s\n",
			ansi.FgYellow, ansi.FgBrightWhite, byte('A'+i), ansi.FgYellow, ansi.Reset,
			ansi.FgWhite, padColumn(w.Term, l.Realm, 16), ansi.Reset, ansi.FgBrightYellow, l.Qty, l.Price, ansi.Reset)
	}
	fmt.Fprintf(s, "%s%s%s\n", dim(marketAccent), sellerRule(), ansi.Reset)
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
	// Only when there is a price to afford: a listing may be given away at 0
	// (SetMarketListing clamps a negative price to it), and UnitsAffordable
	// answers 0 for a zero price, which would cap a free listing at nothing.
	if price > 0 {
		if afford := game.UnitsAffordable(p.Gold, price); afford < max {
			max = afford
		}
	}
	n := promptSuggested(s, fmt.Sprintf(tr(s, "Buy how many %s"), tr(s, good)), 0, max)
	if n <= 0 {
		return
	}
	// The realm name, not the *MarketListing: mutatePlayer reloads the world under
	// the lock, so every pointer taken from the browse list above is stale by the
	// time the purchase runs.
	seller := target.Realm
	buyErr := w.mutatePlayer(func(pp *game.Empire) error {
		return w.World.BuyFromMarket(pp, seller, good, n)
	})
	if buyErr != nil {
		fail(s, buyErr)
		return
	}
	okNoPause(s, "%d %s bought.", n, tr(s, good))
}

// marketDisplayName is a good's name as the market lists it: the two goods that
// are not military units carry a leading '*'. BRE stores the marker in the name
// itself — its shared goods table (BRE.EXE 0x157b7) reads Trooper, Jet, Turret,
// Bomber, *Food, *Gold, Agent, Tank, Carrier — so the seven units are bare and
// only Food and Gold are marked. Gold is not listed on this screen, which is why
// the capture shows a single '*'. The marker sits outside tr(): it is a symbol,
// not a word to translate.
func marketDisplayName(s session.Session, good string) string {
	if good == "Food" || good == "Gold" {
		return "*" + tr(s, good)
	}
	return tr(s, good)
}
