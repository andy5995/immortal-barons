package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The covert handlers all funnel through localAttack / bombingAttack, passing
// the actual operation as a strike callback. Testing those two shared helpers
// with a recording stub covers the target-selection, protection, cost, and
// bomber-gate branches for the whole covert file without depending on any one
// operation's RNG. A couple of real delegators then confirm the wiring.

// recordingStrike returns a strike callback that notes whether it ran and with
// which empires, standing in for a real covert operation.
func recordingStrike(called *bool, gotA, gotD **game.Empire) func(a, d *game.Empire) (string, error) {
	return func(a, d *game.Empire) (string, error) {
		*called = true
		*gotA, *gotD = a, d
		return "STRIKE-REPORT", nil
	}
}

// covertWorld builds a world where the player may attack: both the player and
// the sole rival have their new-realm protection cleared.
func covertWorld() (*ctx, *game.Empire) {
	w := newWorld()
	w.Player().Protection = 0
	target := recipients(w)[0]
	target.Protection = 0
	return w, target
}

// --- localAttack (shared by every covert op) --------------------------

func TestSpecialAttackBlockedByProtection(t *testing.T) {
	w := newWorld() // player keeps the default Protection = 20
	called := false
	var a, d *game.Empire
	f := &fakeSession{}

	localAttack(f, w, "Send Spy", nil, false, recordingStrike(&called, &a, &d))

	if called {
		t.Error("strike ran while under New Realm Protection")
	}
	if !strings.Contains(f.out.String(), "New Realm Protection") {
		t.Errorf("expected protection notice; got:\n%s", f.out.String())
	}
}

func TestSpecialAttackNoTargets(t *testing.T) {
	w := newWorld()
	w.Player().Protection = 0 // rival is still protected, so there are no valid targets
	called := false
	var a, d *game.Empire
	f := &fakeSession{}

	localAttack(f, w, "Send Spy", nil, false, recordingStrike(&called, &a, &d))

	if called {
		t.Error("strike ran with no valid targets")
	}
	// A living-but-protected rival is listed but unselectable, not "no rivals
	// left" — the notice explains the survivors are shielded, so the player isn't
	// misled into thinking the world is empty.
	if !strings.Contains(f.out.String(), "protected or allied") {
		t.Errorf("expected the protected-rivals notice; got:\n%s", f.out.String())
	}
}

func TestSpecialAttackCancel(t *testing.T) {
	w, _ := covertWorld()
	called := false
	var a, d *game.Empire
	f := &fakeSession{keys: []rune("0\r")}

	localAttack(f, w, "Send Spy", nil, false, recordingStrike(&called, &a, &d))

	if called {
		t.Error("strike ran after the player cancelled")
	}
}

func TestSpecialAttackInvalidIndex(t *testing.T) {
	w, _ := covertWorld()
	called := false
	var a, d *game.Empire
	f := &fakeSession{keys: []rune("9\r")} // only one target exists

	localAttack(f, w, "Send Spy", nil, false, recordingStrike(&called, &a, &d))

	if called {
		t.Error("strike ran for an out-of-range target index")
	}
}

func TestSpecialAttackSuccess(t *testing.T) {
	w, target := covertWorld()
	called := false
	var a, d *game.Empire
	f := &fakeSession{keys: []rune("A")} // target A

	localAttack(f, w, "Send Spy", nil, false, recordingStrike(&called, &a, &d))

	if !called {
		t.Fatal("strike did not run for a valid target")
	}
	if a != w.Player() || d != target {
		t.Errorf("strike called with (%v, %v), want (player, %v)", a, d, target)
	}
	if !strings.Contains(f.out.String(), "STRIKE-REPORT") {
		t.Errorf("strike report not shown; got:\n%s", f.out.String())
	}
}

func TestSpecialAttackShowsCost(t *testing.T) {
	w, _ := covertWorld()
	called := false
	var a, d *game.Empire
	// The price is quoted only once a target is named, so pick A and then decline
	// the sale — we are checking the quote, not the strike.
	f := &fakeSession{keys: []rune("An")}

	localAttack(f, w, "Nuclear Assault", func(targetRow) int64 { return 50_000 }, false, recordingStrike(&called, &a, &d))

	if !strings.Contains(f.out.String(), "50,000") {
		t.Errorf("expected the gold cost in the quote; got:\n%s", f.out.String())
	}
	if called {
		t.Error("strike ran after the sale was declined")
	}
}

