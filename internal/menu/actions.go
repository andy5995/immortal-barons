package menu

import (
	"errors"
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// errRealmChanged is the abort-on-conflict notice: a mutating action re-resolves
// the active empire inside its transaction and, if it has vanished (abdicated by
// another node between the prompt and the write), aborts cleanly with this rather
// than dereferencing a nil empire.
var errRealmChanged = errors.New("The realm has changed — try again.")

// errTargetGone aborts a targeted action (attack, strike, covert op) when the
// chosen target empire is no longer among the actor's valid targets after the
// world reloads inside the transaction — eliminated, abdicated, or shielded by
// another node between the target pick and the write.
var errTargetGone = errors.New("Your target is no longer there.")

// errAttacksExhausted is returned inside the attack transaction when the daily
// individual-attack cap was reached between the pre-check and the write (another
// node on the same handle, or a day that did not roll as expected).
var errAttacksExhausted = errors.New("You have used all your attacks for today.")

// buyUnit wraps a "prompt for quantity, apply, report" economy action. The
// max offered is what the empire can currently afford at unit's price.
func buyUnit(label string, military bool, unit func(*ctx) int, apply func(*game.World, *game.Empire, int) error) Action {
	return func(s session.Session, w *ctx) Result {
		p := w.Player()
		if military && !buyMilitaryAllowed(s, w) {
			return Stay
		}
		price := unit(w)
		max := game.UnitsAffordable(p.Gold, price)
		n := promptSuggested(s, fmt.Sprintf("%s — %d gold each. How many?", label, price), 0, max)
		if n <= 0 {
			return Stay
		}
		return applyBuy(s, w, label, n, apply)
	}
}

// buyMilitaryAllowed reports whether the league lets army units be bought on the
// open market, printing the refusal when it does not. The knob's other setting
// makes players build them through industry instead; Limited mode's daily market
// pool isn't built yet, so it behaves like Yes.
func buyMilitaryAllowed(s session.Session, w *ctx) bool {
	if w.Config.BuyMilitary == game.BuyNo {
		fail(s, fmt.Errorf("Buying military units is disabled in this league."))
		return false
	}
	return true
}

// buyJets buys jets, offering — for an order big enough to need one — to fold
// carriers into the same gold so the jets can actually reach a battle. Jets are
// the one unit that is inert without another (JetsPerCarrier of them per
// carrier), and a player who buys the flight and forgets the lift finds out at
// the attack screen. IB's own; the original leaves the two purchases separate.
func buyJets(s session.Session, w *ctx) Result {
	if !buyMilitaryAllowed(s, w) {
		return Stay
	}
	price := w.JetPrice(w.Player())
	n := promptSuggested(s, fmt.Sprintf("Jets — %d gold each. How many?", price),
		0, game.UnitsAffordable(w.Player().Gold, price))
	if n <= 0 {
		return Stay
	}
	if n <= game.JetsPerCarrier || !askCarriersIncluded(s, w, n) {
		return applyBuy(s, w, "Jets", n, (*game.World).BuildJets)
	}
	var jets, carriers int
	err := w.mutatePlayer(func(p *game.Empire) error {
		var e error
		jets, carriers, e = w.World.BuildJetsWithCarriers(p, n)
		return e
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgGreen,
		fmt.Sprintf(tr(s, "%s Jets and %s Carriers purchased."), comma(jets), comma(carriers)), ansi.Reset)
	return Stay
}

// askCarriersIncluded puts the carrier offer to the player, quoting the exact
// counts the same gold would buy rather than saying only that there will be
// fewer jets — the trade is a question of how many, so the screen answers it
// before asking. The explanation is a wrapped line of its own rather than a
// longer question, because a translated question runs past 80 columns where
// AskYesNo cannot break it.
func askCarriersIncluded(s session.Session, w *ctx, n int) bool {
	jets, carriers := w.JetCarrierBundle(w.Player(), n)
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgWhite, WrapIndented(fmt.Sprintf(tr(s,
		"For the same gold you can have %s Jets and %s Carriers to lift them, instead of %s Jets on their own."),
		comma(jets), comma(carriers), comma(n)), "  "), ansi.Reset)
	return askYesNoHere(s, "  Include carriers?", false)
}

// applyBuy is the shared "charge it, then say so" tail of the buy actions.
func applyBuy(s session.Session, w *ctx, label string, n int, apply func(*game.World, *game.Empire, int) error) Result {
	// Re-resolve the empire against the freshly-reloaded world and let apply
	// re-check gold atomically — the price gathered before the prompt may be
	// stale after a concurrent node's transaction.
	if err := w.mutatePlayer(func(p *game.Empire) error {
		return apply(w.World, p, n)
	}); err != nil {
		fail(s, err)
		return Stay
	}
	// No pause: the Spending menu redraws right after with updated Owned
	// counts, so the confirmation stays visible above it instead of
	// forcing an extra keypress (BRE-style). Gold isn't repeated here —
	// it's in the Spending menu's status footer.
	fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgGreen,
		fmt.Sprintf(tr(s, "%s %s purchased."), comma(n), label), ansi.Reset)
	return Stay
}

