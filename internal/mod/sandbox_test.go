package mod

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"

	"github.com/andy5995/immortal-barons/internal/game"
)

// lower sets a limit for one test.
func lowerLimits(t *testing.T) {
	to, mem, seq, bits, grace := limTimeout, limMemory, limSeq, limIntBits, limGrace
	t.Cleanup(func() {
		limTimeout, limMemory, limSeq, limIntBits, limGrace = to, mem, seq, bits, grace
		halted.Store(false)
	})
}

// evalMain compiles src with the guarded surface and returns main()'s answer.
func evalMain(src string) (starlark.Value, error) {
	globals, err := compileScript("t.star", src, predeclaredFor(nil))
	if err != nil {
		return nil, err
	}
	fn, ok := globals["main"].(*starlark.Function)
	if !ok {
		return nil, errors.New("no main")
	}
	th := newThread("t.star")
	var res starlark.Value
	err = runScript(th, func() (err error) {
		res, err = starlark.Call(th, fn, nil, nil)
		return err
	})
	return res, err
}

func TestScriptTimeoutStopsALongRun(t *testing.T) {
	lowerLimits(t)
	limTimeout = time.Millisecond
	_, err := evalMain("def main():\n    for i in range(1000):\n        for j in range(900):\n            pass\n")
	if err == nil || !strings.Contains(err.Error(), "ran for more than") {
		t.Fatalf("err = %v, want the time limit", err)
	}
}

func TestScriptTimeoutAppliesToTheTopLevel(t *testing.T) {
	lowerLimits(t)
	limTimeout = time.Millisecond
	_, err := compileScript("t.star", "x = [i for i in range(900000)]\n", predeclaredFor(nil))
	if err == nil || !strings.Contains(err.Error(), "ran for more than") {
		t.Fatalf("err = %v, want the time limit", err)
	}
}

func TestScriptMemoryLimitStopsGrowth(t *testing.T) {
	lowerLimits(t)
	limMemory = 16 << 20
	_, err := evalMain("def main():\n    x = [0]\n    for i in range(26):\n        x = x + x\n    return len(x)\n")
	if err == nil || !strings.Contains(err.Error(), "memory") {
		t.Fatalf("err = %v, want the memory limit", err)
	}
}

func TestScriptThatStaysUnderTheMemoryLimitRuns(t *testing.T) {
	lowerLimits(t)
	got, err := evalMain("def main():\n    x = [0]\n    for i in range(10):\n        x = x + x\n    return len(x)\n")
	if err != nil || got.String() != "1024" {
		t.Fatalf("got %v, %v", got, err)
	}
}

// Each of these builds something huge in one call, and each is refused with
// the limits lowered so the test allocates almost nothing.
func TestAmplifiersAreStopped(t *testing.T) {
	lowerLimits(t)
	limSeq, limIntBits = 1000, 64
	for _, c := range []struct{ name, body, want string }{
		{"range", "return len(range(1001))", "range"},
		{"range with step", "return len(range(0, 10000, 3))", "range"},
		{"list(range)", "return len(list(range(100000)))", "range"},
		{"string repeat", "return len('ab' * 501)", "excessive repeat"},
		{"string repeat, reversed", "return len(501 * 'ab')", "excessive repeat"},
		{"list repeat", "return len([0] * 1001)", "excessive repeat"},
		{"tuple repeat", "return len((1, 2) * 501)", "excessive repeat"},
		{"bytes repeat", "return len(b'ab' * 501)", "excessive repeat"},
		{"repeat in a variable", "x = [1, 2, 3]\n    n = 400\n    return len(x * n)", "excessive repeat"},
		{"repeat by *=", "x = 'abc'\n    x *= 400\n    return len(x)", "excessive repeat"},
		{"repeat past int32", "return len('a' * (1 << 40))", "too large"},
		{"integer squaring", "x = 1 << 40\n    x = x * x\n    return x", "integer product"},
		{"format by %", "return len(('%s' * 200) % tuple(['x' * 100] * 200))", "formatted string"},
		{"format by % of a long string", "s = 'x' * 900\n    return len('%s%s%s' % (s, s, s))", "formatted string"},
		{"format of a list", "return '%s' % [1]", "cannot be formatted"},
		{"replace", "return len(('a' * 400).replace('a', 'bbbb'))", "replace: the result"},
		{"replace of nothing", "return len(('a' * 400).replace('', 'bbbb'))", "replace: the result"},
		{"join with a long separator", "return len(('x' * 600).join(['a', 'b', 'c']))", "join: the result"},
		{"join of long strings", "s = 'x' * 600\n    return len(','.join([s, s]))", "join: the result"},
		{"format", "return len(('{}' * 300).format(*range(300)))", "format: the result"},
		{"format of a long string", "s = 'x' * 600\n    return len('{}{}'.format(s, s))", "format: the result"},
		{"format of a list", "return '{}'.format([1])", "cannot be formatted"},
		{"join as a value", "f = ','.join\n    return f(['a'])", "join is not available"},
		{"replace by getattr", "return getattr('aaa', 'replace')('a', 'bb')", "replace is not available"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := evalMain("def main():\n    " + c.body + "\n")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want one naming %q", err, c.want)
			}
		})
	}
}

