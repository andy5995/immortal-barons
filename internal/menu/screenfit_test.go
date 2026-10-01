package menu

import (
	"regexp"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
)

var anyEscape = regexp.MustCompile("\x1b\\[[0-9;?]*[A-Za-z]")

// screenRows counts the rows a rendered screen occupies, ignoring color.
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
	rows := splashRows()
	// The pause prompt sits on the row after the art and is part of the screen.
	if n := len(rows) + 1; n > ansi.ScreenRows {
		t.Errorf("the splash and its prompt need %d rows, more than the %d assumed (trim rows from screens/splash.ans)",
			n, ansi.ScreenRows)
	}
	// Width matters as much as height: one column over and the terminal wraps
	// every art row, which shifts everything below it.
	for i, l := range rows {
		if n := len([]rune(anyEscape.ReplaceAllString(l, ""))); n > ansi.ScreenCols {
			t.Errorf("splash row %d is %d columns, more than the %d assumed", i+1, n, ansi.ScreenCols)
		}
	}
}

// Every art row is placed by position, never reached by a line break: a
// terminal that wraps at column 80 regardless of DECAWM (mTelnet, NetRunner,
// RGTerm) turns a CR/LF after a full row into a blank line, which
// double-spaced the splash.
func TestSplashPlacesEveryRowByPosition(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")}
	Splash(f)
	out := f.out.String()
	rows := splashRows()
	for i := range rows {
		if !strings.Contains(out, ansi.MoveTo(i+1, 1)+splashBlack) {
			t.Errorf("row %d is not positioned on black", i+1)
		}
	}
	art := out[:strings.Index(out, ansi.MoveTo(len(rows)+1, 1))]
	if strings.ContainsAny(art, "\r\n") {
		t.Error("the art carries a line break; a full-width row followed by one double-spaces on some terminals")
	}
}

// The Empire Status block grows with the realm — a field appears as soon as its
// figure leaves zero, and the Military and Regions rows gain continuation lines
// — so it is measured at its tallest and widest, with everything a realm can
// hold. A row past the 80th column wraps on top of the next one; a block past
// the screen scrolls its own title away.
func TestEmpireStatusFitsTheScreen(t *testing.T) {
	w := newWorld()
	p := w.Player()
	p.Gold, p.Bank = 999_999_999, 999_999_999
	p.People, p.Food, p.Agents, p.HQ, p.SDI = 999_999, 999_999, 999_999, 99, 75
	p.Troopers, p.Jets, p.Turrets = 9_999_999, 9_999_999, 9_999_999
	p.Tanks, p.Bombers, p.Carriers = 9_999_999, 9_999_999, 9_999_999
	p.Regions = game.RegionMix{
		Coastal: 999_999, River: 999_999, Agricultural: 999_999, Desert: 999_999,
		Industrial: 999_999, Urban: 999_999, Mountain: 999_999, Technology: 999_999,
		Waste: 999_999,
	}
	p.Protection = 999

	out := empireStatusBlock(&fakeSession{}, w)
	// One row for the pause prompt that follows the block.
	if rows := screenRows(out) + 1; rows > ansi.ScreenRows {
		t.Errorf("the status block and its prompt need %d rows, more than the %d assumed", rows, ansi.ScreenRows)
	}
	// Under the last column, not up to it: painting column 80 makes the terminal
	// wrap by itself, on top of the row's own newline.
	for i, l := range strings.Split(anyEscape.ReplaceAllString(out, ""), "\n") {
		if n := len([]rune(l)); n >= ansi.ScreenCols {
			t.Errorf("status line %d is %d columns, and the screen is %d: %q", i+1, n, ansi.ScreenCols, l)
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

// The splash paints its empty sky with an explicit black, never the default
// background: NetRunner does not clear a 256-color background on ESC[49m or
// ESC[0m, and the next 256-color foreground paints it again.
func TestSplashNeverUsesTheDefaultBackground(t *testing.T) {
	if strings.Contains(string(splashANS), "\x1b[49m") {
		t.Error("screens/splash.ans uses ESC[49m; paint black as ESC[48;5;16m instead")
	}
}
