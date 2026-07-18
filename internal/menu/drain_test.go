package menu

import (
	"testing"

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
