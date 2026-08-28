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

// Message-box geometry, measured off a live BRE capture: the top rule runs 76
// columns, and the rule under the headers is a short 41 — it does not span the
// box.
const (
	mailBoxWidth = 76
	mailSepWidth = 41
)

type mailReply struct {
	toName string
	// board names the planet an interplanetary reply goes back to, and is empty
	// for local mail. The author of an IP message is not a realm anyone here can
	// look up, so without it the reply would quietly go nowhere.
	board string
	// public sends an interplanetary reply to the whole of that planet rather
	// than to its author — BRE's "Public Reply?".
	public bool
	body   string
}

// mailReader shows the player's inbox one message at a time in a BRE-style box
// and acts on a single keypress each: [R]eply, [D]elete, [I]gnore, [Q]uit.
// Ignore keeps the message for next time — BRE lets a message be ignored
// indefinitely — and only Delete removes it. Deletions and replies are
// collected during the read and applied under the world lock afterwards, so the
// lock is never held while waiting on the player and a message that arrives
// mid-read survives (issues #2, #5).
// skipIgnored is true for the turn-start stop, which passes over what this
// session has already ignored, and false for Read Messages, which shows the
// whole inbox (see ctx.ignoredMail).
func mailReader(s session.Session, w *ctx, skipIgnored bool) {
	mail := unreadMail(w, skipIgnored)
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
			// Both questions come before the editor opens, as the original asks
			// them: who the reply is addressed to, then how much of the message it
			// carries over.
			public := m.FromBoard != "" && AskYesNo(s, "Public Reply?", false)
			body, send := composeMessageFrom(s, askQuote(s, m))
			if send && strings.TrimSpace(body) != "" {
				replies = append(replies, mailReply{toName: m.From, board: m.FromBoard, public: public, body: body})
				// A message replied to is done with, like a Delete (#122) — but only
				// once the reply actually went out; aborting the editor leaves the
				// message in the inbox.
				deleted = append(deleted, m)
				break
			}
			// The editor was abandoned. The message stays — nothing was sent — but
			// opening it and backing out is still a decision not to deal with it
			// now, so it counts as ignored and the rest of this session's turns
			// leave it alone (docs/dev/bre-screens.md).
			w.ignoreMail(m)
		default:
			// 'I' (and any unrecognized key): keep the message, note it for the
			// rest of this session, and advance.
			w.ignoreMail(m)
		}
	}
	applyMailActions(w, deleted, replies)
}

// ignoreMail marks m passed over for the rest of this session.
func (w *ctx) ignoreMail(m game.Message) {
	if w.ignoredMail == nil {
		w.ignoredMail = map[game.Message]bool{}
	}
	w.ignoredMail[m] = true
}

// unreadMail copies the player's inbox, less what this session has ignored when
// the caller asked to skip those.
func unreadMail(w *ctx, skipIgnored bool) []game.Message {
	var mail []game.Message
	withPlayer(w, func(p *game.Empire) {
		for _, m := range p.Mail {
			if skipIgnored && w.ignoredMail[m] {
				continue
			}
			mail = append(mail, m)
		}
	})
	return mail
}

// applyMailActions removes the deleted messages from the player's live inbox
// (by value, first match, so concurrently-arrived mail is untouched) and mails
// any replies to their re-resolved recipients.
func applyMailActions(w *ctx, deleted []game.Message, replies []mailReply) {
	if len(deleted) == 0 && len(replies) == 0 {
		return
	}
	when := time.Now().Format(game.StampFormat)
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
			if r.board != "" {
				w.World.ReplyIPMessage(p, r.board, r.toName, r.body, r.public)
				continue
			}
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

// askQuote runs BRE's "Quote Message?" and its line range, returning the lines
// to open the editor with (nil for none). The range is asked in the message's
// own line numbers and defaults to the whole of it.
//
// A line past the end needs no guard here: promptSuggestedTight caps at the
// message's length, and does it the way BRE does everywhere (#9) — the entry is
// rewritten to the maximum on screen and a SECOND Enter commits it. Only the low
// end can come back out of range, as 0 for an empty or unparseable answer.
func askQuote(s session.Session, m game.Message) []string {
	if strings.TrimSpace(m.Body) == "" {
		return nil
	}
	if !AskYesNo(s, "Quote Message?", true) {
		return nil
	}
	n := len(strings.Split(m.Body, "\n"))
	first := max(1, promptSuggestedTight(s, "First Line to Quote", 1, n))
	last := promptSuggestedTight(s, "Last Line to Quote", n, n)
	if last < first {
		return nil
	}
	return quoteLines(m, first, last)
}

// quoteLines builds the quoted block a reply opens with: a "Quote From" header
// and lines first..last of m, each marked "> ". The interplanetary reader names
// the planet as well, as BRE's does ("Quote From <realm> Of <planet>").
//
// Lines that were themselves quoted are carried over like any other, nesting as
// "> > ". Choosing the range is what keeps a thread from growing without bound,
// which is the job BRE gives it.
func quoteLines(m game.Message, first, last int) []string {
	header := fmt.Sprintf("> Quote From %s", m.From)
	if m.FromBoard != "" {
		header = fmt.Sprintf("> Quote From %s Of %s", m.From, m.FromBoard)
	}
	out := []string{header}
	body := strings.Split(m.Body, "\n")
	for _, line := range body[first-1 : last] {
		out = append(out, "> "+line)
	}
	return out
}

// mailChoice reads a single-key command at the message prompt. Unlike every
// other prompt in the game, Enter does NOTHING here: a message is read once and
// then gone, and a player holding Enter through the pre-turn stops would skip
// past it before it registered. Unrecognized keys are ignored until a valid one
// arrives; a dead session quits.
func mailChoice(s session.Session) rune {
	for {
		r, err := readKey(s)
		if err != nil {
			return 'Q'
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
	from := m.From
	if m.FromBoard != "" {
		from = fmt.Sprintf(tr(s, "%s on %s"), m.From, m.FromBoard)
	}
	fmt.Fprintf(s, "%s│ %s%s%s%s%s\n",
		ansi.FgCyan, ansi.FgWhite, tr(s, "Message From: "), ansi.FgBrightCyan, from, ansi.Reset)
	fmt.Fprintf(s, "%s│ %s%s%s%s%s\n",
		ansi.FgCyan, ansi.FgWhite, tr(s, "Message To  : "), ansi.FgBrightGreen, m.To, ansi.Reset)
	fmt.Fprintf(s, "%s├%s%s\n", ansi.FgCyan, insetRule(mailSepWidth-1, 8), ansi.Reset)
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
