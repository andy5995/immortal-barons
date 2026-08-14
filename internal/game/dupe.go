package game

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
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
	h := strings.ToLower(strings.TrimSpace(handle))
	if h == "" {
		return "" // an AI baron belongs to nobody and can't be a duplicate
	}
	sum := sha256.Sum256([]byte(h))
	return hex.EncodeToString(sum[:])[:DupeHashLen]
}

// DupeLocked reports whether e is shut out for playing elsewhere in the league.
// The switch is read here rather than when the lock is set, so a Coordinator who
// turns Dupe Checking off releases every locked baron at once.
func (w *World) DupeLocked(e *Empire) bool {
	return w.Config.IBBS && w.Config.DupeChecking && e.DupeLockedBy != ""
}

// DupeLockMessage explains the lock to the caller it shut out.
func (w *World) DupeLockMessage(e *Empire) string {
	return fmt.Sprintf("You are also playing on %s. A baron may hold one realm in the league, "+
		"so this one is locked until you give up the other.", e.DupeLockedBy)
}

// applyDupeCheck reconciles this board's realms against the owners another board
// has just reported. A local baron whose hash appears in the packet is locked to
// that board; one previously locked to it and no longer listed there is
// released, which is how "until they delete one of their players" ends.
//
// It runs whatever this board's own switch says, so that turning Dupe Checking
// back on does not need a packet to arrive before it means anything — DupeLocked
// is what consults the switch. A packet from a board with the switch off carries
// no hashes and therefore releases that board's locks, which is the right
// reading: it has stopped claiming anyone.
func (w *World) applyDupeCheck(fromBoard string, scores []RemoteScore) {
	if fromBoard == "" || fromBoard == w.Config.BoardID {
		return
	}
	remote := make(map[string]bool, len(scores))
	for _, s := range scores {
		if s.OwnerHash != "" {
			remote[s.OwnerHash] = true
		}
	}
	for _, e := range w.Empires {
		if e.Owner == "" {
			continue
		}
		switch {
		case remote[dupeHash(e.Owner)]:
			if e.DupeLockedBy != fromBoard {
				e.DupeLockedBy = fromBoard
				e.addEvent(fmt.Sprintf("Your handle is also playing on %s. This realm is locked until one of them goes.", fromBoard))
			}
		case e.DupeLockedBy == fromBoard:
			e.DupeLockedBy = ""
			e.addEvent(fmt.Sprintf("%s no longer lists your handle; this realm is unlocked.", fromBoard))
		}
	}
}
