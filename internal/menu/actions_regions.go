package menu

import (
	"fmt"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// printRegionTable renders the BRE-style region picker: a Key / Name / Owned
// table, colored (magenta keys, yellow names) so buy and drop share one look.
// regionRuleWidth/Double are BRE's region-table separator: a single
// bright-magenta line with a short `═` accent inset near the left,
// ─────═════─────────────── — 25 columns, 5 of them the accent
// (docs/dev/bre-screens.md). Measured from the captures 2026-08-16; it was 28
// here, following a code block in that file that no capture produces. 25 is
// also exactly the width of the header this rule sits under.
const (
	regionRuleWidth  = 25
	regionRuleDouble = 5
)

// regionRule draws that separator. Box-drawing only, so it transcodes cleanly
// to CP437.
func regionRule(s session.Session) {
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightMagenta, insetRule(regionRuleWidth, regionRuleDouble), ansi.Reset)
}

// printRegionTable renders the table; advisors adds BRE's "(*) Advisors" entry
// and the closing rule beneath it. Both the Buy Regions screen and the
// captured-region picker carry them (docs/dev/bre-screens.md, measured from
// cap/kd3-01.cap) — one routine draws both screens in the original. The drop
// screen is left without them: no capture of it exists to say either way.
// The names are TRANSLATED and padded by column rather than by byte. Both were
// wrong until 2026-09-11: the table printed reg.Name raw, so it stayed English
// on a screen the status block beside it had already localized, and %-14s counts
// bytes — which pads a Cyrillic name to half its width and walks the column
// after it.
func printRegionTable(s session.Session, t Term, p *game.Empire, advisors bool) {
	// Each heading is clipped one column short of its field before being padded
	// to it, so a word that exactly fills the column still leaves a gap: German
	// "Taste" is five characters in a five-wide Key field and ran straight into
	// the heading beside it.
	head := func(msgid string, width int) string {
		return padColumn(t, fitColumn(t, tr(s, msgid), width-1), width)
	}
	fmt.Fprintf(s, "%s%s%s%s%s\n", ansi.FgBrightWhite,
		head("Key", 5), head("Name", 15), tr(s, "Owned"), ansi.Reset)
	regionRule(s)
	for _, reg := range game.BuyableRegions {
		// BRE's region table (docs/dev/bre-screens.md): magenta parens, a
		// bright-white key letter, a bright-yellow name, a bright-white Owned count.
		fmt.Fprintf(s, " %s(%s%c%s)%s %s%s%s %s%5d%s\n",
			ansi.FgMagenta, ansi.FgBrightWhite, reg.Key, ansi.FgMagenta, ansi.Reset,
			ansi.FgBrightYellow, padColumn(t, tr(s, reg.Name), 14), ansi.Reset,
			ansi.FgBrightWhite, *reg.Count(&p.Regions), ansi.Reset)
	}
	// Waste has no key: it cannot be bought or sold, only decontaminated during
	// maintenance. It is listed anyway so a baron can see what a strike left
	// behind and what it is still paying upkeep on. The blank key column is
	// sized from the "(X)" the rows above print, so the two stay aligned.
	if p.Regions.Waste > 0 {
		fmt.Fprintf(s, " %*s %s%s%s %s%5d%s\n",
			len("(X)"), "", ansi.FgBrightRed, padColumn(t, tr(s, "Waste"), 14), ansi.Reset,
			ansi.FgBrightRed, p.Regions.Waste, ansi.Reset)
	}
	if advisors {
		fmt.Fprintf(s, " %s(%s*%s)%s %s%s%s\n", ansi.FgMagenta, ansi.FgBrightWhite, ansi.FgMagenta, ansi.Reset, ansi.FgBrightYellow, tr(s, "Advisors"), ansi.Reset)
		regionRule(s)
	}
}

// promptRegionType reads a single-letter region choice (case-insensitive),
// returning its 0-based index or -1 to cancel. prompt is the already-translated
// label (the captured-region picker uses BRE's "[N Regions left] Your choice?",
// distinct from the plain "Your choice?" of the sell/buy screens).
func promptRegionType(s session.Session, prompt string) int {
	return promptRegionChoice(s, prompt, false, "")
}

