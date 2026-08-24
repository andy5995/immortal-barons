package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readLog(t *testing.T, dir string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, PlanetaryLogFile))
	if err != nil {
		t.Fatalf("no log written: %v", err)
	}
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

func TestPlanetaryLogStampsAndAppends(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 8, 23, 17, 45, 2, 0, time.UTC)

	AppendPlanetaryLog(dir, []string{"first fault"}, at)
	AppendPlanetaryLog(dir, []string{"second fault", "third fault"}, at.Add(time.Hour))

	lines := readLog(t, dir)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3: %q", len(lines), lines)
	}
	if !strings.HasPrefix(lines[0], "2026-08-23 17:45:02  first fault") {
		t.Errorf("line not stamped as expected: %q", lines[0])
	}
	if !strings.Contains(lines[2], "third fault") {
		t.Errorf("later notices did not append: %q", lines)
	}
}

// The fault this log records repeats every exchange until someone fixes it, so
// an unbounded file would reproduce, on disk, the flooding it was built to stop.
func TestPlanetaryLogIsCapped(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	for i := range planetaryLogLines + 50 {
		AppendPlanetaryLog(dir, []string{"fault"}, at.Add(time.Duration(i)*time.Minute))
	}
	if lines := readLog(t, dir); len(lines) != planetaryLogLines {
		t.Errorf("log holds %d lines, want the %d cap", len(lines), planetaryLogLines)
	}
	// The cap must drop the OLDEST, or the log would answer "what went wrong
	// first" instead of "what is wrong now".
	last := readLog(t, dir)[planetaryLogLines-1]
	if !strings.Contains(last, at.Add(time.Duration(planetaryLogLines+49)*time.Minute).Format("15:04:05")) {
		t.Errorf("the newest line was trimmed instead of the oldest: %q", last)
	}
}

// Nothing to say, nothing written: a log that gets a line every quiet run is
// one nobody reads.
func TestPlanetaryLogStaysAbsentWhenNothingIsWrong(t *testing.T) {
	dir := t.TempDir()
	AppendPlanetaryLog(dir, nil, time.Now())
	if _, err := os.Stat(filepath.Join(dir, PlanetaryLogFile)); !os.IsNotExist(err) {
		t.Error("a quiet run created a log file")
	}
}

// A read-only data directory must not fail the planetary step.
func TestPlanetaryLogSurvivesAnUnwritableDirectory(t *testing.T) {
	AppendPlanetaryLog(filepath.Join(t.TempDir(), "does-not-exist"), []string{"fault"}, time.Now())
}
