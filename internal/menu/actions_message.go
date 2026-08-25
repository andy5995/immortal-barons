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

func readMessages(s session.Session, w *ctx) Result {
	var hadMail bool
	var news []string
	w.With(func() {
		news = append([]string(nil), w.NewsToday...)
		if p := w.Player(); p != nil {
			hadMail = len(p.Mail) > 0
		}
	})
	// Per-message BRE reader (Reply/Delete/Ignore/Quit). The mailbox is no longer
	// cleared on read: Ignore keeps a message for next time, only Delete removes.
	mailReader(s, w)
	if len(news) > 0 {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Planetary Bulletin:"), ansi.Reset)
		for _, b := range news {
			// Wrapped, not left to the terminal: a news line already runs past 80
			// columns with an ordinary board name in it, and a translated one is
			// longer still.
			fmt.Fprintf(s, "%s\n", WrapIndented(b, "  "))
		}
	}
	if !hadMail && len(news) == 0 {
		fmt.Fprintf(s, "\n%s\n", tr(s, "You have no messages."))
	}
	pause(s)
	return Stay
}

// recipients lists the living empires other than the player. Callers must
// already hold w's lock (w.Empires is shared, mutable state) — see
// pickRecipient and sendMessage.
func recipients(w *ctx) []*game.Empire {
	var r []*game.Empire
	for _, e := range w.Empires {
		if e.Alive && e != w.Player() {
			r = append(r, e)
		}
	}
	return r
}

// pickOpts configures the recipient picker for one call site.
type pickOpts struct {
	prompt   string // the trailing prompt text, e.g. "Send to:"
	allowAll bool   // offer Z=All (and *=All Allies when the sender holds a treaty)
	// relations makes '?' list the "-*Relations*-" table instead of the See
	// Scores one. BRE's selection routine takes the same choice as a flag: Send
	// Message passes 1 and gets the score table, the Diplomacy menu passes 0
	// and gets Relations (docs/dev/bre-screens.md).
	relations bool
}

// Recipient-picker geometry, matching BRE's captured list (docs/dev/bre-screens.md,
// the "?=List" roster): a 75-column inset rule over Id / Empire Name / Territory /
// Score / Net Worth.
const pickNameWidth = 26

// pickLetters is how many realms the picker can address: one per slot on the
// planet, since BRE indexes its empire array with the letter itself and reserves
// Z for "All". The picker cannot fall short of the game because no more realms
// than this can exist (game.PlanetSlots bounds creation).
const pickLetters = game.PlanetSlots

// pickRow is one selectable realm. The letter is the empire's SLOT letter
// (Empire.Letter), not its position in the list: BRE uses the letter as an
// index straight into its empire array, so a dead realm or the caller's own
// leaves a gap rather than renumbering everyone below it. That is also the
// letter a message records in "Message To  :".
type pickRow struct {
	e               *game.Empire
	letter          rune
	name            string
	presence        string
	land, score, nw int
	held            []string // pacts held with the caller; gathered for the Relations roster
}

// pickRows collects the realms a picker may address, plus how many treaty
// partners the caller has (which decides whether the all-allies key is live).
func pickRows(w *ctx, opts pickOpts) (rows []pickRow, allies int) {
	w.With(func() {
		p := w.Player()
		for _, e := range w.Empires {
			if !e.Alive || e == p || e.Slot < 1 || e.Slot > pickLetters {
				continue
			}
			row := pickRow{
				e: e, letter: rune('A' + e.Slot - 1), name: e.Name,
				presence: presenceOf(e, false, w.Today),
				land:     e.Land, score: e.Score, nw: w.NetWorth(e),
			}
			if opts.relations {
				row.held = w.TreatiesBetween(p, e)
			}
			rows = append(rows, row)
		}
		if opts.allowAll { // the only callers that can offer the all-allies target
			allies = len(w.TreatyPartners(p))
		}
	})
	return rows, allies
}

// pickIndex finds the row a keypress names, or -1.
func pickIndex(rows []pickRow, r rune) int {
	up := unicode.ToUpper(r)
	for i, row := range rows {
		if row.letter == up {
			return i
		}
	}
	return -1
}