// promptRegionChoice is the picker's key loop. extras adds the '*' Advisors key
// and the '?' redisplay key, for the screens that offer them.
//
// hint is for a picker that cannot be left: when set, Enter, '0' and any other
// key that is not a region letter print it and ask again, instead of returning
// or being ignored. A player who reads "[12 Regions left]" as asking how many
// will type a number, and without the hint the prompt only redrew itself: a
// caller on v0.1.2 was stuck there until the idle timeout dropped them. It is
// printed once per line typed, so "12" and Enter, which a line-mode client
// sends in one burst, is answered once rather than three times.
func promptRegionChoice(s session.Session, prompt string, extras bool, hint string) int {
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, prompt, ansi.Reset)
	hinted := false
	for {
		r, err := readKey(s)
		if err != nil {
			return inputEndedChoice
		}
		enter := r == '\r' || r == '\n'
		if extras && r == '?' {
			fmt.Fprint(s, "?\n")
			return redisplayChoice
		}
		if extras && r == '*' {
			fmt.Fprint(s, "*\n")
			return advisorsChoice
		}
		if i := regionKeyIndex(r); i >= 0 {
			fmt.Fprintf(s, "%c\n", game.BuyableRegions[i].Key) // echo the single keypress; no Enter needed
			return i
		}
		if hint != "" && (enter || r >= ' ') {
			if !hinted {
				fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow, WrapIndented(hint, "  "), ansi.Reset)
				fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, prompt, ansi.Reset)
			}
			hinted = !enter
			if enter {
				drainInput(s) // the LF of a CR LF is the same Enter
			}
			continue
		}
		if enter { // Enter leaves, like '0'
			fmt.Fprint(s, "\n")
			return -1
		}
		if r == '0' {
			fmt.Fprint(s, "0\n")
			return -1
		}
		// invalid key — ignore and wait for a valid region letter, Enter, or 0
	}
}

// regionKeyIndex is the BuyableRegions index whose key r is, in either case, or
// -1 when r picks no region.
func regionKeyIndex(r rune) int {
	u := byte(unicode.ToUpper(r))
	for i, reg := range game.BuyableRegions {
		if reg.Key == u {
			return i
		}
	}
	return -1
}

// advisorsChoice is returned by promptBuyRegionType when the player picks
// the "(*) Advisors" entry instead of a region type.
//
// inputEndedChoice is the read having failed, which is NOT the same answer as
// "0": a picker that must keep asking until every region is placed still has to
// let go of a stream that has ended (AGENTS.md, the -reset editor that redrew
// itself 300,616 times).
const (
	advisorsChoice   = -2
	redisplayChoice  = -3
	inputEndedChoice = -4
)

// promptBuyRegionType is the picker with the Buy Regions screen's extra keys:
// '*' for Advisors (BRE lists "(*) Advisors" at the bottom of the region list)
// and '?' to redisplay the list, which is drawn on entry and then on demand so
// repeat purchases do not rescroll it.
func promptBuyRegionType(s session.Session) int {
	return promptRegionChoice(s, tr(s, "Your choice?"), true, "")
}

