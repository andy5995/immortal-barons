package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A pending treaty offer is surfaced to the player at turn start with the
// proposer's Regions / Net Worth / Score and an Accept? prompt (matching BRE).
// Answering yes forms the treaty and consumes the offer.
func TestReviewTreatyOffersAcceptFormsTreatyWithStats(t *testing.T) {
	w := newWorld()
	p := w.Player()
	other := recipients(w)[0]
	other.Land = 15
	other.Score = 213
	p.TreatyOffers = []game.TreatyOffer{{From: other.Name, Type: "Free Trade Agreement"}}

	f := &fakeSession{keys: []rune("y")}
	reviewTreatyOffers(f, w)
	out := f.out.String()

	if !strings.Contains(out, other.Name) || !strings.Contains(out, "proposes") {
		t.Errorf("offer notice should name the proposer, got:\n%s", out)
	}
	// The stats layout itself is pinned exactly by TestTreatyOfferMatchesBRE;
	// here just tie each figure to its label so a value swapped between fields
	// fails (a bare Contains("15") matched any number on the screen).
	plain := sgr.ReplaceAllString(out, "")
	if !strings.Contains(plain, "Regions: 15 ") || !strings.Contains(plain, "Score: 213 ") {
		t.Errorf("offer notice should show Regions: 15 and Score: 213, got:\n%s", plain)
	}
	if !w.World.HasTreaty(p, other, "Free Trade Agreement") {
		t.Error("accepting should form the treaty")
	}
	if len(w.Player().TreatyOffers) != 0 {
		t.Error("accepted offer should be consumed")
	}
}

// Answering no drops the offer without forming a treaty, so it does not
// re-prompt every turn.
func TestReviewTreatyOffersDeclineDropsOffer(t *testing.T) {
	w := newWorld()
	p := w.Player()
	other := recipients(w)[0]
	p.TreatyOffers = []game.TreatyOffer{{From: other.Name, Type: "Free Trade Agreement"}}

	f := &fakeSession{keys: []rune("n")}
	reviewTreatyOffers(f, w)

	if w.World.HasTreaty(p, other, "Free Trade Agreement") {
		t.Error("declining should not form a treaty")
	}
	if len(w.Player().TreatyOffers) != 0 {
		t.Error("declined offer should be removed so it stops re-prompting")
	}
}

// With no pending offers, the review is silent (no output).
func TestReviewTreatyOffersSilentWhenNone(t *testing.T) {
	w := newWorld()
	f := &fakeSession{}
	reviewTreatyOffers(f, w)
	if f.out.Len() != 0 {
		t.Errorf("no offers should produce no output, got:\n%s", f.out.String())
	}
}

// An offer's attached message is shown to the recipient in the inline review.
func TestReviewTreatyOffersShowsAttachedMessage(t *testing.T) {
	w := newWorld()
	p := w.Player()
	other := recipients(w)[0]
	p.TreatyOffers = []game.TreatyOffer{{From: other.Name, Type: "Free Trade Agreement", Message: "Peace and profit."}}

	f := &fakeSession{keys: []rune("n")}
	reviewTreatyOffers(f, w)
	if !strings.Contains(f.out.String(), "Peace and profit.") {
		t.Errorf("the attached message should be shown to the recipient, got:\n%s", f.out.String())
	}
}
