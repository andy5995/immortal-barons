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

// runCovert opens the Covert Operations menu on a realm that actually holds
// agents. BRE closes that menu as soon as the last agent is gone (Menu.ExitWhen,
// from the loop guard at BRE.OVR 0x0179db), so a scripted covert run has to
// stock the realm or it never sees the menu at all.
func runCovert(t *testing.T, keys string, m *Menu) (*fakeSession, *ctx, error) {
	t.Helper()
	f := &fakeSession{keys: []rune(keys)}
	w := newWorld()
	w.Player().Agents = 5
	return f, w, Run(f, w, m)
}

// TestCovertMenuShowsBREItems checks the Covert Operations menu's layout
// (#73): BRE.OVR's order/labels (Send Spy, Stir Revolts, Set Up, Support
// Dissensions, Demoralize Forces, Spy on Relations, Bomb Enemy Targets,
// Bribery, Expose Enemy Ops, Visit Bank), and that the old IB-specific items
// it replaced are gone.
func TestCovertMenuShowsBREItems(t *testing.T) {
	menus := BuildMenus()
	f, _, err := runCovert(t, "0", menus.Covert) // Quit immediately
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

// Bomb Enemy Targets is ONE flat covert op in BRE, not a submenu: its resolver
// rolls Random(6) and dispatches on the die through six branches of its own
// (run_ai_covert_operations 0x04C3B4), reading no menu table at all. Pressing 7
// must therefore reach the target picker, never a box of lettered variants.
func TestBombEnemyTargetsIsNotASubmenu(t *testing.T) {
	menus := BuildMenus()
	f, _, err := runCovert(t, "7\r0", menus.Covert)
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
	f, _, err := runCovert(t, "0", menus.Covert)
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

// TestCovertMenuClosesWhenTheLastAgentIsSpent: BRE re-reads the agent count at
// the head of the covert menu's own loop (enter_covert_operations_menu, BRE.OVR
// 0x0179db) and returns when it reaches zero, so the operation that spends the
// last agent ends the menu and the turn moves on. IB checked the count only on
// the way in, and went on drawing a menu that could do nothing but refuse.
func TestCovertMenuClosesWhenTheLastAgentIsSpent(t *testing.T) {
	menus := BuildMenus()
	// Two DIFFERENT operations: repeating one would be refused by BRE's
	// once-per-turn cap, and the test would pass without the menu ever closing.
	f := &fakeSession{keys: []rune("4A 5A ")} // Support Dissensions, then Demoralize Forces
	w, _ := covertWorld()
	p := w.Player()
	p.Agents, p.Gold = 1, 1_000_000_000
	if err := Run(f, w, menus.Covert); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := stripANSI(f.out.String())
	// It reached the operation: the agent went out and was spent.
	if !strings.Contains(out, "has set out for") {
		t.Fatalf("the first operation never ran:\n%s", out)
	}
	if p.Agents != 0 {
		t.Fatalf("the operation should have spent the agent, %d left", p.Agents)
	}
	// And the menu closed rather than being drawn a second time: the second
	// keypress never reached an item, so only one operation was queued.
	// Counted on the title rule, not on an item label: the chosen item's label
	// is echoed after the prompt as well, so a label appears twice in one draw.
	if drawn := strings.Count(out, "[Covert Operations]"); drawn != 1 {
		t.Errorf("the covert menu was drawn again after the last agent went (%d listings):\n%s", drawn, out)
	}
	if len(w.CovertQueue) != 1 {
		t.Errorf("queued %d operations, want 1", len(w.CovertQueue))
	}
}

// A realm with agents still gets the menu back after an operation — BRE returns
// to the covert menu after each one and leaves only on "0", so the exit above
// must not fire on any successful operation.
func TestCovertMenuStaysOpenWhileAgentsRemain(t *testing.T) {
	menus := BuildMenus()
	f := &fakeSession{keys: []rune("4A 5A 0")}
	w, _ := covertWorld()
	p := w.Player()
	p.Agents, p.Gold = 5, 1_000_000_000
	if err := Run(f, w, menus.Covert); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(w.CovertQueue) != 2 {
		t.Errorf("queued %d operations, want 2 — the menu did not come back", len(w.CovertQueue))
	}
}
