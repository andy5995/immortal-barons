package game

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/bulletin"
)

// Bulletins (IB's own arrangement; see internal/bulletin for the layout).
//
// A board's own bulletins are its sysop's business and never leave it. The
// league's are the Coordinator's, and travel the way the ruleset and the roster
// do: broadcast on every planetary run, signed, and adopted only from node #1.
// Each broadcast carries the COMPLETE set, so a board that joins late or misses
// a packet is brought level by the next one rather than needing a replay.

// BulletinFile is one bulletin in a league broadcast: enough to write the file
// out at the far end and to name it in the news without opening it.
type BulletinFile struct {
	Name  string
	Title string
	Data  []byte
}

// BulletinSet is the Coordinator's complete league bulletin set. A pointer to
// it rides in a packet, so "no bulletin information" (nil) stays distinct from
// "the league has none" (present and empty) — the second is how a Coordinator
// withdraws the last one.
type BulletinSet struct {
	Files []BulletinFile
}

// bulletinKey identifies a bulletin in the digest map.
func bulletinKey(scope bulletin.Scope, name string) string { return string(scope) + "/" + name }

// RecordBulletins records the bulletins now present for one scope, files a news
// item for each one added or edited, and reports whether the set moved at all.
//
// The first recording of a scope is silent: a board that already had bulletins
// when this arrived would otherwise fill a day's news with files nobody
// touched, and the news cap would push the real events off the page.
func (w *World) RecordBulletins(scope bulletin.Scope, files []BulletinFile) bool {
	if w.BulletinDigest == nil {
		w.BulletinDigest = map[string]string{}
	}
	known := w.BulletinsKnown[string(scope)]
	present := make(map[string]bool, len(files))
	changed := false
	for _, f := range files {
		key := bulletinKey(scope, f.Name)
		present[key] = true
		digest := bulletin.Digest(f.Data)
		previous, had := w.BulletinDigest[key]
		if had && previous == digest {
			continue
		}
		w.BulletinDigest[key] = digest
		changed = true
		if known {
			w.postBulletinNews(scope, f.Title, !had)
		}
	}
	for key := range w.BulletinDigest {
		if strings.HasPrefix(key, string(scope)+"/") && !present[key] {
			delete(w.BulletinDigest, key)
			changed = true
		}
	}
	if !known {
		if w.BulletinsKnown == nil {
			w.BulletinsKnown = map[string]bool{}
		}
		w.BulletinsKnown[string(scope)] = true
	}
	return changed
}

// postBulletinNews names the bulletin that changed, which is the whole point of
// filing it: "a bulletin changed" sends every player to the menu to find out
// which one.
//
// Four whole sentences rather than one with the word "galactic" glued in: a
// news line is one translatable unit, and a fragment spliced mid-clause cannot
// be reordered by a translator (see docs/mechanics-reference.md, "News files").
func (w *World) postBulletinNews(scope bulletin.Scope, title string, isNew bool) {
	switch {
	case scope == bulletin.League && isNew:
		w.postNews(fmt.Sprintf("A new galactic bulletin has been posted: %q.", title))
	case scope == bulletin.League:
		w.postNews(fmt.Sprintf("The galactic bulletin %q has been updated.", title))
	case isNew:
		w.postNews(fmt.Sprintf("A new bulletin has been posted: %q.", title))
	default:
		w.postNews(fmt.Sprintf("The bulletin %q has been updated.", title))
	}
}

// ExportBulletins queues a broadcast of the league's bulletins. Only the
// Coordinator authors one, and only a board whose roster names it node #1
// adopts it (see ApplyPacket).
//
// The set goes out on every run, empty or not, as the roster does: a board that
// joins late is brought level by the next packet rather than needing a replay,
// and an empty set is how the last bulletin is withdrawn from the league.
func (w *World) ExportBulletins(files []BulletinFile) {
	if !w.IsLeagueCoordinator() {
		return
	}
	w.Outbox = append(w.Outbox, Packet{
		FromBoard: w.Config.BoardID,
		Date:      w.LastMaintDate,
		Bulletins: &BulletinSet{Files: append([]BulletinFile(nil), files...)},
	})
}

// applyBulletins takes a Coordinator's league set: it drops anything that fails
// the name or size check — the name builds a path and arrives from another
// board — files the news for what actually changed, and leaves the surviving
// set for internal/store to write to disk.
func (w *World) applyBulletins(set BulletinSet) {
	files := make([]BulletinFile, 0, len(set.Files))
	for _, f := range set.Files {
		if !bulletin.SafeName(f.Name) || len(f.Data) > bulletin.MaxSize {
			continue
		}
		files = append(files, f)
	}
	w.RecordBulletins(bulletin.League, files)
	// Handed on whether or not the digests moved, so a file deleted off disk by
	// hand comes back on the next run instead of staying missing because this
	// board still remembers its fingerprint.
	w.PendingBulletins = &BulletinSet{Files: files}
}
