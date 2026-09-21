package game

import (
	"strings"
	"testing"
)

// Delete this file with internal/game/retire_ai.go.

func TestRetireAIBaronsRemovesBaronsAndKeepsCallers(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	human := w.AddHuman("alice", "Alethia")
	w.AddAIEmpires(3)
	w.AIRetired = false // a world saved before the retirement

	w.RetireAIBarons()

	if len(w.Empires) != 1 || w.Empires[0] != human {
		t.Fatalf("want only the caller's realm left, got %d empires", len(w.Empires))
	}
	if !w.AIRetired {
		t.Error("the sweep did not record itself, so it will run again")
	}
	filed := 0
	for _, n := range w.NewsToday {
		if strings.Contains(n.Text, "withdraws from the planet") {
			filed++
		}
	}
	if filed != 3 {
		t.Errorf("three realms vanished and the news names %d of them", filed)
	}
}

// A baron's treaties key on its realm NAME, and the next caller to claim that
// name would inherit them. dropEmpires is what forgets them; this is here to
// catch a future rewrite that removes the barons some cheaper way.
func TestRetireAIBaronsForgetsTheirTreaties(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	human := w.AddHuman("alice", "Alethia")
	w.AddAIEmpires(1)
	baron := w.AIEmpires()[0]
	w.setRelation(human.Name, baron.Name, FullDefenseAlliance.Name)
	w.AIRetired = false

	w.RetireAIBarons()

	for _, tr := range w.Treaties {
		if tr.A == baron.Name || tr.B == baron.Name {
			t.Fatalf("a retired baron left a %s behind on %q", tr.Type, baron.Name)
		}
	}
}

// The sweep runs once. Were it to run on every load it would undo IB_ADD_AI,
// which is the only way a baron is seeded now.
func TestRetireAIBaronsRunsOnlyOnce(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.RetireAIBarons() // a fresh world: nothing to sweep, but now marked

	w.AddAIEmpires(2)
	w.RetireAIBarons()

	if got := len(w.AIEmpires()); got != 2 {
		t.Errorf("a second sweep removed barons seeded after the first: %d left, want 2", got)
	}
}

// A fresh game is born swept, so no new world pays for the migration.
func TestAFreshWorldIsAlreadyRetired(t *testing.T) {
	if w := NewWorldSeed(DefaultConfig(), 1); !w.AIRetired {
		t.Error("a fresh world starts unswept, so its first load would sweep it")
	}
}
