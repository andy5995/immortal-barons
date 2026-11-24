package game

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// stubLeagueMods stands in for internal/mod's reader, which the engine cannot
// import, and puts the real loader back when the test ends.
func stubLeagueMods(t *testing.T, fn func([]ModRef, bool) (*Rules, error)) {
	t.Helper()
	old := LoadLeagueMods
	LoadLeagueMods = fn
	t.Cleanup(func() { LoadLeagueMods = old })
}

func leagueResetOrder(t *testing.T, lc *World, board string, rules *Rules, mods []ModRef) Packet {
	t.Helper()
	if err := lc.DeclareLeagueReset("2026-10-01", "", rules, mods); err != nil {
		t.Fatal(err)
	}
	lc.StampOutbox()
	for _, p := range lc.Outbox {
		if p.Reset != nil && (p.ToBoard == "" || p.ToBoard == board) {
			return p
		}
	}
	t.Fatalf("no reset order for %s in the Coordinator's outbox", board)
	return Packet{}
}

// A member applying the Coordinator's order gets the rules and the mod copies
// the order carries, and the reset still clears its old game.
func TestLeagueResetInstallsTheCoordinatorsMods(t *testing.T) {
	lc, m := freezePair(t)
	m.Config.AcceptLeagueScripts = true
	refs := []ModRef{{Name: "cheap-tanks", Version: "1.0", Hash: strings.Repeat("a", 64),
		Files: map[string]string{"values.toml": "[units.tank]\nbuild_cost = 250\n"}}}
	called := false
	stubLeagueMods(t, func(got []ModRef, scripts bool) (*Rules, error) {
		called = true
		if !reflect.DeepEqual(got, refs) {
			t.Errorf("the loader got %+v, want the order's %+v", got, refs)
		}
		if !scripts {
			t.Error("the member's AcceptLeagueScripts setting was not passed to the loader")
		}
		r := BaseRules()
		unit := r.Units[Tank]
		unit.Cost = 250
		r.Units[Tank] = unit
		return r, nil
	})
	m.AddHuman("bob", "Bobland")

	m.receive(leagueResetOrder(t, lc, m.Config.BoardID, BaseRules(), refs))

	if !called {
		t.Fatal("the member never asked for the order's mods to be loaded")
	}
	if !reflect.DeepEqual(m.Mods, refs) {
		t.Errorf("the member recorded %+v, want %+v", m.Mods, refs)
	}
	if m.Rules == nil || m.Rules.Units[Tank].Cost != 250 {
		t.Errorf("the member did not play the loaded rules: %+v", m.Rules)
	}
	if len(m.Empires) != 0 || m.StartedDate != "2026-10-01" {
		t.Errorf("the member kept its old game: %d realms, started %q", len(m.Empires), m.StartedDate)
	}
}

// A board that cannot load the order's mods keeps its old game: a reset under
// rules it cannot play would feed a different game into the league.
func TestUnplayableLeagueModsLeaveTheMemberUnreset(t *testing.T) {
	lc, m := freezePair(t)
	refs := []ModRef{{Name: "scarce-tanks", Version: "1.0", Hash: strings.Repeat("b", 64),
		Files: map[string]string{"values.toml": "[units.tank]\nbuild_cost = 900\n"}}}
	stubLeagueMods(t, func([]ModRef, bool) (*Rules, error) {
		return nil, errors.New("this board does not accept scripts")
	})
	m.AddHuman("bob", "Bobland")
	started := m.StartedDate

	m.receive(leagueResetOrder(t, lc, m.Config.BoardID, BaseRules(), refs))

	if len(m.Empires) != 1 || m.StartedDate != started || len(m.Mods) != 0 {
		t.Errorf("a refused reset changed the member: %d realms, started %q, mods %+v",
			len(m.Empires), m.StartedDate, m.Mods)
	}
	joined := strings.Join(m.SysopNotices, "\n")
	if !strings.Contains(joined, "did not reset") || !strings.Contains(joined, "this board does not accept scripts") {
		t.Errorf("the sysop was not told why the reset was refused: %q", joined)
	}
}

// A board with no mods still stamps packets with the settings' fingerprint, so
// adding mods to a league does not move the ruleset of a game without them.
func TestWorldFingerprintIncludesMods(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TurnsPerDay = 5
	w := NewWorldSeed(cfg, 1)
	want := w.Config.RulesetFingerprint()
	if got := w.RulesetFingerprint(); got != want {
		t.Errorf("no mods: fingerprint %q, want the config's %q", got, want)
	}
	w.Mods = []ModRef{{Name: "cheap-tanks", Hash: strings.Repeat("a", 64),
		Files: map[string]string{"values.toml": "[units.tank]\nbuild_cost = 250\n"}}}
	if got := w.RulesetFingerprint(); got == want {
		t.Error("a modded board kept the settings' fingerprint")
	}
}
