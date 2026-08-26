package menu

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// scoretable.go — the roster table BRE draws on every screen that lists
// realms: the scores board, the recipient picker and the attack target list.
// One column layout, so a screen cannot quietly grow its own.

func printScores(s session.Session, w *ctx) {
	// Snapshot every empire's rank inputs together so the board reflects one
	// consistent moment, even if another session mutates the world mid-render.
	type row struct {
		name            string
		letter          string
		alive, isPlayer bool
		presence        string
		protected       bool
		land, score, nw int
	}
	var rows []row
	var lastMaster string
	w.With(func() {
		rows = make([]row, 0, len(w.Empires))
		for _, e := range w.Empires {
			nw := w.NetWorth(e)
			if !e.Alive {
				nw = 0 // BRE's scores screen values a dead realm at nothing
			}
			// Net Worth is the asset value (land + military). Score is BRE's
			// cumulative metric (Empire.Score): the day-start net worth awarded per
			// turn played, minus small riot/spoilage dings — separate from wealth.
			// Never mark your own realm: (O) is redundant — you know you
			// are here, and the row is already singled out by its
			// bright-yellow name. The '+' played-today marker is also
			// suppressed: your own status is obvious from context.
			self := e == w.Player()
			rows = append(rows, row{e.Name, e.Letter(), e.Alive, self, presenceOf(e, self, w.Today), e.Alive && e.Protection > 0, e.Land, e.Score, nw})
		}
		lastMaster = w.LastMaster
	})
	// BRE-style scores screen (matches a live BRE scores screen): a game-name
	// banner, lettered [A]/[B] ids in slot order, Id / Empire Name / Territory /
	// Score / Net Worth columns, magenta header/footer rules. IB-branded.
	fmt.Fprintf(s, "\n%s-*%s%s%s%s*-%s\n\n",
		ansi.FgBrightMagenta, ansi.FgBrightWhite, tr(s, "Immortal Barons"), ansi.Reset, ansi.FgBrightMagenta, ansi.Reset)
	scoreTableHead(s)
	for _, r := range rows {
		name := r.name
		if !r.alive {
			name += " " + tr(s, "(dead)")
		}
		nameColor := ansi.FgBrightWhite
		if r.isPlayer {
			nameColor = ansi.FgBrightYellow // highlight the caller's own realm
		}
		scoreTableRow(s, scoreID(r.letter), name, nameColor, r.presence, r.protected, r.land, r.score, r.nw)
	}
	scoreTableRule(s)
	if lastMaster != "" {
		fmt.Fprintf(s, "\n"+tr(s, "Last Planetary Master: %s")+"\n", lastMaster)
	}
}

const (
	// scoreIDCellWidth is the id column. Three columns is the whole id: a realm
	// is addressed by its slot letter and a planet holds game.PlanetSlots realms,
	// so `[A]`..`[Y]` is every id there is. The attack picker leaves it EMPTY for
	// a realm that cannot be attacked, which this still holds in place.
	scoreIDCellWidth = 3
	// scoreNameWidth is the name column. It carries the presence suffix too, so
	// the figures beside it do not move when a baron comes or goes.
	scoreNameWidth  = 26
	scoreOnlineMark = "O"
	// scoreProtectedMark is the letter inside the New Realm Protection flag,
	// drawn AFTER the realm's name. Translatable like scoreOnlineMark: it is an
	// initial, and P is not the initial of the phrase in every language.
	scoreProtectedMark = "P"
)

// presence captures a realm's activity state for display in score tables and
// rosters. It is computed once when a row is gathered, not re-evaluated at
// render time.
const (
	presenceOnline = "online"
	presencePlayed = "played"
	presenceNone   = ""
)

// presenceOf returns the display presence for an empire relative to the caller.
// Self is always suppressed — a deliberate divergence from BRE, which shows the
// `+` on the caller's own row too (docs/dev/bre-screens.md).
func presenceOf(e *game.Empire, self bool, today string) string {
	if self {
		return presenceNone
	}
	if e.Online() {
		return presenceOnline
	}
	if e.LastPlayed == today {
		return presencePlayed
	}
	return presenceNone
}

// protectedMarkWidth is the protection flag's column cost — the translated
// letter, its parentheses, and the space that sets it off from the name.
func protectedMarkWidth(s session.Session) int {
	return len([]rune(tr(s, scoreProtectedMark))) + 3
}

// protectedMark flags a realm still under New Realm Protection, one space clear
// of the name it belongs to, and renders NOTHING at all for a realm that is not
// — the same shape as the pirate raider mark, which reserved its column on every
// row until that read as an indent for no reason (#197).
//
// It follows the name where the online mark leads it, so the two never collide,
// and it borrows that mark's split: the letter bright white (21.0:1 against
// black on both the VGA/CP437 and xterm palettes) because it is the whole
// message, the parentheses magenta (3.29:1 VGA, 4.48:1 xterm) because they are
// decoration and only have the 3:1 non-text minimum to clear. The letter still
// carries the meaning on a monochrome or ANSI-less session.
func protectedMark(s session.Session, protected bool) string {
	if !protected {
		return ""
	}
	return " " + ansi.FgMagenta + "(" + ansi.FgBrightWhite + tr(s, scoreProtectedMark) +
		ansi.FgMagenta + ")" + ansi.Reset
}

// markWidth is the mark's column cost — "(O)" and whatever a translation makes
// of the letter — so headings and blanks can be sized from one place.
func markWidth(s session.Session) int { return len([]rune(tr(s, scoreOnlineMark))) + 2 }

