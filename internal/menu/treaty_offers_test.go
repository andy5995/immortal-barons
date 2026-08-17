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

// BRE answers a pending offer BEFORE printing the numbered recap entries, and
// takes the trade barter before the treaty. Read out of run_player_turn
// (BRE.EXE 0x36E1): header, then process_trade_offer at 0x3855, then
// process_diplomatic_proposal at 0x385A, then write_data_report at 0x385F for
// the entries. IB ran the entries first and the treaty before the trade, so the
// player answered a proposal underneath news that was meant to follow it.
//
// The captures cannot settle this on their own: no BRE screen anywhere shows an
// offer and numbered entries in one recap, which is why this is pinned against
// the call order instead.
func TestPendingOfferIsAnsweredBeforeTheRecapEntries(t *testing.T) {
	w := newWorld()
	p := w.Player()
	other := recipients(w)[0]
	w.World.ProposeTreaty(other, p, "Full Defense Alliance")
	p.Events = []game.Event{{Text: "Pirates raided your treasury!"}}

	f := &fakeSession{keys: []rune("n\r \r")}
	openTurnRecap(f, w)
	out := f.out.String()

	offer := strings.Index(out, "proposes")
	entry := strings.Index(out, "Pirates raided your treasury!")
	if offer < 0 {
		t.Fatalf("never reached the offer prompt, got:\n%s", out)
	}
	if entry < 0 {
		t.Fatalf("never reached the recap entry, got:\n%s", out)
	}
	if offer > entry {
		t.Errorf("offer at %d rendered AFTER the recap entry at %d; BRE prompts it first:\n%s",
			offer, entry, out)
	}
}