// buyLand is the Buy Regions action. It loops the region-type picker so a
// player can buy several region types in one visit. It ends when they quit
// (0), when input runs out, and as soon as no further region can be bought.
// Picking "(*) Advisors" shows the Advisors screen and returns to the region
// list instead of leaving.
func buyLand(s session.Session, w *ctx) Result {
	p := w.Player()
	// A realm whose land allowance is spent is refused before the screen is
	// drawn, as the original refuses it (docs/dev/bre-screens.md).
	if p.LandAvailable <= 0 {
		okNoPause(s, game.ErrNoLand.Error())
		return Stay
	}
	showMenu := func() {
		fmt.Fprintf(s, "\n%s"+tr(s, "Buy Regions — %d gold each.")+"%s\n", ansi.FgBrightCyan, w.LandPrice(p), ansi.Reset)
		fmt.Fprintf(s, "%s\n", tr(s, "Note: Region prices rise as you expand, so the price shown is only\n      the cost of the first region you buy."))
		fmt.Fprintf(s, tr(s, "You can afford %s%d%s regions.")+"\n\n", ansi.FgBrightCyan, w.MaxAffordableRegions(p), ansi.Reset)
		printRegionTable(s, w.Term, p, true)
	}
	// exitIfLimited prints why nothing more can be bought and reports whether
	// it did, so the caller can leave the screen. It fires before a purchase,
	// for a player who enters already at either limit — without it a capped or
	// gold-out player who picks a region type is asked "Buy how many X
	// regions? (0; 0)" with no way to tell why the answer can only be zero.
	// Gold and the day's region cap zero the same count for different
	// reasons, so the message names which one applies; reusing
	// game.ErrRegionCap's text keeps it in one place with BuyRegions' own
	// refusal. Advisors stays reachable from the System menu, so holding the
	// screen open for it is not worth the trap.
	exitIfLimited := func() bool {
		if w.MaxAffordableRegions(p) != 0 {
			return false
		}
		msg := tr(s, "You cannot afford another region.")
		switch {
		case p.LandAvailable <= 0:
			msg = tr(s, game.ErrNoLand.Error())
		case regionCapReached(w, p):
			msg = tr(s, game.ErrRegionCap.Error())
		}
		fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgBrightWhite, msg, ansi.Reset)
		return true
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
			if exitIfLimited() {
				return Stay
			}
			n := promptSuggested(s, fmt.Sprintf("Buy how many %s regions?", game.BuyableRegions[t].Name), 0, w.MaxAffordableRegions(p))
			if n <= 0 {
				continue
			}
			// Re-resolve inside the transaction: BuyRegions re-checks gold and the
			// per-turn region cap against fresh state, and the region field pointer
			// must index the reloaded empire, not the stale gather.
			err := w.mutatePlayer(func(fp *game.Empire) error {
				return w.World.BuyRegions(fp, game.BuyableRegions[t].Count(&fp.Regions), n)
			})
			if err != nil {
				// No pause: the message stays above the next prompt; the player
				// keeps buying without the region list rescrolling each time.
				fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgBrightRed, tr(s, err.Error()), ansi.Reset)
			} else {
				fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgGreen,
					fmt.Sprintf(tr(s, "%s %s regions purchased."), comma(n), game.BuyableRegions[t].Name), ansi.Reset)
			}
			// A concurrent node may have hit this empire while the player was
			// buying (region losses change the rising land price); surface it
			// here rather than waiting for the next menu redraw.
			flushSessionNews(s, w)
			p = w.Player() // refresh the display pointer for the next iteration
			// Buying the last of what was offered leaves without a word, as the
			// original does: the purchase line is reason enough.
			if err == nil && w.MaxAffordableRegions(p) == 0 {
				return Stay
			}
		}
	}
}

// regionCapReached reports whether this turn's region allowance is spent, which
// zeroes the affordable count for a reason gold has nothing to do with.
func regionCapReached(w *ctx, p *game.Empire) bool {
	return w.Config.MaxRegions > 0 && p.RegionsBoughtThisTurn >= w.Config.MaxRegions
}

// allocateCaptured lets a winning attacker choose the region types for the land
// captured in a Regular Attack or a pirate raid (#58 — BRE decouples the
// winner's freely-chosen composition from the proportional mix the loser bled).
// The land is already parked on Empire.PendingRegions by the save that won it.
func allocateCaptured(s session.Session, w *ctx, n int) {
	allocateRegions(s, w, n,
		fmt.Sprintf(tr(s, "You captured %d regions — choose which types to hold them as."), n))
}

// allocateSpoils is the same picker for land waiting on Empire.PendingRegions at
// the start of a turn: taken on ANOTHER planet days ago on a board this baron was
// not logged in to (#107), or left over from a picker whose session ended.
func allocateSpoils(s session.Session, w *ctx, n int) {
	allocateRegions(s, w, n,
		fmt.Sprintf(tr(s, "%d regions are waiting for a type — choose which types to hold them as."), n))
}

// allocateDecontaminated types land just cleaned of waste. The original pools
// cleaned land with conquered land and asks the same question about both,
// because neither has a type of its own until the owner gives it one; the save
// that took the payment has already parked it on Empire.PendingRegions.
func allocateDecontaminated(s session.Session, w *ctx, n int) {
	allocateRegions(s, w, n,
		fmt.Sprintf(tr(s, "%d regions are clean again — choose which types to hold them as."), n))
}