func TestSizedOperationsStillWork(t *testing.T) {
	got, err := evalMain(`
def main():
    x = [0] * 5
    s = "ab" * 3
    s *= 2
    n = 6 * 7
    n %= 5
    r = len(range(10)) + len(range(0, 100, 10))
    return [len(x), s, n, r, "%d-%s" % (3, "a"), "100%%" % (), ",".join(["a", "b"]), "aaa".replace("a", "bb", 2), "{}-{x}".format(1, x = "y"), "".join([])]
`)
	if err != nil || got.String() != `[5, "abababababab", 2, 20, "3-a", "100%", "a,b", "bbbba", "1-y", ""]` {
		t.Fatalf("got %v, %v", got, err)
	}
}

// A field called join on something that is not a string is an ordinary field.
func TestMethodNamesOnOtherValuesWork(t *testing.T) {
	pre := starlark.StringDict{"thing": starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"join": starlark.NewBuiltin("join", func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
			return starlark.MakeInt(7), nil
		}),
	})}
	globals, err := compileScript("t.star", "def main():\n    return thing.join(1)\n", pre)
	if err != nil {
		t.Fatal(err)
	}
	th := newThread("t.star")
	got, err := starlark.Call(th, globals["main"], nil, nil)
	if err != nil || got.String() != "7" {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := evalMain("def main():\n    return [1].join(2)\n"); err == nil || !strings.Contains(err.Error(), "no .join") {
		t.Errorf("err = %v, want a missing-method error", err)
	}
}

func TestAScriptMayNotUseTheGuardsNames(t *testing.T) {
	_, err := compileScript("t.star", "def "+guardMul+"(a, b):\n    return a\n", predeclaredFor(nil))
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("err = %v", err)
	}
}

// A built-in stuck in Go cannot be cancelled. The run is abandoned and the
// game gets an error back; no script runs after it.
func TestAStuckScriptIsAbandoned(t *testing.T) {
	lowerLimits(t)
	limTimeout, limGrace = 5*time.Millisecond, 20*time.Millisecond
	release := make(chan struct{})
	defer close(release)
	stuck := starlark.NewBuiltin("stuck", func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
		<-release
		return starlark.None, nil
	})
	th := newThread("t.star")
	err := runScript(th, func() error {
		_, err := starlark.Call(th, stuck, nil, nil)
		return err
	})
	if !errors.Is(err, errScriptHalted) || !strings.Contains(err.Error(), "abandoned") {
		t.Fatalf("err = %v, want the run abandoned", err)
	}
	if err := runScript(newThread("t.star"), func() error { return nil }); !errors.Is(err, errScriptHalted) {
		t.Fatalf("a later run: err = %v, want none to start", err)
	}
}

// Three small lists, each holding the one below a thousand times: a few
// kilobytes, and a billion elements as text.
func TestSharedStructureCannotBePrintedWhole(t *testing.T) {
	for _, call := range []string{"str(c)", "repr(c)", "fail(c)"} {
		_, err := evalMain("def main():\n    a = [0] * 1000\n    b = [a] * 1000\n    c = [b] * 1000\n    return " + call + "\n")
		if err == nil || !strings.Contains(err.Error(), "more than") {
			t.Errorf("%s: err = %v, want it refused", call, err)
		}
	}
	// print has no output, so it does no work at all.
	if _, err := evalMain("def main():\n    a = [0] * 1000\n    c = [[a] * 1000] * 1000\n    print(c)\n"); err != nil {
		t.Errorf("print: %v", err)
	}
}

