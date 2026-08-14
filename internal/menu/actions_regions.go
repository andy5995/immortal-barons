package menu

import (
	"fmt"
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
// regionRule is BRE's region-table separator: a single bright-magenta line with
// a short `═` accent inset near the left (─────═════──────────────────, see
// docs/dev/bre-screens.md). Box-drawing only, so it transcodes cleanly to CP437.
func regionRule(s session.Session) {
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightMagenta, insetRule(28, 5), ansi.Reset)
}

func printRegionTable(s session.Session, p *game.Empire) {
	fmt.Fprintf(s, "%s%-5s%-15s%s%s\n", ansi.FgBrightWhite, tr(s, "Key"), tr(s, "Name"), tr(s, "Owned"), ansi.Reset)
	regionRule(s)
	for i, name := range regionTypeNames {
		// BRE's region table (docs/dev/bre-screens.md): magenta parens, a
		// bright-white key letter, a bright-yellow name, a bright-white Owned count.
		fmt.Fprintf(s, " %s(%s%c%s)%s %s%-14s%s %s%5d%s\n",
			ansi.FgMagenta, ansi.FgBrightWhite, regionTypeKeys[i], ansi.FgMagenta, ansi.Reset,
			ansi.FgBrightYellow, name, ansi.Reset,
			ansi.FgBrightWhite, *regionField(p, i), ansi.Reset)
	}
	// Waste has no key: it cannot be bought or sold, only decontaminated during
	// maintenance. It is listed anyway so a baron can see what a strike left
	// behind and what it is still paying upkeep on. The blank key column is
	// sized from the "(X)" the rows above print, so the two stay aligned.
	if p.Regions.Waste > 0 {
		fmt.Fprintf(s, " %*s %s%-14s%s %s%5d%s\n",
			len("(X)"), "", ansi.FgBrightRed, tr(s, "Waste"), ansi.Reset,
			ansi.FgBrightRed, p.Regions.Waste, ansi.Reset)
	}
}

// promptRegionType reads a single-letter region choice (case-insensitive),
// returning its 0-based index or -1 to cancel. prompt is the already-translated
// label (the captured-region picker uses BRE's "[N Regions left] Your choice?",
// distinct from the plain "Your choice?" of the sell/buy screens).
func promptRegionType(s session.Session, prompt string) int {
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, prompt, ansi.Reset)
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
		fmt.Fprintf(s, " %s(%s*%s)%s %s%s%s\n", ansi.FgMagenta, ansi.FgBrightWhite, ansi.FgMagenta, ansi.Reset, ansi.FgBrightYellow, tr(s, "Advisors"), ansi.Reset)
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
			var gold int64
			// Re-resolve inside the transaction: BuyRegions re-checks gold and the
			// per-turn region cap against fresh state, and the region field pointer
			// must index the reloaded empire, not the stale gather.
			err := w.mutatePlayer(func(fp *game.Empire) error {
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
			// A concurrent node may have hit this empire while the player was
			// buying (region losses change the rising land price); surface it
			// here rather than waiting for the next menu redraw.
			flushSessionNews(s, w)
			p = w.Player() // refresh the display pointer for the next iteration
			// Out of gold, so stop asking. BRE keeps prompting; a player who has
			// just spent their last gold would have to read the "you can afford
			// 0" line and quit by hand. Reaching the day's region cap is NOT
			// this case — a capped player may still want the Advisors entry on
			// this screen, so the loop stays open for them.
			if err == nil && w.MaxAffordableRegions(p) == 0 && !regionCapReached(w, p) {
				fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgBrightWhite, tr(s, "You cannot afford another region."), ansi.Reset)
				return Stay
			}
		}
	}
}

// regionCapReached reports whether the day's region allowance is spent, which
// zeroes the affordable count for a reason gold has nothing to do with.
func regionCapReached(w *ctx, p *game.Empire) bool {
	return w.Config.MaxRegions > 0 && p.RegionsBoughtThisTurn >= w.Config.MaxRegions
}

// allocateCaptured lets a winning attacker choose the region types for the land
// captured in a Regular Attack (#58 — BRE decouples the winner's freely-chosen
// composition from the proportional mix the loser bled).
func allocateCaptured(s session.Session, w *ctx, n int) {
	allocateRegions(s, w, n, 0,
		fmt.Sprintf(tr(s, "You captured %d regions — choose which types to hold them as."), n), nil)
}

