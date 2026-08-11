package menu

import (
	"regexp"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/screen"
)

var anyEscape = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

// screenRows counts the rows a rendered screen occupies, ignoring colour.
func screenRows(out string) int {
	out = anyEscape.ReplaceAllString(out, "")
	out = strings.TrimSuffix(out, "\n")
	if out == "" {
		return 0
	}
	return strings.Count(out, "\n") + 1
}

// A screen meant to be read in one piece must fit the caller's screen together
// with its own prompt — otherwise the top has scrolled off by the time the
// prompt appears, which is what happened to the splash (25 rows of art on a
// 24-row terminal) and to the unpaged Configuration Editor.
func TestSplashFitsTheScreen(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")}
	Splash(f)
	// The pause prompt sits on the row after the art and is part of the screen.
	if rows := screenRows(f.out.String()); rows > ansi.ScreenRows {
		t.Errorf("the splash and its prompt need %d rows, more than the %d assumed (trim rows from screens/splash.ans)",
			rows, ansi.ScreenRows)
	}
	// Width matters as much as height and was never checked: one column over
	// and the terminal wraps every art row, which shifts everything below it.
	for i, l := range strings.Split(anyEscape.ReplaceAllString(f.out.String(), ""), "\n") {
		if n := len([]rune(strings.TrimRight(l, "\r"))); n > ansi.ScreenCols {
			t.Errorf("splash line %d is %d columns, more than the %d assumed", i+1, n, ansi.ScreenCols)
		}
	}
}

// The empire status pages are .ans templates, so their height is fixed by the
// file rather than by anything at runtime.
func TestEmpireStatusPagesFitTheScreen(t *testing.T) {
	for _, page := range empireStatusPages {
		tpl, err := empireStatusFS.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		// Rendered through the same decoder the game uses, plus a row for the
		// pause prompt that follows each page.
		if rows := screenRows(screen.FromCP437(tpl)) + 1; rows > ansi.ScreenRows {
			t.Errorf("%s and its prompt need %d rows, more than the %d assumed", page, rows, ansi.ScreenRows)
		}
	}
}

// Every game menu has to fit too: the tree grows by hand, and a menu that runs
// past the screen loses its title before the player picks anything.
func TestGameMenusFitTheScreen(t *testing.T) {
	w := newWorld()
	c := &ctx{World: w.World, handle: "tester", Term: Term{UTF8: true}}
	menus := BuildMenus()
	for _, m := range []*Menu{menus.Game, menus.System, menus.Spending, menus.Attack, menus.Covert, menus.Diplomacy} {
		f := &fakeSession{}
		draw(f, c, m)
		// One row for the choice prompt the menu reads on.
		if rows := screenRows(f.out.String()) + 1; rows > ansi.ScreenRows {
			t.Errorf("menu %q needs %d rows, more than the %d assumed", m.Title, rows, ansi.ScreenRows)
		}
	}
}