func TestAGoPanicIsAFailedScript(t *testing.T) {
	boom := starlark.NewBuiltin("boom", func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
		var m map[string]int
		m["x"] = 1
		return starlark.None, nil
	})
	th := newThread("t.star")
	err := runScript(th, func() error {
		_, err := starlark.Call(th, boom, nil, nil)
		return err
	})
	if !errors.Is(err, errScriptCrash) {
		t.Fatalf("err = %v, want a recovered crash", err)
	}
	// And the top level.
	pre := starlark.StringDict{"boom": boom}
	if _, err := compileScript("t.star", "boom()\n", pre); !errors.Is(err, errScriptCrash) {
		t.Fatalf("top level: err = %v, want a recovered crash", err)
	}
}

// The names a script of each kind may see, beside Starlark's own built-ins. A
// new one must be added here on purpose.
func TestScriptSurfaceIsPinned(t *testing.T) {
	check := func(kind string, got starlark.StringDict, allowed ...string) {
		var names []string
		for n := range got {
			names = append(names, n)
		}
		slices.Sort(names)
		slices.Sort(allowed)
		if !slices.Equal(names, allowed) {
			t.Errorf("%s scripts see %v, want %v", kind, names, allowed)
		}
	}
	opts := &optionValues{}
	guard := []string{"range", "getattr", "str", "repr", "fail", "print", guardMul, guardMod, guardCall}
	check("pirate", guarded(predeclaredFor(opts)), append([]string{"rand", "day", "option"}, guard...)...)
	pre := starlark.StringDict{"option": opts.builtin()}
	check("region", guarded(pre), append([]string{"option"}, guard...)...)

	var universe []string
	for n := range starlark.Universe {
		universe = append(universe, n)
	}
	slices.Sort(universe)
	want := []string{"False", "None", "True", "abs", "any", "all", "bool", "bytes", "chr", "dict", "dir", "enumerate", "fail", "float", "getattr", "hasattr", "hash", "int", "len", "list", "max", "min", "ord", "print", "range", "repr", "reversed", "set", "sorted", "str", "tuple", "type", "zip"}
	slices.Sort(want)
	if !slices.Equal(universe, want) {
		t.Errorf("Starlark's built-ins are %v; a new one needs a look at what it can allocate", universe)
	}
}

func TestScriptsHaveNoLoadAndNoOptionalLanguageFeatures(t *testing.T) {
	if *scriptFileOptions != (syntax.FileOptions{}) {
		t.Fatalf("script file options are %+v, want every option off", *scriptFileOptions)
	}
	for name, src := range map[string]string{
		"load":           "load('other.star', 'x')\n",
		"while":          "def main():\n    while True:\n        pass\n",
		"recursion":      "def f():\n    f()\ndef main():\n    f()\n",
		"set":            "x = set([1])\n",
		"reassign":       "x = 1\nx = 2\n",
		"top-level loop": "for i in [1]:\n    pass\n",
		"top-level if":   "if True:\n    pass\n",
	} {
		if _, err := evalMain(src); err == nil {
			t.Errorf("%s: a script using it ran", name)
		}
	}
}

func TestScriptErrorTextIsCleaned(t *testing.T) {
	buf := captureWarnings(t)
	s := scriptRules
	_ = s
	dir := writeMod(t, t.TempDir(), "pirates", map[string]string{
		ManifestFile: manifest("pirates"),
		PiratesFile:  "def raid(realm, factions):\n    fail(\"\\x1b[2J\\x1b]0;pwned\\x07 \\u202e\")\n",
	})
	_, r, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	w, e := scriptWorld(r, 1)
	for range 3 {
		w.PlayTurn(e, "2026-11-23")
	}
	out := buf.String()
	if out == "" {
		t.Fatal("the failing script was not reported")
	}
	for _, r := range out {
		if r == '\x1b' || r == '\x07' || r == '\u202e' {
			t.Fatalf("the warning holds a raw control character: %q", out)
		}
	}
	if !strings.Contains(out, `\u001b`) {
		t.Errorf("the escape is not shown as text: %q", out)
	}
}

func TestCleanLeavesPlainTextAlone(t *testing.T) {
	if got := clean("Raid: 100% of the Tank-Corps (v1.2) naïve 日本"); !strings.HasPrefix(got, "Raid") || strings.Contains(got, `\u`) {
		t.Errorf("clean changed plain text: %q", got)
	}
}

