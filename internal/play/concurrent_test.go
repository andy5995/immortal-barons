package play

import (
	"fmt"
	"sync"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestConcurrentSessionsShareWorld drives N goroutines each onboarding and
// quitting against ONE shared world, concurrently with a "writer" goroutine
// that advances daily maintenance under the world lock the way the real
// web-server ticker does. This is the headline hazard the shared-world
// design (Tasks 1-5) exists to survive: session entry (FindByOwner,
// JoinOpen, BoardFull) and per-empire bookkeeping (showEvents) must not
// read the World unlocked while the ticker mutates it.
func TestConcurrentSessionsShareWorld(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	w.Today = "2026-07-05"

	const n = 4
	var sessions sync.WaitGroup
	var saveMu sync.Mutex
	save := func() error { saveMu.Lock(); defer saveMu.Unlock(); return nil }

	// Writer: repeatedly advances the world clock and runs daily maintenance
	// under the lock, the way cmd/barons-web's ticker does. Stops once all
	// player sessions have finished.
	stop := make(chan struct{})
	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		day := 6
		for {
			select {
			case <-stop:
				return
			default:
			}
			today := fmt.Sprintf("2026-07-%02d", day)
			w.With(func() {
				w.Today = today
				w.DailyMaintenance(today)
			})
			day++
			if day > 40 {
				day = 6
			}
		}
	}()

	for i := 0; i < n; i++ {
		sessions.Add(1)
		go func(i int) {
			defer sessions.Done()
			// splash dismiss, realm name, then quit.
			f := &fakeSession{keys: []rune(fmt.Sprintf(" Realm%d\r0", i))}
			if _, err := Session(f, Identity{Handle: fmt.Sprintf("caller%d", i)}, w, cfg, save); err != nil {
				t.Errorf("session %d: %v", i, err)
			}
		}(i)
	}

	sessions.Wait()
	close(stop)
	writer.Wait()

	humans := 0
	for _, e := range w.Empires {
		if e.Gold < 0 {
			t.Errorf("empire %q has negative gold: %d", e.Name, e.Gold)
		}
		if e.Owner != "" {
			humans++
		}
	}
	if humans != n {
		t.Fatalf("onboarded humans = %d, want %d", humans, n)
	}
}