// allocateRegions reuses the Buy Regions table and picker as an allocate-N loop
// (no gold): the player assigns n of the untyped regions parked on
// Empire.PendingRegions across types until none remain. There is no quitting it
// — '0', Enter and any other key that is not a region letter print a hint and
// ask again. BRE does the same: its picker (select_regions_to_lose, BRE.OVR
// 0x030ebb) sends CR, '?' and '*' alike to the bottom of the loop at +0x124c,
// which reloads the remaining count and jumps back to the prompt unless it has
// reached zero.
//
// The table shows the land actually held plus what has been placed so far, never
// the untyped land: BRE's total_regions skips its pool too, and a table that
// counted the pending land as Coastal read as land lost once it was typed.
//
// When the INPUT ends, what was placed is granted and the rest stays parked; the
// spoils stage offers it at the start of the next turn, as BRE's run_player_turn
// re-offers its pool.
func allocateRegions(s session.Session, w *ctx, n int, headline string) {
	if n <= 0 {
		return
	}
	alloc := make([]int, len(game.BuyableRegions))
	showTable := func() {
		view := *w.Player()
		for i, cnt := range alloc {
			*game.BuyableRegions[i].Count(&view.Regions) += cnt
		}
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, headline, ansi.Reset)
		printRegionTable(s, w.Term, &view, true)
	}
	showTable()

	remaining := n
	for remaining > 0 {
		// BRE's captured-region prompt: "[N Regions left] Your choice?" then
		// "How many <Type> regions? (0; N)" — distinct from the buy/sell screens.
		// The Advisors and redisplay keys are the buy screen's, and BRE offers
		// them here too: one routine draws both region screens.
		t := promptRegionChoice(s, fmt.Sprintf(tr(s, "[%d Regions left] Your choice?"), remaining), true,
			fmt.Sprintf(tr(s, "Press the letter of a region type to place the remaining %d."), remaining))
		if t == advisorsChoice {
			advisorsMenu(s, w)
			showTable()
			continue
		}
		if t == redisplayChoice {
			showTable()
			continue
		}
		if t == inputEndedChoice { // the stream is gone: the rest stays parked
			break
		}
		got := promptSuggested(s, fmt.Sprintf("How many %s regions?", game.BuyableRegions[t].Name), 0, remaining)
		if got <= 0 {
			continue
		}
		alloc[t] += got
		remaining -= got
	}
	if remaining == n {
		return
	}

	if err := w.mutatePlayer(func(fp *game.Empire) error {
		// Take the grant out of the pool in the same save, clamped to what the
		// fresh record still holds, so no land is handed over twice.
		for i, cnt := range alloc {
			cnt = min(cnt, fp.PendingRegions)
			fp.PendingRegions -= cnt
			w.World.GrantRegions(fp, game.BuyableRegions[i].Count(&fp.Regions), cnt)
		}
		return nil
	}); err != nil {
		fail(s, err)
		return
	}
	okNoPause(s, "Regions added to your empire. You now hold %d land.", w.Player().Land)
}

// sellLand is the Sell menu's Regions item, and it drops rather than sells —
// land has no sale price, so the item exists to say so and to offer the drop.
// Dropping is irreversible and the item sits among eight that pay gold, so the
// notice is followed by a confirmation defaulting to NO (#257): a player who
// reached this expecting a sale can leave with a keypress, and one who meant it
// says so before any region table is drawn.
func sellLand(s session.Session, w *ctx) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow, tr(s, "NOTE: Regions may not be sold, only dropped..."), ansi.Reset)
	if !AskYesNo(s, "Drop regions?", false) {
		return Stay
	}
	// The heading names what this screen does, so the region table below cannot be
	// mistaken for the Buy Regions one it otherwise resembles.
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightRed, tr(s, "-* Drop Regions *-"), ansi.Reset)
	printRegionTable(s, w.Term, p, false)
	t := promptRegionType(s, tr(s, "Which regions will you give up?"))
	if t < 0 {
		return Stay
	}
	field := game.BuyableRegions[t].Count(&p.Regions)
	n := promptSuggested(s, fmt.Sprintf("Drop how many %s regions?", game.BuyableRegions[t].Name), 0, *field)
	if n <= 0 {
		return Stay
	}
	var land int
	err := w.mutatePlayer(func(fp *game.Empire) error {
		e := w.World.DropRegions(fp, game.BuyableRegions[t].Count(&fp.Regions), n)
		land = fp.Land
		return e
	})
	if err != nil {
		fail(s, err)
	} else {
		okNoPause(s, "%d %s regions dropped. You now hold %d land.", n, game.BuyableRegions[t].Name, land)
	}
	return Stay
}
