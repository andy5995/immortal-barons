package store

import (
	"os"
	"path/filepath"

	"github.com/andy5995/immortal-barons/internal/bulletin"
	"github.com/andy5995/immortal-barons/internal/game"
)

// Bulletin syncing: the filesystem half of internal/game/bulletin.go.
//
// Two directions meet here. A board's OWN bulletins (bull/local) are read off
// disk and recorded, which is what puts a newly added one in the news. The
// LEAGUE's (bull/league) go the other way on a member board — the Coordinator's
// packet is authoritative and this writes it out — while on the Coordinator's
// own board they are read off disk like the local ones and handed back for
// broadcast.

// SyncBulletins reconciles the bulletin directories with the world. It returns
// the league set to broadcast, which is empty anywhere but the Coordinator's
// board: every other board receives that set rather than authoring it.
//
// Call it inside whatever transaction owns the world — a login's maintenance
// pass, or a planetary run — since it files news and updates digests.
func SyncBulletins(w *game.World) ([]game.BulletinFile, error) {
	dataDir := w.Config.DataDir
	// Anything a Coordinator packet just delivered reaches the disk first, so the
	// scan below sees the same files the world already recorded.
	if set := w.PendingBulletins; set != nil {
		if err := writeLeagueBulletins(dataDir, set.Files); err != nil {
			return nil, err
		}
		w.PendingBulletins = nil
	}
	local, err := readBulletins(dataDir, bulletin.Local)
	if err != nil {
		return nil, err
	}
	w.RecordBulletins(bulletin.Local, local)
	// bull/league is read off disk everywhere EXCEPT a member board, where the
	// Coordinator's packet is authoritative and a local edit would be reverted
	// on the next run anyway. A stand-alone board has no Coordinator, so what a
	// sysop puts there is theirs and is recorded like the local ones.
	if receivesLeagueBulletins(w) {
		return nil, nil
	}
	league, err := readBulletins(dataDir, bulletin.League)
	if err != nil {
		return nil, err
	}
	w.RecordBulletins(bulletin.League, league)
	if !w.IsLeagueCoordinator() {
		return nil, nil // nothing to broadcast: this board is not in a league
	}
	return league, nil
}

// receivesLeagueBulletins reports whether this board's bull/league is filled by
// the League Coordinator rather than by its own sysop.
func receivesLeagueBulletins(w *game.World) bool {
	return !w.IsLeagueCoordinator() && w.CoordinatorBoardID() != ""
}

// readBulletins loads one scope's bulletins with their contents. A file past
// bulletin.MaxSize is skipped rather than carried: on the Coordinator's board
// this set is copied into a packet bound for every other board.
func readBulletins(dataDir string, scope bulletin.Scope) ([]game.BulletinFile, error) {
	listed, err := bulletin.List(dataDir, scope)
	if err != nil {
		return nil, err
	}
	var out []game.BulletinFile
	for _, b := range listed {
		data, err := os.ReadFile(b.Path)
		if err != nil {
			if os.IsNotExist(err) {
				continue // removed between the listing and the read
			}
			return nil, err
		}
		if len(data) > bulletin.MaxSize {
			continue
		}
		out = append(out, game.BulletinFile{Name: b.Name, Title: b.Title, Data: data})
	}
	return out, nil
}

// writeLeagueBulletins makes bull/league match the Coordinator's set exactly:
// files whose contents differ are rewritten, and ones the league no longer
// carries are removed. Rewriting only what differs keeps a sysop's directory
// timestamps meaningful, and means a run where nothing changed touches nothing.
func writeLeagueBulletins(dataDir string, files []game.BulletinFile) error {
	keep := make(map[string]bool, len(files))
	for _, f := range files {
		if !bulletin.SafeName(f.Name) {
			continue
		}
		keep[f.Name] = true
		current, err := os.ReadFile(filepath.Join(bulletin.Dir(dataDir, bulletin.League), f.Name))
		if err == nil && bulletin.Digest(current) == bulletin.Digest(f.Data) {
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := bulletin.Write(dataDir, bulletin.League, f.Name, f.Data); err != nil {
			return err
		}
	}
	existing, err := bulletin.List(dataDir, bulletin.League)
	if err != nil {
		return err
	}
	for _, b := range existing {
		if keep[b.Name] {
			continue
		}
		if err := bulletin.Remove(dataDir, bulletin.League, b.Name); err != nil {
			return err
		}
	}
	return nil
}
