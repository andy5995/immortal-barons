package menu

import (
	"testing"
	"time"
)

// The entry-menu clock exists to tell a player when the game day turns, which is
// the door host's local midnight — so the countdown, not the clock, is the part
// that has to be right.
func TestPlanetaryClockCountsDownToMidnight(t *testing.T) {
	now := time.Date(2026, 8, 30, 23, 47, 12, 0, time.Local)
	got := planetaryClock(now, "2026-08-30", "en")
	want := "Planetary time: 23:47   New day in 0:12"
	if got != want {
		t.Errorf("planetaryClock = %q, want %q", got, want)
	}

	now = time.Date(2026, 8, 30, 9, 5, 0, 0, time.Local)
	got = planetaryClock(now, "2026-08-30", "en")
	want = "Planetary time: 09:05   New day in 14:55"
	if got != want {
		t.Errorf("planetaryClock = %q, want %q", got, want)
	}
}

// A pinned -date, or a session held open past midnight, leaves the wall clock on
// a different day from the world's; the line goes away rather than lie.
func TestPlanetaryClockIsSilentOffTheWorldsDay(t *testing.T) {
	now := time.Date(2026, 8, 31, 0, 5, 0, 0, time.Local)
	if got := planetaryClock(now, "2026-08-30", "en"); got != "" {
		t.Errorf("planetaryClock on a stale world date = %q, want empty", got)
	}
	if got := planetaryClock(now, "", "en"); got != "" {
		t.Errorf("planetaryClock with no world date = %q, want empty", got)
	}
}
