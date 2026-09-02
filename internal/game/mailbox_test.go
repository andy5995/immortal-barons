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
