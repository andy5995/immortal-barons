package menu

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/numfmt"
	"github.com/andy5995/immortal-barons/internal/session"
)

// groupattacktable.go — the nine-column table Join Group Attack lists the
// forming strikes in (#251). Captured live, with colours, in
// cap/eots-ibbs-02.cap; the layout and every column width below are read off
// that capture rather than chosen here.
//
// What the table carries that a one-line summary cannot is the force ALREADY
// COMMITTED, split by unit type: whether a group has enough jets to be worth
// reinforcing is the whole decision the screen is asking about, and an offense
// total answers a different question.

// Column widths, left to right, from the capture. They come to 79 with the
// eight separators between them, which is what keeps the table inside an
// 80-column terminal without wrapping.
const (
	gaWidthID       = 2
	gaWidthBy       = 2
	gaWidthPlanet   = 14
	gaWidthTarget   = 19
	gaWidthTroopers = 8
	gaWidthJets     = 6
	gaWidthTanks    = 7
	gaWidthBombers  = 7
	gaWidthLeave    = 6
)

// gaColumns is the table read left to right: the heading over each column and
// the width it occupies. One list, so the head, the rule and every row are
// sized from the same place and cannot drift apart.
var gaColumns = []struct {
	head  string
	width int
}{
	{"Id", gaWidthID},
	{"By", gaWidthBy},
	{"Planet", gaWidthPlanet},
	{"Individual Target", gaWidthTarget},
	{"Troopers", gaWidthTroopers},
	{"Jets", gaWidthJets},
	{"Tanks", gaWidthTanks},
	{"Bombers", gaWidthBombers},
	{"Leave", gaWidthLeave},
}

// gaRow is one forming attack, gathered under the lock so the whole table
// reflects a single moment.
type gaRow struct {
	id       int    // the party's slot -- the two-column Id the table shows
	attack   int    // the world-wide GroupAttack.ID behind it, never displayed
	by       string // the creating realm's slot letter, its public identity
	planet   string // the target board
	target   string // the named baron, or the whole-planet stand-in
	troopers int
	jets     int
	tanks    int
	bombers  int
	hours    int // whole hours until the force leaves; -1 when unknown
}

// gaSep is the column separator, and gaCross the rule's junction under it. The
// plain-ASCII writer rewrites them as "|" and "+" below every layer that counts
// columns, so both still occupy one column on every charset.
const (
	gaSep   = "│"
	gaCross = "┼"
)

// printGroupAttackTable draws the head, the rule and one line per forming
// attack. Colours are the capture's: white headings and Id, bright-black
// separators and rule, a bright-yellow By letter, bright-white names, bright-cyan
// unit counts and a bright-green Leave.
func printGroupAttackTable(s session.Session, t Term, rows []gaRow) {
	var head, rule strings.Builder
	for i, c := range gaColumns {
		if i > 0 {
			head.WriteString(ansi.FgBrightBlack + gaSep + ansi.FgWhite)
			rule.WriteString(gaCross)
		}
		head.WriteString(gaPad(t, tr(s, c.head), c.width, true))
		rule.WriteString(strings.Repeat("─", c.width))
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgWhite, strings.TrimRight(head.String(), " "), ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightBlack, rule.String(), ansi.Reset)
	for _, r := range rows {
		printGroupAttackRow(s, t, r)
	}
}

// printGroupAttackRow draws one forming attack. A unit count is right-justified
// one column short of its cell and given a trailing space, which is how the
// capture spaces the figures off the separator to their right; the text columns
// are centred in theirs.
func printGroupAttackRow(s session.Session, t Term, r gaRow) {
	sep := ansi.FgBrightBlack + gaSep
	cells := []string{
		ansi.FgWhite + fmt.Sprintf("%*s", gaWidthID, gaSlot(r.id)),
		ansi.FgBrightYellow + padColumn(t, r.by, gaWidthBy),
		ansi.FgBrightWhite + gaPad(t, r.planet, gaWidthPlanet, false),
		ansi.FgBrightWhite + gaPad(t, r.target, gaWidthTarget, false),
		ansi.FgBrightCyan + gaFigure(numfmt.Short(r.troopers), gaWidthTroopers),
		ansi.FgBrightCyan + gaFigure(numfmt.Short(r.jets), gaWidthJets),
		ansi.FgBrightCyan + gaFigure(numfmt.Short(r.tanks), gaWidthTanks),
		ansi.FgBrightCyan + gaFigure(numfmt.Short(r.bombers), gaWidthBombers),
		ansi.FgBrightGreen + gaFigure(gaLeave(r.hours), gaWidthLeave),
	}
	fmt.Fprintf(s, "%s%s\n", strings.TrimRight(strings.Join(cells, sep), " "), ansi.Reset)
}