func TestSpecialAttackPricesOffTarget(t *testing.T) {
	w, target := covertWorld()
	called := false
	var a, d *game.Empire
	f := &fakeSession{keys: []rune("An")}

	localAttack(f, w, "Nuclear Assault", nukePrice, false, recordingStrike(&called, &a, &d))

	want := comma(game.NukeCostForLand(target.Land))
	if !strings.Contains(f.out.String(), want) {
		t.Errorf("expected the target-sized price %s; got:\n%s", want, f.out.String())
	}
}

// --- bombingAttack (the 500-Bomber gate) --------------------------------

func TestBombingAttackRequiresBombers(t *testing.T) {
	w, _ := covertWorld()
	w.Player().Bombers = game.BombingBombersRequired - 1
	called := false
	var a, d *game.Empire
	f := &fakeSession{keys: []rune("1\r")}

	bombingAttack(f, w, "Bomb Food Market", nil, recordingStrike(&called, &a, &d))

	if called {
		t.Error("strike ran without enough Bombers")
	}
	if !strings.Contains(f.out.String(), "Bombers") {
		t.Errorf("expected the Bombers requirement notice; got:\n%s", f.out.String())
	}
}

func TestBombingAttackProceedsWithBombers(t *testing.T) {
	w, _ := covertWorld()
	w.Player().Bombers = game.BombingBombersRequired
	called := false
	var a, d *game.Empire
	f := &fakeSession{keys: []rune("A")} // target A

	bombingAttack(f, w, "Bomb Food Market", nil, recordingStrike(&called, &a, &d))

	if !called {
		t.Error("strike did not run with enough Bombers")
	}
}

// --- slappenheimerStrike (mode branches) --------------------------------

func TestSlappenheimerDisabled(t *testing.T) {
	w, _ := covertWorld()
	w.Config.SlappenheimerHandling = game.SlappenheimerNone
	f := &fakeSession{}

	slappenheimerStrike(f, w)

	if !strings.Contains(f.out.String(), "disabled") {
		t.Errorf("expected the disabled notice; got:\n%s", f.out.String())
	}
}

func TestSlappenheimerUserSelectPromptsDial(t *testing.T) {
	w, _ := covertWorld()
	w.Config.SlappenheimerHandling = game.SlappenheimerUserSelect
	w.Player().Bombers = game.BombingBombersRequired
	w.Player().Agents = 100
	f := &fakeSession{keys: []rune("5\r1\r")} // dial 5, then attack target 1

	slappenheimerStrike(f, w)

	if !strings.Contains(f.out.String(), "dial") {
		t.Errorf("expected the R5-Slappenheimer dial prompt; got:\n%s", f.out.String())
	}
}

// --- real delegators (wiring check) -------------------------------------

// The wiring claim needs the op's own report on screen, not just "some
// output": these tests once fed "1\r" to a prompt that wants a LETTER, so
// they aborted at target selection, never invoked the op, and passed on the
// target table alone.
func TestSendSpyDelegates(t *testing.T) {
	w, _ := covertWorld()
	w.Player().Agents = 100
	f := &fakeSession{keys: []rune("a")} // target A

	if got := sendSpy(f, w); got != Stay {
		t.Errorf("sendSpy returned %v, want Stay", got)
	}
	if out := f.out.String(); !strings.Contains(out, "Intel on Gale Horde") {
		t.Errorf("sendSpy should deliver the spy's intel report, got:\n%s", out)
	}
}

func TestBombFoodMarketDelegates(t *testing.T) {
	w, _ := covertWorld()
	w.Player().Agents = 100
	w.Player().Gold = 10_000_000 // the op has a gold cost; broke = "cannot afford", op never runs
	w.Player().Bombers = game.BombingBombersRequired
	f := &fakeSession{keys: []rune("a")}

	if got := bombFoodMarket(f, w); got != Stay {
		t.Errorf("bombFoodMarket returned %v, want Stay", got)
	}
	if out := f.out.String(); !strings.Contains(out, "food") {
		t.Errorf("bombFoodMarket should report the food strike outcome, got:\n%s", out)
	}
}