// writePickRoster prints the "?=List" roster — the See Scores table, or the
// "-*Relations*-" one when the caller asked for it (opts.relations).
//
// IB comma-groups the figures BRE prints bare, a recorded divergence
// (docs/dev/bre-screens.md).
func writePickRoster(s session.Session, rows []pickRow, opts pickOpts) {
	if opts.relations {
		rel := make([]relationsRow, 0, len(rows))
		for _, r := range rows {
			rel = append(rel, relationsRow{
				id: string(r.letter), name: r.name,
				relations: relationsText(s, r.held), presence: r.presence,
			})
		}
		writeRelationsTable(s, rel)
		return
	}
	rule := rosterRule(ansi.FgMagenta)
	fmt.Fprintf(s, "\n%s-*%s%s%s*-%s\n\n",
		ansi.FgBrightMagenta, ansi.FgBrightWhite, tr(s, "Immortal Barons"), ansi.FgBrightMagenta, ansi.Reset)
	fmt.Fprintf(s, "%s%-*s%-*s %10s %11s %11s%s\n%s\n", ansi.FgBrightWhite,
		scoreIDCellWidth, tr(s, "Id"), pickNameWidth, strings.Repeat(" ", markWidth(s))+tr(s, "Empire Name"),
		tr(s, "Territory"), tr(s, "Score"), tr(s, "Net Worth"), ansi.Reset, rule)
	for _, r := range rows {
		// idCell also squares the roster with its own heading: the rows used
		// to put the name one column left of where "Empire Name" starts.
		fmt.Fprintf(s, "%s%s %s%10s%s %s%11s%s %s%11s%s\n",
			idCell(fmt.Sprintf("(%c)", r.letter)),
			nameCell(s, r.name, ansi.FgBrightWhite, r.presence, pickNameWidth),
			ansi.FgBrightMagenta, comma(r.land), ansi.Reset,
			ansi.FgBrightWhite, comma(r.score), ansi.Reset,
			ansi.FgWhite, comma(r.nw), ansi.Reset)
	}
	fmt.Fprintln(s, rule)
}

// writePickPrompt draws the picker's prompt line. The letter range is a fixed
// "A-Y" whatever the realm count — BRE prints it that way even in a two-realm
// game (docs/dev/bre-screens.md).
//
// BRE colours the prompt piece by piece: bright-blue parens, bright-cyan
// selection keys, plain cyan for the connecting text, white for the trailing
// prompt, and the answer echoes bright cyan. Same scheme as its y/n hints.
func writePickPrompt(s session.Session, allies int, opts pickOpts) {
	var b strings.Builder
	key := func(k string) { b.WriteString(ansi.FgBrightCyan + k + ansi.FgCyan) }
	b.WriteString("\n" + ansi.FgBrightBlue + "(" + ansi.FgCyan)
	key("A")
	b.WriteString("-")
	key(string(rune('A' + pickLetters - 1)))
	if opts.allowAll {
		b.WriteString(",")
		key("Z")
		b.WriteString(tr(s, "=All"))
	}
	if allies > 0 {
		b.WriteString(",")
		key("*")
		b.WriteString(tr(s, "=All Allies"))
	}
	b.WriteString(",")
	key("?")
	b.WriteString(tr(s, "=List"))
	b.WriteString(ansi.FgBrightBlue + ")" + ansi.FgWhite + " " + tr(s, opts.prompt) + " " + ansi.FgBrightCyan)
	fmt.Fprint(s, b.String())
}

// pickRecipient is the single-target form of the picker, for an action that can
// only address one realm — Send Trade Deal. It reads a SINGLE keypress: a letter
// selects that realm, '?' prints the roster, and anything else cancels, so it
// takes no opts.allowAll (Z=All and *=All Allies address a list, which is what
// pickRecipients is for).
//
// The roster prints ON DEMAND, as BRE prints it — the prompt comes first and
// '?' lists.
func pickRecipient(s session.Session, w *ctx, opts pickOpts) *game.Empire {
	rows, _ := pickRows(w, opts)
	if len(rows) == 0 {
		ok(s, "There is no one to reach.")
		return nil
	}
	for {
		writePickPrompt(s, 0, opts)
		r, err := readKey(s)
		if err != nil {
			fmt.Fprint(s, ansi.Reset)
			return nil
		}
		echo := func(text string) { fmt.Fprintf(s, "%s%s\n", text, ansi.Reset) }
		if r == '?' {
			echo(tr(s, "List"))
			writePickRoster(s, rows, opts)
			continue
		}
		idx := pickIndex(rows, r)
		if idx < 0 {
			echo("")
			return nil
		}
		echo(rows[idx].name)
		return rows[idx].e
	}
}

