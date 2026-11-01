package game

import (
	"crypto/sha256"
	"encoding/hex"
)

// Duplicate-user checking, BRE's "Dupe Checking": a league setting that finds
// callers playing on more than one board in the league and shuts them out here
// until they give one of the realms up (game/reset.hlp, "InterBBS: Duplicate
// User Checking").
//
// BRE compares handles. IB compares a hash of the handle instead, so a scores
// packet — a file that travels between strangers' boards and is kept on each —
// never carries anyone's BBS handle. The match is the same one either way,
// because every board hashes the same normalized form.

// DupeHashLen is how much of the digest a packet carries. Sixteen hex
// characters is 64 bits: far past a collision at league scale (a few hundred
// barons), and short enough that a packet stays readable.
const DupeHashLen = 16

// dupeHash identifies a caller across the league. It hashes the same normalized
// handle FindByOwner matches on, so the two agree about who a baron is.
func dupeHash(handle string) string {
	h := OwnerKey(handle)
	if h == "" {
		return "" // an AI baron belongs to nobody and can't be a duplicate
	}
	sum := sha256.Sum256([]byte(h))
	return hex.EncodeToString(sum[:])[:DupeHashLen]
}

// dupeCheckingOn reports whether this run enforces duplicate-user checking: the
// sysop's saved setting, unless the -dupe-check testing switch overrides it for
// the process. Every site that ASKS goes through here, so a reader added later
// cannot quietly ignore the switch.
//
// What the league is TOLD the rule is stays Config.DupeChecking — see
// ExportLeagueConfig, which broadcasts the saved setting. A testing switch on
// one board must not rewrite the rule for the league.
func (w *World) dupeCheckingOn() bool {
	if w.Config.DupeCheckOverride != nil {
		return *w.Config.DupeCheckOverride
	}
	return w.Config.DupeChecking
}

// DupeLocked reports whether e is shut out for playing elsewhere in the league.
// The switch is read here rather than when the lock is set, so a Coordinator who
// turns Dupe Checking off releases every locked baron at once.
func (w *World) DupeLocked(e *Empire) bool {
	return e.DupeLockedBy != "" && len(w.dupeLockWaivers()) == 0
}

// dupeLockWaivers names each setting that keeps a recorded lock from being
// enforced, nil when none does. DupeLocked and the player list both read it, so
// the report cannot give a reason the gate does not apply.
func (w *World) dupeLockWaivers() []string {
	var why []string
	if !w.Config.IBBS {
		why = append(why, "inter-BBS play is off")
	}
	if !w.dupeCheckingOn() {
		why = append(why, "duplicate checking is off")
	}
	return why
}

// ReleaseDepartedDupeLocks releases every lock held by a board the league
// roster no longer lists. Such a board sends no more scores to lift what it
// set, so taking a dead board off the roster is how a Coordinator frees the
// barons it held. The lock is cleared rather than ignored, so a board that later
// rejoins, or a new one given its name, does not shut a baron out again with
// nothing said; its next scores lock them afresh, with an event.
func (w *World) ReleaseDepartedDupeLocks() {
	for _, e := range w.Empires {
		if e.DupeLockedBy == "" || w.Routable(e.DupeLockedBy) {
			continue
		}
		board := e.DupeLockedBy
		e.DupeLockedBy = ""
		if w.Config.IBBS && w.dupeCheckingOn() {
			e.addEvent(say("{board} has left the league; this realm is unlocked.", "board", board))
		}
	}
}

// newerHere reports whether a realm founded at here is newer than one founded at
// there, both Recorded stamps. A realm with no stamp was founded before stamps
// existed, so it is the older of a stamped pair. Two unknown stamps, or two
// equal ones, count as newer, so both boards lock rather than neither.
func newerHere(here, there string) bool {
	h, okH := ParseStamp(here)
	t, okT := ParseStamp(there)
	if !okH || !okT {
		return okH || !okT
	}
	return !h.Before(t)
}

