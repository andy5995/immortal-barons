package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// Send Message files the sender's own copy, addressed as the recipients' copies
// are, beside the mail the sender received.
func TestSendMessageKeepsTheSendersCopy(t *testing.T) {
	w := newWorld()
	w.With(func() { w.AddAIEmpires(1) })
	rows, _ := pickRows(w, pickOpts{})
	to := rows[0]
	f := &fakeSession{keys: []rune(string(to.letter) + "\rMeet at dawn.\r/s")}
	sendMessage(f, w)

	if len(to.e.Mail) != 1 {
		t.Fatalf("recipient got %d messages, want 1:\n%s", len(to.e.Mail), f.out.String())
	}
	mine := w.Player().Mail
	if len(mine) != 1 || !mine[0].Sent {
		t.Fatalf("sender's inbox = %+v, want their one copy", mine)
	}
	if mine[0].Body != to.e.Mail[0].Body || mine[0].To != to.e.Mail[0].To || mine[0].When != to.e.Mail[0].When {
		t.Errorf("copy %+v does not match what was sent %+v", mine[0], to.e.Mail[0])
	}
}

// A copy is read only from View Sent Messages: Read Messages, the stop on
// choosing Play Game and the stop at a later turn all show received mail alone.
func TestSentCopyShowsOnlyInViewSent(t *testing.T) {
	w := newWorld()
	w.Player().Mail = []game.Message{
		{From: "Ashland", To: "A", Body: "their words"},
		{From: w.Player().Name, To: "B", Body: "my own words", Sent: true},
	}
	for _, skip := range []bool{false, true} {
		got := unreadMail(w, skip, false)
		if len(got) != 1 || got[0].Sent {
			t.Errorf("received mail (skipIgnored %v) = %+v, want only theirs", skip, got)
		}
	}

	f := &fakeSession{keys: []rune("q")}
	viewSentMessages(f, w)
	out := f.out.String()
	if !strings.Contains(out, "my own words") || strings.Contains(out, "their words") {
		t.Errorf("View Sent Messages should show only the sent copy:\n%s", out)
	}

	w.Player().Mail = w.Player().Mail[:1]
	f = &fakeSession{keys: []rune(" ")}
	viewSentMessages(f, w)
	if !strings.Contains(f.out.String(), "You have no sent messages.") {
		t.Errorf("an empty sent list should say so:\n%s", f.out.String())
	}
}

// Replying to your own copy works like forwarding: pick who it goes to (never
// yourself), quote, write. The pick lands the reply and files a new copy.
func TestReplyToOwnCopyPicksRecipients(t *testing.T) {
	w := newWorld()
	w.With(func() { w.AddAIEmpires(1) })
	rows, _ := pickRows(w, pickOpts{})
	to := rows[0]
	w.Player().Mail = []game.Message{{From: w.Player().Name, To: "Z", Body: "first words", When: "07/24/2026", Sent: true}}
	// r, pick the realm + Enter, Enter (quote), the text, /s, d (delete the old copy).
	f := &fakeSession{keys: []rune("r" + string(to.letter) + "\r\rmore words\r/sd")}
	mailReader(f, w, false, true)

	if !strings.Contains(f.out.String(), "Send to:") {
		t.Fatalf("never reached the recipient picker:\n%s", f.out.String())
	}
	if len(to.e.Mail) != 1 {
		t.Fatalf("recipient got %d messages, want 1", len(to.e.Mail))
	}
	if got := to.e.Mail[0].Body; !strings.Contains(got, "> first words") || !strings.Contains(got, "more words") {
		t.Errorf("reply body = %q, want the quote and the new text", got)
	}
	mine := w.Player().Mail
	if len(mine) != 1 || !mine[0].Sent || mine[0].Body != to.e.Mail[0].Body {
		t.Errorf("sender's inbox = %+v, want only the new copy", mine)
	}
}

// A sent copy's address can list many planets; it wraps inside the box rather
// than running past the terminal's edge.
func TestLongAddressWrapsInsideTheBox(t *testing.T) {
	f := &fakeSession{}
	to := strings.TrimSuffix(strings.Repeat("Planet Number Something, ", 8), ", ")
	messageBox(f, game.Message{From: "Testland", To: to, Body: "hi", Sent: true})
	plain := stripANSI(f.out.String())
	for _, line := range strings.Split(plain, "\n") {
		if n := len([]rune(line)); n > mailBoxWidth {
			t.Errorf("line runs %d columns, past the %d-column box: %q", n, mailBoxWidth, line)
		}
	}
	if strings.Count(plain, "Planet") != 8 || strings.Count(plain, "Something") != 8 {
		t.Errorf("the address lost a planet in wrapping:\n%s", plain)
	}
}

// A full inbox's losses reach the recap as ONE line however many there were,
// and the tally clears once shown.
func TestRecapReportsMailLostOnce(t *testing.T) {
	w := newWorld()
	w.Player().MailLost = &game.MailLoss{Count: 3, Oldest: "01/02/2026  03:04:05 UTC", At: game.Now()}
	f := &fakeSession{keys: []rune(" ")}
	showTurnEvents(f, w)
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Mailbox full, deleted 3 messages, the oldest from 01/02/2026") {
		t.Errorf("recap should report the three losses in one line:\n%s", out)
	}
	if w.Player().MailLost != nil {
		t.Error("the tally should clear once the recap has shown it")
	}
}
