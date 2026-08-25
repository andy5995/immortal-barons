package game

import (
	"errors"
	"testing"
)

// failingStore commits nothing and reports why, standing in for a save that
// cannot reach the disk (a full filesystem, a rename refused by another process
// holding the file open — the Windows case in #215).
type failingStore struct {
	err   error
	calls int
}

func (f *failingStore) Transact(fn func()) error {
	f.calls++
	return f.err
}

// TestWithRecordsAFailedTransaction is the fix for a save failure that read as
// a completed turn: With cannot return an error to its 74 call sites, so it
// records the first one for Run to report. Before this, Transact's error was
// discarded outright and a lost write was indistinguishable from a successful
// one.
func TestWithRecordsAFailedTransaction(t *testing.T) {
	boom := errors.New("rename world.json: permission denied")
	w := NewWorldSeed(DefaultConfig(), 1)
	w.SetStore(&failingStore{err: boom})

	if err := w.StoreErr(); err != nil {
		t.Fatalf("a fresh world should have no store error, got %v", err)
	}
	w.With(func() {})
	if !errors.Is(w.StoreErr(), boom) {
		t.Fatalf("StoreErr() = %v, want the failed transaction's error %v", w.StoreErr(), boom)
	}
}

// TestStoreErrKeepsTheFirstFailure locks in which error is reported. Once a save
// has failed, the in-memory world has drifted from the file, so every later
// failure is a consequence of the first — reporting the last one would name a
// symptom and hide the cause.
func TestStoreErrKeepsTheFirstFailure(t *testing.T) {
	first := errors.New("first failure")
	store := &failingStore{err: first}
	w := NewWorldSeed(DefaultConfig(), 1)
	w.SetStore(store)

	w.With(func() {})
	store.err = errors.New("a later, derived failure")
	w.With(func() {})

	if !errors.Is(w.StoreErr(), first) {
		t.Errorf("StoreErr() = %v, want the FIRST failure %v", w.StoreErr(), first)
	}
	if store.calls != 2 {
		t.Errorf("both transactions should still have been attempted, got %d", store.calls)
	}
}

// TestWithLeavesNoErrorWhenTransactionsCommit is the other half: a world whose
// saves all succeed must not report a fault, or Run would end every healthy
// session with an error.
func TestWithLeavesNoErrorWhenTransactionsCommit(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.SetStore(&MemStore{w: w})
	w.With(func() {})
	if err := w.StoreErr(); err != nil {
		t.Errorf("StoreErr() = %v, want nil after a committed transaction", err)
	}
}
