package game

import (
	"bytes"
	"encoding/json"
	"flag"
)

// Store runs a world mutation as one atomic transaction. It is the seam that
// lets the door reload and save the JSON world file per action (so several BBS
// nodes play concurrently) while an in-process front-end keeps one long-lived
// in-memory world. World.With delegates here; a mutating action does its gather,
// re-validate, and mutate inside a single Transact so a concurrent node can't
// change the world between the check and the change.
type Store interface {
	Transact(fn func()) error
	// Snapshot runs fn over a FRESH world without saving afterwards. A screen
	// that only reads still needs the reload — another node may have moved the
	// world since this session's last action — but it has nothing to write back,
	// and writing anyway is not free: on a door that is a full rewrite of
	// world.json under the exclusive lock every other node is waiting on, per
	// screen drawn. Worse, a Save that fails is session-fatal (see World.With),
	// so a read could end a session over a write it never needed.
	Snapshot(fn func()) error
}

// MemStore is the in-memory Store: it serializes fn under the world mutex with
// no file I/O. Used by a front-end holding one shared in-memory world across
// goroutine sessions, and as the default for tests.
type MemStore struct{ w *World }

func (m *MemStore) Transact(fn func()) error {
	m.w.mu.Lock()
	defer m.w.mu.Unlock()
	fn()
	return nil
}

// Snapshot is Transact for an in-memory world: there is no file to reload or
// save, so the two differ only on a door.
//
// Which is exactly why it checks. Under MemStore a body wrongly routed through
// Read behaves identically to one under With, so the whole test suite would
// stay green while the door silently discarded the mutation — the one defect
// this split can introduce is the one tests cannot otherwise see. Under a test
// binary the world is fingerprinted either side of fn and a change panics, so
// the misconversion fails loudly where it is cheap to fix.
func (m *MemStore) Snapshot(fn func()) error {
	if !UnderTest() {
		return m.Transact(fn)
	}
	m.w.mu.Lock()
	defer m.w.mu.Unlock()
	before, err := json.Marshal(m.w)
	if err != nil { // an unmarshalable world is a separate fault; do not mask it
		fn()
		return nil
	}
	fn()
	if after, err := json.Marshal(m.w); err == nil && !bytes.Equal(before, after) {
		panic("World.Read body mutated the world: nothing persists it, and a door " +
			"would discard the change at the next reload. Use World.With instead.")
	}
	return nil
}

// UnderTest reports whether this binary is `go test`. Checked through the flag
// the test harness registers rather than by importing testing, which a
// production package should not pull in.
//
// Called, not cached in a package var: testing.Init registers that flag from the
// generated TestMain, which runs AFTER package-level initialization, so a var
// would be computed while the answer was still no and the guard would never arm.
func UnderTest() bool { return flag.Lookup("test.v") != nil }

// SetStore swaps the transaction backend (e.g. the door installs a file-backed
// Store that reloads/saves per Transact).
func (w *World) SetStore(s Store) { w.store = s }
