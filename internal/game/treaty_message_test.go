package game

import "testing"

// ProposeTreaty stores an attached message on the offer, so the recipient can
// read the note that came with the proposal.
func TestProposeTreatyStoresAttachedMessage(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")

	w.ProposeTreatyWithMessage(from, to, "Free Trade Agreement", "Peace and profit.")

	if len(to.TreatyOffers) != 1 {
		t.Fatalf("want 1 pending offer, got %d", len(to.TreatyOffers))
	}
	if got := to.TreatyOffers[0].Message; got != "Peace and profit." {
		t.Errorf("attached message not stored: %q", got)
	}
}
