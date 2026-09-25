package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// seedMail puts messages in the test player's inbox.
func seedMail(w *ctx, msgs ...game.Message) {
	p := w.Player()
	p.Mail = append(p.Mail, msgs...)
}

func TestMailReaderIgnoreKeepsMessage(t *testing.T) {
	f := &fakeSession{keys: []rune("i")}
	w := newWorld()
	seedMail(w, game.Message{From: "Ashland", To: "A", When: "07/24/2026", Body: "hi"})
	mailReader(f, w, false, false)
	if got := len(w.Player().Mail); got != 1 {
		t.Fatalf("Ignore should keep the message; Mail len = %d, want 1", got)
	}
}

func TestMailReaderDeleteRemovesMessage(t *testing.T) {
	f := &fakeSession{keys: []rune("d")}
	w := newWorld()
	seedMail(w, game.Message{From: "Ashland", To: "A", When: "07/24/2026", Body: "hi"})
	mailReader(f, w, false, false)
	if got := len(w.Player().Mail); got != 0 {
		t.Fatalf("Delete should remove the message; Mail len = %d, want 0", got)
	}
}

func TestMailReaderQuitKeepsRemaining(t *testing.T) {
	f := &fakeSession{keys: []rune("q")}
	w := newWorld()
	seedMail(w,
		game.Message{From: "Ashland", Body: "one"},
		game.Message{From: "Ashland", Body: "two"},
	)
	mailReader(f, w, false, false)
	if got := len(w.Player().Mail); got != 2 {
		t.Fatalf("Quit should keep unread messages; Mail len = %d, want 2", got)
	}
}

func TestMailReaderReplyQuotesAndMailsSender(t *testing.T) {
	// r, Enter (Quote Message? = Yes), the reply text, /s, then d to delete the
	// original. The message is one line, so no line range is asked for (#244).
	f := &fakeSession{keys: []rune("r\rthanks\r/sd")}
	w := newWorld()
	// A real recipient empire is the sender, so the reply can find them.
	var sender *game.Empire
	w.With(func() { sender = recipients(w)[0] })
	seedMail(w, game.Message{From: sender.Name, To: "A", When: "07/24/2026", Body: "nice one"})

	mailReader(f, w, false, false)

	if len(sender.Mail) != 1 {
		t.Fatalf("Reply should mail the sender; sender Mail len = %d, want 1", len(sender.Mail))
	}
	got := sender.Mail[0]
	if got.From != w.Player().Name {
		t.Errorf("reply From = %q, want %q", got.From, w.Player().Name)
	}
	if !strings.Contains(got.Body, "> Quote From "+sender.Name) {
		t.Errorf("reply should quote the original; body = %q", got.Body)
	}
	if !strings.Contains(got.Body, "> nice one") {
		t.Errorf("reply should carry the quoted line itself; body = %q", got.Body)
	}
	if !strings.Contains(got.Body, "thanks") {
		t.Errorf("reply should carry the new text; body = %q", got.Body)
	}
	// Stamped on the game's clock with its zone, so each reader sees it on their
	// own clock (#267).
	if _, ok := game.ParseStamp(got.When); !ok {
		t.Errorf("reply stamp %q carries no zone", got.When)
	}
	// Answering Delete after a sent reply removes the original (#122), and what
	// is left is the replier's own copy of the reply.
	mail := w.Player().Mail
	if len(mail) != 1 || !mail[0].Sent || mail[0].Body != got.Body || mail[0].To != got.To {
		t.Errorf("player's inbox = %+v, want only their copy of the reply", mail)
	}
}

// TestMailReaderReplyEnterKeepsOriginal: Enter at the Delete-or-Keep question
// after a sent reply keeps the message, and the reply still goes out.
func TestMailReaderReplyEnterKeepsOriginal(t *testing.T) {
	f := &fakeSession{keys: []rune("r\rthanks\r/s\r")}
	w := newWorld()
	var sender *game.Empire
	w.With(func() { sender = recipients(w)[0] })
	seedMail(w, game.Message{From: sender.Name, To: "A", When: "07/24/2026", Body: "nice one"})

	mailReader(f, w, false, false)

	if !strings.Contains(f.out.String(), "Keep original message") {
		t.Fatalf("never reached the Delete-or-Keep question:\n%s", f.out.String())
	}
	// A prompt that is already a question takes no ">" after it.
	if plain := stripANSI(f.out.String()); !strings.Contains(plain, "Keep original message? Keep") {
		t.Errorf("want the question closed by ? alone, got:\n%s", plain)
	}
	if len(sender.Mail) != 1 {
		t.Fatalf("the reply should still be sent; sender Mail len = %d, want 1", len(sender.Mail))
	}
	// The original and the replier's copy of the reply.
	if mail := w.Player().Mail; len(mail) != 2 || mail[0].Sent || !mail[1].Sent {
		t.Errorf("Enter should keep the original beside the copy; player Mail = %+v", mail)
	}
}

