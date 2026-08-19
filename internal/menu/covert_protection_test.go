package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// shelteredCovertWorld is a world where the PLAYER is still under New Realm
// Protection and the sole rival is not — the arrangement the original's covert
// menu splits on, since the caller's own shield is what it tests.
func shelteredCovertWorld(t *testing.T) (*ctx, *game.Empire) {
	t.Helper()
	w := newWorld()
	p := w.Player()
	if p.Protection <= 0 {
		t.Fatal("newWorld() should seed the player under New Realm Protection")
	}
	p.Gold, p.Agents = 1_000_000_000, 50
	target := recipients(w)[0]
	target.Protection = 0 // selectable, so nothing but the caller's shield can refuse
	return w, target
}

// TestCovertEffectOpRefusedUnderProtection: driving the real Covert Operations
// menu to Demoralize Forces (5) while sheltered is refused, and the refusal
// costs nothing — no gold, no agent, no queued operation. The original tests
// the caller's protection at the menu, before the affordability gate
// (BRE.OVR 0x017716).
func TestCovertEffectOpRefusedUnderProtection(t *testing.T) {
	w, _ := shelteredCovertWorld(t)
	p := w.Player()
	goldBefore, agentsBefore := p.Gold, p.Agents

	f := &fakeSession{keys: []rune("5 0")} // Demoralize Forces, dismiss the refusal, Quit
	if err := Run(f, w, BuildMenus().Covert); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := f.out.String()

	if !strings.Contains(out, "New Realm Protection shelters you") {
		t.Fatalf("the Covert menu never refused on the caller's own protection, got:\n%s", out)
	}
	// The attack menus refuse for the opposite reason; the two must not be
	// confused for one another.
	if strings.Contains(out, "cannot attack") {
		t.Errorf("the covert refusal borrowed the attack wording, got:\n%s", out)
	}
	if p.Gold != goldBefore {
		t.Errorf("a refused operation charged %d gold", goldBefore-p.Gold)
	}
	if p.Agents != agentsBefore {
		t.Errorf("a refused operation spent %d agents", agentsBefore-p.Agents)
	}
	if len(w.CovertQueue) != 0 {
		t.Errorf("a refused operation queued %d operations", len(w.CovertQueue))
	}
	if p.TurnProgress.CovertOpsUsed[game.OpDemoralizeForces] {
		t.Error("a refused operation burned the per-turn slot")
	}
}

// TestExposeEnemyOpsRefusedUnderProtection covers the one effect op that does
// not share the target picker: it is menu digit 9, which the original does not
// exempt.
func TestExposeEnemyOpsRefusedUnderProtection(t *testing.T) {
	w, target := shelteredCovertWorld(t)
	p := w.Player()
	p.Bribed = append(p.Bribed, target.Name) // an agent is already on the payroll
	goldBefore := p.Gold

	f := &fakeSession{keys: []rune("9 0")}
	if err := Run(f, w, BuildMenus().Covert); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := f.out.String()

	if !strings.Contains(out, "New Realm Protection shelters you") {
		t.Fatalf("Expose Enemy Ops never refused on the caller's own protection, got:\n%s", out)
	}
	if p.Gold != goldBefore {
		t.Errorf("a refused operation charged %d gold", goldBefore-p.Gold)
	}
	if len(p.ExposedFrom) != 0 {
		t.Errorf("a refused operation raised a shield: %v", p.ExposedFrom)
	}
}

// TestCovertInfoOpsRunUnderProtection: menu digits 1 and 6 gather information
// and nothing else, and the original jumps its protection test for exactly
// those two. Both must still reach their target picker and charge their fee
// while the caller is sheltered.
func TestCovertInfoOpsRunUnderProtection(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
		fee  int64
	}{
		{"Send Spy", "1", 5_000},           // BRE-verified fee, asserted as a literal
		{"Spy on Relations", "6", 100_000}, // IB charges the advertised fee (a recorded divergence)
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, target := shelteredCovertWorld(t)
			p := w.Player()
			goldBefore := p.Gold

			f := &fakeSession{keys: []rune(tc.key + "A  0")} // op, target A, two pauses, Quit
			if err := Run(f, w, BuildMenus().Covert); err != nil {
				t.Fatalf("Run: %v", err)
			}
			out := f.out.String()

			if strings.Contains(out, "New Realm Protection shelters you") {
				t.Fatalf("%s was refused while sheltered, but the original allows it:\n%s", tc.name, out)
			}
			// The picker is unique to the op's own screen: reaching it proves the
			// keypress dispatched and got past the gate.
			if !strings.Contains(out, "Choose a target") {
				t.Fatalf("%s never reached its target picker, got:\n%s", tc.name, out)
			}
			if !strings.Contains(out, target.Name) {
				t.Errorf("%s never named the rival %q, got:\n%s", tc.name, target.Name, out)
			}
			if got := goldBefore - p.Gold; got != tc.fee {
				t.Errorf("%s charged %d gold, want %d", tc.name, got, tc.fee)
			}
		})
	}
}

// TestSendSpyGuyRefusedUnderProtection: the interplanetary Special Operations
// menu is gated as a whole on the caller's own protection in the original, with
// no exemption for any item inside it. Send SpyGuy is IB's only item on that
// menu that does not run through ipSpecialOp, and it was the one left open.
func TestSendSpyGuyRefusedUnderProtection(t *testing.T) {
	w, _ := shelteredCovertWorld(t)
	p := w.Player()
	agentsBefore := p.Agents

	f := &fakeSession{keys: []rune(" ")} // the pause key after the refusal
	if res := sendSpyGuy(f, w); res != Stay {
		t.Fatalf("sendSpyGuy = %v, want Stay", res)
	}
	out := f.out.String()

	if !strings.Contains(out, "New Realm Protection shelters you") {
		t.Fatalf("Send SpyGuy never refused on the caller's own protection, got:\n%s", out)
	}
	// Without the gate the op walks straight into its price quote and picker.
	if strings.Contains(out, "gold per day") {
		t.Errorf("Send SpyGuy reached its target picker while sheltered:\n%s", out)
	}
	if p.Agents != agentsBefore {
		t.Errorf("a refused operation spent %d agents", agentsBefore-p.Agents)
	}
}
