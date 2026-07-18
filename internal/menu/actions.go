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
		// The league's Buy Military knob can forbid buying army units on the
		// open market (players must then build them through industry). Limited
		// mode's daily market pool isn't built yet, so it behaves like Yes.
		if military && w.Config.BuyMilitary == game.BuyNo {
			fail(s, fmt.Errorf("Buying military units is disabled in this league."))
			return Stay
		}
		price := unit(w)
		max := 0
		if price > 0 {
			max = p.Gold / price
		}
		n := promptSuggested(s, fmt.Sprintf("%s — %d gold each. How many?", label, price), 0, max)
		if n <= 0 {
			return Stay
		}
		// Re-resolve the empire against the freshly-reloaded world and let apply
		// re-check gold atomically — the p/price gathered above (before the prompt)
		// may be stale after a concurrent node's transaction.
		err := w.mutatePlayer(func(p *game.Empire) error {
			return apply(w.World, p, n)
		})
		if err != nil {
			fail(s, err)
		} else {
			// No pause: the Spending menu redraws right after with updated Owned
			// counts, so the confirmation stays visible above it instead of
			// forcing an extra keypress (BRE-style). Gold isn't repeated here —
			// it's in the Spending menu's status footer.
			fmt.Fprintf(s, "\n  %s%s%s\n", ansi.FgGreen,
				fmt.Sprintf(tr(s, "%d %s purchased."), n, label), ansi.Reset)
		}
		return Stay
	}
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
		var gold int
		err := w.mutatePlayer(func(p *game.Empire) error {
			e := apply(w.World, p, n) // apply re-checks stock atomically
			gold = p.Gold
			return e
		})
		if err != nil {
			fail(s, err)
		} else {
			// No pause: like buyUnit, the Spending/Sell menu redraws with updated
			// counts right after, so the confirmation stays visible above it.
			okNoPause(s, "Sold %d. Gold: %d", n, gold)
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
	p := w.Player()
	if p.Macros == nil {
		p.Macros = map[string]string{}
	}
	fmt.Fprintf(s, "\n%s%s%s\n\n", ansi.FgBrightCyan, tr(s, "Macro Editor"), ansi.Reset)
	for _, k := range macroKeys {
		val := p.Macros[string(k)]
		if val == "" {
			val = tr(s, "None")
		}
		fmt.Fprintf(s, "Ctrl-%c: %s%s%s\n", k, ansi.FgGreen, val, ansi.Reset)
	}

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

// titleBar prints a full-width white-on-blue panel header spanning the rule.
func titleBar(s session.Session, text string) {
	bar := " " + text + " "
	// Pad to the rule's DISPLAY width, not its byte length: `rule` is 60 box-drawing
	// runes at 3 UTF-8 bytes each, so len(rule) is 180 and would stretch the blue
	// bar across ~2.25 lines (#8). len([]rune(...)) counts columns, like titleRule.
	if pad := len([]rune(rule)) - len([]rune(bar)); pad > 0 {
		bar += strings.Repeat(" ", pad)
	}
	fmt.Fprintf(s, "\n%s%s%s%s\n", ansi.BgBlue, ansi.FgBrightWhite, bar, ansi.Reset)
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
			p.Tax = rate
		}
	})
	ok(s, "Tax rate set to %d%%.", rate)
	return Stay
}
