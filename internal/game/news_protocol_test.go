package game

import (
	"strings"
	"testing"
)

// A held packet's two directions need opposite reactions from a sysop, and the
// notice is where they find out which one they are in. Saying "once both boards
// run the same release" for both was true only of the first: releaseHeld asks
// whether THIS build speaks the packet's protocol, so upgrading here recovers a
// newer board's backlog and nothing ever recovers an older board's.
func TestProtocolHoldNoticeSaysWhichWayTheMismatchRuns(t *testing.T) {
	newer := NewWorldSeed(DefaultConfig(), 1)
	newer.NoteProtocolHold("Alpha BBS", Protocol+1)
	if len(newer.SysopNotices) != 1 {
		t.Fatalf("notices = %+v, want one", newer.SysopNotices)
	}
	if !strings.Contains(newer.SysopNotices[0], "Upgrading this board applies them") {
		t.Errorf("a newer board's packets do not offer the fix that works: %q", newer.SysopNotices[0])
	}

	older := NewWorldSeed(DefaultConfig(), 1)
	older.NoteProtocolHold("Ancient BBS", Protocol-1)
	if len(older.SysopNotices) != 1 {
		t.Fatalf("notices = %+v, want one", older.SysopNotices)
	}
	note := older.SysopNotices[0]
	if !strings.Contains(note, "NOT be applied") {
		t.Errorf("an older board's packets are not reported as unrecoverable: %q", note)
	}
	// The specific promise that was wrong, and the one a sysop would act on.
	for _, wrong := range []string{"Upgrading this board applies them", "when both boards run the same release"} {
		if strings.Contains(note, wrong) {
			t.Errorf("an older board's packets still promise %q: %q", wrong, note)
		}
	}
}

// One line per board per run, however many of its packets are held: a mismatch
// affects everything that board sends, and repeating it per file buries it.
func TestProtocolHoldNoticeIsOncePerBoard(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	for i := 0; i < 5; i++ {
		w.NoteProtocolHold("Alpha BBS", Protocol+1)
	}
	w.NoteProtocolHold("Bravo BBS", Protocol+1)
	if len(w.SysopNotices) != 2 {
		t.Fatalf("notices = %d, want one per board", len(w.SysopNotices))
	}
}
