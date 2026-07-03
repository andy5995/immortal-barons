package game

import (
	"strings"
	"testing"
)

func TestSendGoldTransfersAndMails(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("sender", "Senderland")
	to := w.AddHuman("recipient", "Recipientland")

	from.Gold = 1000
	to.Gold = 500

	if err := w.SendGold(from, to, 300); err != nil {
		t.Fatalf("SendGold: %v", err)
	}
	if from.Gold != 700 {
		t.Errorf("from.Gold: want 700, got %d", from.Gold)
	}
	if to.Gold != 800 {
		t.Errorf("to.Gold: want 800, got %d", to.Gold)
	}
	if len(to.Mail) != 1 {
		t.Fatalf("to.Mail: want 1 entry, got %d", len(to.Mail))
	}
	if !strings.Contains(to.Mail[0], from.Name) || !strings.Contains(to.Mail[0], "300") {
		t.Errorf("to.Mail[0] = %q, want it to mention sender and amount", to.Mail[0])
	}
}

func TestSendGoldRejectsWhenBroke(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("sender", "Senderland")
	to := w.AddHuman("recipient", "Recipientland")

	from.Gold = 100
	to.Gold = 500

	if err := w.SendGold(from, to, 200); err != ErrCantAfford {
		t.Fatalf("SendGold: want ErrCantAfford, got %v", err)
	}
	if from.Gold != 100 {
		t.Errorf("from.Gold should be unchanged: got %d", from.Gold)
	}
	if to.Gold != 500 {
		t.Errorf("to.Gold should be unchanged: got %d", to.Gold)
	}
	if len(to.Mail) != 0 {
		t.Errorf("to.Mail should be unchanged: got %d entries", len(to.Mail))
	}
}

func TestSendGoldRespectsMoneyCap(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("sender", "Senderland")
	to := w.AddHuman("recipient", "Recipientland")

	from.Gold = 1000
	to.Gold = MoneyCap - 50
	requestedAmount := 100

	if err := w.SendGold(from, to, requestedAmount); err != nil {
		t.Fatalf("SendGold: %v", err)
	}
	// Recipient can only receive 50 (to reach the cap)
	if to.Gold != MoneyCap {
		t.Errorf("to.Gold: want %d (capped), got %d", MoneyCap, to.Gold)
	}
	// Sender should only lose 50 (not the full requested 100)
	if from.Gold != 950 {
		t.Errorf("from.Gold: want 950 (lost only 50), got %d", from.Gold)
	}
	// Mail should report only the amount actually received (50)
	if len(to.Mail) != 1 {
		t.Fatalf("to.Mail: want 1 entry, got %d", len(to.Mail))
	}
	if !strings.Contains(to.Mail[0], "50") {
		t.Errorf("to.Mail[0] = %q, want it to mention 50 gold (actual amount received)", to.Mail[0])
	}
}
