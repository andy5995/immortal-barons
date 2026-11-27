package game

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Counting changes nothing: a world with a tally plays out exactly as one
// without, and the tally is not in the save.
func TestTallyIsBehaviorNeutralAndNotSaved(t *testing.T) {
	// Events are stamped with the clock, and a slow machine can cross a
	// second between the two runs; hold it still so only play is compared.
	restore := timeNow
	t.Cleanup(func() { timeNow = restore })
	frozen := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	timeNow = func() time.Time { return frozen }
	play := func(tally bool) (string, *World) {
		w := NewWorldSeed(DefaultConfig(), 3)
		if tally {
			w.Tally = NewTally()
		}
		w.AddAIEmpires(6)
		for d := 1; d <= 20; d++ {
			w.DailyMaintenance(fmt.Sprintf("2026-02-%02d", d))
		}
		b, err := json.Marshal(w)
		if err != nil {
			t.Fatal(err)
		}
		return string(b), w
	}
	plain, _ := play(false)
	counted, w := play(true)
	if plain != counted {
		t.Error("a world with a tally plays differently from one without")
	}
	if strings.Contains(plain, "Tally") {
		t.Error("the tally is in the saved world")
	}
	if w.Tally.Battles == 0 {
		t.Error("twenty days of AI play counted no battle")
	}
}