// DupeLockMessage explains the lock to the caller it shut out.
func (w *World) DupeLockMessage(e *Empire) Msg {
	return say("You are also playing on {board}. A baron may hold one realm in the league, "+
		"so this one is locked until you give one of them up.", "board", e.DupeLockedBy)
}

// DupeElsewhere names a board whose last scores list handle as one of its
// barons, or "" when none does or Dupe Checking is off. BRE refuses such a
// caller a new realm rather than letting them found one and locking both. It
// can only see the scores that have arrived, so a realm founded there since is
// caught by the lock instead, once that board's scores do arrive.
func (w *World) DupeElsewhere(handle string) string {
	h := dupeHash(handle)
	if h == "" || !w.Config.IBBS || !w.dupeCheckingOn() {
		return ""
	}
	for _, b := range w.RemoteBoards {
		// RemoteBoards keeps a board's last scores after it leaves the league,
		// and one that has left will never send the scores that lift this.
		if b.BoardID == w.Config.BoardID || !w.Routable(b.BoardID) {
			continue
		}
		for _, s := range b.Scores {
			if s.OwnerHash == h {
				return b.BoardID
			}
		}
	}
	return ""
}

// DupeJoinMessage is what a caller DupeElsewhere found is told in place of
// founding a realm.
func DupeJoinMessage(board string) Msg {
	return say("You are already playing on {board}. A baron may hold one realm in the league, "+
		"so you cannot found one here. If you have given that realm up, this lifts once {board}'s "+
		"next scores reach this board. If that realm is not yours, tell your sysop.", "board", board)
}

// applyDupeCheck reconciles this board's realms against the owners another board
// has just reported. A local baron whose hash appears in the packet is locked to
// that board when the realm here is the newer of the two, so the baron keeps
// playing where they began (#323); one previously locked to it and no longer
// listed there is released, which is how "until they delete one of their
// players" ends. When either realm's founding is unknown both are locked, as
// before the stamp existed, and the erase offer at login is the way out.
//
// A lock is set only while this board's checking is on, so every lock on record
// was announced to its baron when it was set; one that would have started while
// checking was off waits for the next packet after it is turned on. A lock set
// earlier survives the switch going off, and DupeLocked, which consults the
// switch, enforces it again the moment it comes back on. Releases happen either
// way, since a release can only free a baron. A packet from a board with the
// switch off carries no hashes and therefore releases that board's locks, which
// is the right reading: it has stopped claiming anyone.
func (w *World) applyDupeCheck(fromBoard string, scores []RemoteScore) {
	if fromBoard == "" || fromBoard == w.Config.BoardID {
		return
	}
	on := w.Config.IBBS && w.dupeCheckingOn()
	remote := make(map[string]string, len(scores)) // owner hash -> Founded
	for _, s := range scores {
		if s.OwnerHash != "" {
			remote[s.OwnerHash] = s.Founded
		}
	}
	for _, e := range w.Empires {
		if e.Owner == "" {
			continue
		}
		founded, listed := remote[dupeHash(e.Owner)]
		switch {
		case listed && !newerHere(e.Founded, founded):
			// The realm there is the newer one, and that board locks it; this one
			// stays playable. A lock it took before that was known is released.
			if e.DupeLockedBy == fromBoard {
				e.DupeLockedBy = ""
				if on {
					e.addEvent(say("The realm on {board} is newer, so this one is unlocked.", "board", fromBoard))
				}
			}
		case listed:
			// A board the roster does not list rules nobody out, as in
			// ReleaseDepartedDupeLocks, so no lock is recorded and none is announced.
			if on && w.Routable(fromBoard) && e.DupeLockedBy != fromBoard {
				e.DupeLockedBy = fromBoard
				e.addEvent(say("Your handle is also playing on {board}. This realm is locked until one of them goes.", "board", fromBoard))
			}
		case e.DupeLockedBy == fromBoard:
			e.DupeLockedBy = ""
			if on {
				e.addEvent(say("{board} no longer lists your handle; this realm is unlocked.", "board", fromBoard))
			}
		}
	}
}
