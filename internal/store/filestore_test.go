package store

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestFileStoreCrossStoreVisibility proves the file-per-transaction model:
// a mutation committed inside store A's Transact is visible when a SEPARATE
// World+FileStore over the same file opens its next Transact — the change
// travels through world.json, not shared memory.
func TestFileStoreCrossStoreVisibility(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()

	wa := game.NewWorldSeed(cfg, 1)
	wa.AddHuman("alice", "Alice")
	if err := Save(wa, cfg); err != nil {
		t.Fatalf("seed save: %v", err)
	}
	wa.SetStore(NewFileStore(wa, cfg))

	// A second, independent World over the same file.
	wb, err := Load(cfg)
	if err != nil {
		t.Fatalf("load B: %v", err)
	}
	wb.SetStore(NewFileStore(wb, cfg))

	// Store A deducts gold inside a transaction (which reloads, mutates, saves).
	wa.With(func() {
		wa.FindByOwner("alice").Gold = 4242
	})

	// Store B, in its own transaction, must see A's committed value — even though
	// wb's in-memory empire pointers predate the change.
	var got int
	wb.With(func() {
		got = wb.FindByOwner("alice").Gold
	})
	if got != 4242 {
		t.Fatalf("B saw gold %d, want 4242 (A's change not visible through the file)", got)
	}
}
