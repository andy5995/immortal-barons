package game

import (
	"encoding/json"
	"strings"
	"testing"
)

// An unfiled planet reads None, and a filed one reads back what was filed.
func TestPlanetRelationDefaultsToNone(t *testing.T) {
	w := NewWorld(DefaultConfig())
	if got := w.PlanetRelationWith("Nova Hub"); got != PlanetNone {
		t.Errorf("an unfiled planet = %q, want %q", got, PlanetNone)
	}
	w.SetPlanetRelationWith("Nova Hub", PlanetAllied)
	if got := w.PlanetRelationWith("Nova Hub"); got != PlanetAllied {
		t.Errorf("after filing Allied = %q", got)
	}
	w.SetPlanetRelationWith("Nova Hub", PlanetNone)
	if got := w.PlanetRelationWith("Nova Hub"); got != PlanetNone {
		t.Errorf("after filing back to None = %q", got)
	}
}

// Allied Planets addresses only the planets the chart calls Allied, and never
// this board itself.
func TestAlliedPlanetNames(t *testing.T) {
	w := NewWorld(DefaultConfig())
	w.Config.BoardID = "Eye of the Storm"
	w.LeagueNodes = []LeagueNode{
		{Number: 1, Name: "Nova Hub"},
		{Number: 2, Name: "Starship Junkyard"},
		{Number: 3, Name: "Eye of the Storm"},
		{Number: 4, Name: "The Eclipse"},
	}
	w.SetPlanetRelationWith("Starship Junkyard", PlanetAllied)
	w.SetPlanetRelationWith("The Eclipse", PlanetPeace)
	w.SetPlanetRelationWith("Eye of the Storm", PlanetAllied) // nonsense, but filed

	got := w.AlliedPlanetNames()
	if len(got) != 1 || got[0] != "Starship Junkyard" {
		t.Errorf("AlliedPlanetNames() = %v, want [Starship Junkyard]", got)
	}
}

// The chart is local: it must never leave on a packet, whatever else the board
// is exporting. BRE's own note is that none of it is reported to the other
// planets, and a board that shipped it would be asserting another board's
// diplomacy for it.
func TestPlanetDiplomacyNeverLeavesTheBoard(t *testing.T) {
	w := NewWorld(DefaultConfig())
	w.Config.BoardID = "Eye of the Storm"
	w.Config.IBBS = true
	w.LeagueNodes = []LeagueNode{{Number: 1, Name: "Nova Hub"}, {Number: 2, Name: "Eye of the Storm"}}
	w.Empires = append(w.Empires, &Empire{Name: "Iron Dominion", Owner: "iron", Alive: true})
	w.SetPlanetRelationWith("Nova Hub", PlanetEnemy)
	w.ExportScores()
	w.ExportNodeList()
	w.SendIPMessage(w.Empires[len(w.Empires)-1], []string{"Nova Hub"}, false, "hello")

	if len(w.Outbox) == 0 {
		t.Fatal("nothing was exported, so the packets prove nothing")
	}
	for _, p := range w.Outbox {
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, word := range []string{string(PlanetEnemy), "PlanetDiplomacy"} {
			if strings.Contains(string(data), word) {
				t.Errorf("packet to %q carries %q:\n%s", p.ToBoard, word, data)
			}
		}
	}
}

// A new season starts the chart empty: the coordinator who filed it is gone
// with every other empire.
func TestSeasonResetClearsPlanetDiplomacy(t *testing.T) {
	w := NewWorld(DefaultConfig())
	w.SetPlanetRelationWith("Nova Hub", PlanetAllied)
	w.ResetForNewSeason("2026-08-08")
	if got := w.PlanetRelationWith("Nova Hub"); got != PlanetNone {
		t.Errorf("after a season reset = %q, want %q", got, PlanetNone)
	}
}