// allocateSpoils is the same picker for land taken on ANOTHER planet. The strike
// resolved days ago on a board this baron was not logged in to, so the land has
// been parked on Empire.PendingRegions ever since; this is the first moment
// there is anybody to ask (#107). The pending count is cleared inside the same
// transaction that grants the land, so a session that drops mid-picker cannot
// hand it over twice.
func allocateSpoils(s session.Session, w *ctx, n int) {
	allocateRegions(s, w, n, 0,
		fmt.Sprintf(tr(s, "Your forces brought home %d regions — choose which types to hold them as."), n),
		func(fp *game.Empire) { fp.PendingRegions -= min(n, fp.PendingRegions) })
}

// allocateDecontaminated re-types land just cleaned of waste. The original pools
// cleaned land with conquered land and asks the same question about both,
// because neither has a type of its own until the owner gives it one. IB has
// already restored the land as Coastal by this point — the gold is spent, so it
// must not be in limbo if the session drops — hence reclaim: the n regions come
// back out of Coastal and are shared out again.
func allocateDecontaminated(s session.Session, w *ctx, n int) {
	allocateRegions(s, w, n, n,
		fmt.Sprintf(tr(s, "%d regions are clean again — choose which types to hold them as."), n), nil)
}

// allocateRegions reuses the Buy Regions table and picker as an allocate-N loop
// (no gold): the player assigns n untyped regions across types until none
// remain. Any left unassigned when they quit early default to Coastal, so the
// land is never lost.
//
// reclaim is how many of the n are ALREADY counted as Coastal and must be taken
// back before they are shared out, so the total does not double. It is 0 for
// land that has not been placed yet (a fresh conquest) and n for land restored
// before the player was asked (decontamination).
//
// also runs inside the granting transaction, for a caller that has to retire
// bookkeeping in the same save — the parked interplanetary spoils, which must
// not survive the picker that hands them over.
func allocateRegions(s session.Session, w *ctx, n, reclaim int, headline string, also func(*game.Empire)) {
	if n <= 0 {
		return
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, headline, ansi.Reset)
	printRegionTable(s, w.Player())

	alloc := make([]int, len(regionTypeNames))
	remaining := n
	for remaining > 0 {
		// BRE's captured-region prompt: "[N Regions left] Your choice?" then
		// "How many <Type> regions? (0; N)" — distinct from the buy/sell screens.
		t := promptRegionType(s, fmt.Sprintf(tr(s, "[%d Regions left] Your choice?"), remaining))
		if t < 0 { // 0/Enter: assign the rest as Coastal and finish
			break
		}
		got := promptSuggested(s, fmt.Sprintf("How many %s regions?", regionTypeNames[t]), remaining, remaining)
		if got <= 0 {
			continue
		}
		alloc[t] += got
		remaining -= got
	}
	alloc[0] += remaining // regionTypeNames[0] == Coastal: soak up any unassigned remainder

	if err := w.mutatePlayer(func(fp *game.Empire) error {
		// Reclaim and re-grant inside ONE transaction: a save between the two would
		// leave the land missing if the session dropped in the gap.
		fp.Regions.Coastal -= min(reclaim, fp.Regions.Coastal)
		for i, cnt := range alloc {
			w.World.GrantRegions(fp, regionField(fp, i), cnt)
		}
		if also != nil {
			also(fp)
		}
		return nil
	}); err != nil {
		fail(s, err)
		return
	}
	okNoPause(s, "Regions added to your empire. You now hold %d land.", w.Player().Land)
}

func sellLand(s session.Session, w *ctx) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow, tr(s, "NOTE: You cannot sell Regions, only drop them..."), ansi.Reset)
	printRegionTable(s, p)
	t := promptRegionType(s, tr(s, "Your choice?"))
	if t < 0 {
		return Stay
	}
	field := regionField(p, t)
	n := promptSuggested(s, fmt.Sprintf("Drop how many %s regions?", regionTypeNames[t]), 0, *field)
	if n <= 0 {
		return Stay
	}
	var land int
	err := w.mutatePlayer(func(fp *game.Empire) error {
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
