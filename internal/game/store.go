package game

// Store runs a world mutation as one atomic transaction. It is the seam that
// lets the door reload and save the JSON world file per action (so several BBS
// nodes play concurrently) while the web server keeps one long-lived in-memory
// world. World.With delegates here; a mutating action does its gather,
// re-validate, and mutate inside a single Transact so a concurrent node can't
// change the world between the check and the change.
type Store interface {
	Transact(fn func()) error
}

// MemStore is the in-memory Store: it serializes fn under the world mutex with
// no file I/O — today's behavior. Used by the web server (one shared in-memory
// world across goroutine sessions) and as the default for tests.
type MemStore struct{ w *World }

func (m *MemStore) Transact(fn func()) error {
	m.w.mu.Lock()
	defer m.w.mu.Unlock()
	fn()
	return nil
}

// SetStore swaps the transaction backend (e.g. the door installs a file-backed
// Store that reloads/saves per Transact).
func (w *World) SetStore(s Store) { w.store = s }
