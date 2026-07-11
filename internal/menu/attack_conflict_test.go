package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// hookSession fires hook exactly once, on the first ReadKey taken after the
// session output contains marker. It stands in for another BBS node committing
// a transaction in the window between an action's target snapshot and its
// mutating transaction — the interleaving a plain scripted session can't
// express because both of the action's w.With reloads run inside one call.
type hookSession struct {
	fakeSession
	marker string
	hook   func()
	fired  bool
}

func (h *hookSession) ReadKey() (rune, error) {
	if !h.fired && h.marker != "" && strings.Contains(h.out.String(), h.marker) {
		h.fired = true
		h.hook()
	}
	return h.fakeSession.ReadKey()
}

// TestRegularAttackShapeShiftConflict is the discriminating attack conflict: it
// proves regularAttack re-finds its target by realm name, not by a pointer
// cached before the world reloaded. Node B snapshots its targets [Victimville,
// Decoyland], picks Victimville, and while B is at the target prompt another
// node eliminates Victimville. Because encoding/json reuses *Empire pointers by
// slice INDEX, B's reload after the pick rebinds Victimville's old slot to
// Decoyland — so a pointer-identity re-check would pass and B would attack the
// WRONG realm. Re-finding by name instead sees Victimville is gone and aborts,
// leaving Decoyland untouched.
func TestRegularAttackShapeShiftConflict(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Protection = 0
		p.Troopers = 100000 // enough offense to plunder on a win — makes a wrong attack loud
	})
	commitOnFile(t, cfg, func(w *game.World) {
		v := w.AddHuman("victim", "Victimville")
		v.Protection = 0
	})
	commitOnFile(t, cfg, func(w *game.World) {
		d := w.AddHuman("decoy", "Decoyland")
		d.Protection = 0
	})

	fb := &hookSession{
		fakeSession: fakeSession{keys: []rune("1\r")},
		marker:      "Attack which empire",
		hook: func() {
			commitOnFile(t, cfg, func(w *game.World) { w.RemoveEmpire(w.FindByOwner("victim")) })
		},
	}
	regularAttack(fb, b)

	if d := committedEmpire(t, cfg, "decoy"); len(d.Events) != 0 {
		t.Fatalf("Decoyland was attacked (%d events) — a stale pointer struck the wrong realm", len(d.Events))
	}
	if out := fb.out.String(); !strings.Contains(out, "no longer there") {
		t.Fatalf("node B should have aborted with the target-gone notice, got: %q", out)
	}
}

// TestPirateRaidVanishedRaiderConflict proves Attack Pirates re-resolves the
// raider inside its transaction. Node B commits its raid; while B is entering
// the committed force another node abdicates B's own realm, leaving only a
// decoy whose slot B's reload rebinds the raider pointer onto. Spending the
// stale pointer would hand the pirate loot to the decoy; re-resolving by handle
// finds no realm and aborts, leaving the decoy untouched.
func TestPirateRaidVanishedRaiderConflict(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Troopers = 5000
	})
	commitOnFile(t, cfg, func(w *game.World) {
		d := w.AddHuman("decoy", "Decoyland")
		d.Troopers = 0
		d.Gold = 0
	})

	fb := &hookSession{
		fakeSession: fakeSession{keys: []rune("1\r\r\r")}, // faction 1, then troopers/jets/tanks (defaults)
		marker:      "Commit how many Troopers",
		hook: func() {
			commitOnFile(t, cfg, func(w *game.World) { w.RemoveEmpire(w.FindByOwner("alice")) })
		},
	}
	attackPirates(fb, b)

	if d := committedEmpire(t, cfg, "decoy"); d.Troopers != 0 || d.Gold != 0 {
		t.Fatalf("Decoyland gained pirate loot (troopers=%d gold=%d) — a stale pointer raided for the wrong realm", d.Troopers, d.Gold)
	}
	if out := fb.out.String(); !strings.Contains(out, "realm has changed") {
		t.Fatalf("node B should have aborted with the realm-changed notice, got: %q", out)
	}
}
