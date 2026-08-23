package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/bulletin"
)

// bulletinCtx points a test world's data directory at a temp dir and fills the
// two bulletin scopes.
func bulletinCtx(t *testing.T) *ctx {
	t.Helper()
	w := newWorld()
	w.Config.DataDir = t.TempDir()
	for _, b := range []struct {
		scope      bulletin.Scope
		name, body string
	}{
		{bulletin.League, "10-rules.txt", "League rules\nNo bullying.\n"},
		{bulletin.League, "20-welcome.txt", "Welcome, barons\nGood luck.\n"},
		{bulletin.Local, "board.txt", "News from the board\nWe are back up.\n"},
	} {
		if err := bulletin.Write(w.Config.DataDir, b.scope, b.name, []byte(b.body)); err != nil {
			t.Fatal(err)
		}
	}
	return w
}

// TestBulletinListNumbersLocalOnFromTheGalacticOnes is the listing contract: the
// league's bulletins come first under the Galactic heading, and the board's own
// carry on from the last number rather than starting at 1 again.
func TestBulletinListNumbersLocalOnFromTheGalacticOnes(t *testing.T) {
	w := bulletinCtx(t)
	f := &fakeSession{keys: []rune("0")} // read the list, then quit
	gameBulletins(f, w)
	out := stripANSI(f.out.String())
	for _, want := range []string{"Galactic", "1) League rules", "2) Welcome, barons",
		"Local", "3) News from the board", "0) Quit"} {
		if !strings.Contains(out, want) {
			t.Errorf("the bulletin list is missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "Galactic") > strings.Index(out, "Local") {
		t.Errorf("the local heading comes before the galactic one:\n%s", out)
	}
}

// TestReadingABulletinPrintsTheFile: choosing a number shows that file's body,
// not just its title from the list.
func TestReadingABulletinPrintsTheFile(t *testing.T) {
	w := bulletinCtx(t)
	f := &fakeSession{keys: []rune("1 0")} // read #1, dismiss its pause, quit
	gameBulletins(f, w)
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "No bullying.") {
		t.Errorf("the chosen bulletin's body was not shown:\n%s", out)
	}
	if strings.Contains(out, "We are back up.") {
		t.Errorf("a bulletin nobody chose was shown too:\n%s", out)
	}
}

// A board with an empty bull/ says so rather than drawing a headless list.
func TestNoBulletinsSaysSo(t *testing.T) {
	w := newWorld()
	w.Config.DataDir = t.TempDir()
	f := &fakeSession{keys: []rune(" ")}
	gameBulletins(f, w)
	if !strings.Contains(f.out.String(), "no bulletins") {
		t.Errorf("an empty bulletin directory rendered:\n%s", f.out.String())
	}
}

// The Game Bulletins item on the opening menu reaches this screen — the item
// used to be wired to Today's News.
func TestGameBulletinsItemOpensTheBulletinScreen(t *testing.T) {
	menus := BuildMenus()
	f := &fakeSession{keys: []rune("8 0")}
	w := bulletinCtx(t)
	if err := Run(f, w, menus.Game); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(f.out.String(), "League rules") {
		t.Errorf("item 8 did not open the bulletin list:\n%s", f.out.String())
	}
}
