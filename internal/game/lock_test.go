package game

import (
	"sync"
	"testing"
)

func TestWorldWithIsMutuallyExclusive(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	const goroutines, incs = 8, 1000
	counter := 0
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incs; j++ {
				w.With(func() { counter++ })
			}
		}()
	}
	wg.Wait()
	if counter != goroutines*incs {
		t.Fatalf("counter = %d, want %d (lost updates => not mutually exclusive)", counter, goroutines*incs)
	}
}
