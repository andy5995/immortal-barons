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
	mailReader(f, w)
	if got := len(w.Player().Mail); got != 1 {
		t.Fatalf("Ignore should keep the message; Mail len = %d, want 1", got)
	}
}

func TestMailReaderDeleteRemovesMessage(t *testing.T) {
	f := &fakeSession{keys: []rune("d")}
	w := newWorld()
	seedMail(w, game.Message{From: "Ashland", To: "A", When: "07/24/2026", Body: "hi"})
	mailReader(f, w)
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
	mailReader(f, w)
	if got := len(w.Player().Mail); got != 2 {
		t.Fatalf("Quit should keep unread messages; Mail len = %d, want 2", got)
	}
}

func TestMailReaderReplyQuotesAndMailsSender(t *testing.T) {
	// r, Enter (Quote Message? = Yes), Enter (first line), Enter (last line),
	// the reply text, /s.
	f := &fakeSession{keys: []rune("r\r\r\rthanks\r/s")}
	w := newWorld()
	// A real recipient empire is the sender, so the reply can find them.
	var sender *game.Empire
	w.With(func() { sender = recipients(w)[0] })
	seedMail(w, game.Message{From: sender.Name, To: "A", When: "07/24/2026", Body: "nice one"})

	mailReader(f, w)

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
	// A sent reply removes the original, like Delete (#122).
	if got := len(w.Player().Mail); got != 0 {
		t.Errorf("a sent reply should remove the original; player Mail len = %d, want 0", got)
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

	mailReader(f, w)

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

	mailReader(f, w)

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

	mailReader(f, w)

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

	mailReader(f, w)

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