// gaSlot spells a party's Id. A party that could not be given one -- every
// number in use, which takes 99 parties at once -- shows "?" rather than a 0 no
// prompt would accept.
func gaSlot(slot int) string {
	if slot < 1 {
		return "?"
	}
	return strconv.Itoa(slot)
}

// gaFigure right-justifies a figure one column short of its cell, leaving the
// trailing column blank so the number never touches the separator beside it.
func gaFigure(text string, width int) string {
	return fmt.Sprintf("%*s ", width-1, text)
}

// gaPad centres text in width. Where the padding does not divide evenly the two
// callers want opposite halves, and the capture shows both: a HEADING keeps its
// odd column on the left ("Leave" sits under a 6-wide column as " Leave"), while
// a data cell keeps it on the right ("The Eclipse" sits one column in on a
// 14-wide Planet, not two).
//
// It clips first. Nothing caps a realm or board name at the width of a cell, and
// neither does a translation of a heading, so an over-long one would otherwise
// walk every column after it.
func gaPad(t Term, text string, width int, oddLeft bool) string {
	text = fitColumn(t, text, width)
	pad := width - visWidth(t, text)
	left := pad / 2
	if oddLeft {
		left = pad - pad/2
	}
	return strings.Repeat(" ", left) + text + strings.Repeat(" ", pad-left)
}

// gaLeave spells the wait the way the original does: whole hours and an "h",
// never a clock time. A clock time has to be subtracted from the current time
// before it means anything, and the current time is not on this screen.
//
// An attack saved before departures carried an hour (GroupAttack.DepartDay)
// knows only the game day it leaves on, which cannot be rendered in hours; it
// shows "?" rather than a figure invented for it.
func gaLeave(hours int) string {
	if hours < 0 {
		return "?"
	}
	return fmt.Sprintf("%dh", hours)
}

// hoursUntil is the whole hours a forming attack still has before it leaves,
// rounded UP so an attack filed with an eight-hour delay reads "8h" for its
// first hour rather than dropping to "7h" the instant it is created. A legacy
// attack that carries only a departure DAY returns -1, which gaLeave spells
// "?".
func hoursUntil(now time.Time, g game.GroupAttack) int {
	if g.DepartAt.IsZero() {
		return -1
	}
	left := g.DepartAt.Sub(now)
	if left <= 0 {
		return 0
	}
	return int((left + time.Hour - 1) / time.Hour)
}

// promptGroupChoice asks the original's own question -- "Join which group?",
// answered with the table's Id -- rather than the numbered-list convention the
// rest of IB's menus use. The whole point of #251 is that this screen is the
// original's table, and the table is addressed by Id.
//
// It MATCHES on the Id shown and returns the GroupAttack.ID behind it — which is
// what the join transaction re-resolves against — alongside the Id itself, for
// the notice that reports the join back in the number the player used. The two
// are different and only the second is ever on the screen.
//
// Empty input is 0, which cancels: the capture shows Enter echoing a 0 at this
// prompt. So does an Id no row on the table carries -- the answer names a party
// or it names nothing. A read error ends the session rather than returning 0
// forever, as every other prompt here does.
func promptGroupChoice(s session.Session, rows []gaRow) (attack, id int) {
	fmt.Fprintf(s, "\n%s%s %s", ansi.FgWhite, tr(s, "Join which group?"), ansi.FgBrightWhite)
	line, err := session.ReadLine(s)
	fmt.Fprint(s, ansi.Reset)
	if err != nil {
		session.End(err)
	}
	want := parseAmount(line, math.MaxInt)
	for _, r := range rows {
		if r.id == want && want != 0 {
			return r.attack, r.id
		}
	}
	return 0, 0
}
