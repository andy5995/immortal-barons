package play

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// markDead loads the world, marks the handle's empire dead on diedDay with the
// world at gameDay, and persists it.
func markDead(t *testing.T, cfg game.Config, handle string, gameDay, diedDay int) {
	t.Helper()
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w.SetStore(store.NewFileStore(w, cfg))
	w.With(func() {
		e := w.FindByOwner(handle)
		if e == nil {
			t.Fatal("expected empire to exist")
		}
		e.Alive = false
		e.DiedDay = diedDay
		w.GameDay = gameDay
	})
}

// TestLoginDeadSameDayEndsSession: a realm that died today (GameDay <= DiedDay)
// gets no play — the session ends with a "return later" notice and the husk is
// kept (no new realm).
func TestLoginDeadSameDayEndsSession(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	f1 := &fakeSession{keys: []rune(" \rKhanate\r0")}
	if _, err := Run(f1, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	markDead(t, cfg, "khan", 5, 5)

	f2 := &fakeSession{keys: []rune(" 0")}
	reason, err := Run(f2, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	if err != nil {
		t.Fatal(err)
	}
	if reason != "dead" {
		t.Errorf("reason = %q, want \"dead\"", reason)
	}
	if !strings.Contains(f2.out.String(), "Return on a later day") {
		t.Errorf("expected the return-later notice, got %q", f2.out.String())
	}
	w, _ := store.Load(cfg)
	e := w.FindByOwner("khan")
	if e == nil {
		t.Fatal("husk should be kept when death is same-day")
	}
	if e.Alive {
		t.Error("empire should still be dead")
	}
	if e.Name != "Khanate" {
		t.Errorf("no new realm should be created; name = %q", e.Name)
	}
}

// TestLoginDeadPastDayRebuilds: a realm that died on a past day (GameDay >
// DiedDay) is swept and the fresh-onboarding path builds a new realm.
func TestLoginDeadPastDayRebuilds(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	f1 := &fakeSession{keys: []rune(" \rKhanate\r0")}
	if _, err := Run(f1, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	markDead(t, cfg, "khan", 5, 3)

	f2 := &fakeSession{keys: []rune(" \rRebornia\r0")}
	if _, err := Run(f2, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f2.out.String(), "begin anew") {
		t.Errorf("expected the fresh-start notice, got %q", f2.out.String())
	}
	w, _ := store.Load(cfg)
	e := w.FindByOwner("khan")
	if e == nil {
		t.Fatal("a fresh realm should have been built")
	}
	if !e.Alive {
		t.Error("the rebuilt realm should be alive")
	}
	if e.Name != "Rebornia" {
		t.Errorf("rebuilt realm name = %q, want \"Rebornia\"", e.Name)
	}
	if e.DiedDay != 0 {
		t.Errorf("rebuilt realm DiedDay = %d, want 0", e.DiedDay)
	}
}
