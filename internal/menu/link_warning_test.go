package menu

import (
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A baron addressing a planet nothing has come back from is warned where the
// planet is chosen — before the turn is spent — and not warned about one that
// is simply new (#187).
func TestQuietPlanetIsFlaggedAtThePrompt(t *testing.T) {
	for _, c := range []struct {
		name  string
		stamp string
		warn  bool
	}{
		{"Quiet", game.Recorded(time.Now().Add(-(game.LinkSilentMax + 2) * 24 * time.Hour)), true},
		{"Chatty", game.Recorded(time.Now().Add(-time.Hour)), false},
		{"Stranger", "", false},
	} {
		w := newWorld()
		w.With(func() {
			w.World.Config.IBBS = true
			w.World.ImportBoard(game.RemoteBoard{BoardID: c.name})
			if c.stamp != "" {
				w.World.LastPacketFrom = map[string]string{c.name: c.stamp}
			}
		})
		f := &fakeSession{}
		showRelation(f, w, c.name)

		out := stripANSI(f.out.String())
		if !strings.Contains(out, "Our current relations with") {
			t.Fatalf("%s: the relation line is missing:\n%s", c.name, out)
		}
		if got := strings.Contains(out, "Nothing has come from"); got != c.warn {
			t.Errorf("%s: warned = %v, want %v:\n%s", c.name, got, c.warn, out)
		}
	}
}

// The sysop's line appears on Game Setup once faults have been recorded, and
// says where to read them.
func TestGameSetupShowsTheFaultCount(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.World.Config.IBBS = true
		w.World.Config.BoardID = "Alpha"
		w.World.FaultsSeen, w.World.FaultsSince = 4, "2026-08-23"
	})
	f := &fakeSession{keys: []rune("\r\r\r\r")}
	gameSetup(f, w)

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "4 faults since 2026-08-23") {
		t.Errorf("no fault line:\n%s", out)
	}
	if !strings.Contains(out, "planetary.log") {
		t.Errorf("the fault line does not say where to look:\n%s", out)
	}
}

// A board with nothing wrong is told nothing: a row that always reads zero is a
// row a sysop stops seeing.
func TestGameSetupHidesACleanFaultCount(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.World.Config.IBBS = true
		w.World.Config.BoardID = "Alpha"
	})
	f := &fakeSession{keys: []rune("\r\r\r\r")}
	gameSetup(f, w)
	if strings.Contains(stripANSI(f.out.String()), "planetary.log") {
		t.Error("a clean board was shown a fault row")
	}
}
