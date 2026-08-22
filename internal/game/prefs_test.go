package game

import "testing"

// Preferences are one player's own settings, not the board's. They lived on the
// World, so three humans sharing a door shared one set and each
// overwrote the others' every session.
func TestPreferencesAreKeptPerEmpire(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("alice", "Avalon")
	b := w.AddHuman("bob", "Bravo")

	a.Prefs.AutoFeed = false
	a.Prefs.EnterExitsBuy = true

	if !b.Prefs.AutoFeed {
		t.Error("one realm turning AutoFeed off turned it off for another realm")
	}
	if b.Prefs.EnterExitsBuy {
		t.Error("one realm turning EnterExitsBuy on turned it on for another realm")
	}
}

// A realm founded after the move starts on IB's defaults rather than the zero
// value: three of the seven default to on.
func TestNewRealmStartsOnTheDefaultPreferences(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("alice", "Avalon")
	if got, want := e.Prefs, DefaultPrefs(); got != want {
		t.Errorf("new realm prefs = %+v, want %+v", got, want)
	}
	if !e.Prefs.Set {
		t.Error("a new realm's prefs are not marked set, so a later load would overwrite them")
	}
}

// A save written before the move carries the settings on the world. Every realm
// in it inherits those, so nobody's board-wide choice is silently reset by the
// upgrade.
func TestEnsurePrefsMigratesTheWorldsPreferences(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("alice", "Avalon")
	b := w.AddHuman("bob", "Bravo")
	a.Prefs, b.Prefs = Prefs{}, Prefs{} // as an older save unmarshals
	w.EnterExitsBuy, w.AutoFeed = true, false

	w.EnsurePrefs()

	for _, e := range []*Empire{a, b} {
		if !e.Prefs.Set {
			t.Fatalf("%s: prefs not marked set after migration", e.Name)
		}
		if !e.Prefs.EnterExitsBuy {
			t.Errorf("%s: EnterExitsBuy not carried over from the world", e.Name)
		}
		if e.Prefs.AutoFeed {
			t.Errorf("%s: AutoFeed not carried over from the world", e.Name)
		}
	}
}

// Migration runs on every load, so it must not overwrite a realm that has since
// set its own.
func TestEnsurePrefsLeavesARealmThatHasItsOwn(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("alice", "Avalon")
	e.Prefs.AutoFeed = false
	w.AutoFeed = true

	w.EnsurePrefs()

	if e.Prefs.AutoFeed {
		t.Error("migration overwrote a realm's own preference with the world's")
	}
}
