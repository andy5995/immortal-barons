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

// A Gooie in the air is listed only from a watcher's report, counting down to
// its landing; once past that hour it reads "now", and once landed it is off
// the list, which the jets' picker takes over.
func TestIncomingShowsAFlightOnlyFromAWatcher(t *testing.T) {
	now := time.Now()
	draw := func(arrives time.Time, daysLeft int, watched bool) string {
		w := newWorld()
		w.With(func() {
			w.World.Incoming = []*game.Annihilator{{
				Creator: "The Eclipse", Launched: true, Intact: 100,
				ArrivesAt: arrives, DaysLeft: daysLeft,
			}}
			if watched {
				w.World.Threats = []game.Threat{{FromBoard: "The Eclipse",
					Kind: game.ThreatGooie, At: game.ThreatAt(arrives)}}
			}
		})
		f := &fakeSession{keys: []rune("\r")}
		showIncoming(f, w)
		return stripANSI(f.out.String())
	}
	if out := draw(now.Add(26*time.Hour+30*time.Minute), 0, false); strings.Contains(out, "The Eclipse") {
		t.Errorf("an unwatched flight is listed:\n%s", out)
	}
	if out := draw(now.Add(26*time.Hour+30*time.Minute), 0, true); !strings.Contains(out, "27h") {
		t.Errorf("the flight is not counted down to its landing:\n%s", out)
	}
	if row := incomingRow(draw(now.Add(-time.Hour), 0, true), "The Eclipse"); !strings.HasSuffix(row, " now") {
		t.Errorf("a flight past its hour reads %q, want it to end in now", row)
	}
	if out := draw(now.Add(-time.Hour), game.AnnihilatorSiegeDays, true); strings.Contains(out, "The Eclipse") {
		t.Errorf("a landed weapon is still listed as incoming:\n%s", out)
	}
}

// A builder can start its next weapon while its last is still besieging us.
// The landed one is off the list; a report of the new one is not.
func TestIncomingShowsTheNextGooieDuringASiege(t *testing.T) {
	w := newWorld()
	now := time.Now()
	w.With(func() {
		w.World.Incoming = []*game.Annihilator{{
			Creator: "The Eclipse", Launched: true, Intact: 100,
			ArrivesAt: now.Add(-time.Hour), DaysLeft: game.AnnihilatorSiegeDays,
		}}
		w.World.Threats = []game.Threat{{FromBoard: "The Eclipse", Kind: game.ThreatGooie}}
	})
	f := &fakeSession{keys: []rune("\r")}
	showIncoming(f, w)
	if row := incomingRow(stripANSI(f.out.String()), "The Eclipse"); !strings.HasSuffix(row, " ?") {
		t.Errorf("the next weapon's row reads %q, want it listed with no hour yet", row)
	}
}

// incomingRow is the line of the Incoming table naming planet, trimmed.
func incomingRow(out, planet string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, planet) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}
