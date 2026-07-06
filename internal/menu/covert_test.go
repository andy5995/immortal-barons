package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestCovertMenuShowsBREItems checks the Covert Operations menu's layout
// (#73): BRE.OVR's order/labels (Send Spy, Stir Revolts, Set Up, Support
// Dissensions, Demoralize Forces, Spy on Relations, Bomb Enemy Targets,
// Bribery, Expose Enemy Ops, Visit Bank), and that the old IB-specific items
// it replaced are gone.
func TestCovertMenuShowsBREItems(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "0", menus.Covert) // Quit immediately
	if err != nil {
		t.Fatalf("got %v", err)
	}
	out := f.out.String()
	for _, want := range []string{
		"Send Spy",
		"Stir Revolts",
		"Set Up",
		"Support Dissensions",
		"Demoralize Forces",
		"Spy on Relations",
		"Bomb Enemy Targets",
		"Bribery",
		"Expose Enemy Ops",
		"Visit Bank",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Covert menu missing item %q; output:\n%s", want, out)
		}
	}
	for _, gone := range []string{"Special Operations", "Bomb Intelligence", "Bomb Airbases", "Bomb HQ", "Bomb Food Stores"} {
		if strings.Contains(out, gone) {
			t.Errorf("Covert menu should no longer show the old item %q; output:\n%s", gone, out)
		}
	}
}

// TestBombEnemyTargetsSubmenuShowsItems drives into the Bomb Enemy Targets
// submenu (BRE.OVR order) and checks its item labels.
func TestBombEnemyTargetsSubmenuShowsItems(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "B00", menus.Covert) // enter submenu, quit submenu, quit Covert
	if err != nil {
		t.Fatalf("got %v", err)
	}
	out := f.out.String()
	for _, want := range []string{
		"Bomb Food Market",
		"Bomb Trading Market",
		"Bomb Trade Routes",
		"Undermine Investments",
		"Nuclear Assault",
		"Chemical Bombing",
		"R5-Slappenheimer",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Bomb Enemy Targets submenu missing item %q; output:\n%s", want, out)
		}
	}
}

// TestDemoralizeForcesAppliesEffect drives the "Demoralize Forces" action
// directly: pick the lone AI target, and it should apply DemoralizeForces's
// morale hit against them.
func TestDemoralizeForcesAppliesEffect(t *testing.T) {
	w := newWorld()
	p := w.Player()
	var target *game.Empire
	for _, e := range w.Empires {
		if e != p {
			target = e
			break
		}
	}
	if target == nil {
		t.Fatal("newWorld() should seed at least one AI empire")
	}
	p.Protection = 0
	target.Protection = 0
	p.Agents = 50
	target.Agents = 0
	target.Morale = 100

	f := &fakeSession{keys: []rune("1\r ")} // pick target 1, then the pause key
	if res := demoralizeForces(f, w); res != Stay {
		t.Fatalf("demoralizeForces = %v, want Stay", res)
	}
	if target.Morale != 85 {
		t.Errorf("expected target morale 85, got %d", target.Morale)
	}
}

// TestBombFoodMarketRequiresBombers checks BRE's Bomb Enemy Targets gate
// (BRE.OVR: "All missiles and bombs require 500 Bombers to deliver their
// payloads") — without enough Bombers the op should fail before even asking
// for a target.
func TestBombFoodMarketRequiresBombers(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Bombers = 0

	f := &fakeSession{} // no keys consumed if the Bombers gate fires first
	if res := bombFoodMarket(f, w); res != Stay {
		t.Fatalf("bombFoodMarket = %v, want Stay", res)
	}
	if !strings.Contains(f.out.String(), "500 Bombers") {
		t.Errorf("expected a 500-Bombers error, got:\n%s", f.out.String())
	}
}

// TestBombFoodMarketAppliesEffectWithBombers checks the Bombers gate passes
// with enough Bombers, and the underlying BombFood effect still applies.
func TestBombFoodMarketAppliesEffectWithBombers(t *testing.T) {
	w := newWorld()
	p := w.Player()
	var target *game.Empire
	for _, e := range w.Empires {
		if e != p {
			target = e
			break
		}
	}
	if target == nil {
		t.Fatal("newWorld() should seed at least one AI empire")
	}
	p.Protection = 0
	target.Protection = 0
	p.Agents = 50
	p.Bombers = game.BombingBombersRequired
	target.Agents = 0
	target.Food = 1000

	f := &fakeSession{keys: []rune("1\r ")}
	if res := bombFoodMarket(f, w); res != Stay {
		t.Fatalf("bombFoodMarket = %v, want Stay", res)
	}
	if target.Food != 500 {
		t.Errorf("expected 500 food after a 50%% strike, got %d", target.Food)
	}
}
