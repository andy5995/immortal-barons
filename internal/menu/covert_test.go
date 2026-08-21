package menu

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// runCovertAction drives a covert menu action against target A and returns what
// the screen showed. An EFFECT operation no longer resolves at the menu — it is
// queued for daily maintenance — so there is nothing random to retry here: the
// acknowledgement is the screen under test and the queue is the state effect.
// What a queued operation then DOES is asserted in the game package, against the
// resolver that does it.
func runCovertAction(t *testing.T, w *ctx, p *game.Empire, action func(session.Session, *ctx) Result) string {
	t.Helper()
	p.TurnProgress = game.TurnProgress{}
	p.Gold, p.Agents = 1_000_000_000, 50
	f := &fakeSession{keys: []rune("A ")} // pick target A, then the pause key
	if res := action(f, w); res != Stay {
		t.Fatalf("covert action returned %v, want Stay", res)
	}
	out := f.out.String()
	if !strings.Contains(out, "has set out for") {
		t.Fatalf("the action never reached its acknowledgement screen, got:\n%s", out)
	}
	return out
}

// queuedOp is the single covert record the world is holding, or a fatal error.
func queuedOp(t *testing.T, w *ctx) game.QueuedCovertOp {
	t.Helper()
	if len(w.CovertQueue) != 1 {
		t.Fatalf("expected exactly one queued covert operation, got %d", len(w.CovertQueue))
	}
	return w.CovertQueue[0]
}

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

// Bomb Enemy Targets is ONE flat covert op in BRE, not a submenu: the eight-item
// bombing table is read by the interplanetary Special Operations menu alone
// (BRE.OVR 0x029EA9, whose only caller is the InterBBS menu). Pressing 7 must
// therefore reach the target picker, never a box of lettered variants.
func TestBombEnemyTargetsIsNotASubmenu(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "7\r0", menus.Covert)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	out := f.out.String()
	// Proof the key was dispatched to the op rather than swallowed: the action
	// reaches localAttack, which refuses under New Realm Protection.
	if !strings.Contains(out, "New Realm Protection") {
		t.Fatalf("pressing 7 never reached the Bomb Enemy Targets action; output:\n%s", out)
	}
	for _, gone := range []string{
		"Bomb Food Market",
		"Bomb Trading Market",
		"Bomb Trade Routes",
		"Undermine Investments",
		"Nuclear Assault",
		"Chemical Bombing",
		"R5-Slappenheimer",
	} {
		if strings.Contains(out, gone) {
			t.Errorf("the local Covert menu still offers the interplanetary item %q; output:\n%s", gone, out)
		}
	}
}

// TestDemoralizeForcesQueuesAgainstTheChosenTarget drives the "Demoralize
// Forces" action directly: picking the lone AI target sends an agent, and the
// target's morale does not move until the queue is drained at maintenance.
func TestDemoralizeForcesQueuesAgainstTheChosenTarget(t *testing.T) {
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
	target.Agents = 0
	target.Morale = 100

	out := runCovertAction(t, w, p, demoralizeForces)
	if !strings.Contains(out, target.Name) {
		t.Errorf("the acknowledgement should name the chosen target %q: %s", target.Name, out)
	}
	rec := queuedOp(t, w)
	if rec.Op != game.OpDemoralizeForces || rec.Target != target.Name || rec.Attacker != p.Name {
		t.Errorf("queued %+v, want a Demoralize Forces from %s against %s", rec, p.Name, target.Name)
	}
	if target.Morale != 100 {
		t.Errorf("the agent has not arrived yet, so morale should still be 100, got %d", target.Morale)
	}
}

// BRE's local Bomb Enemy Targets has no Bombers requirement — the 500-Bomber
// gate lives in the interplanetary bombing menu (BRE.OVR 0x02AEBE) and nowhere
// else. The op runs with an empty airfield and takes a slice of one holding.
func TestBombEnemyTargetsNeedsNoBombers(t *testing.T) {
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
	p.Bombers = 0
	target.Agents = 0
	target.People, target.Troopers = 100_000, 100_000
	target.Tanks, target.Jets, target.Food = 100_000, 100_000, 100_000

	out := runCovertAction(t, w, p, bombEnemyTargets)
	if !strings.Contains(out, target.Name) {
		t.Errorf("the acknowledgement should name the chosen target %q: %s", target.Name, out)
	}
	if rec := queuedOp(t, w); rec.Op != game.OpBombEnemyTargets || rec.Target != target.Name {
		t.Errorf("queued %+v, want a Bomb Enemy Targets against %s", rec, target.Name)
	}
}

// Expose Enemy Ops picks ONE realm, and the only realms it can pick are the ones
// the player already holds a bribed agent inside (BRE.OVR 0x01701B lists exactly
// those). With no bribed agent anywhere the action refuses without a picker; with
// one, choosing that realm records a shield against it and nobody else.
func TestExposeEnemyOpsListsOnlyBribedRealms(t *testing.T) {
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
	p.Gold, p.Agents = 1_000_000_000, 50
	p.Protection = 0 // past New Realm Protection, which bars the effect ops

	f := &fakeSession{keys: []rune(" ")} // the pause key after the refusal
	if res := exposeEnemyOps(f, w); res != Stay {
		t.Fatalf("exposeEnemyOps = %v, want Stay", res)
	}
	if out := f.out.String(); !strings.Contains(out, "no bribed agents") {
		t.Fatalf("with nothing bribed the action should refuse, got:\n%s", out)
	}

	p.Bribed = append(p.Bribed, target.Name)
	goldBefore := p.Gold
	f = &fakeSession{keys: []rune("A ")} // pick the one listed realm, then pause
	if res := exposeEnemyOps(f, w); res != Stay {
		t.Fatalf("exposeEnemyOps = %v, want Stay", res)
	}
	out := f.out.String()
	if !strings.Contains(out, "expose their operations") {
		t.Fatalf("the action never reached its report screen, got:\n%s", out)
	}
	if !strings.Contains(out, target.Name) {
		t.Errorf("the report should name the shielded realm %q, got:\n%s", target.Name, out)
	}
	if _, ok := p.ExposedFrom[target.Name]; !ok {
		t.Errorf("no shield was recorded against %s: %v", target.Name, p.ExposedFrom)
	}
	if len(p.ExposedFrom) != 1 {
		t.Errorf("the shield should cover one realm, got %v", p.ExposedFrom)
	}
	if p.Gold != goldBefore-game.CostExposeEnemyOps {
		t.Errorf("gold %d, want %d", p.Gold, goldBefore-game.CostExposeEnemyOps)
	}
	if p.Agents != 50 {
		t.Errorf("Expose Enemy Ops should spend no agent, got %d agents", p.Agents)
	}
}

// "Support Dissensions" is 19 characters and the Item column was a fixed 18, so
// that one row's price sat a place right of every other row's. Every listed
// price must end in the same column.
func TestCovertMenuPricesAlign(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "0", menus.Covert)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	esc := regexp.MustCompile("\x1b\\[[0-9;]*m")
	end := -1
	for _, line := range strings.Split(esc.ReplaceAllString(f.out.String(), ""), "\n") {
		if !strings.HasSuffix(line, "000") {
			continue
		}
		n := utf8.RuneCountInString(line)
		if end < 0 {
			end = n
		} else if n != end {
			t.Errorf("price column ragged: %q ends at %d, want %d", line, n, end)
		}
	}
	if end < 0 {
		t.Fatalf("no price rows found; output:\n%s", f.out.String())
	}
}