// pickRecipients is BRE's "(A-Y,Z=All,?=List) Send to:" picker as the original
// actually runs it: a MULTI-select list, not one keypress. It serves Send
// Message and the Diplomacy menu alike — BRE's selection routine at
// BRE.OVR 0x1b65e has exactly three callers, the message path and two sites in
// its diplomacy menu (0x1c800+0x08e7 and +0x0a79), which is why a treaty
// proposal and a Declaration Of War both take a list. Read from that routine and
// the per-letter toggle it calls at 0x1b575:
//
//   - a letter TOGGLES that realm. Selecting echoes the letter in bright cyan;
//     pressing it again erases it again.
//   - 'Z' toggles every letter A..Y in turn — it is the same toggle applied to
//     the whole range, so from an empty list it addresses everyone, and it does
//     NOT end the prompt.
//   - '?' prints the roster and re-draws the prompt. Letters already chosen stay
//     chosen but are not re-echoed.
//   - RETURN ends the list. With nothing selected that is a cancel.
//   - anything else — including a letter with no living realm behind it, and the
//     caller's own letter — is ignored without an echo.
//
// '*' (all allies) is IB's own addition and toggles the treaty partners.
func pickRecipients(s session.Session, w *ctx, opts pickOpts) []*game.Empire {
	rows, allies := pickRows(w, opts)
	if len(rows) == 0 {
		ok(s, "There is no one to reach.")
		return nil
	}
	chosen := make([]bool, len(rows))
	var echoed []int // selection order, so a deselect can redraw the line

	// Selecting appends a letter; deselecting rubs the line out and re-writes
	// what is left, which is what BRE's shift-down redraw leaves on screen.
	redraw := func(erase int) {
		fmt.Fprint(s, strings.Repeat("\b \b", erase))
		for _, i := range echoed {
			fmt.Fprintf(s, "%s%c%s", ansi.FgBrightCyan, rows[i].letter, ansi.FgWhite)
		}
	}
	toggle := func(i int) {
		if chosen[i] {
			chosen[i] = false
			for n, j := range echoed {
				if j == i {
					echoed = append(echoed[:n], echoed[n+1:]...)
					break
				}
			}
			redraw(len(echoed) + 1)
			return
		}
		chosen[i] = true
		echoed = append(echoed, i)
		fmt.Fprintf(s, "%s%c%s", ansi.FgBrightCyan, rows[i].letter, ansi.FgWhite)
	}

	writePickPrompt(s, allies, opts)
	for {
		r, err := readKey(s)
		if err != nil {
			fmt.Fprint(s, ansi.Reset)
			return nil
		}
		switch {
		case r == '\r' || r == '\n':
			fmt.Fprintf(s, "%s\n", ansi.Reset)
			var out []*game.Empire
			for i, row := range rows {
				if chosen[i] {
					out = append(out, row.e)
				}
			}
			return out
		case r == '?':
			fmt.Fprintf(s, "%s%s\n", tr(s, "List"), ansi.Reset)
			writePickRoster(s, rows, opts)
			writePickPrompt(s, allies, opts)
			echoed = echoed[:0]
		case opts.allowAll && (r == 'z' || r == 'Z'):
			for i := range rows {
				toggle(i)
			}
		case allies > 0 && r == '*':
			var partners []*game.Empire
			w.With(func() { partners = w.TreatyPartners(w.Player()) })
			for i, row := range rows {
				for _, p := range partners {
					if p == row.e {
						toggle(i)
						break
					}
				}
			}
		default:
			if i := pickIndex(rows, r); i >= 0 {
				toggle(i)
			}
		}
	}
}

// Message-editor geometry, read off BRE's own editor (captured live 2026-08-14
// and cross-read in BRE.OVR's compose_message; docs/dev/bre-screens.md).
const (
	// msgMaxLines is how many lines a single message may hold.
	msgMaxLines = 20
	// msgLineWidth is the width of an editor line in columns. A printable key
	// arriving on a line that already holds this many characters wraps instead
	// of extending it. The ruler is drawn to the same width, from the same
	// constant, so the mark the player types up to and the column that wraps
	// cannot drift apart.
	msgLineWidth = 68
	// msgWrapMinBreakCol is the earliest column a wrap may break at. BRE scans
	// back from msgLineWidth looking for a space and gives up here, so a word
	// filling more than the last msgLineWidth-msgWrapMinBreakCol+1 columns has
	// no reachable space and is split at the margin rather than carried whole.
	msgWrapMinBreakCol = 56
)

