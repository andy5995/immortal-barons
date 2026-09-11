package menu

import (
	"fmt"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// incoming.go — the Incoming view: what other planets have aimed at this one,
// counting down live (#268).
//
// IB's own screen. The original has no such list: its warnings are news lines
// and nothing else, so a baron reading "leaves in 12 hours" six hours later has
// to know when the line was written to make anything of it. The lines stay
// exactly as they are — a frozen log is the right shape for a log — and this
// screen reads the records that ride beside them.
//
// It is not a new source of knowledge. Every row came from a SpyGuy posted on
// the planet it names, or from a weapon already in the air and therefore in
// plain sight; a planet with no watcher out sees an empty list, exactly as it
// learns nothing today.

// Column widths. Sized to this screen's own content, as BRE sizes each of its
// boxes: 2 of indent and four columns with a space between them come to the
// 62-column house rule the menus are drawn to.
const (
	inWidthPlanet = 17
	inWidthThreat = 15
	inWidthTarget = 19
	inWidthWhen   = 6
)

// inRow is one thing aimed at this planet.
type inRow struct {
	planet string
	threat string
	color  string // the threat word's color; the WORDS differ too
	target string
	when   string
	away   bool // its hour has come and it is on its way here
}

// showIncoming draws the list.
func showIncoming(s session.Session, w *ctx) Result {
	rows := incomingRows(s, w)

	head := tr(s, "Incoming")
	banner := ansi.FgRed + "──" + ansi.FgBrightRed + "═" + ansi.FgBrightWhite + head +
		ansi.FgBrightRed + "═" + ansi.FgRed + "──" + ansi.Reset
	indent := (len([]rune(rule)) - (5 + visWidth(w.Term, head))) / 2
	if indent < 0 {
		indent = 0
	}
	fmt.Fprintf(s, "\n%s%s\n", strings.Repeat(" ", indent), banner)
	// The house inset rule under the banner, as the screens that head a table
	// draw it; the closing rules are the dim accent the menu engine closes a box
	// with, so this screen sits among the others rather than beside them.
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgRed, insetRule(len([]rune(rule)), 12), ansi.Reset)

	if len(rows) == 0 {
		okNoPause(s, "Nothing is aimed at this planet.")
		fmt.Fprintf(s, "%s%s%s\n", dim(ansi.FgRed), rule, ansi.Reset)
		pause(s)
		return Stay
	}

	// The heading is laid out to the same widths as a row, with the last column
	// right-aligned over its right-aligned figures.
	fmt.Fprintf(s, "%s  %s %s %s %*s%s\n", ansi.FgBrightCyan,
		padColumn(w.Term, tr(s, "Planet"), inWidthPlanet),
		padColumn(w.Term, tr(s, "Threat"), inWidthThreat),
		padColumn(w.Term, tr(s, "Aimed At"), inWidthTarget),
		inWidthWhen, fitColumn(w.Term, tr(s, "When"), inWidthWhen), ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", dim(ansi.FgRed), rule, ansi.Reset)
	for _, r := range rows {
		// A force already away is painted bright red rather than yellow — and it
		// also says "away" where the others carry a figure, so the color is
		// never the only thing that marks it.
		whenColor := ansi.FgBrightYellow
		if r.away {
			whenColor = ansi.FgBrightRed
		}
		fmt.Fprintf(s, "  %s%s %s%s %s%s %s%*s%s\n",
			ansi.FgBrightWhite, padColumn(w.Term, fitColumn(w.Term, r.planet, inWidthPlanet), inWidthPlanet),
			r.color, padColumn(w.Term, r.threat, inWidthThreat),
			ansi.FgWhite, padColumn(w.Term, fitColumn(w.Term, r.target, inWidthTarget), inWidthTarget),
			whenColor, inWidthWhen, r.when, ansi.Reset)
	}
	fmt.Fprintf(s, "%s%s%s\n", dim(ansi.FgRed), rule, ansi.Reset)
	pause(s)
	return Stay
}

// incomingRows gathers the list under the lock, so every countdown on the screen
// is measured from one moment.
func incomingRows(s session.Session, w *ctx) []inRow {
	var rows []inRow
	now := time.Now()
	w.Read(func() {
		// A weapon already in the air is not a report from anybody: the board that
		// launched it says so openly, and the whole flight is visible (#63). Its
		// row comes first because there is nothing left to be done about it but
		// shoot it down when it lands.
		flying := ""
		if d := w.Incoming; d != nil && d.Launched {
			flying = d.Creator
			rows = append(rows, inRow{
				planet: d.Creator,
				threat: tr(s, "Gooie Kablooie"),
				color:  ansi.FgBrightRed,
				target: tr(s, "this planet"),
				when:   incomingDays(d.ArrivesDay - w.GameDay),
			})
		}
		for _, t := range w.IncomingThreats() {
			// The weapon in the air above and the weapon that was being built are
			// the same weapon. One row for it, the one that is still true.
			if t.Kind == game.ThreatGooie && t.FromBoard == flying {
				continue
			}
			r := inRow{planet: t.FromBoard, when: incomingWhen(now, t)}
			r.away = r.when == incomingAway
			switch t.Kind {
			case game.ThreatGooie:
				r.threat, r.color = tr(s, "Gooie Kablooie"), ansi.FgBrightRed
				r.target = tr(s, "this planet")
			default:
				r.threat, r.color = tr(s, "Group attack"), ansi.FgBrightYellow
				r.target = t.Target
				if r.target == "" {
					// The whole-planet form the group-attack table uses, so the two
					// screens spell the same thing the same way.
					r.target = tr(s, "ALL")
				}
			}
			rows = append(rows, r)
		}
	})
	return rows
}

// incomingWhen spells the wait in the short form the Join Group Attack table
// uses (#269), so a force leaving and a force arriving read alike. A threat past
// its hour says so instead of counting down to nothing, and one whose hour was
// never named — a Gooie still being paid for — says that too.
func incomingWhen(now time.Time, t game.Threat) string {
	at := t.When()
	if at.IsZero() {
		return "?"
	}
	if left := at.Sub(now); left > 0 {
		return shortDuration(left)
	}
	// Its hour has come: the force has left, or the weapon has gone up. Either
	// way it is on its way here and the row stays for a day (ThreatMemoryHours),
	// because that is the moment the warning matters most.
	return incomingAway
}

// incomingAway is what the When column says once a threat's hour has passed.
const incomingAway = "away"

// incomingDays spells a flight measured in whole game days, which is all the
// weapon's arrival is known to (Annihilator.ArrivesDay). It is deliberately a
// different unit from the countdowns above, and reads as one.
func incomingDays(days int) string {
	if days <= 0 {
		return "now"
	}
	return fmt.Sprintf("%dd", days)
}
