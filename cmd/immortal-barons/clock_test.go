package main

import (
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// IB_GAME_DATE must land on the date asked for, whatever zone the machine is in.
// It was translated by measuring a UTC midnight against a LOCAL one, so west of
// Greenwich it came up short by the zone offset and the game ran a day behind
// what the operator asked for, with nothing on screen to say so.
func TestGameDateLandsOnTheDateAsked(t *testing.T) {
	t.Cleanup(func() { game.SetClockOffset(0) })
	for _, days := range []int{1, 3, 7, 30} {
		want := time.Now().AddDate(0, 0, days).Format("2006-01-02")
		t.Setenv("IB_GAME_DATE", want)
		game.SetClockOffset(0)
		applyTestClock()
		if got := game.Today(); got != want {
			t.Errorf("IB_GAME_DATE=%s put the game on %s", want, got)
		}
	}
}

// The two variables are one clock: whatever moves the date must move the
// instants with it, or a rig gets a weapon on one clock and an attack on the
// other.
func TestTheOffsetMovesTheInstantsAndTheDateTogether(t *testing.T) {
	t.Cleanup(func() { game.SetClockOffset(0) })
	t.Setenv("IB_CLOCK_OFFSET", "72h")
	applyTestClock()
	if got, want := game.ClockOffset(), 72*time.Hour; got != want {
		t.Fatalf("offset is %s, want %s", got, want)
	}
	if got, want := game.Today(), time.Now().Add(72*time.Hour).Format("2006-01-02"); got != want {
		t.Errorf("the date reads %s but the instants moved 72h, to %s", got, want)
	}
}
