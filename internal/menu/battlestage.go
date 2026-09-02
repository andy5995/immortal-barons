package menu

import (
	"fmt"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// battlestage.go — a regular attack on a rival realm is drawn over nine
// seconds instead of landing all at once: the realm being hit, a third of the
// casualties, a push, two thirds, then the whole report. An IB addition, not
// the original's behaviour (docs/mechanics-reference.md).
//
// The figures are REAL — the battle is resolved before any of this runs, and
// each stage shows a share of the losses it produced. Nothing here changes an
// outcome or invents a figure.
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

// stageBattle draws the approach and two casualty snapshots. Pirates get none
// of this; only a strike against a realm is worth the wait.
func stageBattle(s session.Session, defender string, o game.BattleOutcome) {
	fmt.Fprintf(s, "\n%s"+tr(s, "Attacking %s...")+"%s\n", ansi.FgBrightCyan, defender, ansi.Reset)
	battlePause()
	stageLosses(s, o, 1)
	fmt.Fprintf(s, "\n%s"+tr(s, "Pushing...")+"%s\n", ansi.FgBrightCyan, ansi.Reset)
	battlePause()
	stageLosses(s, o, 2)
	battlePause()
}

// stageLosses prints the casualties reached by stage n of battleStages, each
// side on its own line.
func stageLosses(s session.Session, o game.BattleOutcome, n int) {
	a, d := partialLoss(o.AttackerLoss, n), partialLoss(o.DefenderLoss, n)
	// Wrap before colouring, as wrapReport says: six-digit losses on four unit
	// types already pass column 80, and a translation runs longer still.
	fmt.Fprintf(s, "\n%s\n", hiNums(WrapIndented(fmt.Sprintf(
		tr(s, "Your losses so far: %d troopers, %d jets, %d tanks, %d bombers"),
		a.Troopers, a.Jets, a.Tanks, a.Bombers), "  ")))
	fmt.Fprintf(s, "%s\n", hiNums(WrapIndented(fmt.Sprintf(
		tr(s, "Their losses so far: %d troopers, %d turrets, %d tanks, %d jets"),
		d.Troopers, d.Turrets, d.Tanks, d.Jets), "  ")))
}

// battleStages is how many casualty snapshots the final report is divided into.
const battleStages = 3

// partialLoss is the share of a casualty count reached by stage n. Integer
// division throughout, so a snapshot never exceeds the total it is a share of
// and the last stage is the report itself rather than anything computed here.
// The product goes through int64 because v*n wraps a 32-bit int — the width the
// door builds have — once a casualty count passes a billion.
func partialLoss(u game.UnitLoss, n int) game.UnitLoss {
	share := func(v int) int { return int(int64(v) * int64(n) / battleStages) }
	return game.UnitLoss{
		Troopers: share(u.Troopers),
		Jets:     share(u.Jets),
		Turrets:  share(u.Turrets),
		Tanks:    share(u.Tanks),
		Bombers:  share(u.Bombers),
	}
}
