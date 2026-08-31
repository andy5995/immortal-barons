package menu

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/numfmt"
	"github.com/andy5995/immortal-barons/internal/session"
)

// scoretable.go — the roster table BRE draws on every screen that lists
// realms: the scores board, the recipient picker and the attack target list.
// One column layout, so a screen cannot quietly grow its own.

// ScoreRow is one realm's line on the scoreboard, gathered once so every
// rendering of it -- the screen, the bulletin file, the HTML page -- reads the
// same figures rather than each walking the world itself (#233).
type ScoreRow struct {
	Name            string
	Letter          string
	Alive, IsPlayer bool
	Presence        string
	Protected       bool
	Land, Score, NW int
}

// scoreRows snapshots every empire's rank inputs together, so a board reflects
// one consistent moment even if another session mutates the world mid-render.
func scoreRows(w *ctx) (rows []ScoreRow, lastMaster string) {
	w.Read(func() {
		rows = make([]ScoreRow, 0, len(w.Empires))
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
			rows = append(rows, ScoreRow{e.Name, e.Letter(), e.Alive, self, presenceOf(e, self, w.Today), e.Alive && e.Protection > 0, e.Land, e.Score, nw})
		}
		lastMaster = w.LastMaster
	})
	return rows, lastMaster
}

func printScores(s session.Session, w *ctx) {
	rows, lastMaster := scoreRows(w)
	// BRE-style scores screen (matches a live BRE scores screen): a game-name
	// banner, lettered [A]/[B] ids in slot order, Id / Empire Name / Territory /
	// Score / Net Worth columns, magenta header/footer rules. IB-branded.
	fmt.Fprintf(s, "\n%s-*%s%s%s%s*-%s\n\n",
		ansi.FgBrightMagenta, ansi.FgBrightWhite, tr(s, "Immortal Barons"), ansi.Reset, ansi.FgBrightMagenta, ansi.Reset)
	scoreTableHead(s, w.Term)
	for _, r := range rows {
		name := r.Name
		if !r.Alive {
			name += " " + tr(s, "(dead)")
		}
		nameColor := ansi.FgBrightWhite
		if r.IsPlayer {
			nameColor = ansi.FgBrightYellow // highlight the caller's own realm
		}
		scoreTableRow(s, w.Term, scoreID(r.Letter, r.Protected), name, nameColor, r.Presence, r.Land, r.Score, r.NW)
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

// markWidth is the mark's column cost — "(O)" and whatever a translation makes
// of the letter — so headings and blanks can be sized from one place.
func markWidth(s session.Session, t Term) int { return visWidth(t, tr(s, scoreOnlineMark)) + 2 }

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
func onlineMark(s session.Session, t Term, presence string) string {
	width := markWidth(s, t)
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
//
// Every width here is measured on the CALLER's terminal, not in runes.
// ValidRealmName accepts any printable rune, so a realm may legally be named
// "Iron—Fist"; the CP437 and plain-ASCII writers rewrite that em dash as two
// hyphens below every layer that counts columns, and a rune count would have
// walked the figures one column right for the default charset (the same defect
// as #192 and #196). Hence Term rather than the session — see fitColumn.
func nameCell(s session.Session, t Term, name, nameColor string, presence string, width int) string {
	mark := markWidth(s, t)
	name = fitColumn(t, name, max(width-mark, 0))
	return onlineMark(s, t, presence) + nameColor + name + ansi.Reset +
		strings.Repeat(" ", max(width-mark-visWidth(t, name), 0))
}

// idCell draws the id in BRE's colours — magenta brackets, bright-white letter,
// e.g. 35( 1;37A 35) — keeping whichever pair scoreID chose, since the pair is
// what says whether the realm is shielded.
func idCell(id string) string {
	inner := strings.Trim(id, "()[]")
	if inner == "" {
		return strings.Repeat(" ", scoreIDCellWidth)
	}
	open, close := "(", ")"
	if strings.HasPrefix(id, "[") {
		open, close = "[", "]"
	}
	return ansi.FgMagenta + open + ansi.FgBrightWhite + inner + ansi.FgMagenta + close + ansi.Reset +
		strings.Repeat(" ", max(scoreIDCellWidth-len(id), 0))
}

// The scores table is drawn on three screens — the Scores display, the attack
// target picker, and the recipient picker — so its geometry and colors live
// here once. They previously carried a copy each of the same format strings,
// which is how a change to one silently leaves the others behind.
func scoreTableRule(s session.Session) {
	fmt.Fprintf(s, "%s\n", rule75(ansi.FgMagenta))
}

func scoreTableHead(s session.Session, t Term) {
	// The heading sits over the names, which the online mark's gutter indents.
	fmt.Fprintf(s, "%s%-*s%-*s %10s %11s %11s%s\n",
		ansi.FgBrightWhite, scoreIDCellWidth, tr(s, "Id"),
		scoreNameWidth, strings.Repeat(" ", markWidth(s, t))+tr(s, "Empire Name"),
		tr(s, "Territory"), tr(s, "Score"), tr(s, "Net Worth"), ansi.Reset)
	scoreTableRule(s)
}

// The three figures are spelled the way BRE spells them, and the three columns
// do NOT agree with each other: Territory is grouped in full, while Score and
// Net Worth are shortened to a bare "k" or "m" once they reach 10,000
// (numfmt.Short — BINARY-VERIFIED, and see the capture rows quoted there).
// That is the original's own split: its show_player_list calls the shortening
// helper for the last two columns and not for the first.
//
// Every screen that draws this table goes through here, so they cannot drift
// apart again. They had: the scores board and the target list printed bare
// figures where the recipient picker grouped them.
func scoreTableRow(s session.Session, t Term, id, name, nameColor string, presence string, land, score, nw int) {
	lang := sessionLang(s)
	scoreTableRowStr(s, t, id, name, nameColor, presence,
		numfmt.GroupLong(land, lang), numfmt.Short(score), numfmt.Short(nw))
}

// scoreTableRowStr is scoreTableRow with the three figures already rendered,
// for a caller that has them as strings. The columns and colours are the shared
// part and the one that drifts.
func scoreTableRowStr(s session.Session, t Term, id, name, nameColor, presence, land, score, nw string) {
	fmt.Fprintf(s, "%s%s %s%10s%s %s%11s%s %s%11s%s\n",
		idCell(id),
		nameCell(s, t, name, nameColor, presence, scoreNameWidth),
		ansi.FgBrightMagenta, land, ansi.Reset,
		ansi.FgBrightWhite, score, ansi.Reset,
		ansi.FgWhite, nw, ansi.Reset)
}

// scoreID is the id for a scores row: the realm's own slot letter
// (game.Empire.Letter), never its position in the sorted table. BRE letters the
// same rows — its captured See Scores board runs A, B, E, skipping the letters
// of the realms that have fallen (docs/dev/bre-screens.md) — and that is what
// makes the key that mailed a realm yesterday reach the same realm today.
//
// The BRACKETS ARE THE PROTECTION FLAG (#214), and IB's own addition: a realm
// under New Realm Protection wears [C] where every other realm wears (C). One
// glyph does the whole job, so a shielded realm needs no marker after its name
// and the row stays the width it was. The shape carries it, not the colour, so
// it survives a monochrome terminal and a reader who cannot tell the two colours
// apart. Everything unshielded keeps BRE's own parentheses.
func scoreID(letter string, protected bool) string {
	if protected {
		return "[" + letter + "]"
	}
	return "(" + letter + ")"
}