// sellUnit wraps a "prompt for quantity, sell, report" unit-selling action.
// The max offered is what the empire currently owns.
func sellUnit(label string, owned func(*game.Empire) int, apply func(*game.World, *game.Empire, int) error) Action {
	return func(s session.Session, w *ctx) Result {
		p := w.Player()
		max := owned(p)
		n := promptSuggested(s, fmt.Sprintf("%s (half price)?", label), 0, max)
		if n <= 0 {
			return Stay
		}
		var sold int
		var gold int64
		err := w.mutatePlayer(func(p *game.Empire) error {
			// The game-level sell clamps to the CURRENT stock, which a concurrent
			// node's strike may have thinned since the prompt was drawn — report
			// what was actually sold, not what was asked for.
			sold = min(n, owned(p))
			e := apply(w.World, p, n) // apply re-checks stock atomically
			gold = p.Gold
			return e
		})
		if err != nil {
			fail(s, err)
		} else {
			// No pause: like buyUnit, the Spending/Sell menu redraws with updated
			// counts right after, so the confirmation stays visible above it.
			okNoPause(s, "Sold %d. Gold: %d", sold, gold)
			if sold < n {
				fmt.Fprintf(s, "  %s%s%s\n", ansi.FgBrightRed,
					tr(s, "Your holdings changed while you decided — only what remained was sold."),
					ansi.Reset)
			}
		}
		return Stay
	}
}

// buildHQ starts HeadQuarters construction for the acting empire.
func buildHQ(s session.Session, w *ctx) Result {
	err := w.mutatePlayer(func(p *game.Empire) error {
		return w.World.StartHQ(p)
	})
	if err != nil {
		fail(s, err)
	} else {
		okNoPause(s, "You have started work on your HeadQuarters.")
	}
	return Stay
}

// macroKeys is BRE's fixed set of macro slots, each triggered in game by
// Ctrl-<letter> (see the Macro Editor, Image #9).
const macroKeys = "DEFRIOKL"

// macroMaxLen caps a recorded macro's length, matching BRE's editor (127 keys,
// from a disassembly of its record loop).
const macroMaxLen = 127

