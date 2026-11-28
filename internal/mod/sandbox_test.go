package mod

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/andy5995/immortal-barons/internal/game"
)

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

// The names a script of each kind may see, beside Starlark's own built-ins. A
// new one must be added here on purpose.
func TestScriptSurfaceIsPinned(t *testing.T) {
	names := func(d starlark.StringDict) []string {
		var out []string
		for n := range d {
			out = append(out, n)
		}
		slices.Sort(out)
		return out
	}
	opts := &optionValues{}
	if got, want := names(predeclaredFor(opts)), []string{"day", "option", "rand"}; !slices.Equal(got, want) {
		t.Errorf("pirate scripts see %v, want %v", got, want)
	}
	region := starlark.StringDict{"option": opts.builtin()}
	if got, want := names(region), []string{"option"}; !slices.Equal(got, want) {
		t.Errorf("region scripts see %v, want %v", got, want)
	}
	universe := names(starlark.Universe)
	want := []string{"False", "None", "True", "abs", "any", "all", "bool", "bytes", "chr", "dict", "dir", "enumerate", "fail", "float", "getattr", "hasattr", "hash", "int", "len", "list", "max", "min", "ord", "print", "range", "repr", "reversed", "set", "sorted", "str", "tuple", "type", "zip"}
	slices.Sort(want)
	if !slices.Equal(universe, want) {
		t.Errorf("Starlark's built-ins are %v; a new one needs a look at what it can do", universe)
	}
}

func TestScriptsHaveNoLoadAndNoOptionalLanguageFeatures(t *testing.T) {
	if *scriptFileOptions != (syntax.FileOptions{}) {
		t.Fatalf("script file options are %+v, want every option off", *scriptFileOptions)
	}
	tail := "def region(realm, row):\n    return {}\n"
	for name, src := range map[string]string{
		"load":           "load('other.star', 'x')\n",
		"while":          "def f():\n    while True:\n        pass\n",
		"recursion":      "def f():\n    f()\nf()\n",
		"set":            "x = set([1])\n",
		"reassign":       "x = 1\nx = 2\n",
		"top-level loop": "for i in [1]:\n    pass\n",
		"top-level if":   "if True:\n    pass\n",
	} {
		if _, err := regionAnswer(t, src+tail); err == nil {
			t.Errorf("%s: a script using it ran", name)
		}
	}
}

func TestErrorTextNamesAScriptValueByTypeOrAShortCleanString(t *testing.T) {
	long := strings.Repeat("x", 500)
	for _, c := range []struct {
		v    starlark.Value
		want string
		not  string
	}{
		{starlark.String(long), `"` + strings.Repeat("x", maxShown) + `..."`, strings.Repeat("x", maxShown+1)},
		{starlark.String("\x1b[2Jhi"), `"\\u001b[2Jhi"`, "\x1b"},
		{starlark.NewList([]starlark.Value{starlark.MakeInt(1)}), "a value of type list", "[1]"},
		{starlark.MakeInt(12345), "a value of type int", "12345"},
	} {
		got := show(c.v)
		if got != c.want || strings.Contains(got, c.not) {
			t.Errorf("show = %q, want %q", got, c.want)
		}
	}
}

func TestAHardLinkedFileIsRefused(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "passwd")
	if err := os.WriteFile(outside, []byte("root:x:0:0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := writeMod(t, t.TempDir(), "hard", map[string]string{ManifestFile: manifest("hard")})
	if err := os.Link(outside, filepath.Join(dir, PiratesFile)); err != nil {
		t.Skip("no hard links here:", err)
	}
	if _, err := Read(dir); err == nil || !strings.Contains(err.Error(), "hard link") {
		t.Fatalf("err = %v", err)
	}
}

// The open itself refuses a link, so a path swapped after the check leads
// nowhere.
func TestOpeningAModFileFollowsNoLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links need privileges here")
	}
	dir := t.TempDir()
	real, link := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	os.WriteFile(real, []byte("x"), 0o644)
	if err := os.Symlink(real, link); err != nil {
		t.Skip(err)
	}
	f, err := openModFile(link)
	if err == nil {
		f.Close()
		t.Skip("this system follows the link on open")
	}
}
