package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// directivesWorld is an inter-BBS game where the test player either holds the
// Coordinator's office or has voted another human into it.
func directivesWorld(coordinator bool) *ctx {
	w := newWorld()
	w.Config.IBBS = true
	w.Player().CoordinatorVote = "tester"
	if !coordinator {
		w.AddHuman("other", "Otherland")
		w.Player().CoordinatorVote = "other"
		w.FindByOwner("other").CoordinatorVote = "other"
	}
	return w
}

func TestCoordinatorWritesDirectives(t *testing.T) {
	f := &fakeSession{keys: []rune("wHold the line.\r/s ")}
	w := directivesWorld(true)
	directivesMenu(f, w)
	if !strings.Contains(f.out.String(), "posted") {
		t.Fatalf("never reached the save:\n%s", f.out.String())
	}
	d := w.Directives
	if d == nil || d.Body != "Hold the line." || d.From != "Testland" {
		t.Fatalf("directives = %+v, want Testland's \"Hold the line.\"", d)
	}
	if _, ok := game.ParseStamp(d.When); !ok {
		t.Errorf("directives stamp %q does not parse", d.When)
	}
}

func TestCoordinatorClearsDirectivesWithAnEmptySave(t *testing.T) {
	f := &fakeSession{keys: []rune("w/s ")}
	w := directivesWorld(true)
	w.Directives = &game.Message{From: "Testland", Body: "old"}
	directivesMenu(f, w)
	if !strings.Contains(f.out.String(), "erased") {
		t.Fatalf("never reached the save:\n%s", f.out.String())
	}
	if w.Directives != nil {
		t.Errorf("an empty save should clear the directives; got %+v", w.Directives)
	}
}

func TestCoordinatorAppendsToDirectives(t *testing.T) {
	f := &fakeSession{keys: []rune("aSecond line.\r/s ")}
	w := directivesWorld(true)
	w.Directives = &game.Message{From: "Testland", Body: "First line."}
	directivesMenu(f, w)
	if !strings.Contains(f.out.String(), "posted") {
		t.Fatalf("never reached the save:\n%s", f.out.String())
	}
	if d := w.Directives; d == nil || d.Body != "First line.\nSecond line." {
		t.Fatalf("directives = %+v, want both lines", d)
	}
}

// A full set leaves Append nothing to add: it must say so rather than open an
// editor with no free line and repost the same text under a new stamp.
func TestAppendToFullDirectivesIsRefused(t *testing.T) {
	f := &fakeSession{keys: []rune("a ")}
	w := directivesWorld(true)
	full := strings.TrimSuffix(strings.Repeat("line\n", msgMaxLines), "\n")
	w.Directives = &game.Message{From: "Testland", When: "old stamp", Body: full}
	directivesMenu(f, w)
	if !strings.Contains(f.out.String(), "already fill") {
		t.Fatalf("Append on a full set was not refused:\n%s", f.out.String())
	}
	if w.Directives.When != "old stamp" {
		t.Errorf("a refused Append reposted the directives; stamp = %q", w.Directives.When)
	}
}

func TestCoordinatorErasesDirectives(t *testing.T) {
	for _, tc := range []struct {
		keys string
		gone bool
	}{{"ey ", true}, {"en", false}} {
		f := &fakeSession{keys: []rune(tc.keys)}
		w := directivesWorld(true)
		w.Directives = &game.Message{From: "Testland", Body: "old"}
		directivesMenu(f, w)
		if !strings.Contains(f.out.String(), "Erase the current directives?") {
			t.Fatalf("%q: never reached the confirmation:\n%s", tc.keys, f.out.String())
		}
		if gone := w.Directives == nil; gone != tc.gone {
			t.Errorf("%q: erased = %v, want %v", tc.keys, gone, tc.gone)
		}
	}
}

// A player who is not the Coordinator is shown the directives, with their
// stamp, and is never offered Write or Erase.
func TestNonCoordinatorOnlyReadsDirectives(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")}
	w := directivesWorld(false)
	w.Directives = &game.Message{From: "Otherland", When: game.StoredStamp(game.Now()), Body: "Hold the line."}
	directivesMenu(f, w)
	out := f.out.String()
	if !strings.Contains(out, "Hold the line.") {
		t.Fatalf("directives not shown:\n%s", out)
	}
	if !strings.Contains(out, " UTC") {
		t.Errorf("directives shown without their stamp:\n%s", out)
	}
	if strings.Contains(out, "Write") || strings.Contains(out, "Erase") {
		t.Errorf("a non-Coordinator was offered Write:\n%s", out)
	}
}

// Every choice of Play shows them, whether turns remain or not.
func TestPlayShowsDirectives(t *testing.T) {
	for _, turns := range []int{0, 5} {
		f := &fakeSession{keys: []rune(" ")}
		w := directivesWorld(false)
		w.Player().TurnsLeft = turns
		w.Directives = &game.Message{From: "Otherland", Body: "Hold the line."}
		runTurnUntilEnd(f, w)
		if !strings.Contains(f.out.String(), "Hold the line.") {
			t.Errorf("turns left %d: Play did not show the directives:\n%s", turns, f.out.String())
		}
	}
}
