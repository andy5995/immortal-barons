package menu

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// regionTypeNames lists the 8 region types in BRE's Buy Regions screen order
// (Coastal, River, Agricultural, Desert, Industrial, Urban, Mountain, Technology
// — verified live, #17 menu audit). regionTypeKeys and regionField track this
// same order; the index is display/selection-only and never persisted.
var regionTypeNames = []string{
	"Coastal", "River", "Agricultural", "Desert",
	"Industrial", "Urban", "Mountain", "Technology",
}

// regionTypeKeys are the single-letter selection keys (BRE style), in the same
// order as regionTypeNames.
var regionTypeKeys = []byte{'C', 'R', 'A', 'D', 'I', 'U', 'M', 'T'}

// regionField returns a pointer to the idx'th (0-based) field of p.Regions,
// in the same order as regionTypeNames.
func regionField(p *game.Empire, idx int) *int {
	fields := []*int{
		&p.Regions.Coastal, &p.Regions.River, &p.Regions.Agricultural, &p.Regions.Desert,
		&p.Regions.Industrial, &p.Regions.Urban, &p.Regions.Mountain, &p.Regions.Technology,
	}
	return fields[idx]
}

// printRegionTable renders the BRE-style region picker: a Key / Name / Owned
// table, colored (magenta keys, yellow names) so buy and drop share one look.
// regionRule is the BRE-style separator around the region table: a short magenta
// accent segment above a longer white rule ("a partial quarter line above the
// longer line"). Both use only the box-drawing horizontal, which transcodes
// cleanly to CP437.
func regionRule(s session.Session) {
	fmt.Fprintf(s, "  %s%s%s\n", ansi.FgBrightMagenta, strings.Repeat("─", 12), ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightWhite, strings.Repeat("─", 36), ansi.Reset)
}

func printRegionTable(s session.Session, p *game.Empire) {
	fmt.Fprintf(s, "%s%-5s%-15s%s%s\n", ansi.FgBrightWhite, tr(s, "Key"), tr(s, "Name"), tr(s, "Owned"), ansi.Reset)
	regionRule(s)
	for i, name := range regionTypeNames {
		fmt.Fprintf(s, " %s(%c)%s %s%-14s%s %5d\n",
			ansi.FgBrightMagenta, regionTypeKeys[i], ansi.Reset,
			ansi.FgBrightYellow, name, ansi.Reset,
			*regionField(p, i))
	}
}

// promptRegionType reads a single-letter region choice (case-insensitive),
// returning its 0-based index or -1 to cancel.
func promptRegionType(s session.Session) int {
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, tr(s, "Your choice?"), ansi.Reset)
	for {
		r, err := readKey(s)
		if err != nil {
			return -1
		}
		if r == '\r' || r == '\n' { // Enter leaves, like '0'
			fmt.Fprint(s, "\n")
			return -1
		}
		if r == '0' {
			fmt.Fprint(s, "0\n")
			return -1
		}
		u := byte(unicode.ToUpper(r))
		for i, k := range regionTypeKeys {
			if k == u {
				fmt.Fprintf(s, "%c\n", u) // echo the single keypress; no Enter needed
				return i
			}
		}
		// invalid key — ignore and wait for a valid region letter, Enter, or 0
	}
}

// advisorsChoice is returned by promptBuyRegionType when the player picks
// the "(*) Advisors" entry instead of a region type.
const (
	advisorsChoice  = -2
	redisplayChoice = -3
)

