package bulletin

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir string, scope Scope, name, body string) {
	t.Helper()
	if err := Write(dir, scope, name, []byte(body)); err != nil {
		t.Fatal(err)
	}
}

func TestTitleComesFromTheFirstLine(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		file string
		want string
	}{
		{"plain", []byte("League rules\nBe nice.\n"), "rules.txt", "League rules"},
		{"colored", []byte("\x1b[1;36mWelcome, barons\x1b[0m\nbody\n"), "welcome.ans", "Welcome, barons"},
		{"crlf", []byte("Board news\r\nbody\r\n"), "news.txt", "Board news"},
		// A first line of block art names nothing, so the file has to.
		{"art", []byte("\xdb\xdb\xdb\xdb\n\x1b[0mtext\n"), "banner.ans", "banner"},
		{"empty first line", []byte("\nlater\n"), "quiet.txt", "quiet"},
	}
	for _, c := range cases {
		if got := Title(c.data, c.file); got != c.want {
			t.Errorf("%s: Title = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSafeNameRejectsAnythingThatEscapesTheDirectory(t *testing.T) {
	good := []string{"welcome.txt", "news.ans", "Rules-2.TXT"}
	for _, n := range good {
		if !SafeName(n) {
			t.Errorf("SafeName(%q) = false, want true", n)
		}
	}
	// A league bulletin's name arrives from another board, so a name that walks
	// out of bull/league, hides itself, or is not a bulletin at all is refused.
	bad := []string{"", "..", ".", "../../world.json", "sub/dir.txt", `..\win.txt`,
		".hidden.txt", "notes.md", "script.sh", "world.json", "bell\x07.txt"}
	for _, n := range bad {
		if SafeName(n) {
			t.Errorf("SafeName(%q) = true, want false", n)
		}
	}
}

func TestTextDecodesCP437ArtButKeepsUTF8(t *testing.T) {
	// 0xDB is CP437's full block; it is not valid UTF-8, so the file is art.
	if got := Text([]byte("\xdb\xdb")); got != "██" {
		t.Errorf("CP437 art decoded to %q, want %q", got, "██")
	}
	// A sysop typing on a modern editor gets what they typed.
	if got := Text([]byte("Grüße")); got != "Grüße" {
		t.Errorf("UTF-8 text decoded to %q", got)
	}
}

func TestListSortsByNameAndSkipsWhatIsNotABulletin(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, Local, "b-second.txt", "Second\n")
	write(t, dir, Local, "a-first.ans", "First\n")
	// Written past Write's own guard: a sysop's stray files share the directory.
	if err := os.WriteFile(filepath.Join(Dir(dir, Local), "notes.md"), []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := List(dir, Local)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("List returned %d bulletins, want 2: %+v", len(got), got)
	}
	if got[0].Name != "a-first.ans" || got[0].Title != "First" {
		t.Errorf("first listed is %+v", got[0])
	}
	if got[1].Title != "Second" {
		t.Errorf("second listed is %+v", got[1])
	}
}

func TestListOfAMissingDirectoryIsEmptyNotAnError(t *testing.T) {
	got, err := List(t.TempDir(), League)
	if err != nil || got != nil {
		t.Errorf("List of a board with no bulletins = %v, %v", got, err)
	}
}

func TestWriteReplacesAndRemoveDeletes(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, League, "rules.txt", "Rules\nv1\n")
	write(t, dir, League, "rules.txt", "Rules\nv2\n")
	data, err := os.ReadFile(filepath.Join(Dir(dir, League), "rules.txt"))
	if err != nil || string(data) != "Rules\nv2\n" {
		t.Fatalf("rewritten bulletin is %q (%v)", data, err)
	}
	// The temp file the atomic write used must not be left listed.
	if got, _ := List(dir, League); len(got) != 1 {
		t.Errorf("directory holds %+v, want one bulletin", got)
	}
	if err := Remove(dir, League, "rules.txt"); err != nil {
		t.Fatal(err)
	}
	if got, _ := List(dir, League); len(got) != 0 {
		t.Errorf("after Remove the directory holds %+v", got)
	}
	if err := Remove(dir, League, "rules.txt"); err != nil {
		t.Errorf("removing an absent bulletin is an error: %v", err)
	}
	if err := Write(dir, League, "../escape.txt", []byte("x")); err == nil {
		t.Error("Write accepted a name that leaves the directory")
	}
}
