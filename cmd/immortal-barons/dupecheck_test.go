package main

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// lockedLeagueBoard is a league board holding one baron the league has reported
// playing elsewhere, saved to disk. The world is reloaded from that disk copy in
// each test, so what is asserted is what a real run would see.
func lockedLeagueBoard(t *testing.T) (dir string, cfg game.Config) {
	t.Helper()
	dir = t.TempDir()
	cfg, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg.IBBS = true // a duplicate means nothing off a league
	cfg.BoardID = "Alpha BBS"
	if err := store.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	w := store.NewGame(cfg)
	w.AddHuman("alice", "Iron Dominion").DupeLockedBy = "Bravo BBS"
	if err := store.Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	return dir, cfg
}

// The whole point of the switch: it changes what THIS run does and writes
// nothing. A run that persisted it would leave a league rule altered behind it,
// which is what -dupe-check must never do.
func TestDupeCheckOverrideNeverReachesDisk(t *testing.T) {
	dir, cfg := lockedLeagueBoard(t)
	off := false
	cfg.DupeCheckOverride = &off

	// SaveConfig is the only place a Config is marshalled (World.Config is
	// itself `json:"-"`, so world.json carries none), and it is what all four
	// of its callers reach — both Configuration Editors, an applied league
	// broadcast, and -reset. The world save runs too, to catch a Config that
	// starts being written there.
	if err := store.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(w, cfg); err != nil {
		t.Fatal(err)
	}

	// A later command loads the config fresh — as every invocation does.
	next, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !next.DupeChecking {
		t.Error("the -dupe-check override was written to config.json; the run changed the league rule")
	}
	if next.DupeCheckOverride != nil {
		t.Error("the override survived a save/load round trip, so it is being persisted somewhere")
	}
	w2, err := store.Load(next)
	if err != nil {
		t.Fatal(err)
	}
	if !w2.DupeLocked(w2.Empires[0]) {
		t.Error("the next run sees the lock lifted, so the override outlived the run that set it")
	}
}

// Off, the gate opens for a baron the league had shut out — and the record of
// who locked them survives, so the run is reversible by construction.
func TestDupeCheckOffLiftsTheGate(t *testing.T) {
	_, cfg := lockedLeagueBoard(t)
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !w.DupeLocked(w.Empires[0]) {
		t.Fatal("the baron should be locked out before the switch is used")
	}

	off := false
	cfg.DupeCheckOverride = &off
	w, err = store.Load(cfg) // the override rides cfg into the world, as in main
	if err != nil {
		t.Fatal(err)
	}
	if w.DupeLocked(w.Empires[0]) {
		t.Error("-dupe-check off should let a locked baron play")
	}
	if w.Empires[0].DupeLockedBy != "Bravo BBS" {
		t.Errorf("DupeLockedBy = %q, want the record kept so the lock returns without the switch", w.Empires[0].DupeLockedBy)
	}
}

// On forces the rule even where the sysop has it saved off, so a lockout can be
// exercised without editing the board's settings.
func TestDupeCheckOnForcesTheRule(t *testing.T) {
	_, cfg := lockedLeagueBoard(t)
	cfg.DupeChecking = false
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if w.DupeLocked(w.Empires[0]) {
		t.Fatal("with the setting off there is no lock to start from")
	}

	on := true
	cfg.DupeCheckOverride = &on
	w, err = store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !w.DupeLocked(w.Empires[0]) {
		t.Error("-dupe-check on should apply the rule despite the saved setting")
	}
}

// Only on and off are settings; anything else is a typo, and silently reading it
// as one of the two would leave the tester with the opposite of what they meant.
func TestParseOnOff(t *testing.T) {
	for _, tc := range []struct {
		in     string
		on, ok bool
	}{
		{"on", true, true},
		{"OFF", false, true},
		{" On ", true, true},
		{"yes", false, false},
		{"true", false, false},
		{"", false, false},
	} {
		on, ok := parseOnOff(tc.in)
		if on != tc.on || ok != tc.ok {
			t.Errorf("parseOnOff(%q) = %v, %v; want %v, %v", tc.in, on, ok, tc.on, tc.ok)
		}
	}
}
