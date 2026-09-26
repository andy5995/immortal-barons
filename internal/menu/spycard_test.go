package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// seedSpyTarget puts one realm on Mars in second place on its roster, so its
// letter is B, with the given spy record filed on it.
func seedSpyTarget(w *ctx, rec game.SpyReport) {
	w.With(func() {
		w.World.Config.IBBS = true
		w.World.Config.BoardID = "Alpha"
		w.World.ImportBoard(game.RemoteBoard{BoardID: "Mars", Scores: []game.RemoteScore{
			{Empire: "Decoy", Land: 100}, {Empire: "Red Baron", Land: 300},
		}})
		w.World.SpyDatabase = append(w.World.SpyDatabase, game.SpyEntry{SpyReport: rec})
		p := w.Player()
		p.Protection = 0
		p.Troopers = 1000
	})
	w.turnPlayed = true
}

// An individual strike prints the target's record once the letter is chosen,
// before the Attack Type menu, as the original's show_player_intelligence does.
func TestIndivAttackShowsTheTargetsSpyRecord(t *testing.T) {
	w := newWorld()
	seedSpyTarget(w, game.SpyReport{Board: "Mars", Empire: "Red Baron", Date: "09/01/2026",
		Land: 5630, Troopers: 1584, Morale: 52, Jets: 3409063, Turrets: 1069712, Tanks: 8712570})
	f := &fakeSession{keys: []rune("Mars\rB0")} // planet, target B, quit the Attack Type menu
	indivAttackForce(f, w)

	out := stripANSI(f.out.String())
	card := strings.Index(out, "Player Name: Red Baron")
	menu := strings.Index(out, "[Attack Type]")
	if card < 0 || menu < 0 {
		t.Fatalf("the script never reached the card and the Attack Type menu:\n%s", out)
	}
	if card > menu {
		t.Errorf("the card came after the Attack Type menu:\n%s", out)
	}
	for _, want := range []string{"BBS Name: Mars", "Player Letter: B", "Regions:      5,630",
		"[Troopers=1,584]    [Morale=52%]", "[Jets=3,409,063]  [Turrets=1,069,712]  [Tanks=8,712,570]"} {
		if !strings.Contains(out, want) {
			t.Errorf("card is missing %q:\n%s", want, out)
		}
	}
}

// A record from before reports carried the military figures has none to show,
// and no record at all prints nothing: either way the strike goes straight on.
func TestIndivAttackWithoutAMilitaryRecordShowsNoCard(t *testing.T) {
	for name, rec := range map[string]game.SpyReport{
		"old report":  {Board: "Mars", Empire: "Red Baron", Land: 300, Offense: 10},
		"other realm": {Board: "Mars", Empire: "Decoy", Troopers: 5, Morale: 80},
	} {
		w := newWorld()
		seedSpyTarget(w, rec)
		f := &fakeSession{keys: []rune("Mars\rB0")}
		indivAttackForce(f, w)
		out := stripANSI(f.out.String())
		if !strings.Contains(out, "[Attack Type]") {
			t.Fatalf("%s: the script never reached the Attack Type menu:\n%s", name, out)
		}
		if strings.Contains(out, "Player Name:") {
			t.Errorf("%s: a card was printed:\n%s", name, out)
		}
	}
}

// Joining a party aimed at one realm shows that realm's record after the party
// is picked and before the force prompts.
func TestJoinGroupAttackShowsTheTargetsSpyRecord(t *testing.T) {
	w := newWorld()
	seedSpyTarget(w, game.SpyReport{Board: "Mars", Empire: "Red Baron", Land: 300, Troopers: 7, Morale: 90})
	var slot int
	w.With(func() {
		ga, err := w.World.CreateGroupAttack(w.Player(), "Mars", "Red Baron", game.GroupAttackHoursMax,
			game.AttackForce{Troopers: 10})
		if err != nil {
			t.Fatalf("CreateGroupAttack: %v", err)
		}
		slot = ga.Slot
	})
	f := &fakeSession{keys: []rune(string(rune('0'+slot)) + "\r0\r0\r0\r0\r")}
	joinGroupAttack(f, w)

	out := stripANSI(f.out.String())
	card := strings.Index(out, "Player Name: Red Baron")
	prompt := strings.Index(out, "Send how many Troopers?")
	if card < 0 || prompt < 0 {
		t.Fatalf("the script never reached the card and the force prompts:\n%s", out)
	}
	if card > prompt {
		t.Errorf("the card came after the force prompts:\n%s", out)
	}
}
