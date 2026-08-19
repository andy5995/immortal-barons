package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A realm still under new-realm protection is a candidate for BBS Coordinator
// (#149). BRE's picker (choose_target_empire, BRE.OVR 0x01aa99) admits every
// slot whose player id is > 0 and applies its protection and net-worth filters
// only when the caller asks for them, which the coordinator vote does not. IB
// skipped protected realms, so in a young game — where every rival is still
// protected — the ballot offered the voter their own realm and nothing else.
func TestProtectedRealmsAreCandidatesForCoordinator(t *testing.T) {
	w := leagueCtx(t)
	rival := w.AddHuman("bravo", "Bravo")
	rival.Protection = 10
	w.Player().Protection = 0

	f := drive(t, w, "0\r", voteCoordinator)
	if out := f.out.String(); !strings.Contains(out, "Bravo") {
		t.Errorf("a protected realm should be on the ballot, got %q", out)
	}
}

// And voting for one records that vote rather than being rejected downstream.
func TestAProtectedRealmCanBeVotedForCoordinator(t *testing.T) {
	w := leagueCtx(t)
	rival := w.AddHuman("bravo", "Bravo")
	rival.Protection = 10

	f := drive(t, w, ballotChoice(w, rival)+"\r", voteCoordinator)
	if w.Player().CoordinatorVote != "bravo" {
		t.Errorf("vote not recorded, got %q; screen was %q", w.Player().CoordinatorVote, f.out.String())
	}
}

// ballotChoice returns the 1-based index voteCoordinator will print for e.
func ballotChoice(w *ctx, want *game.Empire) string {
	n := 0
	for _, e := range w.Empires {
		if !e.Alive || e.Owner == "" {
			continue
		}
		n++
		if e == want {
			break
		}
	}
	return string(rune('0' + n))
}

