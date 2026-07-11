package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestNuclearStrikeShapeShiftConflict is the specialAttack analogue of the
// regular-attack shape-shift test, covering the covert-ops / WMD path (every
// covert op, missile, and bomb runs through specialAttack). Node B picks
// Victimville to nuke; while B is at the target prompt another node eliminates
// it, and B's reload rebinds Victimville's slot to Decoyland. Re-finding the
// target by name sees it is gone and aborts before charging the missile — so no
// gold is spent and Decoyland is untouched. A pointer-identity re-check would
// have nuked the wrong realm.
func TestNuclearStrikeShapeShiftConflict(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Protection = 0
		p.Gold = 100_000_000 // well over NukeCost
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
	nuclearAttack(fb, b)

	if d := committedEmpire(t, cfg, "decoy"); len(d.Events) != 0 {
		t.Fatalf("Decoyland was struck (%d events) — a stale pointer nuked the wrong realm", len(d.Events))
	}
	if a := committedEmpire(t, cfg, "alice"); a.Gold != 100_000_000 {
		t.Fatalf("alice was charged %d gold for an aborted strike", 100_000_000-a.Gold)
	}
	if out := fb.out.String(); !strings.Contains(out, "no longer there") {
		t.Fatalf("node B should have aborted with the target-gone notice, got: %q", out)
	}
}

// TestFundSDIVanishedFunderConflict proves Fund SDI re-resolves the funder
// inside its transaction. Node B opens Fund SDI; while B enters the amount
// another node abdicates B's own realm, leaving only a decoy whose slot B's
// reload rebinds the funder pointer onto. Spending through the stale pointer
// would raise the decoy's SDI with the decoy's gold; re-resolving by handle
// finds no realm and aborts, leaving the decoy untouched.
func TestFundSDIVanishedFunderConflict(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Gold = 100_000_000
	})
	commitOnFile(t, cfg, func(w *game.World) {
		d := w.AddHuman("decoy", "Decoyland")
		d.Gold = 100_000_000
		d.SDI = 0
	})

	fb := &hookSession{
		fakeSession: fakeSession{keys: []rune("6000000\r")}, // 4 SDIStep chunks
		marker:      "Fund SDI",
		hook: func() {
			commitOnFile(t, cfg, func(w *game.World) { w.RemoveEmpire(w.FindByOwner("alice")) })
		},
	}
	sdiProgram(fb, b)

	if d := committedEmpire(t, cfg, "decoy"); d.SDI != 0 || d.Gold != 100_000_000 {
		t.Fatalf("Decoyland's SDI/gold changed (SDI=%d gold=%d) — a stale pointer funded the wrong realm", d.SDI, d.Gold)
	}
	if out := fb.out.String(); !strings.Contains(out, "realm has changed") {
		t.Fatalf("node B should have aborted with the realm-changed notice, got: %q", out)
	}
}
