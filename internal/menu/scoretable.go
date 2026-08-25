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
			rows = append(rows, row{e.Name, e.Letter(), e.Alive, self, presenceOf(e, self, w.Today), e.Land, e.Score, nw})
		}
		lastMaster = w.LastMaster
	})
	// BRE-style scores screen (matches a live BRE scores screen): a game-name
	// banner, lettered (A)/(B) ids in slot order, Id / Empire Name / Territory /
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
		scoreTableRow(s, scoreID(r.letter), name, nameColor, r.presence, r.land, r.score, r.nw)
	}
	scoreTableRule(s)
	if lastMaster != "" {
		fmt.Fprintf(s, "\n"+tr(s, "Last Planetary Master: %s")+"\n", lastMaster)
	}
}

const (
	// scoreIDCellWidth is the id column. Three columns is the whole id: a realm
	// is addressed by its slot letter and a planet holds game.PlanetSlots realms,
	// so `(A)`..`(Y)` is every id there is. The attack picker leaves it EMPTY for
	// a realm that cannot be attacked, which this still holds in place.
	scoreIDCellWidth = 3
	// scoreNameWidth is the name column. It carries the presence suffix too, so
	// the figures beside it do not move when a baron comes or goes.
	scoreNameWidth  = 26
	scoreOnlineMark = "O"
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

// onlineMark renders the indicator, or a blank of the same width. Both the
// score tables and the Relations roster call it, so the two cannot drift into
// showing the same thing two ways.
//
// It is drawn as an inverse-video cell — a lit key against the dim table —
// rather than a colored glyph. Reverse plus a bright FOREGROUND is what gets a
// bright background here: the `10x` bright-background codes are an aixterm
// extension, and on the classic CP437 clients a BBS door actually meets, that
// high-intensity background bit means blink. Black on bright yellow measures
// 19.7:1 against the VGA palette and 19.6:1 against xterm's; plain `43m` yellow
// would be 4.0:1 on VGA and fail. The `O` still carries the meaning on its own
// for a monochrome or ANSI-less session.
// onlineMark marks a baron who is on the board: "(O)" set immediately to the
// LEFT of their name, or a blank of the same width so the name column holds in
// both states. A realm that merely played today but is not online now gets
// BRE's own "+" instead (docs/dev/bre-screens.md), right-justified in the same
// cell so it too hugs the name directly — matching the captured `(A)+Asgard`,
// not a `+` stranded against the id column with a gap before the name.
//
// The LETTER takes the lighter gray and the parens the darker one, not the other
// way round. Measured against a black background, gray (37) is 9.04:1 on the
// VGA/CP437 palette and 11.54:1 on xterm's, while dark gray (90) is 2.82:1 and
// 5.32:1 — so on the CP437 clients a BBS door actually meets, dark gray misses
// the 4.5:1 target by a wide margin. The parens are decoration and can carry
// that; the letter cannot, because it is the whole message.
// markWidth is the mark's column cost — "(O)" and whatever a translation makes
// of the letter — so headings and blanks can be sized from one place.
func markWidth(s session.Session) int { return len([]rune(tr(s, scoreOnlineMark))) + 2 }

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
func nameCell(s session.Session, name, nameColor string, presence string, width int) string {
	mark := markWidth(s)
	if room := width - mark; len([]rune(name)) > room {
		name = string([]rune(name)[:max(room, 0)])
	}
	return onlineMark(s, presence) + nameColor + name + ansi.Reset +
		strings.Repeat(" ", max(width-mark-len([]rune(name)), 0))
}

func idCell(id string) string {
	// BRE: magenta parens, bright-white letter — e.g. 35( 1;37A 35)
	inner := strings.Trim(id, "()")
	if inner == "" {
		return strings.Repeat(" ", scoreIDCellWidth)
	}
	return ansi.FgMagenta + "(" + ansi.FgBrightWhite + inner + ansi.FgMagenta + ")" + ansi.Reset +
		strings.Repeat(" ", max(scoreIDCellWidth-len(id), 0))
}

// The scores table is drawn on three screens — the Scores display, the attack
// target picker, and the recipient picker — so its geometry and colors live
// here once. They previously carried a copy each of the same format strings,
// which is how a change to one silently leaves the others behind.
func scoreTableRule(s session.Session) {
	fmt.Fprintf(s, "%s\n", rosterRule(ansi.FgMagenta))
}

func scoreTableHead(s session.Session) {
	// The heading sits over the names, which the online mark's gutter indents.
	fmt.Fprintf(s, "%s%-*s%-*s %10s %11s %11s%s\n",
		ansi.FgBrightWhite, scoreIDCellWidth, tr(s, "Id"),
		scoreNameWidth, strings.Repeat(" ", markWidth(s))+tr(s, "Empire Name"),
		tr(s, "Territory"), tr(s, "Score"), tr(s, "Net Worth"), ansi.Reset)
	scoreTableRule(s)
}

func scoreTableRow(s session.Session, id, name, nameColor string, presence string, land, score, nw int) {
	scoreTableRowStr(s, id, name, nameColor, presence,
		strconv.Itoa(land), strconv.Itoa(score), strconv.Itoa(nw))
}

// scoreTableRowStr is scoreTableRow with the three figures already rendered, for
// the screens that comma-group them. The columns and colours are the shared
// part and the one that drifts; how a number is spelled is each screen's own
// choice, and they do not currently agree — the scores board and the target
// list print bare figures where the recipient picker groups them.
func scoreTableRowStr(s session.Session, id, name, nameColor, presence, land, score, nw string) {
	fmt.Fprintf(s, "%s%s %s%10s%s %s%11s%s %s%11s%s\n",
		idCell(id),
		nameCell(s, name, nameColor, presence, scoreNameWidth),
		ansi.FgBrightMagenta, land, ansi.Reset,
		ansi.FgBrightWhite, score, ansi.Reset,
		ansi.FgWhite, nw, ansi.Reset)
}

// scoreID is the parenthesised id for a scores row: the realm's own slot letter
// (game.Empire.Letter), never its position in the sorted table. BRE prints the
// same — its captured See Scores board runs (A), (B), (E), skipping the letters
// of the realms that have fallen (docs/dev/bre-screens.md) — and it is what
// makes the key that mailed a realm yesterday reach the same realm today.
func scoreID(letter string) string { return "(" + letter + ")" }
