package store

import (
	"encoding/json"
	"os"

	"github.com/andy5995/immortal-barons/internal/game"
)

// FileStore is the door/-local transaction backend: every world mutation is a
// whole-file transaction. Transact takes the exclusive flock, reloads the JSON
// world file into the caller's existing *game.World (so any pointer the caller
// holds stays valid), runs the mutation, writes the file back, and releases the
// lock. This lets several BBS nodes play one shared world concurrently — each
// action sees the latest committed state and commits atomically.
type FileStore struct {
	w   *game.World
	cfg game.Config
}

// NewFileStore builds a FileStore over an already-loaded world. The world is
// reloaded from disk at the start of every Transact, so the passed-in world is
// only the pointer that stays stable across transactions.
func NewFileStore(w *game.World, cfg game.Config) *FileStore {
	return &FileStore{w: w, cfg: cfg}
}

// Transact runs fn as one file-locked transaction: flock → reload → fn → save →
// release. The blocking flock serializes across processes (BBS nodes) and, since
// each door session drives one world from one goroutine, no in-process mutex is
// needed on top of it.
func (fs *FileStore) Transact(fn func()) error {
	lock, err := Lock(fs.cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()

	if err := fs.reload(); err != nil {
		return err
	}
	fn()
	return Save(fs.w, fs.cfg)
}

// Snapshot is Transact without the save: flock → reload → fn → release. The
// lock is still taken and the world is still reloaded, so what fn reads is
// current and cannot be torn by a node writing mid-read; only the write-back
// goes, because a gathering body has nothing to write. See game.Store.
func (fs *FileStore) Snapshot(fn func()) error {
	lock, err := Lock(fs.cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()

	if err := fs.reload(); err != nil {
		return err
	}
	fn()
	return nil
}

// reload reads world.json and unmarshals it into the existing *World (not a new
// one — the caller's pointer must survive), then re-runs the same migrations
// Load applies. A missing file leaves the world as-is (it was seeded at Load),
// so the first Transact simply saves it.
func (fs *FileStore) reload() error {
	data, err := os.ReadFile(worldPath(fs.cfg))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, fs.w); err != nil {
		return err
	}
	repair(fs.w, fs.cfg)
	// Unmarshal replaced every *Empire; tell per-session caches to re-resolve
	// their active empire by handle.
	fs.w.MarkReloaded()
	return nil
}
