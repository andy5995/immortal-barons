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
	f := &fakeSession{keys: []rune("rthanks\r/s")}
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
	if !strings.Contains(got.Body, "thanks") {
		t.Errorf("reply should carry the new text; body = %q", got.Body)
	}
	// The original stays — only Delete removes.
	if got := len(w.Player().Mail); got != 1 {
		t.Errorf("Reply should keep the original; player Mail len = %d, want 1", got)
	}
}
