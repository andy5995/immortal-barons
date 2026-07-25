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

// mailBoxWidth is the inner width of a message box, matching BRE's ~74-column
// frame.
const mailBoxWidth = 74

type mailReply struct {
	toName string
	body   string
}

// mailReader shows the player's inbox one message at a time in a BRE-style box
// and acts on a single keypress each: [R]eply, [D]elete, [I]gnore, [Q]uit.
// Ignore (also Enter) keeps the message for next time — BRE lets a message be
// ignored indefinitely — and only Delete removes it. Deletions and replies are
// collected during the read and applied under the world lock afterwards, so the
// lock is never held while waiting on the player and a message that arrives
// mid-read survives (issues #2, #5).
func mailReader(s session.Session, w *ctx) {
	var mail []game.Message
	withPlayer(w, func(p *game.Empire) {
		mail = append([]game.Message(nil), p.Mail...)
	})
	if len(mail) == 0 {
		return
	}

	var deleted []game.Message
	var replies []mailReply
	for _, m := range mail {
		renderMessage(s, m)
		switch mailChoice(s) {
		case 'Q':
			applyMailActions(w, deleted, replies)
			return
		case 'D':
			deleted = append(deleted, m)
		case 'R':
			body, send := composeMessage(s)
			if send && strings.TrimSpace(body) != "" {
				replies = append(replies, mailReply{m.From, quoteReply(m) + "\n" + body})
			}
		}
		// 'I' (and any default): keep the message and advance.
	}
	applyMailActions(w, deleted, replies)
}

// applyMailActions removes the deleted messages from the player's live inbox
// (by value, first match, so concurrently-arrived mail is untouched) and mails
// any replies to their re-resolved recipients.
func applyMailActions(w *ctx, deleted []game.Message, replies []mailReply) {
	if len(deleted) == 0 && len(replies) == 0 {
		return
	}
	when := time.Now().Format("01/02/2006  15:04:05")
	w.mutatePlayer(func(p *game.Empire) error {
		for _, d := range deleted {
			for i, m := range p.Mail {
				if m == d {
					p.Mail = append(p.Mail[:i], p.Mail[i+1:]...)
					break
				}
			}
		}
		for _, r := range replies {
			recip := findRealm(w, r.toName)
			if recip == nil || recip == p {
				continue
			}
			w.World.SendMail(p, recip, game.Message{
				To:   w.EmpireLetter(recip),
				When: when,
				Body: r.body,
			})
		}
		return nil
	})
}

// quoteReply builds the quoted prefix a reply carries: a "Quote From" line plus
// each un-quoted line of m marked with "> ". Lines already quoted (an older
// reply) are dropped so quotes don't nest without bound — BRE quotes only the
// message being answered.
func quoteReply(m game.Message) string {
	var b strings.Builder
	fmt.Fprintf(&b, "> Quote From %s", m.From)
	for _, line := range strings.Split(m.Body, "\n") {
		if strings.HasPrefix(line, ">") {
			continue
		}
		fmt.Fprintf(&b, "\n> %s", line)
	}
	return b.String()
}

// mailChoice reads a single-key command at the message prompt. Enter defaults
// to Ignore (the safe skip); unrecognized keys are ignored until a valid one
// arrives; a dead session quits.
func mailChoice(s session.Session) rune {
	for {
		r, err := readKey(s)
		if err != nil {
			return 'Q'
		}
		if r == '\r' || r == '\n' {
			fmt.Fprintf(s, "%s\n", tr(s, "Ignore"))
			return 'I'
		}
		switch unicode.ToUpper(r) {
		case 'R':
			return 'R'
		case 'D':
			fmt.Fprintf(s, "%s\n", tr(s, "Delete"))
			return 'D'
		case 'I':
			fmt.Fprintf(s, "%s\n", tr(s, "Ignore"))
			return 'I'
		case 'Q':
			fmt.Fprintf(s, "%s\n", tr(s, "Quit"))
			return 'Q'
		}
	}
}

// renderMessage draws one message in BRE's boxed layout and colors (cyan frame,
// white labels/body, bright-cyan sender, bright-green recipients, bright-blue
// quotes, bright-white date), then the [R]/[D]/[I]/[Q] prompt.
func renderMessage(s session.Session, m game.Message) {
	when := m.When
	fill := mailBoxWidth - 1 - len([]rune(when)) - 5
	if fill < 0 {
		fill = 0
	}
	fmt.Fprintf(s, "\n%s┌%s%s%s%s%s%s\n",
		ansi.FgCyan, strings.Repeat("─", fill),
		ansi.FgBrightWhite, when,
		ansi.FgCyan, strings.Repeat("─", 5), ansi.Reset)
	fmt.Fprintf(s, "%s│ %s%s%s%s%s\n",
		ansi.FgCyan, ansi.FgWhite, tr(s, "Message From: "), ansi.FgBrightCyan, m.From, ansi.Reset)
	fmt.Fprintf(s, "%s│ %s%s%s%s%s\n",
		ansi.FgCyan, ansi.FgWhite, tr(s, "Message To  : "), ansi.FgBrightGreen, m.To, ansi.Reset)
	dash := mailBoxWidth - 1 - 5 - 8
	if dash < 0 {
		dash = 0
	}
	fmt.Fprintf(s, "%s├%s%s%s%s\n",
		ansi.FgCyan, strings.Repeat("─", 5), strings.Repeat("═", 8), strings.Repeat("─", dash), ansi.Reset)
	for _, line := range strings.Split(m.Body, "\n") {
		body := ansi.FgWhite
		if strings.HasPrefix(line, ">") {
			body = ansi.FgBrightBlue
		}
		fmt.Fprintf(s, "%s│ %s%s%s\n", ansi.FgCyan, body, line, ansi.Reset)
	}
	item := func(k string) string {
		return fmt.Sprintf("%s[%s%s%s]%s", ansi.FgBlue, ansi.FgBrightCyan, k, ansi.FgBlue, ansi.Reset)
	}
	fmt.Fprintf(s, "%s %s%s %s %s%s %s %s%s %s %s%s%s> %s",
		item("R"), ansi.FgWhite, tr(s, "Reply,"),
		item("D"), ansi.FgWhite, tr(s, "Delete,"),
		item("I"), ansi.FgWhite, tr(s, "Ignore, or"),
		item("Q"), ansi.FgWhite, tr(s, "Quit"), ansi.FgCyan, ansi.Reset)
}
