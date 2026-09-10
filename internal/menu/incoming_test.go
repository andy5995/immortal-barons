package menu

import (
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The list draws one row per threat, soonest first, in the short duration form
// the Join Group Attack table uses (#268, #269).
func TestIncomingListsWhatIsAimedHere(t *testing.T) {
	w := newWorld()
	now := time.Now()
	w.With(func() {
		w.World.Threats = []game.Threat{
			{FromBoard: "Nova Hub", Kind: game.ThreatAttack, ID: 7,
				Target: "Testrealm", At: game.ThreatAt(now.Add(90 * time.Minute))},
			{FromBoard: "The Eclipse", Kind: game.ThreatGooie,
				At: game.ThreatAt(now.Add(2 * time.Hour))},
			{FromBoard: "Wildside", Kind: game.ThreatAttack, ID: 3,
				At: game.ThreatAt(now.Add(-time.Minute))},
			{FromBoard: "Far Reach", Kind: game.ThreatGooie}, // still being paid for
		}
	})
	f := &fakeSession{keys: []rune("\r")}
	showIncoming(f, w)

	out := stripANSI(f.out.String())
	for _, want := range []string{
		"Incoming",
		"Wildside", "away", // its hour has come; it stays on the list for a day
		"Nova Hub", "Testrealm", "90m",
		"The Eclipse", "Gooie Kablooie", "2h",
		"Far Reach", "?", // no launch date named yet
		"ALL", // a party aimed at the whole planet
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	// Soonest first, and a threat with no hour named sorts last.
	order := []string{"Wildside", "Nova Hub", "The Eclipse", "Far Reach"}
	at := -1
	for _, name := range order {
		i := strings.Index(out, name)
		if i < at {
			t.Errorf("%s is out of order:\n%s", name, out)
		}
		at = i
	}
	for _, line := range strings.Split(out, "\n") {
		if len([]rune(line)) > 78 {
			t.Errorf("line runs to %d columns:\n%q", len([]rune(line)), line)
		}
	}
}

// A weapon already in the air is listed once, from the board that launched it,
// with its arrival — not twice, beside the record of it being built.
func TestIncomingListsAFlyingWeaponOnce(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.World.GameDay = 10
		w.World.Incoming = &game.Annihilator{
			Creator: "The Eclipse", Launched: true, ArrivesDay: 12, Intact: 100,
		}
		w.World.Threats = []game.Threat{
			{FromBoard: "The Eclipse", Kind: game.ThreatGooie,
				At: game.ThreatAt(time.Now().Add(-time.Hour))},
		}
	})
	f := &fakeSession{keys: []rune("\r")}
	showIncoming(f, w)

	out := stripANSI(f.out.String())
	if n := strings.Count(out, "Gooie Kablooie"); n != 1 {
		t.Errorf("the weapon is listed %d times, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, "2d") {
		t.Errorf("the flight is not counted in days:\n%s", out)
	}
}

// An empty list says so and nothing more: what an empty list means is the
// SpyGuy topic's business, not a line repeated on every draw of this screen.
func TestIncomingEmptySaysSo(t *testing.T) {
	w := newWorld()
	f := &fakeSession{keys: []rune("\r")}
	showIncoming(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Nothing is aimed at this planet") {
		t.Fatalf("no empty-list line:\n%s", out)
	}
	if strings.Contains(out, "SpyGuy") {
		t.Errorf("the screen should not explain itself every time:\n%s", out)
	}
}
