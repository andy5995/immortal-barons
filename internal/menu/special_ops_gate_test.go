package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
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

// Bomb Enemy Targets is one terror-bombing op, so it answers to Bombing Ops
// alone; the missiles it used to sit beside are the Attack menu's and the
// interplanetary menu's.
func TestBombEnemyTargetsAnswersToBombingOps(t *testing.T) {
	w := newWorld()
	covert := BuildMenus().Covert

	w.Config.BombingOps, w.Config.MissileOps = false, true
	if covert.byKey('7', w) != nil {
		t.Error("Bomb Enemy Targets still opens with Bombing Ops = Disabled")
	}
	w.Config.BombingOps, w.Config.MissileOps = true, false
	if covert.byKey('7', w) == nil {
		t.Error("Bomb Enemy Targets should stand on Bombing Ops alone, not on Missile Ops")
	}
}

// The R5-Slappenheimer's handling mode gates the only menu that fires one. It
// moved with the missile when the local Bomb Enemy Targets submenu collapsed to
// BRE's single terror-bombing op, and a mode the editor stores but nothing reads
// is the failure this guards.
func TestSlappenheimerHandlingGatesTheInterplanetaryOp(t *testing.T) {
	w := newWorld()
	w.Player().Protection = 0
	w.Player().Bombers = game.BombingBombersRequired
	w.Config.SlappenheimerHandling = game.SlappenheimerNone

	f := &fakeSession{}
	if got := ipSpecialOp(game.OpSlappenheimer)(f, w); got != Stay {
		t.Fatalf("ipSpecialOp returned %v, want Stay", got)
	}
	if !strings.Contains(f.out.String(), "disabled") {
		t.Errorf("R5-Slappenheimer Handling = None did not stop the op; output:\n%s", f.out.String())
	}
}

// With Local Attacks off, BRE's Attack Menu collapses to the pirate and
// alliance entries — captured live (docs/dev/bre-screens.md, "Attack Menu
// (InterBBS, local attacks OFF)").
func TestLocalAttacksOffCollapsesTheAttackMenu(t *testing.T) {
	w := newWorld()
	w.Config.IBBS = true
	w.Config.LocalAttacks = false
	w.Player().Protection = 0
	attack := BuildMenus().Attack

	f := &fakeSession{keys: []rune("0\r ")}
	if err := Run(f, w, attack); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := f.out.String()
	for _, gone := range []string{"Regular Attack", "Nuclear Attack", "Chemical Attack", "Biological Attack"} {
		if strings.Contains(out, gone) {
			t.Errorf("%q survived Local Attacks = Disabled:\n%s", gone, out)
		}
	}
	for _, kept := range []string{"Attack Pirates", "Alliance Strength"} {
		if !strings.Contains(out, kept) {
			t.Errorf("%q should still be on the menu:\n%s", kept, out)
		}
	}
	if attack.byKey('R', w) != nil {
		t.Error("the Regular Attack hotkey still works with local attacks off")
	}

	// A stand-alone board keeps every entry whatever the switch says.
	w.Config.IBBS = false
	if attack.byKey('R', w) == nil {
		t.Error("a stand-alone board should always allow a Regular Attack")
	}
}
