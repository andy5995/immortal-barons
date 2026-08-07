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

// pickRecipient shows a lettered columns table (Id / Empire / Land / Score /
// Net Worth) and reads a SINGLE keypress: a letter A.. selects that empire;
// '0' cancels. If allowAll, 'Z' means every realm, and '*' means every realm
// the sender holds a treaty with — offered only when they hold one. The
// all-allies target is IB's, not BRE's, which sends to one realm or to all.
func pickRecipient(s session.Session, w *ctx, prompt string, allowAll bool) (*game.Empire, sendTarget) {
	type row struct {
		e               *game.Empire
		name            string
		land, score, nw int
	}
	var rows []row
	allies := 0
	w.With(func() {
		for _, e := range recipients(w) {
			rows = append(rows, row{e, e.Name, e.Land, e.Score, w.NetWorth(e)})
		}
		if allowAll { // the only caller that can offer the all-allies target
			allies = len(w.TreatyPartners(w.Player()))
		}
	})
	if len(rows) == 0 {
		ok(s, "There is no one to reach.")
		return nil, targetNone
	}
	// Same visual language as the scores board (printScores): magenta rule,
	// colored and right-aligned numeric columns.
	fmt.Fprintf(s, "\n%s%-4s %-20s %10s %10s %11s%s\n",
		ansi.FgBrightWhite, tr(s, "Id"), tr(s, "Empire"), tr(s, "Land"), tr(s, "Score"), tr(s, "Net Worth"), ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgMagenta, strings.Repeat("─", 58), ansi.Reset)
	for i, r := range rows {
		if i >= 25 { // A..Y
			break
		}
		fmt.Fprintf(s, "%s(%c)%s %s%-20s%s %s%10d%s %s%10d%s %s%11d%s\n",
			ansi.FgBrightMagenta, 'A'+i, ansi.Reset,
			ansi.FgBrightWhite, r.name, ansi.Reset,
			ansi.FgBrightMagenta, r.land, ansi.Reset,
			ansi.FgBrightWhite, r.score, ansi.Reset,
			ansi.FgWhite, r.nw, ansi.Reset)
	}
	offerAllies := allies > 0
	extra := ""
	if allowAll {
		extra = tr(s, ", Z=All")
	}
	if offerAllies {
		extra += tr(s, ", *=All Allies")
	}
	fmt.Fprintf(s, "\n%s"+tr(s, "(A-%c%s, 0=cancel) %s")+"%s ", ansi.FgBrightWhite, 'A'+min(len(rows), 25)-1, extra, tr(s, prompt), ansi.Reset)
	r, err := readKey(s)
	if err != nil {
		return nil, targetNone
	}
	if allowAll && (r == 'z' || r == 'Z') {
		fmt.Fprintf(s, "%s\n", tr(s, "All"))
		return nil, targetAll
	}
	if offerAllies && r == '*' {
		fmt.Fprintf(s, "%s\n", tr(s, "All Allies"))
		return nil, targetAllies
	}
	idx := recipientIndex(r, len(rows))
	if idx < 0 {
		fmt.Fprint(s, "\n")
		return nil, targetNone
	}
	fmt.Fprintf(s, "%s\n", rows[idx].name)
	return rows[idx].e, targetOne
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
	ruler := "[" + "---+----|" + strings.Repeat("----+----|", 6) + "]"
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
		to, target := pickRecipient(s, w, "Send to:", true)
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

func planetaryPost(s session.Session, w *ctx) Result {
	text := strings.TrimSpace(prompt(s, "Post to the planet:"))
	if text == "" {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		w.World.PostBulletin(p, text)
		return nil
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Posted.")
	return Stay
}
