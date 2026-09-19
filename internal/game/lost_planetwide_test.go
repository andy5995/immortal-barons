package game

import (
	"strings"
	"testing"
)

// A strike aimed at a whole planet carries no TargetEmpire — that emptiness IS
// the whole-planet form — so the recovery notices used to interpolate a hole:
// "Your force sent against  has returned home." Seen in a league recap on
// 2026-09-19, on a group attack that timed out.
func TestLostPlanetWideStrikeNamesThePlanet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.LostForcesDays = 1
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")
	e.Protection, e.Troopers = 0, 500

	w.InFlight = []InFlightStrike{{
		ID: 1, TargetBoard: "faraway", TargetEmpire: "", Whole: true, Group: true, LaunchedDay: w.GameDay,
		Contributors: []Contribution{{Owner: "alice", AttackForce: AttackForce{Troopers: 300}}},
	}}
	w.GameDay += 5
	if got := w.ReturnLostForces(); got != 1 {
		t.Fatalf("recovered %d strikes, want 1", got)
	}
	if len(e.Events) == 0 {
		t.Fatal("the baron was told nothing")
	}
	got := e.Events[len(e.Events)-1].Text
	if strings.Contains(got, "against  ") || strings.Contains(got, "against has") {
		t.Errorf("the notice has a hole where the target goes: %q", got)
	}
	if !strings.Contains(got, "the whole planet") {
		t.Errorf("the notice does not name the planet: %q", got)
	}
}

// The same hole on the SUCCESS path, which players meet far more often than a
// timeout: a bombing op targets the planet (TargetsPlanet), so the answering
// board never looks up a realm and res.TargetEmpire comes back empty. The
// result notices interpolated it raw until 2026-09-19, printing
// "Bomb Trade Routes ( of faraway)".
func TestPlanetWideSpecialOpResultNamesThePlanet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")

	sent := InFlightStrike{
		ID: 7, Kind: "special", Owner: "alice", Op: OpBombRoutes,
		TargetBoard: "faraway", TargetEmpire: "", Whole: true,
	}
	w.applySpecialOpResult(sent, AttackResult{
		ID: 7, TargetBoard: "faraway", TargetEmpire: "", Outcome: OutcomeWon,
	})
	if len(e.Events) == 0 {
		t.Fatal("the baron was told nothing")
	}
	got := e.Events[len(e.Events)-1].Text
	if strings.Contains(got, "( of ") || strings.Contains(got, "against  ") {
		t.Errorf("the notice has a hole where the target goes: %q", got)
	}
	if !strings.Contains(got, "the whole of faraway") {
		t.Errorf("the notice does not name the planet: %q", got)
	}
}