// A message from the planet itself (deliverIPMessage's bounce for an
// interplanetary message it could not deliver) has no author: From is empty
// while FromBoard names the board, which is the only way IB expresses "this
// is a notice, not a baron's mail". Replying to it would otherwise mail the
// empty ToEmpire deliverIPMessage reads as "the whole planet" — one wrong key
// broadcasting a private problem — so Reply is a no-op here, same as any other
// unhandled key.
func TestMailReaderReplyToSystemNoticeIsANoOp(t *testing.T) {
	f := &fakeSession{keys: []rune("r")}
	w := newWorld()
	seedMail(w, game.Message{FromBoard: "The Eclipse", To: "A", When: "07/24/2026", Body: "no such realm there"})
	mailReader(f, w, false, false)
	if got := len(w.Player().Mail); got != 1 {
		t.Fatalf("a system notice has no author to reply to; Mail len = %d, want 1 (kept)", got)
	}
}

// TestMailReaderAbortedReplyKeepsMessage checks the other half of #122: a reply
// started, then aborted with /a, leaves the original message in the inbox — only
// a reply that actually goes out removes it.
func TestMailReaderAbortedReplyKeepsMessage(t *testing.T) {
	// r, n (decline quote), /a (abort the editor).
	f := &fakeSession{keys: []rune("rn/a")}
	w := newWorld()
	var sender *game.Empire
	w.With(func() { sender = recipients(w)[0] })
	seedMail(w, game.Message{From: sender.Name, To: "A", When: "07/24/2026", Body: "nice one"})

	mailReader(f, w, false, false)

	if !strings.Contains(f.out.String(), "You have") {
		t.Fatalf("never reached the message editor:\n%s", f.out.String())
	}
	if len(sender.Mail) != 0 {
		t.Fatalf("an aborted reply should not send mail; sender Mail len = %d, want 0", len(sender.Mail))
	}
	if got := len(w.Player().Mail); got != 1 {
		t.Errorf("an aborted reply should keep the original; player Mail len = %d, want 1", got)
	}
}

// TestMailReaderQuoteRange covers BRE's "Quote Message?" line range: the reply
// opens with the header and ONLY the chosen lines, in the message's own
// numbering. Asserts the editor was reached too, so a flow change upstream
// cannot leave this passing on a script that ran dry.
func TestMailReaderQuoteRange(t *testing.T) {
	// r, y (quote), 2 (first), 3 (last), the reply text, /s.
	f := &fakeSession{keys: []rune("ry2\r3\rmy answer\r/s")}
	w := newWorld()
	var sender *game.Empire
	w.With(func() { sender = recipients(w)[0] })
	seedMail(w, game.Message{From: sender.Name, To: "A", Body: "one\ntwo\nthree\nfour"})

	mailReader(f, w, false, false)

	if !strings.Contains(f.out.String(), "Quote Message?") {
		t.Fatalf("never reached the quote prompt:\n%s", f.out.String())
	}
	if len(sender.Mail) != 1 {
		t.Fatalf("Reply should mail the sender; got %d messages", len(sender.Mail))
	}
	body := sender.Mail[0].Body
	for _, want := range []string{"> Quote From " + sender.Name, "> two", "> three", "my answer"} {
		if !strings.Contains(body, want) {
			t.Errorf("reply body missing %q:\n%s", want, body)
		}
	}
	for _, unwanted := range []string{"> one", "> four"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("reply quoted %q, which is outside the chosen range:\n%s", unwanted, body)
		}
	}
}

// TestMailReaderQuoteDeclined checks answering No leaves the editor empty, so a
// reply carries only what was typed.
func TestMailReaderQuoteDeclined(t *testing.T) {
	f := &fakeSession{keys: []rune("rnjust this\r/s")}
	w := newWorld()
	var sender *game.Empire
	w.With(func() { sender = recipients(w)[0] })
	seedMail(w, game.Message{From: sender.Name, To: "A", Body: "nice one"})

	mailReader(f, w, false, false)

	if len(sender.Mail) != 1 {
		t.Fatalf("Reply should mail the sender; got %d messages", len(sender.Mail))
	}
	if body := sender.Mail[0].Body; strings.Contains(body, "Quote From") || body != "just this" {
		t.Errorf("declining the quote should leave only the typed text; body = %q", body)
	}
}

