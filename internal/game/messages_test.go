package game

import "testing"

func TestSendMail(t *testing.T) {
	w := &World{}
	from := &Empire{Name: "Crimson Horde"}
	to := &Empire{Name: "Iron Dominion"}

	w.SendMail(from, to, Message{To: "B", When: "07/24/2026", Body: "Nice attack."})

	if len(to.Mail) != 1 {
		t.Fatalf("want 1 message, got %d", len(to.Mail))
	}
	got := to.Mail[0]
	if got.From != "Crimson Horde" {
		t.Errorf("From = %q, want %q (SendMail stamps the sender)", got.From, "Crimson Horde")
	}
	if got.To != "B" || got.When != "07/24/2026" || got.Body != "Nice attack." {
		t.Errorf("stored message = %+v", got)
	}
	if len(from.Mail) != 0 {
		t.Errorf("sender Mail should be untouched, got %v", from.Mail)
	}
}
