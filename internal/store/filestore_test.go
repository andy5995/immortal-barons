package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

	w := game.NewWorldSeed(cfg, 1)
	e := w.AddHuman("croesus", "Croesus")
	// Set past the hold cap deliberately: nothing clamps on the way to disk, and
	// the round trip is the subject rather than the cap.
	e.Gold, e.Bank, e.Debt = game.MoneyCapMax, 123_456_789_012, 9_876_543_210
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

// A gathering body goes through Read, and Read must not write. On a door,
// Transact is flock → reload → fn → SAVE → release, so taking a snapshot
// through it rewrites world.json under the exclusive lock every other node is
// queued on, once per screen drawn — and a failed Save is session-fatal, so a
// pure read could end a caller's session over a write it never needed.
//
// The world's mtime is the check: a save rewrites the file whether or not the
// bytes moved, so this catches a Save that happens to be a no-op semantically.
func TestReadDoesNotWriteTheWorld(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	w := game.NewWorldSeed(cfg, 1)
	w.AddHuman("alice", "Alice")
	if err := Save(w, cfg); err != nil {
		t.Fatalf("seed save: %v", err)
	}
	w.SetStore(NewFileStore(w, cfg))

	path := filepath.Join(cfg.DataDir, "world.json")
	stat := func() (time.Time, int64) {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		return fi.ModTime(), fi.Size()
	}
	// Coarse filesystem timestamps would make an immediate second write look
	// like no write at all, so age the file past any plausible granularity.
	before := time.Now().Add(-2 * time.Second)
	if err := os.Chtimes(path, before, before); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	wantTime, wantSize := stat()

	var seen string
	w.Read(func() {
		if p := w.FindByName("Alice"); p != nil {
			seen = p.Name
		}
	})
	if seen != "Alice" {
		t.Fatalf("Read never reached the world (got %q) — the rest proves nothing", seen)
	}
	if gotTime, gotSize := stat(); !gotTime.Equal(wantTime) || gotSize != wantSize {
		t.Errorf("Read rewrote world.json: mtime %v->%v, size %d->%d",
			wantTime, gotTime, wantSize, gotSize)
	}

	// And the contrast: With DOES write, so the check above is measuring
	// something rather than passing because nothing ever touches the file.
	w.With(func() { w.AddHuman("bob", "Bob") })
	if gotTime, _ := stat(); gotTime.Equal(wantTime) {
		t.Error("With did not write world.json; the mtime check cannot detect a write")
	}
}

// Read still RELOADS, so a screen drawn through it shows what another node
// committed rather than what this session last happened to load.
func TestReadSeesAnotherNodesCommit(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	wa := game.NewWorldSeed(cfg, 1)
	wa.AddHuman("alice", "Alice")
	if err := Save(wa, cfg); err != nil {
		t.Fatalf("seed save: %v", err)
	}
	wa.SetStore(NewFileStore(wa, cfg))

	wb, err := Load(cfg)
	if err != nil {
		t.Fatalf("load B: %v", err)
	}
	wb.SetStore(NewFileStore(wb, cfg))
	wb.With(func() { wb.AddHuman("bob", "Bob") })

	var found bool
	wa.Read(func() { found = wa.FindByName("Bob") != nil })
	if !found {
		t.Error("Read served stale state; it must reload like Transact does")
	}
}
