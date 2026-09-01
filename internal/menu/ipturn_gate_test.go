package menu

import (
	"strings"
	"testing"
)

// refusalMarker is a distinctive fragment of the turn gate's refusal, short
// enough to survive the render's wrapping.
const refusalMarker = "at least one turn each entry"

// The original refuses five of the InterPlanetary menu's items until the caller
// has begun a turn this entry, and lets the rest through. Four of the five call
// enforce_interbbs_turn_requirement from run_interbbs_menu (BRE.OVR 0x020caf,
// sites 0x021038 / 0x021051 / 0x021076 / 0x02109b) and Join Group Attack
// carries the same refusal inline (0x02d1c5). Terrorist Ops being exempt is the
// half of this that play on a real board confirms.
func TestInterPlanetaryTurnGateCoversTheRightItems(t *testing.T) {
	gated := map[rune]string{
		'3': "Send Trade Deal",
		'4': "Create Group Attack",
		'5': "Join Group Attack",
		'6': "Indiv. Attack Force",
		'8': "Special Operations",
	}
	// The exempt items whose action only opens another menu, so invoking one
	// reads no keys and writes nothing of its own.
	exempt := map[rune]string{
		'2': "Terrorist Ops",
		'7': "Send Message",
		'V': "Visit Bank",
	}

	for key, label := range gated {
		w := newWorld()
		w.Config.IBBS = true
		w.turnPlayed = false
		it := BuildMenus().InterPlanetary.byKey(key, w)
		if it == nil {
			t.Fatalf("%q (%s) is not reachable on the InterPlanetary menu", key, label)
		}
		f := &fakeSession{}
		if got := it.Do(f, w); got != Stay {
			t.Errorf("%s returned %v before a turn was played, want Stay", label, got)
		}
		if !strings.Contains(stripANSI(f.out.String()), refusalMarker) {
			t.Errorf("%s did not refuse before a turn was played; output:\n%s", label, f.out.String())
		}
	}

	for key, label := range exempt {
		w := newWorld()
		w.Config.IBBS = true
		w.turnPlayed = false
		it := BuildMenus().InterPlanetary.byKey(key, w)
		if it == nil {
			t.Fatalf("%q (%s) is not reachable on the InterPlanetary menu", key, label)
		}
		f := &fakeSession{}
		it.Do(f, w)
		if strings.Contains(stripANSI(f.out.String()), refusalMarker) {
			t.Errorf("%s must not be gated on a turn played; output:\n%s", label, f.out.String())
		}
	}
}

// The gate opens for the rest of the session once a turn has been played, and
// stays shut for a caller who walks in from the opening menu and acts without
// playing. Scripted through the real menu so a re-mapped key or a lost wrapper
// shows up here.
func TestInterPlanetaryTurnGateOpensOnceATurnIsPlayed(t *testing.T) {
	w := newWorld()
	w.Config.IBBS = true
	p := w.Player()
	p.Protection = 0 // otherwise the attack's own protection refusal comes first
	p.Prefs.AutoPayMaint = true
	p.Prefs.VisitCovert, p.Prefs.VisitTrading, p.Prefs.VisitMessage = false, false, false

	// '4' Create Group Attack, then quit the menu.
	before := &fakeSession{keys: []rune("4 0\r")}
	if err := Run(before, w, BuildMenus().InterPlanetary); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := stripANSI(before.out.String())
	// Assert the script actually reached the InterPlanetary menu, not some
	// screen upstream of it: this label appears nowhere else.
	if !strings.Contains(out, "Indiv. Attack Force") {
		t.Fatalf("script never reached the InterPlanetary menu:\n%s", out)
	}
	if !strings.Contains(out, refusalMarker) {
		t.Errorf("Create Group Attack was not refused before a turn was played:\n%s", out)
	}
	if strings.Contains(out, "No other planets are known yet") {
		t.Errorf("Create Group Attack ran anyway before a turn was played:\n%s", out)
	}

	// One turn, then the same item goes through.
	turn := &fakeSession{keys: []rune("0\r   0000nn")}
	runTurn(turn, w)
	if got := w.Player().TurnsPlayed; got != 1 {
		t.Fatalf("TurnsPlayed = %d after the scripted turn, want 1; turn output:\n%s", got, turn.out.String())
	}
	if !w.turnPlayed {
		t.Fatal("the session's turn-played flag is still clear after a turn")
	}

	after := &fakeSession{keys: []rune("4 0\r")}
	if err := Run(after, w, BuildMenus().InterPlanetary); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out = stripANSI(after.out.String())
	if !strings.Contains(out, "Indiv. Attack Force") {
		t.Fatalf("script never reached the InterPlanetary menu:\n%s", out)
	}
	if strings.Contains(out, refusalMarker) {
		t.Errorf("Create Group Attack still refused after a turn was played:\n%s", out)
	}
	if !strings.Contains(out, "No other planets are known yet") {
		t.Errorf("Create Group Attack did not run after a turn was played:\n%s", out)
	}
}