// onlineMark marks a baron who is on the board: "(O)" set immediately to the
// LEFT of their name, or a blank of the same width so the name column holds in
// both states. A realm that merely played today but is not online now gets
// BRE's own "+" instead (docs/dev/bre-screens.md), right-justified in the same
// cell so it too hugs the name directly — matching the captured `(A)+Asgard`,
// not a `+` stranded against the id column with a gap before the name.
//
// Both the score tables and the Relations roster call it, so the two cannot
// drift into showing the same thing two ways.
//
// The LETTER is bright white and the parentheses magenta, not the other way
// round. Measured against black: bright white is 21.0:1 on both the VGA/CP437
// and xterm palettes, while magenta is 3.29:1 on VGA and 4.48:1 on xterm. The
// parentheses are decoration and clear the 3:1 non-text minimum; the letter
// cannot be the dim one, because it is the whole message — and it still carries
// that message on a monochrome or ANSI-less session.
func onlineMark(s session.Session, presence string) string {
	width := markWidth(s)
	switch presence {
	case presenceOnline:
		return ansi.FgMagenta + "(" + ansi.FgBrightWhite + tr(s, scoreOnlineMark) +
			ansi.FgMagenta + ")" + ansi.Reset
	case presencePlayed:
		return strings.Repeat(" ", width-1) + ansi.FgMagenta + "+" + ansi.Reset
	default:
		return strings.Repeat(" ", width)
	}
}

// nameCell renders the presence mark hugging a realm's name on the
// left, then pads the pair to width. It measures the PLAIN text: the mark
// carries escapes that a width verb counts as characters, which is what would
// otherwise ragged the column beside it.
//
// It also CLIPS a name too long for the cell. Nothing caps a realm's name at
// onboarding — only a three-character minimum is enforced — so a long one used
// to push the figures beside it out of true. The mark survives the clip: it is
// the part carrying information the row cannot show twice.
func nameCell(s session.Session, name, nameColor string, presence string, protected bool, width int) string {
	mark := markWidth(s)
	if protected {
		mark += protectedMarkWidth(s)
	}
	if room := width - mark; len([]rune(name)) > room {
		name = string([]rune(name)[:max(room, 0)])
	}
	return onlineMark(s, presence) + nameColor + name + ansi.Reset + protectedMark(s, protected) +
		strings.Repeat(" ", max(width-mark-len([]rune(name)), 0))
}

func idCell(id string) string {
	// Magenta brackets, bright-white letter — e.g. 35[ 1;37A 35]
	inner := strings.Trim(id, "[]")
	if inner == "" {
		return strings.Repeat(" ", scoreIDCellWidth)
	}
	return ansi.FgMagenta + "[" + ansi.FgBrightWhite + inner + ansi.FgMagenta + "]" + ansi.Reset +
		strings.Repeat(" ", max(scoreIDCellWidth-len(id), 0))
}

// The scores table is drawn on three screens — the Scores display, the attack
// target picker, and the recipient picker — so its geometry and colors live
// here once. They previously carried a copy each of the same format strings,
// which is how a change to one silently leaves the others behind.
func scoreTableRule(s session.Session) {
	fmt.Fprintf(s, "%s\n", rule75(ansi.FgMagenta))
}

func scoreTableHead(s session.Session) {
	// The heading sits over the names, which the online mark's gutter indents.
	fmt.Fprintf(s, "%s%-*s%-*s %10s %11s %11s%s\n",
		ansi.FgBrightWhite, scoreIDCellWidth, tr(s, "Id"),
		scoreNameWidth, strings.Repeat(" ", markWidth(s))+tr(s, "Empire Name"),
		tr(s, "Territory"), tr(s, "Score"), tr(s, "Net Worth"), ansi.Reset)
	scoreTableRule(s)
}

func scoreTableRow(s session.Session, id, name, nameColor string, presence string, protected bool, land, score, nw int) {
	scoreTableRowStr(s, id, name, nameColor, presence, protected,
		strconv.Itoa(land), strconv.Itoa(score), strconv.Itoa(nw))
}

// scoreTableRowStr is scoreTableRow with the three figures already rendered, for
// the screens that comma-group them. The columns and colours are the shared
// part and the one that drifts; how a number is spelled is each screen's own
// choice, and they do not currently agree — the scores board and the target
// list print bare figures where the recipient picker groups them.
func scoreTableRowStr(s session.Session, id, name, nameColor, presence string, protected bool, land, score, nw string) {
	fmt.Fprintf(s, "%s%s %s%10s%s %s%11s%s %s%11s%s\n",
		idCell(id),
		nameCell(s, name, nameColor, presence, protected, scoreNameWidth),
		ansi.FgBrightMagenta, land, ansi.Reset,
		ansi.FgBrightWhite, score, ansi.Reset,
		ansi.FgWhite, nw, ansi.Reset)
}

// scoreID is the bracketed id for a scores row: the realm's own slot letter
// (game.Empire.Letter), never its position in the sorted table. BRE letters the
// same rows — its captured See Scores board runs A, B, E, skipping the letters
// of the realms that have fallen (docs/dev/bre-screens.md) — and that is what
// makes the key that mailed a realm yesterday reach the same realm today.
//
// The BRACKETS are IB's, a recorded divergence: BRE parenthesises the id on this
// table and brackets it on -*Relations*-, and one game should not spell the same
// id two ways. Brackets won because parentheses now say something else here — a
// realm's status flags, (O) online and (P) protected, are parenthesised, so the
// shape alone separates the key you press from the state you are being told.
func scoreID(letter string) string { return "[" + letter + "]" }
