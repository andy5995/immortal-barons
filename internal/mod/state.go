package mod

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// skipFile is where a skipped script is remembered between runs of the
// program: a small file beside world.json. It is not part of the world, which
// the screens only read. Several processes may use it at once: a write replaces
// the file whole, and a file that cannot be read is an empty one, so it never
// stops play. A writer that loses a race costs only a later session its three
// timeouts again.
const (
	skipFile       = "script-skips.json"
	skipFileMax    = 64 << 10
	skipEntriesMax = 256
)

// skipEntry is one skipped script. Script says which, for the sysop reading the
// file.
type skipEntry struct {
	Tripped time.Time `json:"tripped"`
	Script  string    `json:"script"`
}

type skipData struct {
	Skipped map[string]skipEntry `json:"skipped"`
}

// stateDir is where skipFile lives, "" for none: a program that only checks or
// simulates a mod keeps what it learns in memory, and writes nothing.
var (
	stateDir  string
	skipClock = time.Now
)

// SetStateDir names the data directory the game keeps skipped scripts in. A
// caller that is playing a game, not checking or simulating a mod, calls it.
func SetStateDir(dir string) {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()
	stateDir = dir
	breaker.loaded = ""
}

// resetBreaker forgets everything held in memory, as a new process would.
func resetBreaker() {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()
	clear(breaker.streak)
	clear(breaker.skipped)
	clear(breaker.told)
	breaker.loaded = ""
}

// until is when the skip ends.
func (e skipEntry) until() time.Time { return e.Tripped.Add(ScriptSkipHold) }

// heldAt reports whether the script is still skipped at now.
func (e skipEntry) heldAt(now time.Time) bool { return now.Before(e.until()) }

func skipUntil(e skipEntry) string {
	return e.until().Local().Format("2006-01-02 15:04 MST")
}

func skipFileHint() string {
	if stateDir == "" {
		return ""
	}
	return " Delete " + skipFile + " in the data directory to try it again sooner."
}

// readSkips reads the file. Anything wrong with it makes it empty.
func readSkips(dir string) map[string]skipEntry {
	f, err := os.Open(filepath.Join(dir, skipFile))
	if err != nil {
		return nil
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, skipFileMax+1))
	if err != nil || len(b) > skipFileMax {
		return nil
	}
	var d skipData
	if json.Unmarshal(b, &d) != nil {
		return nil
	}
	return d.Skipped
}

// loadSkipsLocked reads the file once for each state directory set. The caller
// holds breaker.mu.
func loadSkipsLocked() {
	if stateDir == "" || breaker.loaded == stateDir {
		return
	}
	breaker.loaded = stateDir
	now := skipClock()
	for key, e := range readSkips(stateDir) {
		if e.heldAt(now) {
			if _, ok := breaker.skipped[key]; !ok {
				breaker.skipped[key] = e
			}
		}
	}
}

// saveSkipLocked adds one skipped script to the file, read afresh so another
// process's entries are kept, drops what has expired, and replaces the file.
func saveSkipLocked(key string, e skipEntry) {
	if stateDir == "" {
		return
	}
	now := skipClock()
	all := map[string]skipEntry{}
	for k, v := range readSkips(stateDir) {
		if v.heldAt(now) {
			all[k] = v
		}
	}
	all[key] = e
	if len(all) > skipEntriesMax {
		keys := make([]string, 0, len(all))
		for k := range all {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return all[keys[i]].Tripped.After(all[keys[j]].Tripped) })
		for _, k := range keys[skipEntriesMax:] {
			delete(all, k)
		}
	}
	b, err := json.MarshalIndent(skipData{Skipped: all}, "", "  ")
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(stateDir, "."+skipFile+"-*.tmp")
	if err != nil {
		return
	}
	_, werr := tmp.Write(append(b, '\n'))
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), filepath.Join(stateDir, skipFile)) != nil {
		os.Remove(tmp.Name())
	}
}
