package game

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func dupeWorld(t *testing.T) (*World, *Empire) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.BoardID = "here"
	w := NewWorldSeed(cfg, 1)
	return w, w.AddHuman("Alice", "Alethia")
}

// receive takes a board's scores the way ApplyPacket does: the roster first,
// then the duplicate check against it.
func receive(w *World, board string, scores []RemoteScore) {
	if w.ImportBoard(RemoteBoard{BoardID: board, Scores: scores}) {
		w.applyDupeCheck(board, scores)
	}
}

// A packet naming one of our own barons locks that realm, and a later packet
// that no longer names them releases it — BRE's "until they delete one of their
// players".
func TestDupeCheckLocksAndReleases(t *testing.T) {
	w, e := dupeWorld(t)

	elsewhere := []RemoteScore{{Empire: "Far Reach", OwnerHash: dupeHash("alice")}}
	receive(w, "there", elsewhere)
	if e.DupeLockedBy != "there" {
		t.Fatalf("DupeLockedBy = %q, want \"there\"", e.DupeLockedBy)
	}
	if !w.DupeLocked(e) {
		t.Error("a locked baron should be shut out while Dupe Checking is on")
	}
	if msg := w.DupeLockMessage(e).English(); !strings.Contains(msg, "there") {
		t.Errorf("the message should name the other board, got %q", msg)
	}

	receive(w, "there", []RemoteScore{{Empire: "Someone Else", OwnerHash: dupeHash("bob")}})
	if e.DupeLockedBy != "" {
		t.Errorf("giving up the other realm should release the lock, got %q", e.DupeLockedBy)
	}
}

// The switch is read at the gate, not at the lock, so a Coordinator who turns
// Dupe Checking off frees everyone without a sweep over the world.
func TestDupeCheckingOffReleasesLockedBarons(t *testing.T) {
	w, e := dupeWorld(t)
	receive(w, "there", []RemoteScore{{OwnerHash: dupeHash("alice")}})

	w.Config.DupeChecking = false
	if w.DupeLocked(e) {
		t.Error("Dupe Checking off should let a locked baron play")
	}
	w.Config.DupeChecking, w.Config.IBBS = true, false
	if w.DupeLocked(e) {
		t.Error("a stand-alone board has no league to hold a duplicate")
	}
}

// The -dupe-check testing switch covers BOTH sites that ask whether checking is
// on: the login gate above, and what this board's outgoing packets carry. Off,
// the scores go out with no owner hash, so no other board can lock anyone on
// this run's account either — a half-applied override would leave the tester
// shut out on the far side of the league.
func TestDupeCheckOverrideReachesTheOutgoingPacket(t *testing.T) {
	w, e := dupeWorld(t)
	w.LastMaintDate = "2026-08-16"

	w.ExportScores()
	if len(w.Outbox) != 1 || len(w.Outbox[0].Scores) != 1 {
		t.Fatalf("expected one score in one packet, got %#v", w.Outbox)
	}
	if w.Outbox[0].Scores[0].OwnerHash != dupeHash(e.Owner) {
		t.Fatal("the packet should carry the owner hash while Dupe Checking is on")
	}

	off := false
	w.Config.DupeCheckOverride = &off
	w.Outbox = nil
	w.ExportScores()
	if got := w.Outbox[0].Scores[0].OwnerHash; got != "" {
		t.Errorf("OwnerHash = %q, want the override to keep it out of the packet", got)
	}
}

// Only the board that reported a duplicate can release it, or a quiet board
// would clear a lock another board is still asserting.
func TestDupeLockSurvivesAnotherBoardsPacket(t *testing.T) {
	w, e := dupeWorld(t)
	receive(w, "there", []RemoteScore{{OwnerHash: dupeHash("alice")}})
	receive(w, "elsewhere", []RemoteScore{{OwnerHash: dupeHash("bob")}})
	if e.DupeLockedBy != "there" {
		t.Errorf("DupeLockedBy = %q, want the lock to survive an unrelated board", e.DupeLockedBy)
	}
}

// The wire carries a hash, never the handle: a scores packet is a file that
// lands on every other sysop's board and stays there.
func TestExportedScoresCarryNoHandle(t *testing.T) {
	w, e := dupeWorld(t)
	e.Owner = "alice"
	w.ExportScores()
	if len(w.Outbox) != 1 {
		t.Fatalf("no packet was queued: %+v", w.Outbox)
	}
	data, err := json.Marshal(w.Outbox[0])
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "alice") {
		t.Errorf("the packet carries the caller's handle:\n%s", data)
	}
	if got := w.Outbox[0].Scores[0].OwnerHash; got != dupeHash("alice") {
		t.Errorf("OwnerHash = %q, want the hash of the handle", got)
	}

	// With the switch off the board claims nobody, so it sends no hash either.
	w.Outbox = nil
	w.Config.DupeChecking = false
	w.ExportScores()
	if got := w.Outbox[0].Scores[0].OwnerHash; got != "" {
		t.Errorf("OwnerHash = %q with Dupe Checking off, want empty", got)
	}
}

