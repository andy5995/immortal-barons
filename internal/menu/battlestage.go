package menu

import (
	"fmt"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// battlestage.go — a regular attack on a rival realm is drawn over nine
// seconds instead of landing all at once: the realm being hit, then the advance
// and the push, three seconds apart, then the whole report. An IB addition, not
// the original's behavior (docs/mechanics-reference.md).
//
// The stages carry no figures. Running casualty counts went by too fast to read
// at three seconds a stage; the report that follows is the original's own.
//
// It runs OUTSIDE the world lock, after the mutation has saved. A pause inside
// it would hold the exclusive flock for nine seconds per attack and queue every
// other node behind one player's dramatic pause.

// battleStagePause is how long each of the three stages holds the screen.
const battleStagePause = 3 * time.Second

// battlePause is the wait itself, skipped under a test binary so no suite sits
// through it.
func battlePause() {
	if game.UnderTest() {
		return
	}
	time.Sleep(battleStagePause)
}

// stageBattle draws the approach, the advance and the push. Pirates get none
// of this; only a strike against a realm is worth the wait.
func stageBattle(s session.Session, defender string) {
	for _, line := range []string{
		fmt.Sprintf(tr(s, "Attacking %s..."), defender),
		tr(s, "Advancing..."),
		tr(s, "Pushing..."),
	} {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, line, ansi.Reset)
		battlePause()
	}
}
