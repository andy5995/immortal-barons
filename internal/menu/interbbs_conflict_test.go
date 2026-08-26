package menu

import (
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestJoinGroupAttackVanishedActorConflict proves Join Group Attack re-resolves
// the joining empire inside its transaction. A group attack led by Leaderland
// is forming. Node B selects it and, while entering its offense, another node
// abdicates B's own realm — leaving only Leaderland and a decoy whose slot B's
// reload rebinds the actor pointer onto. Contributing through the stale pointer
// would record the join under the WRONG owner; re-resolving by handle finds no
// realm and aborts, leaving the group attack's contributor list untouched.
func TestJoinGroupAttackVanishedActorConflict(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Troopers = 10_000 // give Alice troopers so the send prompt has a nonzero max
		p.Protection = 0    // past new-realm protection so the join isn't gated
	})
	commitOnFile(t, cfg, func(w *game.World) {
		leader := w.AddHuman("leader", "Leaderland")
		leader.Troopers = 10_000
		w.GameDay = 0
		w.CreateGroupAttack(leader, "Mars", "", game.GroupAttackHoursMax, game.AttackForce{Troopers: 1000}) // still forming
	})
	commitOnFile(t, cfg, func(w *game.World) { w.AddHuman("decoy", "Decoyland") })

	fb := &hookSession{
		fakeSession: fakeSession{keys: []rune("1\r500\r")}, // pick attack 1, send 500 troopers
		marker:      "Send how many Troopers",
		hook: func() {
			commitOnFile(t, cfg, func(w *game.World) { w.RemoveEmpire(w.FindByOwner("alice")) })
		},
	}
	joinGroupAttack(fb, b)

	w := committedWorld(t, cfg)
	if len(w.GroupAttacks) != 1 {
		t.Fatalf("group attack count = %d, want 1", len(w.GroupAttacks))
	}
	if n := len(w.GroupAttacks[0].Contributors); n != 1 {
		t.Fatalf("contributors = %d, want 1 — a stale pointer joined for the wrong realm", n)
	}
	if out := fb.out.String(); !strings.Contains(out, "realm has changed") {
		t.Fatalf("node B should have aborted with the realm-changed notice, got: %q", out)
	}
}

// TestJoinGroupAttackDepartedWindow proves the group-attack window is re-checked
// against fresh state: if the force leaves while B is entering its offense, the
// join aborts with the departed notice and adds no contribution.
func TestJoinGroupAttackDepartedWindow(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Troopers = 10_000
		p.Protection = 0 // past new-realm protection so the join isn't gated
	})
	commitOnFile(t, cfg, func(w *game.World) {
		leader := w.AddHuman("leader", "Leaderland")
		leader.Troopers = 10_000
		w.GameDay = 0
		w.CreateGroupAttack(leader, "Mars", "", game.GroupAttackHoursMax, game.AttackForce{Troopers: 1000})
	})

	fb := &hookSession{
		fakeSession: fakeSession{keys: []rune("1\r500\r")},
		marker:      "Send how many Troopers",
		hook: func() {
			commitOnFile(t, cfg, func(w *game.World) {
				w.GroupAttacks[0].DepartAt = time.Now().Add(-time.Hour) // the force leaves
			})
		},
	}
	joinGroupAttack(fb, b)

	w := committedWorld(t, cfg)
	// The attack has departed; LaunchDueGroupAttacks hasn't run in this test, so it
	// still sits in GroupAttacks, but no contribution should have been added.
	if n := len(w.GroupAttacks[0].Contributors); n != 1 {
		t.Fatalf("contributors = %d, want 1 — a join slipped in after departure", n)
	}
	if out := fb.out.String(); !strings.Contains(out, "already left") {
		t.Fatalf("node B should have aborted with the departed notice, got: %q", out)
	}
}

// A scores packet says which realms are still under New Realm Protection, so a
// board that reads it can flag them on its target lists. Every baron is listed —
// the flag is what marks the ones a strike would be refused (#214) — and the
// target board still refuses an arriving strike on its own authority.
func TestProtectedRealmsAreFlaggedAsTargets(t *testing.T) {
	scores := []game.RemoteScore{
		{Empire: "Fresh Meat", Protected: true},
		{Empire: "Fair Game"},
		{Empire: "Also New", Protected: true},
	}
	rows := remoteBarons(scores, hostile)
	if len(rows) != 3 {
		t.Fatalf("rows = %v, want all three barons listed", rows)
	}
	want := map[string]bool{"Fresh Meat": true, "Fair Game": false, "Also New": true}
	for _, r := range rows {
		if r.protected != want[r.name] {
			t.Errorf("%s protected = %v, want %v", r.name, r.protected, want[r.name])
		}
	}
	// Spying is not stopped by protection, so an observer is told nothing of it.
	for _, r := range remoteBarons(scores, observing) {
		if r.protected {
			t.Errorf("%s was flagged on an observing list", r.name)
		}
	}
}

// Export carries the flag, so the far side has something to read.
func TestExportedScoresCarryProtection(t *testing.T) {
	w := newWorld()
	w.Config.IBBS = true
	w.Config.BoardID = "Alpha BBS"
	var scores []game.RemoteScore
	w.With(func() {
		p := w.Player()
		p.Protection = 5
		w.World.Outbox = nil
		w.World.ExportScores()
		for _, pk := range w.World.Outbox {
			scores = append(scores, pk.Scores...)
		}
	})
	found := false
	for _, sc := range scores {
		if sc.Empire == w.Player().Name {
			found = true
			if !sc.Protected {
				t.Error("a realm under protection was exported as attackable")
			}
		}
	}
	if !found {
		t.Fatalf("the player was not in the exported scores: %+v", scores)
	}
}
