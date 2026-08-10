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
	var got int64
	wb.With(func() {
		got = wb.FindByOwner("alice").Gold
	})
	if got != 4242 {
		t.Fatalf("B saw gold %d, want 4242 (A's change not visible through the file)", got)
	}
}

// Money is int64, so a treasury past the old 2-billion ceiling must survive the
// JSON round trip bit for bit. A figure this size is exactly where a narrower
// field, or a decode through float64, loses the low digits — and losing them
// silently is the failure this width was widened to stop.
func TestLargeMoneySurvivesSaveAndLoad(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.MoneyCapBillions = game.MoneyCapMaxBillions

	w := game.NewWorldSeed(cfg, 1)
	e := w.AddHuman("croesus", "Croesus")
	e.Gold, e.Bank, e.Debt = w.MoneyCap(), 123_456_789_012, 9_876_543_210
	if err := Save(w, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	back, err := Load(cfg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got := back.FindByOwner("croesus")
	if got.Gold != game.MoneyCapMax {
		t.Errorf("Gold came back %d, want %d", got.Gold, game.MoneyCapMax)
	}
	if got.Bank != 123_456_789_012 {
		t.Errorf("Bank came back %d, want 123,456,789,012", got.Bank)
	}
	if got.Debt != 9_876_543_210 {
		t.Errorf("Debt came back %d, want 9,876,543,210", got.Debt)
	}
}