// A caller another board lists is named, so onboarding can refuse them a realm;
// nobody is named with checking off, off a league, or for an AI baron.
func TestDupeElsewhereNamesTheListingBoard(t *testing.T) {
	w, _ := dupeWorld(t)
	w.ImportBoard(RemoteBoard{BoardID: "there", Scores: []RemoteScore{{Empire: "Far Reach", OwnerHash: dupeHash("Bob")}}})
	if got := w.DupeElsewhere("bob"); got != "there" {
		t.Errorf("DupeElsewhere(bob) = %q, want \"there\"", got)
	}
	if got := w.DupeElsewhere("carol"); got != "" {
		t.Errorf("DupeElsewhere(carol) = %q, want none", got)
	}
	if got := w.DupeElsewhere(""); got != "" {
		t.Errorf("an AI baron was named a duplicate of %q", got)
	}
	w.Config.DupeChecking = false
	if got := w.DupeElsewhere("bob"); got != "" {
		t.Errorf("with Dupe Checking off, DupeElsewhere(bob) = %q", got)
	}
}

// With checking off no lock is recorded, so none can ever exist without the
// event that announces it; the first packet after checking comes back on sets
// it, with the event.
func TestDupeLockIsOnlySetWhileCheckingIsOn(t *testing.T) {
	w, e := dupeWorld(t)
	listed := []RemoteScore{{OwnerHash: dupeHash("alice")}}
	w.Config.DupeChecking = false
	receive(w, "there", listed)
	if e.DupeLockedBy != "" || len(e.Events) != 0 {
		t.Fatalf("with checking off: DupeLockedBy %q, events %v", e.DupeLockedBy, e.Events)
	}
	w.Config.DupeChecking = true
	if w.DupeLocked(e) {
		t.Fatal("a lock appeared that no packet set while checking was on")
	}
	receive(w, "there", listed)
	if !w.DupeLocked(e) || len(e.Events) != 1 {
		t.Errorf("after checking came back on: locked %v, events %v", w.DupeLocked(e), e.Events)
	}
}

// A board that has left the league never sends the scores that would lift the
// refusal, so its last roster refuses nobody.
func TestDupeElsewhereIgnoresBoardsOffTheRoster(t *testing.T) {
	w, _ := dupeWorld(t)
	w.ImportBoard(RemoteBoard{BoardID: "gone", Scores: []RemoteScore{{OwnerHash: dupeHash("bob")}}})
	w.LeagueNodes = []LeagueNode{{Number: 1, Name: "here"}, {Number: 2, Name: "there"}}
	if got := w.DupeElsewhere("bob"); got != "" {
		t.Errorf("a board off the roster refused bob: %q", got)
	}
}

// A lock held by a board that has left the league would never lift, since that
// board sends no more scores, so it is not enforced.
func TestDupeLockFromABoardOffTheRosterIsNotEnforced(t *testing.T) {
	w, e := dupeWorld(t)
	receive(w, "gone", []RemoteScore{{OwnerHash: dupeHash("alice")}})
	if !w.DupeLocked(e) {
		t.Fatal("with no roster yet, the lock should hold")
	}
	w.LeagueNodes = []LeagueNode{{Number: 2, Name: "there"}}
	if !w.DupeLocked(e) {
		t.Error("a board the roster does not list yet ruled the lock out")
	}
	w.LeagueNodes = []LeagueNode{{Number: 1, Name: "here"}, {Number: 2, Name: "there"}}
	if w.DupeLocked(e) {
		t.Error("a lock from a board off the roster is still enforced")
	}
	if got := w.PlayerListReport(); !strings.Contains(got, "that board has left the league") {
		t.Errorf("the report does not say why the lock is not enforced:\n%s", got)
	}
	w.LeagueNodes = append(w.LeagueNodes, LeagueNode{Number: 3, Name: "gone"})
	if !w.DupeLocked(e) {
		t.Error("a lock from a board on the roster is not enforced")
	}
}

// A saved lock is not enforced with inter-BBS play off, and the player list
// says which clause of DupeLocked is the one holding it back.
func TestPlayerListExplainsALockUnenforcedByInterBBSOff(t *testing.T) {
	w, _ := dupeWorld(t)
	receive(w, "there", []RemoteScore{{OwnerHash: dupeHash("alice")}})
	w.Config.IBBS = false
	if got := w.PlayerListReport(); !strings.Contains(got, "inter-BBS play is off") {
		t.Errorf("the report does not say why the lock is not enforced:\n%s", got)
	}
}

