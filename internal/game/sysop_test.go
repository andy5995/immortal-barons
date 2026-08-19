package game

import (
	"errors"
	"testing"
)

// A sysop rename repoints the same references a player's does, and leaves the
// player's one rename unspent — the whole point of keeping the old name in
// PriorNames rather than FormerName.
func TestSysopRenameKeepsThePlayersOwnRename(t *testing.T) {
	w, old, _ := renameWorld(t)
	old.Protection = 5 // no bar to the sysop: the protection rule is the player's
	if err := w.SysopRenameEmpire(old, "Imposed"); err != nil {
		t.Fatalf("SysopRenameEmpire: %v", err)
	}
	if old.Name != "Imposed" {
		t.Errorf("name = %q, want Imposed", old.Name)
	}
	if old.FormerName != "" {
		t.Errorf("FormerName = %q, want empty: a sysop rename must not spend the player's", old.FormerName)
	}
	if w.Treaties[0].A != "Imposed" && w.Treaties[0].B != "Imposed" {
		t.Errorf("treaty = %q/%q, want Imposed in it", w.Treaties[0].A, w.Treaties[0].B)
	}
	// The old name still takes delivery, and nobody else may claim it.
	if got := w.FindByNameOrFormer("Old"); got != old {
		t.Error("a packet addressed to the old name should still find the realm")
	}
	if !w.RealmNameTaken("Old") {
		t.Error("the old name must stay held against re-use")
	}
	// The player's rename is still theirs to make.
	old.Protection = 0
	if err := w.RenameEmpire(old, "Chosen"); err != nil {
		t.Fatalf("the player's own rename should still be available: %v", err)
	}
	if old.FormerName != "Imposed" || len(old.PriorNames) != 1 {
		t.Errorf("FormerName = %q, PriorNames = %v", old.FormerName, old.PriorNames)
	}
	if got := w.FindByNameOrFormer("Old"); got != old {
		t.Error("the sysop-era name must still take delivery after the player renames too")
	}
}

func TestSysopRenameRefusesABadName(t *testing.T) {
	w, old, _ := renameWorld(t)
	if err := w.SysopRenameEmpire(old, " "); !errors.Is(err, ErrRealmNameInvalid) {
		t.Errorf("blank name: err = %v, want ErrRealmNameInvalid", err)
	}
	if err := w.SysopRenameEmpire(old, "Rival"); !errors.Is(err, ErrRealmNameTaken) {
		t.Errorf("taken name: err = %v, want ErrRealmNameTaken", err)
	}
	if old.Name != "Old" {
		t.Errorf("name = %q, want Old: a refused rename changes nothing", old.Name)
	}
}

// The handle is the login key, and it is stored beside things that outlive a
// turn. A re-key that missed one of them would strand a force abroad, or leave
// a vote counted for nobody.
func TestRenameOwnerRewritesEveryHandleReference(t *testing.T) {
	w, old, other := renameWorld(t)
	w.GroupAttacks = []GroupAttack{{ID: 1, Contributors: []Contribution{{Owner: "someone"}, {Owner: "rival"}}}}
	w.InFlight = []InFlightStrike{
		{ID: 2, Kind: "terror", Owner: "someone"},
		{ID: 3, Contributors: []Contribution{{Owner: "someone"}}},
	}
	w.Annihilator = &Annihilator{Creator: "someone"}
	w.Incoming = &Annihilator{Creator: "someone"} // another planet's builder, same spelling
	old.CoordinatorVote = "someone"
	other.CoordinatorVote = "someone"

	if err := w.RenameOwner(old, "  Andy5995  "); err != nil {
		t.Fatalf("RenameOwner: %v", err)
	}
	if old.Owner != "andy5995" {
		t.Errorf("owner = %q, want andy5995 (trimmed and lowercased)", old.Owner)
	}
	if w.FindByOwner("Andy5995") != old {
		t.Error("the realm should be found under the new handle")
	}
	if w.FindByOwner("someone") != nil {
		t.Error("the old handle should find nothing")
	}
	if w.GroupAttacks[0].Contributors[0].Owner != "andy5995" || w.GroupAttacks[0].Contributors[1].Owner != "rival" {
		t.Errorf("group attack contributors = %+v", w.GroupAttacks[0].Contributors)
	}
	if w.InFlight[0].Owner != "andy5995" {
		t.Errorf("terror op owner = %q, want andy5995", w.InFlight[0].Owner)
	}
	if w.InFlight[1].Contributors[0].Owner != "andy5995" {
		t.Errorf("in-flight contributor = %q, want andy5995", w.InFlight[1].Contributors[0].Owner)
	}
	if w.Annihilator.Creator != "andy5995" {
		t.Errorf("annihilator creator = %q, want andy5995", w.Annihilator.Creator)
	}
	if w.Incoming.Creator != "someone" {
		t.Errorf("incoming creator = %q, want someone: it is a handle on ANOTHER board", w.Incoming.Creator)
	}
	if old.CoordinatorVote != "andy5995" || other.CoordinatorVote != "andy5995" {
		t.Errorf("votes = %q/%q, want both andy5995", old.CoordinatorVote, other.CoordinatorVote)
	}
}

// Returning forces are handed back by handle, so a re-key that stopped at
// Empire.Owner would lose them. This is the failure the rewrite exists for.
func TestRenamedOwnerStillGetsLostForcesBack(t *testing.T) {
	w, old, _ := renameWorld(t)
	w.Config.LostForcesDays = 1
	w.GameDay = 10
	w.InFlight = []InFlightStrike{{ID: 1, TargetBoard: "Elsewhere", TargetEmpire: "Someone", LaunchedDay: 1,
		Contributors: []Contribution{{Owner: "someone", AttackForce: AttackForce{Troopers: 500}}}}}
	if err := w.RenameOwner(old, "newhandle"); err != nil {
		t.Fatal(err)
	}
	before := old.Troopers
	if n := w.ReturnLostForces(); n != 1 {
		t.Fatalf("ReturnLostForces = %d, want 1", n)
	}
	if old.Troopers != before+500 {
		t.Errorf("troopers = %d, want %d: the force should come home to the re-keyed baron", old.Troopers, before+500)
	}
}

func TestRenameOwnerRefusals(t *testing.T) {
	w, old, other := renameWorld(t)
	if err := w.RenameOwner(old, "  "); !errors.Is(err, ErrOwnerInvalid) {
		t.Errorf("blank handle: err = %v, want ErrOwnerInvalid", err)
	}
	if err := w.RenameOwner(old, "RIVAL"); !errors.Is(err, ErrOwnerTaken) {
		t.Errorf("taken handle: err = %v, want ErrOwnerTaken", err)
	}
	if old.Owner != "someone" {
		t.Errorf("owner = %q, want someone: a refused rename changes nothing", old.Owner)
	}
	// Renaming a realm to the handle it already has is a no-op, not a collision.
	if err := w.RenameOwner(old, "SOMEONE"); err != nil {
		t.Errorf("same handle: err = %v, want nil", err)
	}
	ai := w.addAIEmpire("Computer")
	if err := w.RenameOwner(ai, "someone-else"); !errors.Is(err, ErrOwnerIsAI) {
		t.Errorf("AI baron: err = %v, want ErrOwnerIsAI", err)
	}
	_ = other
}