// msgRuler draws the editor's column ruler for a width-column line, as BRE
// draws it: counting the opening bracket as column 1, a '|' marks every tenth
// column and a '+' every fifth.
func msgRuler(width int) string {
	var b strings.Builder
	b.WriteByte('[')
	for c := 1; c <= width; c++ {
		switch n := c + 1; {
		case n%10 == 0:
			b.WriteByte('|')
		case n%5 == 0:
			b.WriteByte('+')
		default:
			b.WriteByte('-')
		}
	}
	b.WriteByte(']')
	return b.String()
}

// wrapMessageLine splits a full editor line, returning the text that stays and
// the word carried down to the next line. The break is the last space at or
// after msgWrapMinBreakCol; that space is dropped, since the line ends there
// now. With no space in reach — a single word longer than the search window —
// the whole line stays and nothing is carried, so the word is split at the
// margin. Both behaviours are BRE's.
func wrapMessageLine(b []rune) (head, carry []rune) {
	for c := len(b); c >= msgWrapMinBreakCol; c-- {
		if b[c-1] == ' ' {
			return b[:c-1], b[c:]
		}
	}
	return b, nil
}

// composeMessage runs the editor with nothing in it.
func composeMessage(s session.Session) (string, bool) { return composeMessageFrom(s, nil) }

// composeMessageFrom runs BRE's multi-line message editor: up to msgMaxLines
// lines under a column ruler. Entering "/" on a line opens the command prompt
// [A]bort / [S]ave / [C]lear. Returns the joined text and whether to send it
// (false = aborted).
//
// A reply starts with the quoted lines already IN the editor, as BRE starts one
// — they are ordinary lines from there on, numbered, counted against the limit,
// and cleared by /C along with everything else.
func composeMessageFrom(s session.Session, initial []string) (string, bool) {
	// Banner white, ruler plain cyan — read off a live capture (2026-08-14),
	// which corrected an earlier note here claiming both were bright cyan
	// "verified from a screenshot". Both clear 4.5:1 on black: the white 9.0:1
	// on the VGA palette, the cyan 7.3:1. The line-number prompts below are
	// BRE's green.
	fmt.Fprintf(s, "\n    %s"+tr(s, "You have %d lines for your message.  /S=save /A=abort /C=clear")+"%s\n",
		ansi.FgWhite, msgMaxLines, ansi.Reset)
	fmt.Fprintf(s, "    %s%s%s\n", ansi.FgCyan, msgRuler(msgLineWidth), ansi.Reset)

	lines := make([]string, 0, len(initial))
	for _, q := range initial {
		if len(lines) >= msgMaxLines {
			break
		}
		lines = append(lines, q)
		// Quoted lines are numbered in blue where a line being typed is green, so
		// what was carried over is told from what is being written.
		fmt.Fprintf(s, "%s%2d>%s %s\n", ansi.FgBrightBlue, len(lines), ansi.Reset, q)
	}
	// reopenPrev implements BRE's backspace at column 1: the line above is taken
	// back out of the message and re-opened with the cursor at its end — the way
	// to undo a wrap you did not want. BRE redraws that line under a fresh prompt
	// below rather than moving the cursor up a row, and colours the prompt bright
	// red where a new line is green, so a line being revisited reads as one.
	reopenPrev := func() []rune {
		if len(lines) == 0 {
			return nil
		}
		b := []rune(lines[len(lines)-1])
		lines = lines[:len(lines)-1]
		fmt.Fprintf(s, "\n%s%2d>%s %s", ansi.FgBrightRed, len(lines)+1, ansi.Reset, string(b))
		return b
	}

	// slashCommand answers the "/-Command?" prompt with the letter typed, upper-
	// cased. It is reached from anywhere the cursor sits at column 1 — a fresh
	// line, or a line backspaced empty again, which is the same place to the
	// player. BRE tests the COLUMN, not whether a key has been typed yet
	// (compose_message, BRE.OVR: `cmp word [bp-0x4],0x1` at 0x3A4 falls through
	// to the `cmp byte [bp-0xb],0x2f` slash test at 0x3AD, and jumps to the
	// ordinary-text path otherwise). IB peeked only the first key of a line, so
	// backspacing to the start and typing /A left a literal "/a" in the message.
	slashCommand := func() (rune, error) {
		fmt.Fprintf(s, tr(s, "/-Command?")+"  [%sA%s,%sS%s,%sC%s] ",
			ansi.FgBrightCyan, ansi.Reset, ansi.FgBrightCyan, ansi.Reset, ansi.FgBrightCyan, ansi.Reset)
		r, err := readKey(s)
		if err != nil {
			return 0, err
		}
		return unicode.ToUpper(r), nil
	}

	for len(lines) < msgMaxLines {
		fmt.Fprintf(s, "%s%2d>%s ", ansi.FgBrightGreen, len(lines)+1, ansi.Reset)

		var b []rune
		// restart abandons the line without recording it, after a command that
		// left the editor open (Clear, or an unrecognized letter).
		restart := false
		for {
			r, err := readKey(s)
			if err != nil {
				return "", false
			}
			if r == '/' && len(b) == 0 {
				c, err := slashCommand()
				if err != nil {
					return "", false
				}
				switch c {
				case 'A':
					fmt.Fprintf(s, "%s\n", tr(s, "Abort"))
					return "", false
				case 'S':
					fmt.Fprintf(s, "%s\n", tr(s, "Save"))
					return trimTrailingBlank(lines), true
				case 'C':
					fmt.Fprintf(s, "%s\n", tr(s, "Clear"))
					lines = nil
				default:
					fmt.Fprint(s, "\n")
				}
				restart = true
				break
			}
			if r == '\r' || r == '\n' {
				fmt.Fprint(s, "\n")
				break
			}
			if r == 127 || r == 8 { // backspace / DEL
				if len(b) > 0 {
					b = b[:len(b)-1]
					fmt.Fprint(s, "\b \b")
					continue
				}
				b = reopenPrev()
				continue
			}
			if r >= 32 {
				if len(b) >= msgLineWidth {
					// The last line has nowhere to wrap to, so it stops taking
					// keys at the margin. BRE instead carries the word onto a
					// 21st line, accepts nothing more, and drops that line when
					// it saves (observed live) — text the player watched
					// himself type, silently lost.
					if len(lines)+1 >= msgMaxLines {
						continue
					}
					head, carry := wrapMessageLine(b)
					// The carried word — and the space before it — were already
					// echoed on the line being left, so erase them there before
					// the newline. A margin split carries nothing and erases
					// nothing.
					fmt.Fprint(s, strings.Repeat("\b \b", len(b)-len(head)))
					fmt.Fprint(s, "\n")
					lines = append(lines, string(head))
					fmt.Fprintf(s, "%s%2d>%s %s", ansi.FgBrightGreen, len(lines)+1, ansi.Reset, string(carry))
					b = carry
				}
				b = append(b, r)
				fmt.Fprintf(s, "%c", r)
			}
		}
		if restart {
			continue
		}
		lines = append(lines, string(b))
	}
	fmt.Fprintf(s, "%s"+tr(s, "You have used all %d lines.")+"%s\n", ansi.FgYellow, msgMaxLines, ansi.Reset)
	return trimTrailingBlank(lines), true
}

