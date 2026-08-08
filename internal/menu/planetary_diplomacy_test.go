package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// leagueCtx is a board in a league with three other planets on the roster and
// the caller elected its BBS Coordinator, which is who may edit the chart.
func leagueCtx(t *testing.T) *ctx {
	t.Helper()
	w := newWorld()
	w.Config.IBBS = true
	w.Config.BoardID = "Eye of the Storm"
	w.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Nova Hub"},
		{Number: 2, Name: "Starship Junkyard"},
		{Number: 3, Name: "Eye of the Storm"},
		{Number: 4, Name: "The Eclipse"},
	}
	w.VoteCoordinator(w.Player(), w.Player().Owner)
	if w.BBSCoordinator() != w.Player() {
		t.Fatal("the caller should be the elected BBS Coordinator")
	}
	return w
}

func drive(t *testing.T, w *ctx, keys string, do func(session.Session, *ctx) Result) *fakeSession {
	t.Helper()
	f := &fakeSession{keys: []rune(keys)}
	do(f, w)
	return f
}

// The Coordinator files a status and the chart reads it back — through the
// screens, not through the model, so a change to either prompt shows up here.
func TestDiplomacyModificationFilesAStatus(t *testing.T) {
	w := leagueCtx(t)

	// Planet 2, then "A" for Ally.
	out := drive(t, w, "2\rA", diplomacyModification).out.String()
	if !strings.Contains(out, "Diplomacy Modification") {
		t.Fatalf("never reached the editing screen:\n%s", out)
	}
	if !strings.Contains(out, "Change status to") {
		t.Fatalf("never reached the status prompt:\n%s", out)
	}
	if got := w.PlanetRelationWith("Starship Junkyard"); got != game.PlanetAllied {
		t.Fatalf("filed status = %q, want %q\n%s", got, game.PlanetAllied, out)
	}

	// The chart now shows it, with this board itself left out.
	list := drive(t, w, "\r", planetaryTreaties).out.String()
	if !strings.Contains(list, "Planetary Treaties") {
		t.Fatalf("never reached the chart:\n%s", list)
	}
	for _, want := range []string{"Starship Junkyard", "Allied", "Nova Hub", "None"} {
		if !strings.Contains(list, want) {
			t.Errorf("chart is missing %q:\n%s", want, list)
		}
	}
	if strings.Contains(list, "Eye of the Storm") {
		t.Errorf("the chart lists this board's own planet:\n%s", list)
	}
}

// A baron who is not the Coordinator is turned away before any prompt.
func TestDiplomacyModificationIsCoordinatorOnly(t *testing.T) {
	w := newWorld()
	w.Config.IBBS = true
	w.Config.BoardID = "Eye of the Storm"
	w.LeagueNodes = []game.LeagueNode{{Number: 1, Name: "Nova Hub"}}

	out := drive(t, w, "1\rA", diplomacyModification).out.String()
	if strings.Contains(out, "Change status to") {
		t.Errorf("a non-Coordinator reached the status prompt:\n%s", out)
	}
	if got := w.PlanetRelationWith("Nova Hub"); got != game.PlanetNone {
		t.Errorf("a non-Coordinator filed %q", got)
	}
}

// Allied Planets addresses the allies and says so; with none filed it says that
// instead of opening an editor on an empty address list.
func TestAlliedPlanetsMessageTarget(t *testing.T) {
	w := leagueCtx(t)

	empty := drive(t, w, "", ipMessageAllied).out.String()
	if !strings.Contains(empty, "no allied planets") {
		t.Errorf("with no allies filed, got:\n%s", empty)
	}

	w.SetPlanetRelationWith("The Eclipse", game.PlanetAllied)
	// Abort at the message editor: the addressing is what is under test.
	out := drive(t, w, "/A\r", ipMessageAllied).out.String()
	if !strings.Contains(out, "The Eclipse") {
		t.Errorf("allied planet not addressed:\n%s", out)
	}
	if strings.Contains(out, "Nova Hub") {
		t.Errorf("a planet with no alliance was addressed:\n%s", out)
	}
}
