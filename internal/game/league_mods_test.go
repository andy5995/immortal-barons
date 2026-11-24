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
	m.PendingLeagueReset = &LeagueReset{OnDate: "2026-09-01"} // an older order

	m.receive(leagueResetOrder(t, lc, m.Config.BoardID, BaseRules(), refs))

	if len(m.Empires) != 1 || m.StartedDate != started || len(m.Mods) != 0 {
		t.Errorf("a refused reset changed the member: %d realms, started %q, mods %+v",
			len(m.Empires), m.StartedDate, m.Mods)
	}
	joined := strings.Join(m.SysopNotices, "\n")
	if !strings.Contains(joined, "did not reset") || !strings.Contains(joined, "this board does not accept scripts") {
		t.Errorf("the sysop was not told why the reset was refused: %q", joined)
	}
	if !strings.Contains(joined, "resets on the first planetary run after it can play them") {
		t.Errorf("the sysop was not told the order will be retried: %q", joined)
	}
	if pending := m.PendingLeagueReset; pending == nil || pending.OnDate != "2026-10-01" ||
		!reflect.DeepEqual(pending.Mods, refs) {
		t.Errorf("the refused order was not kept in place of the older one: %+v", pending)
	}
}

// A sysop who makes the mods playable gets the refused order carried out on the
// next planetary run, with the order's rules, mods and date — not a fresh order.
func TestPendingLeagueResetAppliesOnRetry(t *testing.T) {
	lc, m := freezePair(t)
	refs := []ModRef{{Name: "cheap-tanks", Version: "1.0", Hash: strings.Repeat("a", 64),
		Files: map[string]string{"values.toml": "[units.tank]\nbuild_cost = 250\n"}}}
	allowed := false
	called := 0
	stubLeagueMods(t, func(got []ModRef, scripts bool) (*Rules, error) {
		called++
		if !reflect.DeepEqual(got, refs) {
			t.Errorf("the loader got %+v, want the order's %+v", got, refs)
		}
		if scripts != allowed {
			t.Errorf("the loader got scripts=%v, want %v", scripts, allowed)
		}
		if !allowed {
			return nil, errors.New("this board does not accept scripts")
		}
		r := BaseRules()
		unit := r.Units[Tank]
		unit.Cost = 250
		r.Units[Tank] = unit
		return r, nil
	})
	m.AddHuman("bob", "Bobland")

	m.receive(leagueResetOrder(t, lc, m.Config.BoardID, BaseRules(), refs))
	if m.PendingLeagueReset == nil {
		t.Fatal("the refused order was not kept")
	}
	m.Config.AcceptLeagueScripts = true
	allowed = true
	m.SysopNotices = nil // the refusal belongs to the earlier run

	m.RetryPendingLeagueReset()

	if called != 2 {
		t.Errorf("the loader was called %d times, want a refusal and a retry", called)
	}
	if m.PendingLeagueReset != nil {
		t.Errorf("the applied retry left an order pending: %+v", m.PendingLeagueReset)
	}
	if !reflect.DeepEqual(m.Mods, refs) || m.Rules == nil || m.Rules.Units[Tank].Cost != 250 {
		t.Errorf("the retry did not install the order's mods: mods %+v, rules %+v", m.Mods, m.Rules)
	}
	if len(m.Empires) != 0 || m.StartedDate != "2026-10-01" {
		t.Errorf("the retry kept its old game: %d realms, started %q", len(m.Empires), m.StartedDate)
	}
	if len(m.SysopNotices) != 0 {
		t.Errorf("a successful retry reported a fault: %q", m.SysopNotices)
	}
}

// A later order that can be played clears the one that was refused.
func TestAppliedLeagueResetClearsThePendingOrder(t *testing.T) {
	lc, m := freezePair(t)
	m.PendingLeagueReset = &LeagueReset{OnDate: "2026-09-01"}
	stubLeagueMods(t, func([]ModRef, bool) (*Rules, error) { return BaseRules(), nil })

	m.receive(leagueResetOrder(t, lc, m.Config.BoardID, BaseRules(), nil))

	if m.PendingLeagueReset != nil {
		t.Errorf("an applied reset left the older order pending: %+v", m.PendingLeagueReset)
	}
	if m.StartedDate != "2026-10-01" {
		t.Errorf("the new order did not start the game: started %q", m.StartedDate)
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

// A frozen board keeps its refused order until the thaw: a reset clears every
// saved field, so retrying it would thaw this board alone.
func TestPendingLeagueResetWaitsForTheThaw(t *testing.T) {
	lc, m := freezePair(t)
	refs := []ModRef{{Name: "scripted", Version: "1.0", Hash: strings.Repeat("c", 64),
		Files: map[string]string{"values.toml": ""}}}
	accept := false
	stubLeagueMods(t, func([]ModRef, bool) (*Rules, error) {
		if !accept {
			return nil, errors.New("this board does not accept scripts")
		}
		return BaseRules(), nil
	})
	m.receive(leagueResetOrder(t, lc, m.Config.BoardID, BaseRules(), refs))
	if m.PendingLeagueReset == nil {
		t.Fatal("the refused order was not kept")
	}
	m.Frozen = true
	accept = true
	m.RetryPendingLeagueReset()
	if m.PendingLeagueReset == nil || !m.Frozen || len(m.Mods) != 0 {
		t.Fatalf("a frozen board retried its order: pending %v, frozen %v, mods %v", m.PendingLeagueReset, m.Frozen, m.Mods)
	}
	m.Frozen = false
	m.RetryPendingLeagueReset()
	if m.PendingLeagueReset != nil || len(m.Mods) != 1 {
		t.Errorf("after the thaw: pending %v, mods %v; want the order applied", m.PendingLeagueReset, m.Mods)
	}
}
