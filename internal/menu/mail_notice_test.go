package menu

import (
	"strings"
	"testing"
)

// TestShowUnreadMailReadsInline: a player with mail sees the notice and, on
// 'y', reads the messages inline; the mailbox is then cleared (#3).
func TestShowUnreadMailReadsInline(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Mail = []string{"Alice: hi", "Bob: gg"}

	f := &fakeSession{keys: []rune("y ")} // y=read, space=pause key
	showUnreadMail(f, w)

	out := f.out.String()
	if !strings.Contains(out, "2 new messages") {
		t.Errorf("expected new-mail notice, got: %q", out)
	}
	if !strings.Contains(out, "Alice: hi") || !strings.Contains(out, "Bob: gg") {
		t.Errorf("expected messages read inline, got: %q", out)
	}
	if len(p.Mail) != 0 {
		t.Errorf("mail should be cleared after reading, got %v", p.Mail)
	}
}

// TestShowUnreadMailDeclineKeepsMail: declining ('n') leaves the mail for the
// Messages menu; a single message uses the singular notice.
func TestShowUnreadMailDeclineKeepsMail(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Mail = []string{"Alice: hi"}

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
