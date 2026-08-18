package play

import (
	"strings"
	"sync"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// TestConcurrentOnboardSameRealmName drives two first-time callers who both
// try to claim the SAME realm name against one temp world file. The insert is
// re-checked (FindByOwner/BoardFull/RealmNameTaken) inside the same FileStore
// transaction that appends the empire, so the file-lock serializes them:
// exactly one empire ends up named "Contested", and the loser is bounced to a
// different name. This extends the #40 name TOCTOU fix to the multi-node file
// world — two nodes must not both create the same realm.
func TestConcurrentOnboardSameRealmName(t *testing.T) {
	cfg := cfgIn(t.TempDir())

	var wg sync.WaitGroup
	onboard := func(handle string) {
		defer wg.Done()
		// Splash dismiss, realm name "Contested", confirm. The caller who loses
		// the race is told the name was taken and re-prompted, and names their
		// realm after their own handle instead — a read error at this prompt ends
		// the session with no realm at all, so the fallback has to be typed.
		f := &fakeSession{keys: []rune(" \rContested\ry" + handle + "land\ry0")}
		if _, err := Run(f, Identity{Handle: handle}, cfg, "2026-07-10"); err != nil {
			t.Errorf("%s: %v", handle, err)
		}
	}
	wg.Add(2)
	go onboard("alice")
	go onboard("bob")
	wg.Wait()

	w, err := store.Load(cfg)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	contested, humans := 0, 0
	for _, e := range w.Empires {
		if e.Owner != "" {
			humans++
		}
		if strings.EqualFold(e.Name, "Contested") {
			contested++
		}
	}
	if contested != 1 {
		t.Fatalf("empires named %q = %d, want exactly 1 (no duplicate realm across nodes)", "Contested", contested)
	}
	if humans != 2 {
		t.Fatalf("onboarded humans = %d, want 2 (both callers should get an empire)", humans)
	}
}

// TestLoginMaintenanceIdempotentOnRollover has two callers log in at the same
// time on a date that has rolled over since the last maintenance, and asserts
// GameDay climbs by exactly one. Be honest about what that proves: it is the
// IDEMPOTENCE of DailyMaintenance for an already-current date. It is NOT a
// lock test — with the flock removed, both racers would load day 5, both
// compute 6, both save 6, and the assertion still passes; lock coverage lives
// in store's lock tests and the cross-process test.
func TestLoginMaintenanceIdempotentOnRollover(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorld(cfg)
	w.AddHuman("alice", "Alicia")
	w.AddHuman("bob", "Bobsland")
	w.LastMaintDate = "2026-07-09"
	w.GameDay = 5
	if err := store.Save(w, cfg); err != nil {
		t.Fatalf("seed save: %v", err)
	}

	var wg sync.WaitGroup
	login := func(handle string) {
		defer wg.Done()
		f := &fakeSession{keys: []rune(" 0")} // splash dismiss, quit (returning player)
		if _, err := Run(f, Identity{Handle: handle}, cfg, "2026-07-10"); err != nil {
			t.Errorf("%s: %v", handle, err)
		}
	}
	wg.Add(2)
	go login("alice")
	go login("bob")
	wg.Wait()

	w2, err := store.Load(cfg)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if w2.GameDay != 6 {
		t.Fatalf("GameDay = %d, want 6 (one rollover must advance the day once, not once per logging-in node)", w2.GameDay)
	}
	if w2.LastMaintDate != "2026-07-10" {
		t.Errorf("LastMaintDate = %q, want %q", w2.LastMaintDate, "2026-07-10")
	}
}
