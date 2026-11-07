package menu

import (
	"io"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
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
	if n := w.NewsToday; len(n) == 0 || !strings.Contains(n[len(n)-1].Text, "has abdicated") {
		t.Errorf("abdication posted no news: %v", n)
	}
}

// A baron another board lists is refused a new realm (game.DupeElsewhere), so
// neither abdicating nor collapsing may promise them one on a later day.
func TestEndOfRealmDoesNotPromiseADupeANewRealm(t *testing.T) {
	listed := func() *ctx {
		w := newWorld()
		w.Config.IBBS, w.Config.DupeChecking, w.Config.BoardID = true, true, "Alpha"
		far := game.NewWorldSeed(w.Config, 1)
		far.Config.BoardID = "Bravo"
		far.AddHuman("tester", "Far Testland")
		far.ExportScores()
		w.ImportBoard(game.RemoteBoard{BoardID: "Bravo", Scores: far.Outbox[0].Scores})
		return w
	}
	check := func(name, out string) {
		t.Helper()
		if !strings.Contains(out, "already playing on Bravo") || strings.Contains(out, "Return on a later day") {
			t.Errorf("%s: want the duplicate refusal and no return-later promise:\n%s", name, out)
		}
	}

	f := &fakeSession{keys: []rune("y ")}
	var err error
	func() {
		defer session.GuardEnd(&err)
		abdicate(f, listed())
	}()
	check("abdicate", f.out.String())

	fc := &fakeSession{}
	func() {
		defer session.GuardEnd(&err)
		endCollapsed(fc, listed())
	}()
	check("collapse", fc.out.String())
	// The door exits right after it, so it waits for a key.
	if out := fc.out.String(); !strings.Contains(out[strings.Index(out, "collapsed"):], "Paused") {
		t.Errorf("the collapse notice is not followed by a pause:\n%s", out)
	}
}
