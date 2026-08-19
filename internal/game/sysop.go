package game

import (
	"errors"
	"strings"
)

// Sysop edits to a player's record (#161). BRE's VIEW command lists the players
// and lets the sysop change a caller's user name, change their realm's name, or
// delete the realm; these are the two of the three that need more than what the
// player-facing code already does. The realm rename is SysopRenameEmpire in
// rename.go, and deleting is RemoveEmpire, which every removal path shares.

var (
	// ErrOwnerInvalid is returned for an empty caller handle. The empty handle
	// is the AI marker (see FindByOwner), so a human realm can never carry it.
	ErrOwnerInvalid = errors.New("A caller name cannot be blank.")
	// ErrOwnerTaken is returned when another realm is already keyed to the
	// handle. One handle to one realm is what makes the login lookup unambiguous.
	ErrOwnerTaken = errors.New("Another realm already belongs to that caller.")
	// ErrOwnerIsAI is returned for a computer baron: it belongs to nobody, and
	// giving it a handle would hand it to that caller at their next login.
	ErrOwnerIsAI = errors.New("A computer baron has no caller to rename.")
)

// RenameOwner re-keys e to a different BBS handle and repoints everything the
// world still holds under the old one.
//
// The handle is the login key: a caller who renames their account on the board
// otherwise arrives as a stranger and is offered a fresh realm while their own
// sits waiting for the abandonment sweep. The sysop is the only person who can
// see both names, so this is theirs to fix.
//
// The handle is stored beside several things that outlive a turn — a group
// attack's contributors, a strike or bid away on another planet, a doomsday
// weapon's builder, everyone's Coordinator vote — and rewriteOwner walks all of
// them in the caller's transaction, the way rewriteRealmName walks the realm
// name's own set.
//
// Nothing needs holding against a packet already in the air, which is where
// this differs from a realm rename: every interplanetary answer is matched to
// the LOCAL record it belongs to (by strike or bid id) and the handle is read
// off that record, never off the packet.
func (w *World) RenameOwner(e *Empire, newHandle string) error {
	h := strings.ToLower(strings.TrimSpace(newHandle))
	switch {
	case e.Owner == "":
		return ErrOwnerIsAI
	case h == "":
		return ErrOwnerInvalid
	}
	if other := w.FindByOwner(h); other != nil && other != e {
		return ErrOwnerTaken
	}
	old := e.Owner
	if old == h {
		return nil
	}
	e.Owner = h
	w.rewriteOwner(old, h)
	return nil
}

// rewriteOwner repoints every stored reference to caller handle `old` at `h`.
// Prose (events, news, mail) is left as written, as rewriteRealmName leaves it:
// it records what was said at the time and resolves nothing.
func (w *World) rewriteOwner(old, h string) {
	swap := func(s *string) {
		if *s == old {
			*s = h
		}
	}
	for i := range w.GroupAttacks {
		for j := range w.GroupAttacks[i].Contributors {
			swap(&w.GroupAttacks[i].Contributors[j].Owner)
		}
	}
	for i := range w.InFlight {
		swap(&w.InFlight[i].Owner)
		for j := range w.InFlight[i].Contributors {
			swap(&w.InFlight[i].Contributors[j].Owner)
		}
	}
	// This planet's own weapon only. Incoming is another planet's, and its
	// Creator is a handle on THAT board — one that may well be spelled the same
	// as somebody here.
	if w.Annihilator != nil {
		swap(&w.Annihilator.Creator)
	}
	for _, e := range w.Empires {
		swap(&e.CoordinatorVote)
	}
}
