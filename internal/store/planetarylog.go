package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PlanetaryLogFile records transport faults — an undeliverable packet, orders
// that failed their check — in the data directory.
//
// The run report prints them, but almost every board runs the planetary step
// from a scheduler, and a scheduler throws stdout away. Without this a sysop who
// automated the step (which the setup guide tells them to do) would see nothing
// at all, which is worse than the player bulletin these moved out of.
const PlanetaryLogFile = "planetary.log"

// planetaryLogLines caps the file. The fault this log exists for repeats every
// exchange until someone fixes it, so an append-only file would grow without
// limit for exactly the case it is meant to record — the same mistake the news
// cap already guards against, one layer down.
const planetaryLogLines = 500

// AppendPlanetaryLog adds one timestamped line per notice and trims the file to
// its last planetaryLogLines. A failure is not the caller's problem: the
// planetary step must not fail because a data directory is read-only, and the
// notices reach the run report either way.
func AppendPlanetaryLog(dataDir string, notices []string, when time.Time) {
	if len(notices) == 0 {
		return
	}
	path := filepath.Join(dataDir, PlanetaryLogFile)
	stamp := when.Format("2006-01-02 15:04:05")

	var lines []string
	if old, err := os.ReadFile(path); err == nil {
		lines = strings.Split(strings.TrimRight(string(old), "\n"), "\n")
		if len(lines) == 1 && lines[0] == "" {
			lines = nil
		}
	}
	for _, n := range notices {
		lines = append(lines, fmt.Sprintf("%s  %s", stamp, n))
	}
	if len(lines) > planetaryLogLines {
		lines = lines[len(lines)-planetaryLogLines:]
	}
	os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
