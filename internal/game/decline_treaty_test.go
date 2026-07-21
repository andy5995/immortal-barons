package game

import "testing"

// DeclineTreaty drops a pending offer without forming a treaty — the counterpart
// to AcceptTreaty, used when a player rejects an incoming proposal.
func TestDeclineTreatyRemovesOfferWithoutForming(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	me := w.AddHuman("me", "Mine")
	other := w.AddHuman("them", "Theirs")
	me.TreatyOffers = []TreatyOffer{{From: "Theirs", Type: "Free Trade Agreement"}}

	if !w.DeclineTreaty(me, "Theirs", "Free Trade Agreement") {
		t.Fatal("DeclineTreaty should report the matching offer was found")
	}
	if len(me.TreatyOffers) != 0 {
		t.Errorf("declined offer should be removed, %d left", len(me.TreatyOffers))
	}
	if w.HasTreaty(me, other, "Free Trade Agreement") {
		t.Error("declining must not form a treaty")
	}
	if w.DeclineTreaty(me, "Theirs", "Free Trade Agreement") {
		t.Error("declining a non-existent offer should report false")
	}
}
