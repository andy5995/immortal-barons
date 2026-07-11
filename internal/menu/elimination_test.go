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
// next-day rebuild rule applies) rather than removing the empire outright.
func TestAbdicateMarksDeadNotRemoved(t *testing.T) {
	w := newWorld()
	w.GameDay = 4
	// Confirm by retyping the realm name, then one key to dismiss the pause.
	f := &fakeSession{keys: []rune("Testland\r ")}
	if res := abdicate(f, w); res != Quit {
		t.Fatalf("abdicate should Quit the session, got %+v", res)
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
