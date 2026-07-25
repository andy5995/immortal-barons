package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestShowUnreadMailReadsInline: a player with mail sees the notice and, on
// 'y', reads the messages inline in the per-message reader; Deleting both
// empties the inbox (#3).
func TestShowUnreadMailReadsInline(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Mail = []game.Message{
		{From: "Alice", To: "A", When: "07/24/2026", Body: "hi"},
		{From: "Bob", To: "A", When: "07/24/2026", Body: "gg"},
	}

	f := &fakeSession{keys: []rune("ydd")} // y=read, then Delete each
	showUnreadMail(f, w)

	out := f.out.String()
	if !strings.Contains(out, "2 new messages") {
		t.Errorf("expected new-mail notice, got: %q", out)
	}
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "Bob") {
		t.Errorf("expected both senders shown, got: %q", out)
	}
	if len(p.Mail) != 0 {
		t.Errorf("deleting both should empty the inbox, got %v", p.Mail)
	}
}

// TestShowUnreadMailDeclineKeepsMail: declining ('n') leaves the mail for the
// Messages menu; a single message uses the singular notice.
func TestShowUnreadMailDeclineKeepsMail(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Mail = []game.Message{{From: "Alice", Body: "hi"}}

	f := &fakeSession{keys: []rune("n")}
	showUnreadMail(f, w)

	if out := f.out.String(); !strings.Contains(out, "new message") {
		t.Errorf("expected singular new-message notice, got: %q", out)
	}
	if len(p.Mail) != 1 {
		t.Errorf("declining should keep mail, got %v", p.Mail)
	}
}

// TestShowUnreadMailNoneSilent: no mail -> no output.
func TestShowUnreadMailNoneSilent(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Mail = nil
	f := &fakeSession{}
	showUnreadMail(f, w)
	if out := f.out.String(); out != "" {
		t.Errorf("no mail should be silent, got: %q", out)
	}
}
