package menu

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

func readMessages(s session.Session, w *ctx) Result {
	// Snapshot the mailbox and today's news under the world lock, clearing the
	// mailbox in the same critical section. An unlocked read of p.Mail would
	// race a concurrent sender, and reading-then-clearing separately could wipe
	// a message that arrived mid-display; a message that arrives after the
	// snapshot stays in p.Mail for next time. p is re-resolved inside the lock so
	// a reload can't rebind it to another empire's private mail. (issues #2, #5)
	var mail, news []string
	w.With(func() {
		news = append([]string(nil), w.NewsToday...)
		p := w.Player()
		if p == nil {
			return
		}
		mail = p.Mail
		p.Mail = nil
	})
	if len(mail) == 0 {
		fmt.Fprintf(s, "\n%s\n", tr(s, "You have no messages."))
	} else {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Your messages:"), ansi.Reset)
		for _, m := range mail {
			fmt.Fprintf(s, "  %s\n", m)
		}
	}
	if len(news) > 0 {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Planetary Bulletin:"), ansi.Reset)
		for _, b := range news {
			fmt.Fprintf(s, "  %s\n", b)
		}
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

// pickRecipient shows a lettered columns table (Id / Empire / Land / Score /
// Net Worth) and reads a SINGLE keypress: a letter A.. selects that empire;
// '0' cancels (returns nil, false). If allowAll, 'Z' returns (nil, true)
// meaning "all". Returns (empire, all).
func pickRecipient(s session.Session, w *ctx, prompt string, allowAll bool) (*game.Empire, bool) {
	type row struct {
		e               *game.Empire
		name            string
		land, land2, nw int
	}
	var rows []row
	w.With(func() {
		for _, e := range recipients(w) {
			rows = append(rows, row{e, e.Name, e.Land, e.Land, w.NetWorth(e)})
		}
	})
	if len(rows) == 0 {
		ok(s, "There is no one to reach.")
		return nil, false
	}
	fmt.Fprintf(s, "\n%s%-4s %-20s %-6s %-6s %s%s\n", ansi.FgBrightCyan, tr(s, "Id"), tr(s, "Empire"), tr(s, "Land"), tr(s, "Score"), tr(s, "Net Worth"), ansi.Reset)
	for i, r := range rows {
		if i >= 25 { // A..Y
			break
		}
		fmt.Fprintf(s, "(%c) %-20s %-6d %-6d %d\n", 'A'+i, r.name, r.land, r.land2, r.nw)
	}
	extra := ""
	if allowAll {
		extra = tr(s, ", Z=All")
	}
	fmt.Fprintf(s, "\n%s"+tr(s, "(A-%c%s, 0=cancel) %s")+"%s ", ansi.FgBrightWhite, 'A'+min(len(rows), 25)-1, extra, tr(s, prompt), ansi.Reset)
	r, err := readKey(s)
	if err != nil {
		return nil, false
	}
	if allowAll && (r == 'z' || r == 'Z') {
		fmt.Fprintf(s, "%s\n", tr(s, "All"))
		return nil, true
	}
	idx := recipientIndex(r, len(rows))
	if idx < 0 {
		fmt.Fprint(s, "\n")
		return nil, false
	}
	fmt.Fprintf(s, "%s\n", rows[idx].name)
	return rows[idx].e, false
}

// msgMaxLines is how many lines a single message may hold (BRE: 20).
const msgMaxLines = 20

// composeMessage runs BRE's multi-line message editor: up to msgMaxLines lines
// under a column ruler. Entering "/" on a line opens the command prompt
// [A]bort / [S]ave / [C]lear. Returns the joined text and whether to send it
// (false = aborted).
func composeMessage(s session.Session) (string, bool) {
	// The banner and ruler are uniformly bright cyan in BRE (verified from a
	// live message-editor screenshot); the line-number prompts below are green.
	fmt.Fprintf(s, "\n    %s"+tr(s, "You have %d lines for your message.  /S=save /A=abort /C=clear")+"%s\n",
		ansi.FgBrightCyan, msgMaxLines, ansi.Reset)
	ruler := "[" + "---+----|" + strings.Repeat("----+----|", 6) + "]"
	fmt.Fprintf(s, "    %s%s%s\n", ansi.FgBrightCyan, ruler, ansi.Reset)

	var lines []string
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
		to, all := pickRecipient(s, w, "Send to:", true)
		if !all && to == nil {
			return Stay
		}
		var toName string
		if to != nil {
			toName = to.Name
		}
		text, send := composeMessage(s)
		if send && strings.TrimSpace(text) != "" {
			fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Saving..."), ansi.Reset)
			// Re-resolve sender and recipient by handle/name against the freshly
			// reloaded world, so a concurrent send to the same inbox appends (both
			// messages land) and a vanished recipient aborts.
			err := w.mutatePlayer(func(p *game.Empire) error {
				if all {
					for _, e := range recipients(w) {
						w.World.SendMail(p, e, text)
					}
					return nil
				}
				recip := findRealm(w, toName)
				if recip == nil || recip == p {
					return errTargetGone
				}
				w.World.SendMail(p, recip, text)
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

func sendTradeDeal(s session.Session, w *ctx) Result {
	to, _ := pickRecipient(s, w, "Trade with:", false)
	if to == nil {
		return Stay
	}
	toName := to.Name
	amount := promptInt(s, "How much gold?")
	if amount <= 0 {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		recip := findRealm(w, toName)
		if recip == nil || recip == p {
			return errTargetGone
		}
		return w.World.SendGold(p, recip, amount) // re-checks the sender's fresh balance
	})
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "Sent %d gold to %s.", amount, toName)
	}
	return Stay
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