// TestMailReaderQuoteClampsRangeAtThePrompt walks the real prompt rather than
// the helper behind it: a last line past the end of the message is corrected to
// the message's length on screen and committed by a SECOND Enter, which is how
// every over-max entry behaves (#9). An earlier version of this test called the
// clamp directly and so proved nothing about what a player meets.
func TestMailReaderQuoteClampsRangeAtThePrompt(t *testing.T) {
	// r, y (quote), 1 + Enter (first), 20 + Enter (corrected to 3) + Enter
	// (commit), the reply text, /s.
	f := &fakeSession{keys: []rune("ry1\r20\r\rmy answer\r/s")}
	w := newWorld()
	var sender *game.Empire
	w.With(func() { sender = recipients(w)[0] })
	seedMail(w, game.Message{From: sender.Name, To: "A", Body: "one\ntwo\nthree"})

	mailReader(f, w, false, false)

	if !strings.Contains(f.out.String(), "Last Line to Quote") {
		t.Fatalf("never reached the range prompt:\n%s", f.out.String())
	}
	if len(sender.Mail) != 1 {
		t.Fatalf("an over-range last line lost the reply; got %d messages", len(sender.Mail))
	}
	body := sender.Mail[0].Body
	for _, want := range []string{"> one", "> two", "> three", "my answer"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
}

// TestQuoteLinesNamesThePlanet checks the interplanetary header, which BRE
// writes as "Quote From <realm> Of <planet>".
func TestQuoteLinesNamesThePlanet(t *testing.T) {
	m := game.Message{From: "Asgard", FromBoard: "Nova Hub", Body: "terms"}
	if got := quoteLines(m, 1, 1)[0]; got != "> Quote From Asgard Of Nova Hub" {
		t.Errorf("header = %q", got)
	}
}

// Ignore means "not now", not "ask me again next turn". The mail stop at the head
// of a later turn passes over what this session has ignored; Read Messages and
// choosing Play Game again still show it, and a new session starts with an empty
// ignore set.
func TestIgnoredMailIsNotRepeatedEveryTurn(t *testing.T) {
	w := newWorld()
	m := game.Message{From: "Ashland", To: "A", When: "07/24/2026", Body: "sekret plans"}
	seedMail(w, m)

	f := &fakeSession{keys: []rune("i")}
	mailReader(f, w, true, false)
	if !strings.Contains(stripANSI(f.out.String()), "sekret plans") {
		t.Fatalf("the message was never shown:\n%s", f.out.String())
	}

	// The next turn's stop: nothing to show, and the "no messages" line does not
	// fire either, since the inbox is not empty.
	f = &fakeSession{keys: []rune("i")}
	readTurnMail(f, w, false)
	if out := stripANSI(f.out.String()); strings.Contains(out, "sekret plans") {
		t.Errorf("an ignored message came back at the next turn:\n%s", out)
	}

	// Choosing Play Game again shows the whole inbox, as BRE's reader does.
	f = &fakeSession{keys: []rune("i")}
	readTurnMail(f, w, true)
	if !strings.Contains(stripANSI(f.out.String()), "sekret plans") {
		t.Errorf("Play Game should show an ignored message again:\n%s", f.out.String())
	}

	// Asking to read messages asks for all of them.
	f = &fakeSession{keys: []rune("i")}
	mailReader(f, w, false, false)
	if !strings.Contains(stripANSI(f.out.String()), "sekret plans") {
		t.Errorf("Read Messages should still show an ignored message:\n%s", f.out.String())
	}

	// A fresh session (a new ctx over the same world) has ignored nothing.
	if got := len(unreadMail(&ctx{World: w.World, handle: w.handle}, true, false)); got != 1 {
		t.Errorf("a new session sees %d messages, want the ignored one back", got)
	}
	if got := len(w.Player().Mail); got != 1 {
		t.Errorf("Ignore must not delete: Mail len = %d, want 1", got)
	}
}

// Opening a reply and backing out of the editor is also a decision not to deal
// with the message now, so it counts as ignored — the message stays in the inbox
// but the rest of the session's turns leave it alone. Quit marks nothing.
func TestAnAbandonedReplyCountsAsIgnored(t *testing.T) {
	w := newWorld()
	seedMail(w,
		game.Message{From: "Ashland", To: "A", When: "07/24/2026", Body: "sekret plans"},
		game.Message{From: "Ashland", To: "A", When: "07/24/2026", Body: "second thoughts"},
	)

	// r, Enter (Quote Message? = Yes), then abandon the editor; q at the second
	// message. Both are one line, so no line range is asked for (#244).
	f := &fakeSession{keys: []rune("r\r/Aq")}
	mailReader(f, w, true, false)
	if got := len(w.Player().Mail); got != 2 {
		t.Fatalf("nothing was sent, so nothing should be removed; Mail len = %d, want 2", got)
	}

	f = &fakeSession{keys: []rune("ii")}
	readTurnMail(f, w, false)
	out := stripANSI(f.out.String())
	if strings.Contains(out, "sekret plans") {
		t.Errorf("the abandoned reply's message came back this session:\n%s", out)
	}
	if !strings.Contains(out, "second thoughts") {
		t.Errorf("Quit marks nothing, so the message behind it should return:\n%s", out)
	}
}

// A one-line message has one possible line range, so IB asks for neither end of
// it (#244) and quotes the line. Asserts the editor was reached, so a flow
// change upstream cannot leave this passing on a script that ran dry.
func TestMailReaderOneLineMessageSkipsTheRangePrompts(t *testing.T) {
	// r, y (quote), the reply text, /s — with no answer for a range prompt.
	f := &fakeSession{keys: []rune("rymy answer\r/s")}
	w := newWorld()
	var sender *game.Empire
	w.With(func() { sender = recipients(w)[0] })
	seedMail(w, game.Message{From: sender.Name, To: "A", Body: "just the one line"})

	mailReader(f, w, false, false)

	out := f.out.String()
	if !strings.Contains(out, "Quote Message?") {
		t.Fatalf("never reached the quote prompt:\n%s", out)
	}
	if strings.Contains(out, "Line to Quote") {
		t.Errorf("a one-line message must not ask for a line range:\n%s", out)
	}
	if len(sender.Mail) != 1 {
		t.Fatalf("Reply should mail the sender; got %d messages", len(sender.Mail))
	}
	body := sender.Mail[0].Body
	for _, want := range []string{"> Quote From " + sender.Name, "> just the one line", "my answer"} {
		if !strings.Contains(body, want) {
			t.Errorf("reply body missing %q:\n%s", want, body)
		}
	}
}

// Choosing Play Game shows every message in the inbox, one at a time with the
// reader's prompt after each, including those ignored earlier in the session:
// BRE's run_player_turn calls read_local_messages, which takes no argument and
// so cannot tell this stop from Read Messages (BRE.EXE 0x3869). Both of runTurn's
// entry paths are covered, since they reach the stop separately.
func TestPlayGameShowsTheWholeInbox(t *testing.T) {
	for _, tc := range []struct {
		name      string
		turnsLeft int
		keys      string
	}{
		// Out of turns: the recap has nothing to pause on, then the reader.
		{"out of turns", 0, "dd"},
		// A turn to play: the recap pause, then the reader, then filler for the
		// rest of the turn.
		{"with turns", 3, " dd   0000nn"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld()
			w.Player().Prefs.AutoPayMaint = true
			w.Player().TurnsLeft = tc.turnsLeft
			seedMail(w,
				game.Message{From: "Ashland", To: "A", When: "07/24/2026", Body: "sekret plans"},
				game.Message{From: "Ashland", To: "A", When: "07/24/2026", Body: "second thoughts"},
			)
			// Both ignored earlier this session, from Read Messages.
			mailReader(&fakeSession{keys: []rune("ii")}, w, false, false)

			f := &fakeSession{keys: []rune(tc.keys)}
			runTurn(f, w)
			out := stripANSI(f.out.String())
			for _, body := range []string{"sekret plans", "second thoughts"} {
				if !strings.Contains(out, body) {
					t.Errorf("Play Game should show %q:\n%s", body, out)
				}
			}
			if n := strings.Count(out, "[Q] Quit>"); n < 2 {
				t.Errorf("want the reader's prompt after each message, saw it %d times:\n%s", n, out)
			}
			if got := len(w.Player().Mail); got != 0 {
				t.Errorf("both messages were deleted at the prompt; Mail len = %d", got)
			}
		})
	}
}

// Read Messages is mail only: the day's news has its own Entry-menu items and
// never follows the mail here.
func TestReadMessagesShowsNoNews(t *testing.T) {
	w := newWorld()
	w.NewsToday = append(w.NewsToday, game.NewsLine{Text: "Inspiron has seized the title of Planetary Master!"})
	f := &fakeSession{keys: []rune(" ")}
	readMessages(f, w)
	out := f.out.String()
	if !strings.Contains(out, "You have no messages.") {
		t.Fatalf("an empty inbox should say so:\n%s", out)
	}
	if strings.Contains(out, "Planetary Master") || strings.Contains(out, "Bulletin") {
		t.Errorf("Read Messages printed the news:\n%s", out)
	}
}
