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

// The Player List names realms, not the callers behind them — BRE prints a BBS
// account name only in the sysop's PLAYERS.LST, never on a screen.
func TestPlayerListDoesNotShowBBSHandles(t *testing.T) {
	w := leagueCtx(t)
	w.AddHuman("bravo", "Bravo")

	f := drive(t, w, " ", playerList)
	out := f.out.String()
	if !strings.Contains(out, "Bravo") {
		t.Fatalf("the roster should list the realm, got %q", out)
	}
	if strings.Contains(out, "bravo") || strings.Contains(out, "tester") {
		t.Errorf("the roster names a caller's BBS handle: %q", out)
	}
}

// Dismantle is the Coordinator's lever, not the builder's — BRE's Dismantle
// Gooie, item 1 of the Coordinator Ops menu (#45). The weapon here belongs to
// another baron, which is the whole point: peace is made and the strike aimed
// at the new ally is called off over its builder's head.
func TestCoordinatorDismantlesAnotherBaronsAnnihilator(t *testing.T) {
	w := leagueCtx(t)
	w.Config.GooieKablooie = true
	w.RemoteBoards = []game.RemoteBoard{{BoardID: "Nova Hub"}}
	builder := w.AddHuman("bravo", "Bravo")
	if err := w.StartAnnihilator(builder, "Nova Hub"); err != nil {
		t.Fatalf("start: %v", err)
	}

	f := drive(t, w, "y", dismantleAnnihilator)
	if out := f.out.String(); !strings.Contains(out, "Nova Hub") {
		t.Errorf("the screen should name the target, got %q", out)
	}
	if w.Annihilator != nil {
		t.Error("the weapon should have been dismantled")
	}
}

// An ordinary baron gets the refusal, whoever built it.
func TestOnlyTheCoordinatorDismantlesTheAnnihilator(t *testing.T) {
	w := leagueCtx(t)
	w.Config.GooieKablooie = true
	w.RemoteBoards = []game.RemoteBoard{{BoardID: "Nova Hub"}}
	if err := w.StartAnnihilator(w.Player(), "Nova Hub"); err != nil {
		t.Fatalf("start: %v", err)
	}
	other := w.AddHuman("bravo", "Bravo")
	w.VoteCoordinator(w.Player(), other.Owner) // hand the office to Bravo

	f := drive(t, w, "y", dismantleAnnihilator)
	if out := f.out.String(); !strings.Contains(out, "Coordinator") {
		t.Errorf("want the coordinator-only refusal, got %q", out)
	}
	if w.Annihilator == nil {
		t.Error("a baron who does not hold the office dismantled the weapon")
	}
}

// Send SpyGuy quotes the price per day, asks how long, and charges gold — no
// agent is spent, and the target is a PLANET rather than a baron. The whole
// shape is BRE's (see internal/game/spyguy.go).
func TestSendSpyGuyChargesGoldForDays(t *testing.T) {
	w := leagueCtx(t)
	w.Player().Protection = 0
	w.Player().Gold = 1_000_000_000
	agents, gold := w.Player().Agents, w.Player().Gold
	perDay := w.SpyGuyCostPerDay()

	// Pick the first planet by number, then take the default stay.
	f := drive(t, w, "1\r\r ", sendSpyGuy)
	out := f.out.String()
	if !strings.Contains(out, "gold per day") {
		t.Fatalf("the price per day was never quoted:\n%s", out)
	}
	if w.Player().Agents != agents {
		t.Errorf("a SpyGuy cost %d agents; he is not a covert agent", agents-w.Player().Agents)
	}
	want := perDay * int64(game.SpyGuyDefaultDays)
	if spent := gold - w.Player().Gold; spent != want {
		t.Errorf("charged %d gold, want %d for the %d-day default", spent, want, game.SpyGuyDefaultDays)
	}
}

// The Coordinator Vote item is withheld until new-realm protection ends, as the
// original withholds it (show_game_settings +0x082f), and never appears off a
// league at all.
func TestCoordinatorVoteIsHiddenUnderProtection(t *testing.T) {
	menus := BuildMenus()
	var item *Item
	for i := range menus.System.Items {
		if it := &menus.System.Items[i]; it.Label == "Coordinator Vote" {
			item = it
			break
		}
	}
	if item == nil || item.Hidden == nil {
		t.Fatal("the System menu has no Coordinator Vote item with a visibility rule")
	}

	w := newWorld()
	w.Config.IBBS = true
	w.Player().Protection = 10
	if !item.Hidden(w) {
		t.Error("a protected realm cannot vote, so the item should not be drawn")
	}
	w.Player().Protection = 0
	if item.Hidden(w) {
		t.Error("the item should appear once protection ends")
	}
	w.Config.IBBS = false
	if !item.Hidden(w) {
		t.Error("a standalone board has no Coordinator to vote for")
	}
}
