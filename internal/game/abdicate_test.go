package game

import "testing"

// TestAbdicateLeavesTheAlreadyDeadAlone: a realm crushed while its baron sat at
// the confirmation keeps the day it died, and no abdication line is posted for
// a death the planet already read about.
func TestAbdicateLeavesTheAlreadyDeadAlone(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("gone", "Gonemark")
	w.GameDay = 3
	w.Kill(e)
	w.GameDay = 4
	w.NewsToday = nil

	w.Abdicate(e)
	if e.DiedDay != 3 {
		t.Errorf("DiedDay = %d, want 3, the day it was crushed", e.DiedDay)
	}
	if len(w.NewsToday) != 0 {
		t.Errorf("abdicating a dead realm posted news: %v", w.NewsToday)
	}
}
