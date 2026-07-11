package game

import (
	"sync"
	"testing"
)

// TestMemStoreSerializes: concurrent With calls through the default MemStore
// don't lose updates (the same guarantee the raw mutex gave). Run with -race.
func TestMemStoreSerializes(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	n := 0
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				w.With(func() { n++ })
			}
		}()
	}
	wg.Wait()
	if n != 50*1000 {
		t.Errorf("lost updates: n = %d, want %d", n, 50*1000)
	}
}

// TestWithNilStoreFallsBackToMutex: a bare World (no store set, as some tests
// build) still runs fn under the mutex fallback.
func TestWithNilStoreFallsBackToMutex(t *testing.T) {
	w := &World{}
	got := false
	w.With(func() { got = true })
	if !got {
		t.Fatal("With with nil store did not run fn")
	}
}
