package menu

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/bulletin"
	"github.com/andy5995/immortal-barons/internal/session"
)

// The Game Bulletins screen: the league's bulletins under a Galactic heading,
// then the board's own, numbered straight on from the last galactic one so a
// player picks by number without minding which list a bulletin came from.
//
// The original keeps every bulletin in one bulletin.lst with ^NAME markers.
// IB uses a file per bulletin under bull/ instead (see internal/bulletin): a
// league's bulletins have to travel between boards as files anyway, and .ans
// artwork does not survive being pasted into a list file.

// bulletinLines is how many lines of a bulletin are shown before pausing.
const bulletinLines = 20

// gameBulletins lists the bulletins and shows whichever one is chosen.
func gameBulletins(s session.Session, w *ctx) Result {
	dataDir := w.Config.DataDir
	for {
		league, err := bulletin.List(dataDir, bulletin.League)
		if err != nil {
			fail(s, err)
			return Stay
		}
		local, err := bulletin.List(dataDir, bulletin.Local)
		if err != nil {
			fail(s, err)
			return Stay
		}
		all := append(append([]bulletin.Bulletin(nil), league...), local...)
		if len(all) == 0 {
			okNoPause(s, "There are no bulletins to read.")
			pause(s)
			return Stay
		}
		listBulletins(s, league, local)
		n := chooseBulletin(s, len(all))
		if n <= 0 {
			return Stay
		}
		showBulletinFile(s, all[n-1])
	}
}

// listBulletins draws the numbered list, each group under its own heading. A
// group with nothing in it gets no heading — a stand-alone board is not in a
// league and has no galactic bulletins to head.
func listBulletins(s session.Session, league, local []bulletin.Bulletin) {
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Game Bulletins"), ansi.Reset)
	n := 0
	group := func(heading string, items []bulletin.Bulletin) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(s, "\n%s  %s%s\n", ansi.FgBrightYellow, tr(s, heading), ansi.Reset)
		for _, b := range items {
			n++
			fmt.Fprintf(s, "%s  %2d)%s %s%s%s\n",
				ansi.FgBrightWhite, n, ansi.Reset, ansi.FgWhite, bulletinTitle(b), ansi.Reset)
		}
	}
	group("Galactic", league)
	group("Local", local)
	fmt.Fprintf(s, "\n%s   0)%s %s%s%s\n",
		ansi.FgBrightWhite, ansi.Reset, ansi.FgWhite, tr(s, "Quit"), ansi.Reset)
}

// bulletinTitle keeps a title inside the screen. A title comes from the first
// line of a file the sysop wrote, so nothing upstream has bounded its length.
func bulletinTitle(b bulletin.Bulletin) string {
	const max = 66
	title := []rune(b.Title)
	if len(title) <= max {
		return string(title)
	}
	return string(title[:max-3]) + "..."
}

// chooseBulletin reads the selection. Up to nine bulletins it is the menu
// engine's one-key prompt; past that a number needs more than one key, so the
// list falls back to reading a line, as the help browser's numbered list does.
func chooseBulletin(s session.Session, max int) int {
	if max <= 9 {
		return ChoiceQuit(s, max)
	}
	for {
		in := strings.TrimSpace(prompt(s, ">"))
		if in == "" {
			return 0 // Enter takes the shown Quit, as ChoiceQuit does
		}
		if q := unicode.ToLower([]rune(in)[0]); q == 'q' {
			return 0
		}
		n, err := strconv.Atoi(in)
		if err != nil || n < 0 || n > max {
			continue
		}
		return n
	}
}

// showBulletinFile prints one bulletin a screen at a time. Autowrap is off for
// the same reason the splash turns it off: ANSI artwork fills all 80 columns,
// and column 80 would otherwise wrap the cursor on top of the row's own CR/LF
// (see ansi.WrapOff).
func showBulletinFile(s session.Session, b bulletin.Bulletin) {
	data, err := os.ReadFile(b.Path)
	if err != nil {
		fail(s, err)
		return
	}
	fmt.Fprint(s, "\n", ansi.WrapOff)
	defer fmt.Fprint(s, ansi.WrapOn, ansi.Reset)
	shown := 0
	for _, line := range strings.Split(strings.ReplaceAll(bulletin.Text(data), "\r\n", "\n"), "\n") {
		fmt.Fprintf(s, "%s\n", line)
		shown++
		if shown < bulletinLines {
			continue
		}
		shown = 0
		fmt.Fprintf(s, "\n%s%s%s", ansi.FgBrightCyan, tr(s, "─»>Enter to continue, Q to quit<«─"), ansi.Reset)
		k, err := readKey(s)
		drainInput(s)
		fmt.Fprint(s, "\n")
		if err != nil || k == 'q' || k == 'Q' {
			return
		}
	}
	if shown > 0 { // lines remain since the last page break
		pause(s)
	}
}
