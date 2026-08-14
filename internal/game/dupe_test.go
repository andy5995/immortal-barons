package game

import (
	"encoding/json"
	"strings"
	"testing"
)

func dupeWorld(t *testing.T) (*World, *Empire) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.BoardID = "here"
	w := NewWorldSeed(cfg, 1)
	return w, w.AddHuman("Alice", "Alethia")
}

// A packet naming one of our own barons locks that realm, and a later packet
// that no longer names them releases it — BRE's "until they delete one of their
// players".
func TestDupeCheckLocksAndReleases(t *testing.T) {
	w, e := dupeWorld(t)

	elsewhere := []RemoteScore{{Empire: "Far Reach", OwnerHash: dupeHash("alice")}}
	w.applyDupeCheck("there", elsewhere)
	if e.DupeLockedBy != "there" {
		t.Fatalf("DupeLockedBy = %q, want \"there\"", e.DupeLockedBy)
	}
	if !w.DupeLocked(e) {
		t.Error("a locked baron should be shut out while Dupe Checking is on")
	}
	if msg := w.DupeLockMessage(e); !strings.Contains(msg, "there") {
		t.Errorf("the message should name the other board, got %q", msg)
	}

	w.applyDupeCheck("there", []RemoteScore{{Empire: "Someone Else", OwnerHash: dupeHash("bob")}})
	if e.DupeLockedBy != "" {
		t.Errorf("giving up the other realm should release the lock, got %q", e.DupeLockedBy)
	}
}

// The switch is read at the gate, not at the lock, so a Coordinator who turns
// Dupe Checking off frees everyone without a sweep over the world.
func TestDupeCheckingOffReleasesLockedBarons(t *testing.T) {
	w, e := dupeWorld(t)
	w.applyDupeCheck("there", []RemoteScore{{OwnerHash: dupeHash("alice")}})

	w.Config.DupeChecking = false
	if w.DupeLocked(e) {
		t.Error("Dupe Checking off should let a locked baron play")
	}
	w.Config.DupeChecking, w.Config.IBBS = true, false
	if w.DupeLocked(e) {
		t.Error("a stand-alone board has no league to hold a duplicate")
	}
}

// Only the board that reported a duplicate can release it, or a quiet board
// would clear a lock another board is still asserting.
func TestDupeLockSurvivesAnotherBoardsPacket(t *testing.T) {
	w, e := dupeWorld(t)
	w.applyDupeCheck("there", []RemoteScore{{OwnerHash: dupeHash("alice")}})
	w.applyDupeCheck("elsewhere", []RemoteScore{{OwnerHash: dupeHash("bob")}})
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
