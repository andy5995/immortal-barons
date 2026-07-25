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

func TestPostBulletinCapsAt20(t *testing.T) {
	w := &World{}
	from := &Empire{Name: "Ashfall Clan"}

	for i := 0; i < 25; i++ {
		w.PostBulletin(from, string(rune('A'+i)))
	}

	if len(w.NewsToday) != 20 {
		t.Fatalf("want 20 entries, got %d", len(w.NewsToday))
	}
	// oldest 5 (A-E) should have been dropped; newest (Y, index 24) present.
	newest := "Ashfall Clan: Y"
	if w.NewsToday[len(w.NewsToday)-1] != newest {
		t.Errorf("newest entry = %q, want %q", w.NewsToday[len(w.NewsToday)-1], newest)
	}
	oldestKept := "Ashfall Clan: F"
	if w.NewsToday[0] != oldestKept {
		t.Errorf("oldest kept entry = %q, want %q", w.NewsToday[0], oldestKept)
	}
}
