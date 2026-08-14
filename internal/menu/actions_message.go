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

// recipientIndex maps an uppercased letter key ('A'..) to an index into a
// recipient list of length n, capped at 25 entries (A..Y). Returns -1 for
// keys with no match.
func recipientIndex(r rune, n int) int {
	idx := int(unicode.ToUpper(r) - 'A')
	if idx < 0 || idx >= n || idx >= 25 {
		return -1
	}
	return idx
}

// sendTarget is what a pickRecipient keypress resolved to.
type sendTarget int

const (
	targetNone   sendTarget = iota // cancelled
	targetOne                      // the empire pickRecipient returned
	targetAll                      // every living realm
	targetAllies                   // every realm the sender holds a treaty with
)

// pickOpts configures pickRecipient for one call site.
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

// pickRecipient is BRE's shared "(A-Y,Z=All,?=List) Send to:" picker, used
// wherever a realm is chosen — Send Messages, Send Trade Deal, and proposing a
// treaty. It reads a SINGLE keypress: a letter selects that realm, '?' prints
// the roster, and anything else cancels. With opts.allowAll, 'Z' means every
// realm and '*' every realm the sender holds a treaty with.
//
// The roster prints ON DEMAND, as BRE prints it — the prompt comes first and
// '?' lists.
//
// IB comma-groups the figures BRE prints bare, a recorded divergence
// (docs/dev/bre-screens.md). The all-allies target is IB's too; BRE sends to one
// realm or to all.
func pickRecipient(s session.Session, w *ctx, opts pickOpts) (*game.Empire, sendTarget) {
	type row struct {
		e               *game.Empire
		name            string
		online          bool
		land, score, nw int
	}
	var rows []row
	allies := 0
	w.With(func() {
		for _, e := range recipients(w) {
			rows = append(rows, row{e: e, name: e.Name, online: e.Online(), land: e.Land, score: e.Score, nw: w.NetWorth(e)})
		}
		if opts.allowAll { // the only callers that can offer the all-allies target
			allies = len(w.TreatyPartners(w.Player()))
		}
	})
	if len(rows) == 0 {
		ok(s, "There is no one to reach.")
		return nil, targetNone
	}
	if len(rows) > 25 { // A..Y; Z is reserved for "All"
		rows = rows[:25]
	}
	list := func() {
		rule := ansi.FgMagenta + insetRule(pickRuleWidth, pickRuleDouble) + ansi.Reset
		fmt.Fprintf(s, "\n%s-*%s%s%s*-%s\n\n",
			ansi.FgBrightMagenta, ansi.FgBrightWhite, tr(s, "Immortal Barons"), ansi.FgBrightMagenta, ansi.Reset)
		fmt.Fprintf(s, "%s%-*s%-*s %10s %11s %11s%s\n%s\n", ansi.FgBrightWhite,
			scoreIDCellWidth, tr(s, "Id"), pickNameWidth, strings.Repeat(" ", markWidth(s))+tr(s, "Empire Name"),
			tr(s, "Territory"), tr(s, "Score"), tr(s, "Net Worth"), ansi.Reset, rule)
		for i, r := range rows {
			// idCell also squares the roster with its own heading: the rows used
			// to put the name one column left of where "Empire Name" starts.
			fmt.Fprintf(s, "%s%s %s%10s%s %s%11s%s %s%11s%s\n",
				idCell(fmt.Sprintf("(%c)", 'A'+i), ansi.FgBrightMagenta),
				nameCell(s, r.name, ansi.FgBrightWhite, r.online, pickNameWidth),
				ansi.FgBrightMagenta, comma(r.land), ansi.Reset,
				ansi.FgBrightWhite, comma(r.score), ansi.Reset,
				ansi.FgWhite, comma(r.nw), ansi.Reset)
		}
		fmt.Fprintln(s, rule)
	}
	for {
		// BRE colours this prompt piece by piece (captured in cap/bre-3-color.cap):
		// bright-blue parens, bright-cyan selection keys, plain cyan for the
		// connecting text, white for the trailing prompt, and the answer echoes
		// bright cyan. Same scheme as its y/n hints.
		var b strings.Builder
		key := func(k string) { b.WriteString(ansi.FgBrightCyan + k + ansi.FgCyan) }
		b.WriteString("\n" + ansi.FgBrightBlue + "(" + ansi.FgCyan)
		key("A")
		b.WriteString("-")
		key(string(rune('A' + len(rows) - 1)))
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
		r, err := readKey(s)
		if err != nil {
			fmt.Fprint(s, ansi.Reset)
			return nil, targetNone
		}
		echo := func(text string) { fmt.Fprintf(s, "%s%s\n", text, ansi.Reset) }
		if r == '?' {
			echo(tr(s, "List"))
			list()
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
		idx := recipientIndex(r, len(rows))
		if idx < 0 {
			echo("")
			return nil, targetNone
		}
		echo(rows[idx].name)
		return rows[idx].e, targetOne
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
	// IB draws the banner and ruler bright cyan. BRE draws the banner white and
	// the ruler plain cyan — a 2026-08-14 capture disagrees with the earlier
	// screenshot this was taken from; see docs/dev/bre-screens.md, which records
	// both and leaves the difference standing rather than changing colours
	// inside a wrapping fix. The line-number prompts below do match BRE's green.
	fmt.Fprintf(s, "\n    %s"+tr(s, "You have %d lines for your message.  /S=save /A=abort /C=clear")+"%s\n",
		ansi.FgBrightCyan, msgMaxLines, ansi.Reset)
	fmt.Fprintf(s, "    %s%s%s\n", ansi.FgBrightCyan, msgRuler(msgLineWidth), ansi.Reset)

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

		// Ordinary line: echo the first key, then read the rest until Enter. A
		// control key is not text — backspace here is already at column 1, so it
		// reaches the line above instead of starting this one with a stray byte.
		var b []rune
		switch {
		case first >= 32:
			b = append(b, first)
			fmt.Fprintf(s, "%c", first)
		case first == 127 || first == 8:
			b = reopenPrev()
		}
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
		to, target := pickRecipient(s, w, pickOpts{prompt: "Send to:", allowAll: true})
		if target == targetNone {
			return Stay
		}
		var toName string
		if to != nil {
			toName = to.Name
		}
		text, send := composeMessage(s)
		if send && strings.TrimSpace(text) != "" {
			fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Saving..."), ansi.Reset)
			when := time.Now().Format(game.StampFormat)
			// Re-resolve sender and recipient by handle/name against the freshly
			// reloaded world, so a concurrent send to the same inbox appends (both
			// messages land) and a vanished recipient aborts.
			err := w.mutatePlayer(func(p *game.Empire) error {
				if target != targetOne {
					recips := recipients(w)
					if target == targetAllies {
						recips = w.TreatyPartners(p)
					}
					if len(recips) == 0 {
						return errTargetGone
					}
					var letters strings.Builder
					for _, e := range recips {
						letters.WriteString(w.EmpireLetter(e))
					}
					to := letters.String()
					for _, e := range recips {
						w.World.SendMail(p, e, game.Message{To: to, When: when, Body: text})
					}
					return nil
				}
				recip := findRealm(w, toName)
				if recip == nil || recip == p {
					return errTargetGone
				}
				w.World.SendMail(p, recip, game.Message{To: w.EmpireLetter(recip), When: when, Body: text})
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
