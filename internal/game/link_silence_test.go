package game

import (
	"testing"
	"time"
)

// A planet that has gone quiet is a different thing from one the roster cannot
// place, and a different thing again from one never heard from (#187).
func TestLinkQuietOnlyCountsPlanetsHeardFrom(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	w.LastPacketFrom = map[string]string{
		"Chatty": Recorded(now.Add(-2 * time.Hour)),
		"Quiet":  Recorded(now.Add(-(LinkSilentMax + 2) * 24 * time.Hour)),
	}
	for _, c := range []struct {
		board string
		days  int
		quiet bool
	}{
		{"Chatty", 0, false},
		{"Quiet", LinkSilentMax + 2, true},
		{"Stranger", -1, false}, // never heard from: new, not quiet
	} {
		if got := w.LinkSilentDays(c.board, now); got != c.days {
			t.Errorf("%s silent %d days, want %d", c.board, got, c.days)
		}
		if got := w.LinkQuiet(c.board, now); got != c.quiet {
			t.Errorf("%s quiet = %v, want %v", c.board, got, c.quiet)
		}
	}
	// Exactly at the threshold is not yet quiet: a board that exchanges mail on
	// a slow schedule must not be reported as broken.
	w.LastPacketFrom["Edge"] = Recorded(now.Add(-LinkSilentMax * 24 * time.Hour))
	if w.LinkQuiet("Edge", now) {
		t.Errorf("a board silent exactly %d days should not read as quiet", LinkSilentMax)
	}
}

// The sysop's tally counts NEW faults and dates itself from the first one, so
// the figure measures how much is wrong rather than how often the step runs.
func TestFaultTallyCountsAndDatesItself(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-08-23"
	w.CountFaults(0)
	if w.FaultsSeen != 0 || w.FaultsSince != "" {
		t.Fatalf("a clean run started a count: %d since %q", w.FaultsSeen, w.FaultsSince)
	}
	w.CountFaults(2)
	w.LastMaintDate = "2026-08-30"
	w.CountFaults(2)
	if w.FaultsSeen != 4 {
		t.Errorf("faults = %d, want 4", w.FaultsSeen)
	}
	if w.FaultsSince != "2026-08-23" {
		t.Errorf("the count is dated %q, want the day it started", w.FaultsSince)
	}
}
