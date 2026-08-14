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
			fmt.Fprintf(s, "  %s\n", b)
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

// sendTarget is what a pickRecipient keypress resolved to.
type sendTarget int

const (
	targetNone   sendTarget = iota // cancelled
	targetOne                      // the empire pickRecipient returned
	targetAll                      // every living realm
	targetAllies                   // every realm the sender holds a treaty with
)

// pickOpts configures the recipient picker for one call site.
type pickOpts struct {
	prompt   string // the trailing prompt text, e.g. "Send to:"
	allowAll bool   // offer Z=All (and *=All Allies when the sender holds a treaty)
}

// Recipient-picker geometry, matching BRE's captured list (docs/dev/bre-screens.md,
// the "?=List" roster): a 75-column inset rule over Id / Empire Name / Territory /
// Score / Net Worth.
const (
	pickRuleWidth  = 75
	pickRuleDouble = 15
	pickNameWidth  = 26
)

// pickLetters is how many realms the picker can address. BRE indexes its empire
// array with the letter itself and reserves Z for "All", so A..Y is the whole
// addressable range and a realm past the 25th slot cannot be reached at all.
const pickLetters = 25

// pickRow is one selectable realm. The letter is the empire's SLOT letter
// (World.EmpireLetter), not its position in the list: BRE uses the letter as an
// index straight into its empire array, so a dead realm or the caller's own
// leaves a gap rather than renumbering everyone below it. That is also the
// letter a message records in "Message To  :".
type pickRow struct {
	e               *game.Empire
	letter          rune
	name            string
	online          bool
	land, score, nw int
}

