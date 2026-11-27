package mod

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// The limits on a running script. None is a gameplay number: they keep a
// hostile or buggy script from taking the game down, so they live here with
// ScriptSteps and not in balance*.go. Starlark has no file, network or clock
// access and IB gives a script none; these cover CPU and memory, which
// Starlark does not limit by itself. A script runs in a worker process (see
// worker.go), so the operating system can enforce them where it can. "Scripts"
// in docs/dev/mods-design.md says what each system does.
const (
	// ScriptTimeout is the wall-clock time one run of a script may take, the
	// worker's start-up included. A run past it is killed with its worker.
	ScriptTimeout = 3 * time.Second

	// ScriptTimeoutsToSkip is how many runs of one script in a row may time out
	// before the game stops running it for the rest of the process. Another
	// kind of failure does not count, and a run that succeeds starts the count
	// again.
	ScriptTimeoutsToSkip = 3

	// ScriptSkipHold is how long a skipped script stays skipped, across runs of
	// the program, once it has timed out ScriptTimeoutsToSkip times. A mod's
	// files or options changing makes it a different script, which is not held.
	ScriptSkipHold = 24 * time.Hour

	// ScriptMemory is how much memory a worker may use. The operating system
	// enforces it where it can (Linux, the BSDs, Windows); a watchdog in the
	// worker enforces it everywhere, by sampling the heap.
	ScriptMemory = 256 << 20

	// MaxModFileSize is the largest file a mod may hold, and MaxModFiles the
	// most files. They apply to a mod read from a folder, one saved in a
	// world, and one carried in a league's reset order.
	MaxModFileSize = 256 << 10
	MaxModFiles    = 64
)

// The limits as the code reads them, so a test can lower one.
var (
	limTimeout = ScriptTimeout
	limMemory  = uint64(ScriptMemory)
)

// clean makes s safe to print on a terminal or write to a log: a control
// character, and a character that reorders or hides text (Unicode format
// characters), becomes a \u escape. Anything a script or a mod file chose
// goes through it before it is shown.
func clean(s string) string {
	if !strings.ContainsFunc(s, game.UnsafeText) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if game.UnsafeText(r) {
			fmt.Fprintf(&b, "\\u%04x", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Clean is clean for a screen outside this package that shows what a mod said.
func Clean(s string) string { return clean(s) }

// hasUnsafe reports whether s holds a character clean would escape.
func hasUnsafe(s string) bool { return strings.ContainsFunc(s, game.UnsafeText) }

// scriptFileOptions is what every script is parsed with: Starlark's defaults,
// all off. There is no while loop, no recursion, no set(), no reassigning a
// top-level name, no if or for at the top level, and load() has nothing to
// load from.
var scriptFileOptions = &syntax.FileOptions{}

// truncate cuts s to at most n runes, marking the cut.
func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "..."
}

// maxShown is how much of a script's own string an error message repeats.
const maxShown = 40

// show is how an error message names a value a script chose: a string,
// cut short and cleaned; any other value by its type only, so a message never
// prints a large or nested value.
func show(v starlark.Value) string {
	if s, ok := starlark.AsString(v); ok {
		return strconv.Quote(clean(truncate(s, maxShown)))
	}
	return "a value of type " + v.Type()
}