// Scores dated before the ones held change nothing: a late packet must not
// bring back a baron who has since gone, nor lock them again.
func TestOlderScoresNeitherReplaceTheRosterNorLock(t *testing.T) {
	w, e := dupeWorld(t)
	w.ImportBoard(RemoteBoard{BoardID: "there", Date: "2026-10-06"})
	late := []RemoteScore{{Empire: "Far Reach", OwnerHash: dupeHash("alice")}}
	if w.ImportBoard(RemoteBoard{BoardID: "there", Date: "2026-10-05", Scores: late}) {
		t.Error("an older packet was imported")
	}
	if len(w.RemoteBoards[0].Scores) != 0 || w.RemoteBoards[0].Date != "2026-10-06" {
		t.Errorf("the roster became %+v, want the newer one kept", w.RemoteBoards[0])
	}
	if w.DupeElsewhere("alice") != "" || e.DupeLockedBy != "" {
		t.Error("an older packet refused or locked alice")
	}
}

// A board whose scores have stopped arriving, as one left with no players
// does (#314), neither refuses a new realm nor holds a lock.
func TestQuietBoardNeitherRefusesNorLocks(t *testing.T) {
	w, e := dupeWorld(t)
	receive(w, "there", []RemoteScore{{OwnerHash: dupeHash("alice")}, {OwnerHash: dupeHash("bob")}})
	if !w.DupeLocked(e) || w.DupeElsewhere("bob") != "there" {
		t.Fatal("fresh scores should lock alice and refuse bob")
	}
	w.RemoteBoards[0].Received = timeNow().Add(-time.Duration(DupeScoresMaxAgeDays+1) * 24 * time.Hour)
	if w.DupeLocked(e) {
		t.Error("a lock from a board gone quiet is still enforced")
	}
	if got := w.DupeElsewhere("bob"); got != "" {
		t.Errorf("a board gone quiet refused bob: %q", got)
	}
	if got := w.PlayerListReport(); !strings.Contains(got, "no scores from that board") {
		t.Errorf("the report does not say why the lock is not enforced:\n%s", got)
	}
}

// Only the newer of a baron's two realms is locked, so the baron keeps playing
// where they began (#323). Both boards compare the same two stamps, so exactly
// one of them locks when the stamps differ. A realm with no stamp predates the
// field and is the older; two unknown stamps lock both.
func TestDupeLocksOnlyTheNewerRealm(t *testing.T) {
	early, late := Recorded(timeNow().Add(-time.Hour)), Recorded(timeNow())
	for _, tc := range []struct {
		name        string
		here, there string
		lockedHere  bool
		lockedThere bool
	}{
		{"newer here", late, early, true, false},
		{"older here", early, late, false, true},
		{"unknown here", "", late, false, true},
		{"unknown there", early, "", true, false},
		{"unknown both", "", "", true, true},
	} {
		a, ea := dupeWorld(t)
		b, eb := dupeWorld(t)
		b.Config.BoardID = "there"
		ea.Founded, eb.Founded = tc.here, tc.there
		a.ExportScores()
		b.ExportScores()
		receive(a, "there", b.Outbox[0].Scores)
		receive(b, "here", a.Outbox[0].Scores)
		if a.DupeLocked(ea) != tc.lockedHere || b.DupeLocked(eb) != tc.lockedThere {
			t.Errorf("%s: locked here %v there %v, want %v and %v", tc.name,
				a.DupeLocked(ea), b.DupeLocked(eb), tc.lockedHere, tc.lockedThere)
		}
	}
}

// A lock taken while neither founding was known is released once the other
// board's realm turns out to be the newer.
func TestDupeLockOnTheOlderRealmIsReleased(t *testing.T) {
	w, e := dupeWorld(t)
	e.Founded = ""
	receive(w, "there", []RemoteScore{{OwnerHash: dupeHash("alice")}})
	if !w.DupeLocked(e) {
		t.Fatal("two unknown foundings should lock")
	}
	receive(w, "there", []RemoteScore{{OwnerHash: dupeHash("alice"), Founded: Recorded(timeNow())}})
	if w.DupeLocked(e) {
		t.Error("the older realm is still locked once the other is known to be newer")
	}
	last := e.Events[len(e.Events)-1].Text
	if !strings.Contains(last, "newer") || strings.Contains(last, "no longer lists") {
		t.Errorf("the release should say the other realm is newer, got %q", last)
	}
}

// A scores packet from a board the roster does not list records no lock: the
// baron would be told they were locked, and DupeLocked would never enforce it.
func TestDupeCheckDoesNotLockFromABoardOffTheRoster(t *testing.T) {
	w, e := dupeWorld(t)
	w.LeagueNodes = []LeagueNode{{Number: 1, Name: "here"}, {Number: 2, Name: "there"}}
	receive(w, "gone", []RemoteScore{{OwnerHash: dupeHash("alice")}})
	if e.DupeLockedBy != "" {
		t.Errorf("a board off the roster recorded a lock: %q", e.DupeLockedBy)
	}
	for _, ev := range e.Events {
		if strings.Contains(ev.Text, "locked") {
			t.Errorf("a board off the roster announced a lock: %q", ev.Text)
		}
	}
}
