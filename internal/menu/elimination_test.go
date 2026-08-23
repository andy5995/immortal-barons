package menu

import (
	"io"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/session"
)

// TestMidSessionDeathEndsSession proves the active player cannot keep playing a
// corpse: once an action (or a concurrent node) marks the empire !Alive, the
// next Run iteration prints the collapse notice and unwinds the session via
// session.End. Player() returns the dead husk (non-nil) here, so the guard must
// key off !Alive, and reading the husk must not panic.
func TestMidSessionDeathEndsSession(t *testing.T) {
	w := newWorld()
	ran := false
	m := &Menu{Title: "Kill Test", Items: []Item{
		{Key: 'k', Label: "Kill", Do: func(s session.Session, g *ctx) Result {
			g.With(func() { g.Player().Alive = false })
			ran = true
			return Stay
		}},
	}}
	f := &fakeSession{keys: []rune("k")}
	var err error
	func() {
		defer session.GuardEnd(&err)
		err = Run(f, w, m)
	}()
	if !ran {
		t.Fatal("kill action never ran")
	}
	if err != io.EOF {
		t.Fatalf("dead player should end the session via session.End (io.EOF), got %v", err)
	}
	if !strings.Contains(f.out.String(), "collapsed") {
		t.Errorf("expected the collapse notice, got %q", f.out.String())
	}
}

// TestAbdicateMarksDeadNotRemoved verifies abdication keeps the husk (so the
// next-day rebuild rule applies) rather than removing the empire outright, and
// that it takes ONE yes/no answer defaulting to No — BRE's confirm_end_game
// asks "Are you POSITIVE you wish to erase your empire?" and nothing more.
func TestAbdicateMarksDeadNotRemoved(t *testing.T) {
	// Enter alone must NOT abdicate: the default is No.
	wSafe := newWorld()
	fSafe := &fakeSession{keys: []rune("\r ")}
	if res := abdicate(fSafe, wSafe); res != Stay {
		t.Fatalf("Enter at the confirmation abdicated; the default must be No (got %+v)", res)
	}
	if e := wSafe.FindByOwner("tester"); e == nil || !e.Alive {
		t.Error("a declined abdication killed the realm")
	}
	if !strings.Contains(fSafe.out.String(), "changed your mind") {
		t.Errorf("no reprieve notice:\n%s", fSafe.out.String())
	}

	w := newWorld()
	w.GameDay = 4
	// "y" to confirm, then one key to dismiss the pause. A confirmed abdication
	// ENDS the session as BRE's does, so it unwinds through session.End rather
	// than returning — the same route an elimination takes.
	f := &fakeSession{keys: []rune("y ")}
	var err error
	func() {
		defer session.GuardEnd(&err)
		abdicate(f, w)
	}()
	if err != io.EOF {
		t.Fatalf("abdication should end the session via session.End (io.EOF), got %v", err)
	}
	e := w.FindByOwner("tester")
	if e == nil {
		t.Fatal("abdication must keep the husk, not remove the empire")
	}
	if e.Alive {
		t.Error("abdicated empire should be marked dead")
	}
	if e.DiedDay != 4 {
		t.Errorf("DiedDay = %d, want the current GameDay (4)", e.DiedDay)
	}
}