// writeMacros is BRE's Macro Editor: it lists the eight macro slots, lets the
// player pick one, then records keystrokes live until the player presses that
// same Ctrl-<letter> to end the edit. In game the macro replays when the player
// presses Ctrl-<letter> (see session.MacroExpander).
func writeMacros(s session.Session, w *ctx) Result {
	// Snapshot for display only — no unlocked writes to the shared empire. The
	// save path (mutatePlayer below) does its own nil-map init under the lock,
	// and delete on a nil map is a no-op, so nothing here needs to initialize
	// p.Macros.
	macros := map[string]string{}
	withPlayer(w, func(p *game.Empire) {
		for k, v := range p.Macros {
			macros[k] = v
		}
	})
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Macro Editor"), ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightCyan, InsetRule, ansi.Reset)
	for _, k := range macroKeys {
		val := macros[string(k)]
		if val == "" {
			val = tr(s, "None")
		}
		fmt.Fprintf(s, "Ctrl-%c: %s%s%s\n", k, ansi.FgGreen, val, ansi.Reset)
	}
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightCyan, InsetRule, ansi.Reset)

	fmt.Fprintf(s, "\n%s ", tr(s, "Edit which macro [D,E,F,R,I,O,K,L]?"))
	r, err := readKey(s)
	if err != nil {
		return Stay
	}
	letter := byte(r)
	if letter >= 'a' && letter <= 'z' {
		letter -= 'a' - 'A'
	}
	if !strings.ContainsRune(macroKeys, rune(letter)) {
		fmt.Fprint(s, "\n")
		return Stay
	}
	fmt.Fprintf(s, "%c\n", letter)

	// Clear the slot first so pressing its own Ctrl-<letter> ends the edit
	// (passes through the expander) instead of replaying the old macro.
	w.With(func() {
		if p := w.Player(); p != nil {
			delete(p.Macros, string(letter))
		}
	})
	ctrl := rune(letter - 'A' + 1)
	fmt.Fprintf(s, "\n"+tr(s, "Editing Macro Ctrl-%c    Press Ctrl-%c to end edit.")+"\n", letter, letter)
	// Record like BRE's editor (disassembled): only printable ASCII and Enter
	// go into the macro; Backspace edits the recording; other control keys are
	// ignored; capped at macroMaxLen. Ending on the macro's own Ctrl-key is
	// handled by the loop condition (the slot was cleared above so it passes
	// through the expander instead of replaying).
	var seq []rune
	for {
		k, err := readKey(s)
		if err != nil || k == ctrl {
			break
		}
		switch {
		case k == '\b' || k == 127: // Backspace: delete the last recorded key
			if len(seq) > 0 {
				seq = seq[:len(seq)-1]
				fmt.Fprint(s, "\b \b")
			}
		case len(seq) >= macroMaxLen:
			// full: ignore further keys until the end key
		case k == '\r' || k == '\n': // Enter is part of the macro (BRE records CR)
			seq = append(seq, '\r')
			fmt.Fprint(s, "\r\n")
		case k >= 0x20 && k <= 0x7e: // printable ASCII: record and echo
			seq = append(seq, k)
			fmt.Fprintf(s, "%c", k)
		}
	}
	if len(seq) > 0 {
		saveErr := w.mutatePlayer(func(p *game.Empire) error {
			if p.Macros == nil {
				p.Macros = map[string]string{}
			}
			p.Macros[string(letter)] = string(seq)
			return nil
		})
		if saveErr != nil {
			fail(s, saveErr)
			return Stay
		}
		ok(s, "Macro Ctrl-%c saved.", letter)
	} else {
		ok(s, "Macro Ctrl-%c cleared.", letter)
	}
	return Stay
}

// centered prints text centered on the rule's display width in the given color.
func centered(s session.Session, color, text string) {
	pad := (len([]rune(rule)) - len([]rune(text))) / 2
	if pad < 0 {
		pad = 0
	}
	fmt.Fprintf(s, "%s%s%s%s\n", strings.Repeat(" ", pad), color, text, ansi.Reset)
}

func setTaxRate(s session.Session, w *ctx) Result {
	p := w.Player()
	maxRate := w.Config.MaxTaxRate
	fmt.Fprintf(s, "\n%s"+tr(s, "Current tax rate: %d%%")+"%s\n", ansi.FgBrightCyan, p.Tax, ansi.Reset)
	// Suggest the current rate so a bare Enter keeps it (promptSuggested clamps to
	// [0, maxRate]); the old promptInt read empty input as 0 and zeroed the tax.
	rate := promptSuggested(s, "New tax rate?", p.Tax, maxRate)
	w.With(func() {
		if p := w.Player(); p != nil {
			w.World.SetTax(p, rate)
			rate = p.Tax
		}
	})
	ok(s, "Tax rate set to %d%%.", rate)
	return Stay
}
