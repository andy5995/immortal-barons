package menu

import (
	"strings"
	"testing"
)

// The sysop's three special-attack switches take their operations off the
// menus. Each was stored, edited and reported but gated nothing, so a league
// that turned missiles off still played with them.
func TestSpecialOpSwitchesHideTheirOperations(t *testing.T) {
	cases := []struct {
		name   string
		set    func(w *ctx)
		menu   func(m *Menus) *Menu
		key    rune
		label  string
		hidden bool
	}{
		{"missiles off hides Nuclear Attack", func(w *ctx) { w.Config.MissileOps = false },
			func(m *Menus) *Menu { return m.Attack }, 'N', "Nuclear Attack", true},
		{"missiles on keeps Nuclear Attack", func(w *ctx) { w.Config.MissileOps = true },
			func(m *Menus) *Menu { return m.Attack }, 'N', "Nuclear Attack", false},
		{"annihilator off hides its entry", func(w *ctx) { w.Config.ClingyAnnihilator = false },
			func(m *Menus) *Menu { return m.InterPlanetary }, '9', "Clingy Annihilator Ops", true},
		{"annihilator on keeps its entry", func(w *ctx) { w.Config.ClingyAnnihilator = true },
			func(m *Menus) *Menu { return m.InterPlanetary }, '9', "Clingy Annihilator Ops", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld()
			tc.set(w)
			m := tc.menu(BuildMenus())

			f := &fakeSession{keys: []rune("0\r ")}
			if err := Run(f, w, m); err != nil {
				t.Fatalf("Run: %v", err)
			}
			shown := strings.Contains(f.out.String(), tc.label)
			if shown == tc.hidden {
				t.Errorf("%q drawn = %v, want %v:\n%s", tc.label, shown, !tc.hidden, f.out.String())
			}
			// A hidden item's hotkey must be inert too, not merely undrawn.
			if got := m.byKey(tc.key, w); (got == nil) != tc.hidden {
				t.Errorf("byKey(%q) reachable = %v, want %v", tc.key, got != nil, !tc.hidden)
			}
		})
	}
}

// The Bomb Enemy Targets submenu splits between the two switches: the four
// bombing entries answer to Bombing Ops, the three warheads to Missile Ops.
func TestBombEnemyTargetsSplitsBetweenTheTwoSwitches(t *testing.T) {
	open := func(set func(w *ctx)) string {
		w := newWorld()
		set(w)
		// 7 opens Bomb Enemy Targets from the Covert menu, then quit both boxes.
		f := &fakeSession{keys: []rune("7\r0\r0\r ")}
		if err := Run(f, w, BuildMenus().Covert); err != nil {
			t.Fatalf("Run: %v", err)
		}
		return f.out.String()
	}

	// Both-on is TestBombEnemyTargetsSubmenuShowsItems; each case below asserts a
	// surviving item too, so it also proves the submenu really opened.
	out := open(func(w *ctx) { w.Config.BombingOps, w.Config.MissileOps = false, true })
	if strings.Contains(out, "Bomb Food Market") {
		t.Errorf("Bomb Food Market survived Bombing Ops = Disabled:\n%s", out)
	}
	if !strings.Contains(out, "Nuclear Assault") {
		t.Errorf("Bombing Ops = Disabled should leave the warheads alone:\n%s", out)
	}

	out = open(func(w *ctx) { w.Config.BombingOps, w.Config.MissileOps = true, false })
	if strings.Contains(out, "Nuclear Assault") {
		t.Errorf("Nuclear Assault survived Missile Ops = Disabled:\n%s", out)
	}
	if !strings.Contains(out, "Bomb Food Market") {
		t.Errorf("Missile Ops = Disabled should leave the bombing runs alone:\n%s", out)
	}
}

// With both switches off the Bomb Enemy Targets submenu would hold nothing, so
// the entry that opens it goes too.
func TestBombEnemyTargetsHiddenWhenNothingIsLeftInIt(t *testing.T) {
	w := newWorld()
	w.Config.BombingOps, w.Config.MissileOps = false, false
	covert := BuildMenus().Covert
	if covert.byKey('7', w) != nil {
		t.Error("Bomb Enemy Targets still opens with every operation in it disabled")
	}
	w.Config.BombingOps = true
	if covert.byKey('7', w) == nil {
		t.Error("Bomb Enemy Targets should return once bombing ops are back on")
	}
}
