package play

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// leagueCfgIn is cfgIn on a league board with Dupe Checking on.
func leagueCfgIn(t *testing.T) game.Config {
	t.Helper()
	cfg := cfgIn(t.TempDir())
	cfg.IBBS = true
	cfg.BoardID = "Alpha"
	cfg.DupeChecking = true
	return cfg
}

// editWorld loads the saved world, applies fn under the lock and saves it.
func editWorld(t *testing.T, cfg game.Config, fn func(w *game.World)) {
	t.Helper()
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w.SetStore(store.NewFileStore(w, cfg))
	w.With(func() { fn(w) })
}

// bravoScores is Bravo's scores packet listing a realm of Khan's.
func bravoScores(cfg game.Config) []game.RemoteScore {
	far := game.NewWorldSeed(cfg, 1)
	far.Config.BoardID = "Bravo"
	far.AddHuman("Khan", "Far Khanate")
	far.ExportScores()
	return far.Outbox[0].Scores
}

// lockToBravo locks Khan's realm to Bravo, with the Bravo scores that set it.
func lockToBravo(w *game.World) {
	w.ImportBoard(game.RemoteBoard{BoardID: "Bravo", Scores: bravoScores(w.Config)})
	w.FindByOwner("khan").DupeLockedBy = "Bravo"
}

// A caller another board already lists is refused a new realm, as in BRE,
// instead of founding one and having both realms locked (#323).
func TestDupeCallerCannotFoundARealm(t *testing.T) {
	cfg := leagueCfgIn(t)
	far := game.NewWorldSeed(cfg, 1)
	far.Config.BoardID = "Bravo"
	far.AddHuman("Khan", "Far Khanate")
	far.ExportScores()
	if len(far.Outbox) != 1 {
		t.Fatalf("Bravo queued %d packets, want its scores", len(far.Outbox))
	}
	editWorld(t, cfg, func(w *game.World) {
		w.ImportBoard(game.RemoteBoard{BoardID: "Bravo", Scores: far.Outbox[0].Scores})
	})

	f := &fakeSession{keys: []rune(" \r1Khanate\ry0")}
	reason, err := Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	if err != nil {
		t.Fatal(err)
	}
	if reason != "closed" {
		t.Errorf("reason = %q, want \"closed\"", reason)
	}
	if !strings.Contains(f.out.String(), "already playing on Bravo") {
		t.Errorf("expected the duplicate refusal, got %q", f.out.String())
	}
	w, _ := store.Load(cfg)
	if w.FindByOwner("khan") != nil {
		t.Error("a realm was founded for a caller Bravo lists")
	}
}

// A locked baron is offered BRE's way out: erase this realm, so the other
// board's lock can lift. Declining leaves the realm as it was.
func TestDupeLockedBaronMayEraseTheRealm(t *testing.T) {
	for _, tc := range []struct {
		key   rune
		alive bool
	}{{'n', true}, {'y', false}} {
		cfg := leagueCfgIn(t)
		f1 := &fakeSession{keys: []rune(" \r1Khanate\ry0")}
		if _, err := Run(f1, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
			t.Fatal(err)
		}
		editWorld(t, cfg, func(w *game.World) {
			lockToBravo(w)
		})

		f2 := &fakeSession{keys: []rune{' ', tc.key}}
		reason, err := Run(f2, Identity{Handle: "Khan"}, cfg, "2026-07-03")
		if err != nil {
			t.Fatal(err)
		}
		if reason != "dupe" {
			t.Errorf("%c: reason = %q, want \"dupe\"", tc.key, reason)
		}
		if !strings.Contains(f2.out.String(), "wish to erase your empire?") {
			t.Errorf("%c: the erase offer never appeared: %q", tc.key, f2.out.String())
		}
		w, _ := store.Load(cfg)
		e := w.FindByOwner("khan")
		if e == nil || e.Alive != tc.alive {
			t.Errorf("%c: realm %+v, want alive=%v", tc.key, e, tc.alive)
		}
	}
}

// Input that ends at the offer ends the session like any other prompt.
func TestDupeLockOfferEndsOnEOF(t *testing.T) {
	cfg := leagueCfgIn(t)
	f1 := &fakeSession{keys: []rune(" \r1Khanate\ry0")}
	if _, err := Run(f1, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	editWorld(t, cfg, func(w *game.World) {
		lockToBravo(w)
	})
	f2 := &fakeSession{keys: []rune(" ")}
	if _, err := Run(f2, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f2.out.String(), "wish to erase your empire?") {
		t.Fatalf("the erase offer never appeared: %q", f2.out.String())
	}
	w, _ := store.Load(cfg)
	if e := w.FindByOwner("khan"); e == nil || !e.Alive {
		t.Error("the realm was erased by input that ended")
	}
}

// A realm erased at the offer is dead for the rest of the day, so a second
// login gets the died-today refusal, not the offer again.
func TestDupeErasedRealmIsNotOfferedAgain(t *testing.T) {
	cfg := leagueCfgIn(t)
	f1 := &fakeSession{keys: []rune(" \r1Khanate\ry0")}
	if _, err := Run(f1, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	editWorld(t, cfg, func(w *game.World) {
		lockToBravo(w)
	})
	if _, err := Run(&fakeSession{keys: []rune(" y")}, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	f3 := &fakeSession{keys: []rune(" y")}
	reason, err := Run(f3, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	if err != nil {
		t.Fatal(err)
	}
	if reason != "dead" {
		t.Errorf("reason = %q, want \"dead\"", reason)
	}
	if strings.Contains(f3.out.String(), "wish to erase your empire?") {
		t.Error("an erased realm was offered the erase again")
	}
	// Bravo still lists Khan, so a later day would refuse a new realm too; the
	// usual "return on a later day" would promise one. The realm is still
	// reported gone, or the baron is never told it was erased.
	if !strings.Contains(f3.out.String(), "was destroyed") {
		t.Errorf("the erased realm was never reported gone: %q", f3.out.String())
	}
	if !strings.Contains(f3.out.String(), "already playing on Bravo") {
		t.Errorf("expected the duplicate refusal in place of the return-later notice: %q", f3.out.String())
	}
}

// A baron whose fallen realm was swept, and whom another board lists, is told
// the realm fell and refused a new one, not promised a fresh start first.
func TestDupeRebornCallerIsNotPromisedANewRealm(t *testing.T) {
	cfg := leagueCfgIn(t)
	w := game.NewWorldSeed(cfg, 1)
	w.ImportBoard(game.RemoteBoard{BoardID: "Bravo", Scores: bravoScores(cfg)})
	f := &fakeSession{keys: []rune(" ")}
	reason, err := Session(f, Identity{Handle: "Khan"}, w, cfg, "Old Khanate", game.MaintReport{}, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if reason != "closed" || !strings.Contains(out, "already playing on Bravo") {
		t.Fatalf("reason %q, want the duplicate refusal: %q", reason, out)
	}
	if !strings.Contains(out, "Old Khanate was destroyed") || strings.Contains(out, "begin anew") {
		t.Errorf("the reborn notice should report the fall without a fresh start: %q", out)
	}
}