// promptBuyRegionType is promptRegionType plus a '*' key for Advisors (BRE's
// Buy Regions screen lists "(*) Advisors" at the bottom of the region list) and
// a '?' key to redisplay the region list (the list is only drawn on entry, then
// on demand, so repeat purchases don't rescroll it).
func promptBuyRegionType(s session.Session) int {
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, tr(s, "Your choice?"), ansi.Reset)
	for {
		r, err := readKey(s)
		if err != nil {
			return -1
		}
		if r == '\r' || r == '\n' { // Enter leaves, like '0'
			fmt.Fprint(s, "\n")
			return -1
		}
		if r == '0' {
			fmt.Fprint(s, "0\n")
			return -1
		}
		if r == '?' {
			fmt.Fprint(s, "?\n")
			return redisplayChoice
		}
		if r == '*' {
			fmt.Fprint(s, "*\n")
			return advisorsChoice
		}
		u := byte(unicode.ToUpper(r))
		for i, k := range regionTypeKeys {
			if k == u {
				fmt.Fprintf(s, "%c\n", u) // echo the single keypress; no Enter needed
				return i
			}
		}
		// invalid key — ignore and wait for a valid region letter, '*', or 0
	}
}

// buyLand is the Buy Regions action. It loops the region-type picker so a
// player can buy several region types in one visit; the loop only ends when
// they quit (0) or run out of input. Picking "(*) Advisors" shows the
// Advisors screen and returns to the region list instead of leaving.
func buyLand(s session.Session, w *ctx) Result {
	p := w.Player()
	showMenu := func() {
		fmt.Fprintf(s, "\n%s"+tr(s, "Buy Regions — %d gold each.")+"%s\n", ansi.FgBrightCyan, w.LandPrice(p), ansi.Reset)
		fmt.Fprintf(s, "%s\n", tr(s, "Note: Region prices rise as you expand, so the price shown is only\n      the cost of the first region you buy."))
		fmt.Fprintf(s, tr(s, "You can afford %s%d%s regions.")+"\n\n", ansi.FgBrightCyan, w.MaxAffordableRegions(p), ansi.Reset)
		printRegionTable(s, p)
		fmt.Fprintf(s, " %s(*)%s %s%s%s\n", ansi.FgBrightMagenta, ansi.Reset, ansi.FgBrightYellow, tr(s, "Advisors"), ansi.Reset)
		regionRule(s)
	}
	showMenu()
	for {
		switch t := promptBuyRegionType(s); {
		case t == redisplayChoice:
			showMenu()
		case t == advisorsChoice:
			advisorsMenu(s, w)
			showMenu()
		case t < 0:
			return Stay
		default:
			n := promptSuggested(s, fmt.Sprintf("Buy how many %s regions?", regionTypeNames[t]), 0, w.MaxAffordableRegions(p))
			if n <= 0 {
				continue
			}
			var gold int
			// Re-resolve inside the transaction: BuyRegions re-checks gold and the
			// per-turn region cap against fresh state, and the region field pointer
			// must index the reloaded empire, not the stale gather.
			err := w.withPlayer(func(fp *game.Empire) error {
				e := w.World.BuyRegions(fp, regionField(fp, t), n)
				gold = fp.Gold
				return e
			})
			if err != nil {
				// No pause: the message stays above the next prompt; the player
				// keeps buying without the region list rescrolling each time.
				fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgRed, tr(s, err.Error()), ansi.Reset)
			} else {
				fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgGreen,
					fmt.Sprintf(tr(s, "%d %s regions purchased. Gold: %d"), n, regionTypeNames[t], gold), ansi.Reset)
			}
			p = w.Player() // refresh the display pointer for the next iteration
		}
	}
}

func sellLand(s session.Session, w *ctx) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow, tr(s, "NOTE: You cannot sell Regions, only drop them..."), ansi.Reset)
	printRegionTable(s, p)
	t := promptRegionType(s)
	if t < 0 {
		return Stay
	}
	field := regionField(p, t)
	n := promptSuggested(s, fmt.Sprintf("Drop how many %s regions?", regionTypeNames[t]), 0, *field)
	if n <= 0 {
		return Stay
	}
	var land int
	err := w.withPlayer(func(fp *game.Empire) error {
		e := w.World.DropRegions(fp, regionField(fp, t), n)
		land = fp.Land
		return e
	})
	if err != nil {
		fail(s, err)
	} else {
		okNoPause(s, "%d %s regions dropped. You now hold %d land.", n, regionTypeNames[t], land)
	}
	return Stay
}
