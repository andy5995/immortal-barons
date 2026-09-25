package game

import "time"

// clock.go — the test clock. IB's instants (a weapon's launch and arrival, the
// Travel Times probe, every stamp) are read from timeNow, and a board on a test
// rig has to be able to move them: a Gooie Kablooie takes three real days to
// build and two more to fly, which is not a thing anyone can sit through to find
// out whether it lands. faketime cannot help, because Go reads the clock without
// going through libc.

// clockOffset shifts every instant the game reads. Zero in every real game; set
// from IB_CLOCK_OFFSET (or the older IB_GAME_DATE) by the command line front
// end, which also prints a banner on every run while it is non-zero.
var clockOffset time.Duration

// timeNow is the game's clock, indirected so a test can hold it still and so a
// test rig can shift it.
var timeNow = func() time.Time { return time.Now().Add(clockOffset) }

// Three sites read the real clock ON PURPOSE and must keep doing so:
// NewWorldSeed (a shifted seed would make every reset build the same world),
// LastPacketFrom (the question it answers, "has this board gone quiet?", is
// about real elapsed time), and the sysop log stamps in internal/store. Anything
// else that writes an instant into the save or onto the wire goes through
// timeNow, or a rig ends up with a Gooie Kablooie on the shifted clock and a
// group attack departing on the real one.

// SetClockOffset shifts the game's clock. It is called once at startup, before
// anything reads an instant.
func SetClockOffset(d time.Duration) { clockOffset = d }

// ClockOffset is the shift in force.
func ClockOffset() time.Duration { return clockOffset }

// Today is the game's date, derived from the same clock as every instant, so the
// date string and the instants can never disagree about what day it is. That is
// why the offset is a duration rather than a date: one source, two views.
func Today() string { return timeNow().Format("2006-01-02") }

// Now is the game's clock for callers outside this package, so an instant they
// save is on the same clock as the game's own.
func Now() time.Time { return timeNow() }
