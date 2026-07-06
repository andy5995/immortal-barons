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
	// under the lock, the way cmd/immortal-barons-web's ticker does. Stops once all
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
			f := &fakeSession{keys: []rune(fmt.Sprintf(" \rRealm%d\r0", i))}
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

// TestConcurrentTurnAndDiplomacyRaceMaintenance drives one session through a
// FULL turn (runTurn's income/status/payment pipeline, then a detour through
// the System -> Diplomacy -> View Treaties screen, which ranges w.Empires)
// concurrently with: a maintenance-ticker goroutine mutating the same
// empire's fields under the lock, and several onboarding/quitting sessions
// whose AddHuman appends to w.Empires. This is the shape of race the
// finding-#1 snapshot fixes in gameflow.go/actions.go close — every read of
// p.TurnsLeft/p.Events/p.Gold/... and every "for _, e := range w.Empires"
// must happen under w.With, or -race reports a data race here.
func TestConcurrentTurnAndDiplomacyRaceMaintenance(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 2)
	w.Today = "2026-07-05"

	const n = 3
	var sessions sync.WaitGroup
	var saveMu sync.Mutex
	save := func() error { saveMu.Lock(); defer saveMu.Unlock(); return nil }

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

	// Plain onboard-and-quit sessions, to keep w.Empires' backing slice
	// churning (AddHuman appends) while the turn-playing session below reads
	// it via View Treaties.
	for i := 0; i < n; i++ {
		sessions.Add(1)
		go func(i int) {
			defer sessions.Done()
			f := &fakeSession{keys: []rune(fmt.Sprintf(" \rRealm%d\r0", i))}
			if _, err := Session(f, Identity{Handle: fmt.Sprintf("caller%d", i)}, w, cfg, save); err != nil {
				t.Errorf("session %d: %v", i, err)
			}
		}(i)
	}

	// The turn-playing session: splash, realm name, Play Game, accept the
	// suggested forces/regions maintenance payment, detour through
	// System Menu -> Diplomacy -> View Treaties (ranges w.Empires), back
	// out through Attack/Covert/Trading, decline the message prompt and the
	// next-turn prompt, then quit from the Game Menu. Keys verified against
	// internal/menu/tree.go's hotkeys and the proven sequence in
	// internal/menu/gameflow_test.go's TestRunTurnConsumesATurn.
	turnKeys := " \rTurnPlayer\r" + "1 \r\r *D9 000000n\r n\r0"
	sessions.Add(1)
	go func() {
		defer sessions.Done()
		f := &fakeSession{keys: []rune(turnKeys)}
		if _, err := Session(f, Identity{Handle: "turnplayer"}, w, cfg, save); err != nil {
			t.Errorf("turn-playing session: %v", err)
		}
	}()

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
	if want := n + 1; humans != want {
		t.Fatalf("onboarded humans = %d, want %d", humans, want)
	}
	tp := w.FindByOwner("turnplayer")
	if tp == nil {
		t.Fatal("turn-playing empire should have been onboarded")
	}
	// Not asserting an exact TurnsLeft: the writer goroutine keeps advancing
	// the game day throughout, and each daily maintenance pass resets
	// TurnsLeft — so the only timing-independent invariant is that it's a
	// valid, non-negative value.
	if tp.TurnsLeft < 0 {
		t.Errorf("turn player's TurnsLeft went negative: %d", tp.TurnsLeft)
	}
}