func TestManifestFieldsRefuseTerminalEscapes(t *testing.T) {
	for _, field := range []string{"label", "description", "version"} {
		for _, bad := range []string{"\\u001b[2J", "\\u202e", "a\\nb"} {
			m := strings.Replace(manifest("evil"), "version = \"1.0\"", "version = \"1.0\"", 1)
			if field == "version" {
				m = strings.Replace(m, "version = \"1.0\"", "version = \"1"+bad+"\"", 1)
			} else {
				m += field + " = \"x" + bad + "\"\n"
			}
			dir := writeMod(t, t.TempDir(), "evil", map[string]string{ManifestFile: m})
			if _, err := Read(dir); err == nil || !strings.Contains(err.Error(), "control") {
				t.Errorf("%s %q: err = %v", field, bad, err)
			}
		}
	}
}

func TestOptionLabelsRefuseTerminalEscapes(t *testing.T) {
	m := manifest("evil") + "[options.x]\ntype = \"bool\"\nlabel = \"a\\u001b[2Jb\"\ndefault = true\n"
	dir := writeMod(t, t.TempDir(), "evil", map[string]string{ManifestFile: m})
	if _, err := Read(dir); err == nil {
		t.Fatal("an option label with an escape was read")
	}
	// A saved copy or a league order skips Read, and a screen reads it too.
	if _, err := ManifestFromRef(game.ModRef{Name: "evil", Files: map[string]string{ManifestFile: m}}); err == nil {
		t.Fatal("a saved copy's option label with an escape was shown")
	}
}

func TestAModFileOverTheLimitIsRefused(t *testing.T) {
	big := strings.Repeat("# x\n", MaxModFileSize/4+1)
	dir := writeMod(t, t.TempDir(), "big", map[string]string{ManifestFile: manifest("big"), PiratesFile: big})
	if _, err := Read(dir); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("a folder: err = %v", err)
	}
	ref := game.ModRef{Name: "big", Files: map[string]string{ManifestFile: manifest("big"), PiratesFile: big}}
	if _, err := LoadForLeague([]game.ModRef{ref}, true); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("a league order: err = %v", err)
	}
}

func TestAModWithTooManyFilesIsRefused(t *testing.T) {
	files := map[string]string{ManifestFile: manifest("many")}
	for i := range MaxModFiles + 1 {
		files["f"+string(rune('a'+i%26))+string(rune('a'+i/26))] = ""
	}
	ref := game.ModRef{Name: "many", Files: files}
	if _, err := LoadForLeague([]game.ModRef{ref}, true); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("err = %v", err)
	}
}

func TestALinkOutOfTheModFolderIsRefused(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "secret.star")
	if err := os.WriteFile(outside, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := writeMod(t, t.TempDir(), "linked", map[string]string{ManifestFile: manifest("linked")})
	if err := os.Symlink(outside, filepath.Join(dir, PiratesFile)); err != nil {
		t.Skip("no symlinks here:", err)
	}
	if _, err := Read(dir); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("err = %v", err)
	}
}

func TestALinkInsideTheModFolderIsRead(t *testing.T) {
	dir := writeMod(t, t.TempDir(), "linked", map[string]string{
		ManifestFile: manifest("linked"),
		"LICENSE":    "text\n",
	})
	if err := os.Symlink("LICENSE", filepath.Join(dir, "COPYING")); err != nil {
		t.Skip("no symlinks here:", err)
	}
	if _, err := Read(dir); err != nil {
		t.Fatal(err)
	}
}

// The cost of the limits on one call: a timer and a watchdog entry.
func BenchmarkRunScriptOverhead(b *testing.B) {
	th := newThread("b.star")
	for b.Loop() {
		_ = runScript(th, func() error { return nil })
	}
}

func BenchmarkScriptCallWithLimits(b *testing.B) {
	globals, err := compileScript("b.star", "def main(a):\n    return a + 1\n", predeclaredFor(nil))
	if err != nil {
		b.Fatal(err)
	}
	fn := globals["main"].(*starlark.Function)
	args := starlark.Tuple{starlark.MakeInt(1)}
	b.Run("guarded", func(b *testing.B) {
		for b.Loop() {
			th := newThread("b.star")
			_ = runScript(th, func() error { _, err := starlark.Call(th, fn, args, nil); return err })
		}
	})
	b.Run("bare", func(b *testing.B) {
		for b.Loop() {
			th := newThread("b.star")
			_, _ = starlark.Call(th, fn, args, nil)
		}
	})
}
