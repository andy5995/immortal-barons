package game

import (
	"fmt"
	"testing"
	"time"
)

// An inbox holds at most MailboxMax messages; the oldest go first, so a
// mail-bomb cannot grow the world file.
func TestMailboxKeepsOnlyTheNewest(t *testing.T) {
	w := ipWorld("Nova Hub")
	from, to := w.Empires[0], w.Empires[1]
	for i := 0; i < MailboxMax+5; i++ {
		w.SendMail(from, to, Message{Body: fmt.Sprint(i)})
	}
	if len(to.Mail) != MailboxMax {
		t.Fatalf("inbox holds %d, want %d", len(to.Mail), MailboxMax)
	}
	if to.Mail[0].Body != "5" || to.Mail[len(to.Mail)-1].Body != fmt.Sprint(MailboxMax+4) {
		t.Fatalf("kept %q..%q, want the newest", to.Mail[0].Body, to.Mail[len(to.Mail)-1].Body)
	}
}

// Interplanetary delivery is bounded the same way.
func TestIPMailboxIsCapped(t *testing.T) {
	holdClock(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	here := ipWorld("Nova Hub")
	there := ipWorld("The Eclipse")
	for i := 0; i < MailboxMax+5; i++ {
		here.SendIPMessage(here.Empires[0], []string{"The Eclipse"}, false, fmt.Sprint(i))
	}
	for _, p := range here.Outbox {
		there.ApplyPacket(p)
	}
	for _, e := range there.Empires {
		if len(e.Mail) != MailboxMax {
			t.Fatalf("%s holds %d, want %d", e.Name, len(e.Mail), MailboxMax)
		}
	}
}

// A full inbox gives up the owner's own sent copies before any mail they
// received, and files a recap line naming the date of each message it drops.
func TestFullInboxDropsSentCopiesFirst(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Bravo")
	KeepSentCopy(a, "", "B", "01/02/2026  03:04:05 UTC", "my own")
	for i := 0; i < MailboxMax; i++ {
		w.SendMail(b, a, Message{To: "A", When: fmt.Sprintf("02/%02d/2026  00:00:00 UTC", i%28+1), Body: fmt.Sprint(i)})
	}
	if len(a.Mail) != MailboxMax {
		t.Fatalf("inbox holds %d, want %d", len(a.Mail), MailboxMax)
	}
	for _, m := range a.Mail {
		if m.Sent {
			t.Fatal("the sent copy should have gone before any received mail")
		}
	}
	if a.Mail[0].Body != "0" {
		t.Errorf("oldest received = %q, want 0 kept", a.Mail[0].Body)
	}
	if l := a.MailLost; l == nil || l.Count != 1 || l.Oldest != "01/02/2026  03:04:05 UTC" {
		t.Errorf("tally = %+v, want one loss dated as the dropped copy", l)
	}

	// With no copies left, the oldest received message goes next, and the
	// tally grows while keeping the oldest date.
	w.SendMail(b, a, Message{To: "A", When: "03/01/2026  00:00:00 UTC", Body: "late"})
	if a.Mail[0].Body != "1" || a.MailLost.Count != 2 || a.MailLost.Oldest != "01/02/2026  03:04:05 UTC" {
		t.Errorf("oldest now %q, tally %+v; want 1 and two losses from the first date", a.Mail[0].Body, a.MailLost)
	}
	if len(a.Events) != 0 {
		t.Errorf("a full inbox filed %d events; the tally replaces them", len(a.Events))
	}
}

// An inbox full of mail received keeps no copy of what its owner sends, and
// says nothing about the copy it never kept.
func TestFullInboxOfReceivedKeepsNoCopySilently(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Bravo")
	for i := 0; i < MailboxMax; i++ {
		w.SendMail(b, a, Message{To: "A", Body: fmt.Sprint(i)})
	}
	KeepSentCopy(a, "", "B", "01/02/2026  03:04:05 UTC", "my own")
	if len(a.Mail) != MailboxMax || a.Mail[0].Body != "0" {
		t.Errorf("inbox = %d messages from %q, want the %d received untouched", len(a.Mail), a.Mail[0].Body, MailboxMax)
	}
	for _, m := range a.Mail {
		if m.Sent {
			t.Error("the copy should not have been kept")
		}
	}
	if a.MailLost != nil {
		t.Errorf("tally = %+v, want none", a.MailLost)
	}
}
