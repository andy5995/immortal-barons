package store

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The per-turn figures the income report and End of Turn Statistics print are
// written in one transaction and read in the NEXT one — Manufacture runs inside
// collectTurnIncome's withPlayer, incomeReport reads the result inside its own;
// PlayTurn and endOfTurnStats are the same pair. On a door every transaction is
// flock → reload → fn → save, and reload unmarshals into a fresh world, so a
// field the save does not carry is zero by the time the screen asks for it.
// Under MemStore the object survives, which is why every menu test passed while
// a real board showed no "Your Industrial Zones built:" line at all.
func TestTurnReportFieldsSurviveATransaction(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()

	w := game.NewWorldSeed(cfg, 1)
	w.AddHuman("alice", "Alice")
	if err := Save(w, cfg); err != nil {
		t.Fatalf("seed save: %v", err)
	}
	w.SetStore(NewFileStore(w, cfg))

	w.With(func() {
		e := w.FindByOwner("alice")
		e.MadeTroopers, e.MadeJets, e.MadeTurrets = 11, 22, 33
		e.MadeBombers, e.MadeTanks, e.MadeCarriers = 44, 55, 66
		e.LastPopGrowth, e.LastSpoiled, e.LastMoraleDesertion = 77, 88, 99
		e.LastCivilWar, e.LastRiot = 7, true
	})

	var got game.Empire
	w.Read(func() { got = *w.FindByOwner("alice") })

	for _, c := range []struct {
		name string
		have int
		want int
	}{
		{"MadeTroopers", got.MadeTroopers, 11},
		{"MadeJets", got.MadeJets, 22},
		{"MadeTurrets", got.MadeTurrets, 33},
		{"MadeBombers", got.MadeBombers, 44},
		{"MadeTanks", got.MadeTanks, 55},
		{"MadeCarriers", got.MadeCarriers, 66},
		{"LastPopGrowth", got.LastPopGrowth, 77},
		{"LastSpoiled", got.LastSpoiled, 88},
		{"LastMoraleDesertion", got.LastMoraleDesertion, 99},
		{"LastCivilWar", got.LastCivilWar, 7},
	} {
		if c.have != c.want {
			t.Errorf("%s = %d after one transaction, want %d", c.name, c.have, c.want)
		}
	}
	if !got.LastRiot {
		t.Error("LastRiot = false after one transaction, want true")
	}
}
