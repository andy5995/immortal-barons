package menu

import (
	"strings"
	"testing"

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
		p.Gold = 10_000 // give Alice gold so the funding prompt has a nonzero max
	})
	commitOnFile(t, cfg, func(w *game.World) {
		leader := w.AddHuman("leader", "Leaderland")
		leader.Gold = 10_000
		w.GameDay = 0
		w.CreateGroupAttack(leader, "Mars", "", 5, 1000) // departs day 5, still forming
	})
	commitOnFile(t, cfg, func(w *game.World) { w.AddHuman("decoy", "Decoyland") })

	fb := &hookSession{
		fakeSession: fakeSession{keys: []rune("1\r500\r")}, // pick attack 1, add 500 gold
		marker:      "Add how much gold for funding",
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
// against fresh state: if the attack departs (GameDay advances past DepartDay)
// while B is entering its offense, the join aborts with the departed notice and
// adds no contribution.
func TestJoinGroupAttackDepartedWindow(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Gold = 10_000
	})
	commitOnFile(t, cfg, func(w *game.World) {
		leader := w.AddHuman("leader", "Leaderland")
		leader.Gold = 10_000
		w.GameDay = 0
		w.CreateGroupAttack(leader, "Mars", "", 5, 1000)
	})

	fb := &hookSession{
		fakeSession: fakeSession{keys: []rune("1\r500\r")},
		marker:      "Add how much gold for funding",
		hook: func() {
			commitOnFile(t, cfg, func(w *game.World) { w.GameDay = 10 }) // the force leaves
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
