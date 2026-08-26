package menu

import (
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/session"
)

// drainSpy is a Session that records whether DrainInput was forwarded to it.
type drainSpy struct {
	session.Session
	drained bool
}

func (d *drainSpy) DrainInput() { d.drained = true }

// TestLangSessionForwardsDrainInput checks that langSession — the top of the
// real game-loop input stack — forwards DrainInput to its inner session, so the
// drain after every single-key prompt is not a silent no-op.
func TestLangSessionForwardsDrainInput(t *testing.T) {
	spy := &drainSpy{}
	drainInput(&langSession{Session: spy})
	if !spy.drained {
		t.Fatal("langSession did not forward DrainInput to its inner session")
	}
}

// The real door stack is langSession -> Deadline -> charset writer -> base, and
// each wrapper has to say the charset markers out loud: embedding promotes only
// the Session interface's own methods, and both IsUTF8 and IsASCII answer from a
// default rather than failing, so a wrapper that stays quiet reports the WRONG
// charset instead of no charset. That is what left the pirate raider's ASCII
// mark unreachable (7e2e5630) and it would silently break the next reader too.
func TestCharsetMarkersSurviveTheWholeSessionStack(t *testing.T) {
	for _, tc := range []struct {
		name        string
		wrap        func(session.Session) session.Session
		utf8, ascii bool
	}{
		{"ASCII", session.NewASCIIWriter, false, true},
		{"CP437", session.NewCP437Writer, false, false},
		{"UTF-8", func(s session.Session) session.Session { return s }, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := &fakeSession{}
			stack := session.Session(&langSession{
				Session: session.NewDeadline(tc.wrap(base), 0, 0, time.Time{}),
			})
			if got := session.IsUTF8(stack); got != tc.utf8 {
				t.Errorf("IsUTF8 through the stack = %v, want %v", got, tc.utf8)
			}
			if got := session.IsASCII(stack); got != tc.ascii {
				t.Errorf("IsASCII through the stack = %v, want %v", got, tc.ascii)
			}
		})
	}
}