// trimTrailingBlank joins message lines, dropping trailing empty ones.
func trimTrailingBlank(lines []string) string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func sendMessage(s session.Session, w *ctx) Result {
	for {
		to := pickRecipients(s, w, pickOpts{prompt: "Send to:", allowAll: true})
		if len(to) == 0 {
			return Stay
		}
		names := make([]string, 0, len(to))
		w.With(func() {
			for _, e := range to {
				names = append(names, e.Name)
			}
		})
		text, send := composeMessage(s)
		if send && strings.TrimSpace(text) != "" {
			fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Saving..."), ansi.Reset)
			when := time.Now().Format(game.StampFormat)
			// Re-resolve sender and recipients by handle/name against the freshly
			// reloaded world, so a concurrent send to the same inbox appends (both
			// messages land) and a vanished recipient drops out.
			err := w.mutatePlayer(func(p *game.Empire) error {
				var recips []*game.Empire
				for _, name := range names {
					if e := findRealm(w, name); e != nil && e != p {
						recips = append(recips, e)
					}
				}
				if len(recips) == 0 {
					return errTargetGone
				}
				// One message, addressed to the whole list: every copy carries the
				// same "Message To  :" letters, in letter order as BRE prints them.
				var letters strings.Builder
				for _, e := range w.Empires {
					for _, r := range recips {
						if r == e {
							letters.WriteString(w.EmpireLetter(e))
							break
						}
					}
				}
				addressed := letters.String()
				for _, e := range recips {
					w.World.SendMail(p, e, game.Message{To: addressed, When: when, Body: text})
				}
				return nil
			})
			if err != nil {
				fail(s, err)
			}
		}
		if !AskYesNo(s, "Do you wish to send another message?", false) {
			return Stay
		}
	}
}