// pickRows collects the realms a picker may address, plus how many treaty
// partners the caller has (which decides whether the all-allies key is live).
func pickRows(w *ctx, opts pickOpts) (rows []pickRow, allies int) {
	w.With(func() {
		p := w.Player()
		for i, e := range w.Empires {
			if i >= pickLetters || !e.Alive || e == p {
				continue
			}
			rows = append(rows, pickRow{
				e: e, letter: rune('A' + i), name: e.Name, online: e.Online(),
				land: e.Land, score: e.Score, nw: w.NetWorth(e),
			})
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

// writePickRoster prints the "?=List" roster.
//
// IB comma-groups the figures BRE prints bare, a recorded divergence
// (docs/dev/bre-screens.md).
func writePickRoster(s session.Session, rows []pickRow) {
	rule := ansi.FgMagenta + insetRule(pickRuleWidth, pickRuleDouble) + ansi.Reset
	fmt.Fprintf(s, "\n%s-*%s%s%s*-%s\n\n",
		ansi.FgBrightMagenta, ansi.FgBrightWhite, tr(s, "Immortal Barons"), ansi.FgBrightMagenta, ansi.Reset)
	fmt.Fprintf(s, "%s%-*s%-*s %10s %11s %11s%s\n%s\n", ansi.FgBrightWhite,
		scoreIDCellWidth, tr(s, "Id"), pickNameWidth, strings.Repeat(" ", markWidth(s))+tr(s, "Empire Name"),
		tr(s, "Territory"), tr(s, "Score"), tr(s, "Net Worth"), ansi.Reset, rule)
	for _, r := range rows {
		// idCell also squares the roster with its own heading: the rows used
		// to put the name one column left of where "Empire Name" starts.
		fmt.Fprintf(s, "%s%s %s%10s%s %s%11s%s %s%11s%s\n",
			idCell(fmt.Sprintf("(%c)", r.letter), ansi.FgBrightMagenta),
			nameCell(s, r.name, ansi.FgBrightWhite, r.online, pickNameWidth),
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

// pickRecipient is the single-target form of the picker, used where one realm
// is chosen — Send Trade Deal and proposing a treaty. It reads a SINGLE
// keypress: a letter selects that realm, '?' prints the roster, and anything
// else cancels. With opts.allowAll, 'Z' means every realm and '*' every realm
// the sender holds a treaty with.
//
// The roster prints ON DEMAND, as BRE prints it — the prompt comes first and
// '?' lists. The all-allies target is IB's own; BRE has no such key.
func pickRecipient(s session.Session, w *ctx, opts pickOpts) (*game.Empire, sendTarget) {
	rows, allies := pickRows(w, opts)
	if len(rows) == 0 {
		ok(s, "There is no one to reach.")
		return nil, targetNone
	}
	for {
		writePickPrompt(s, allies, opts)
		r, err := readKey(s)
		if err != nil {
			fmt.Fprint(s, ansi.Reset)
			return nil, targetNone
		}
		echo := func(text string) { fmt.Fprintf(s, "%s%s\n", text, ansi.Reset) }
		if r == '?' {
			echo(tr(s, "List"))
			writePickRoster(s, rows)
			continue
		}
		if opts.allowAll && (r == 'z' || r == 'Z') {
			echo(tr(s, "All"))
			return nil, targetAll
		}
		if allies > 0 && r == '*' {
			echo(tr(s, "All Allies"))
			return nil, targetAllies
		}
		idx := pickIndex(rows, r)
		if idx < 0 {
			echo("")
			return nil, targetNone
		}
		echo(rows[idx].name)
		return rows[idx].e, targetOne
	}
}

// pickRecipients is BRE's "(A-Y,Z=All,?=List) Send to:" picker as the original
// actually runs it: a MULTI-select list, not one keypress. Read from the
// selection routine at BRE.OVR 0x1b65e and the per-letter toggle it calls at
// 0x1b575:
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
			writePickRoster(s, rows)
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

// msgMaxLines is how many lines a single message may hold (BRE: 20).
const msgMaxLines = 20

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
	// The banner and ruler are uniformly bright cyan in BRE (verified from a
	// live message-editor screenshot); the line-number prompts below are green.
	fmt.Fprintf(s, "\n    %s"+tr(s, "You have %d lines for your message.  /S=save /A=abort /C=clear")+"%s\n",
		ansi.FgBrightCyan, msgMaxLines, ansi.Reset)
	// 68 columns between the brackets, ticked every 5 — measured off a live
	// capture. The last group is short: BRE's ruler ends "----+----]", not with
	// a seventh "|".
	ruler := "[" + "---+----|" + strings.Repeat("----+----|", 5) + "----+----" + "]"
	fmt.Fprintf(s, "    %s%s%s\n", ansi.FgBrightCyan, ruler, ansi.Reset)

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
	for len(lines) < msgMaxLines {
		fmt.Fprintf(s, "%s%2d>%s ", ansi.FgBrightGreen, len(lines)+1, ansi.Reset)

		// A '/' as the FIRST key of a line opens the command sub-menu right away
		// — BRE reads key-by-key and reacts on the bare '/', so we peek the first
		// key instead of reading a whole line. '/' anywhere else stays literal
		// text (e.g. "line s /s"), so only the leading key is special.
		first, err := readKey(s)
		if err != nil {
			return "", false
		}
		if first == '/' {
			fmt.Fprintf(s, tr(s, "/-Command?")+"  [%sA%s,%sS%s,%sC%s] ",
				ansi.FgBrightCyan, ansi.Reset, ansi.FgBrightCyan, ansi.Reset, ansi.FgBrightCyan, ansi.Reset)
			r, err := readKey(s)
			if err != nil {
				return "", false
			}
			switch unicode.ToUpper(r) {
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
			continue
		}
		if first == '\r' || first == '\n' {
			fmt.Fprint(s, "\n")
			lines = append(lines, "")
			continue
		}

		// Ordinary line: echo the first key, then read the rest until Enter.
		b := []rune{first}
		fmt.Fprintf(s, "%c", first)
		for {
			r, err := readKey(s)
			if err != nil {
				return "", false
			}
			if r == '\r' || r == '\n' {
				fmt.Fprint(s, "\n")
				break
			}
			if r == 127 || r == 8 { // backspace / DEL
				if len(b) > 0 {
					b = b[:len(b)-1]
					fmt.Fprint(s, "\b \b")
				}
				continue
			}
			if r >= 32 {
				b = append(b, r)
				fmt.Fprintf(s, "%c", r)
			}
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
