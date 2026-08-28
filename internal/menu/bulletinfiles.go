package menu

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// bulletinfiles.go — the scoreboard and news, written out as files a BBS can
// show on its own bulletin menu (#233). The original wrote these too; a sysop
// coming from it expects the entry on their menu to fill in.
//
// Every screen here is the one a player sees, rendered to a file instead of a
// terminal: a Session is an io.Writer, so the same function draws both. That
// is the point of the seam, and it is why these files cannot drift away from
// what the game shows. Do NOT add a second layout here.
//
// Two files per bulletin, as the original wrote them: .ans keeps the colour,
// .txt is the same screen with the escapes stripped by the plain writer the
// door already uses for a caller with no ANSI.

// bulletinRender is one bulletin: the file's base name and how to draw it.
type bulletinRender struct {
	base string
	draw func(s session.Session, w *ctx)
}

// WriteBulletins draws every bulletin file for this board. A blank directory
// means the sysop does not want them, which is how the original's own setting
// worked -- there, an empty path per file; here, one directory for the set.
//
// Errors are collected rather than returned on the first: a bulletin nobody can
// write is worth reporting, but it must not stop the rest, and it must never
// stop the run that called this.
func WriteBulletins(w *game.World, dir string) []error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return []error{err}
	}
	// Today is a per-SESSION field, empty in the scheduled run that writes these
	// files, so the bulletins would carry no date at all. The game day is the
	// right one for a bulletin anyway: it is the day the news being reported
	// belongs to, not the wall clock of whoever ran the step.
	day := w.Today
	if day == "" {
		day = w.LastMaintDate
	}
	c := &ctx{World: w, Term: Term{UTF8: true}, day: day}
	bulletins := []bulletinRender{
		{"scores", func(s session.Session, c *ctx) { printScores(s, c) }},
		{"tdynews", func(s session.Session, c *ctx) { writeNewsBulletin(s, c, true) }},
		{"yesnews", func(s session.Session, c *ctx) { writeNewsBulletin(s, c, false) }},
	}
	// The World Report is the LEAGUE's wars. A board that plays alone has no
	// world to report on, and a page headed "World Report" listing one planet's
	// skirmishes would promise something the board does not have.
	if w.Config.InterBBSEnabled() {
		bulletins = append(bulletins, bulletinRender{"world", writeWorldReport})
	}
	var errs []error
	for _, b := range bulletins {
		var buf bytes.Buffer
		b.draw(session.NewWriter(&buf), c)
		if err := writeBulletinPair(dir, b.base, buf.Bytes()); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// writeBulletinPair writes the coloured and the plain form of one bulletin.
// The plain one goes through the same writer a caller with no ANSI gets, so the
// two can never disagree about what stripping means.
func writeBulletinPair(dir, base string, ansi []byte) error {
	if err := writeFileIfChanged(filepath.Join(dir, base+".ans"), ansi); err != nil {
		return err
	}
	var plain bytes.Buffer
	if _, err := session.NewPlain(session.NewWriter(&plain)).Write(ansi); err != nil {
		return err
	}
	return writeFileIfChanged(filepath.Join(dir, base+".txt"), plain.Bytes())
}

// writeBulletinPair's files are read by whatever the BBS shows them with, which
// may be reading one at the moment it is rewritten. Writing only on a change
// keeps a quiet board's bulletins from being rewritten every run, and the
// rename makes the swap atomic for a reader.
func writeFileIfChanged(path string, data []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// dayBefore is the previous calendar day, for the yesterday bulletin's
// masthead. An unparseable date is returned unchanged rather than guessed at:
// a wrong date on a bulletin is worse than the one the world already holds.
func dayBefore(date string) string {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return d.AddDate(0, 0, -1).Format("2006-01-02")
}

// writeNewsBulletin draws one day's planet news, the same screen the news menu
// shows, with its masthead.
func writeNewsBulletin(s session.Session, w *ctx, today bool) {
	var date string
	var lines []string
	var bulletin game.DailyBulletin
	w.Read(func() {
		if today {
			date, lines, bulletin = w.day, w.NewsToday, w.BulletinToday
		} else {
			// The frozen day carries no date of its own, so the masthead names
			// the day it is: the one before the board's current date.
			date, lines, bulletin = dayBefore(w.day), w.NewsYesterday, w.BulletinYesterday
		}
	})
	renderNewsMasthead(s, w.Term, date)
	renderDailyBulletin(s, w.Term, bulletin, w.Config.BoardID)
	fmt.Fprint(s, "\n")
	if len(lines) == 0 {
		okNoPause(s, "No news.")
		return
	}
	for _, line := range lines {
		fmt.Fprintf(s, "  %s\n", strings.TrimSpace(line))
	}
}

// The report is wider than the menu rule because a board name and two realm
// names have to fit side by side, and BRE sizes each screen to its own content
// rather than to a house width. Still inside 80 columns.
const (
	planetCol = 20
	sideCol   = 18
)

var worldRule = strings.Repeat("─", 2+planetCol+1+sideCol+1+sideCol+1+8)

// writeWorldReport draws the league's wars: every attack fought anywhere the
// board has heard from, newest first. Asked for by a sysop who had each board's
// own scoreboard and wanted to see the fighting (#233).
//
// ATTACKS only. A nuclear, chemical or biological strike is not here, nor a
// terror op: the report is about armies meeting, and a weapon landing on a city
// is a different story the news already tells. Nothing filters them out at this
// end — they never enter the log, which is why they cannot leak in later.
func writeWorldReport(s session.Session, w *ctx) {
	var battles []game.BattleLogEntry
	var here string
	w.Read(func() {
		here = w.Config.BoardID
		battles = append(battles, w.Battles...)
	})
	// Its own heading, not the news masthead: that one names the news file and
	// draws the planet's banner under it, neither of which belongs on a report
	// about the whole league.
	date := w.day
	fmt.Fprintf(s, "\n%s%s%s v%s%s", ansi.FgBrightYellow, tr(s, "Immortal Barons"),
		ansi.FgBrightWhite, game.Version, ansi.Reset)
	fmt.Fprintf(s, "  %s%s%s", ansi.FgWhite, tr(s, "World Report"), ansi.Reset)
	if date != "" {
		fmt.Fprintf(s, "%s%s%s", strings.Repeat(" ", 4), ansi.FgWhite, date)
	}
	fmt.Fprintf(s, "%s\n%s%s%s\n\n", ansi.Reset, ansi.FgBrightRed, worldRule, ansi.Reset)

	if len(battles) == 0 {
		okNoPause(s, "No battles have been fought yet.")
		return
	}
	fmt.Fprintf(s, "%s  %-*s %-*s %-*s %s%s\n", ansi.FgBrightCyan,
		planetCol, tr(s, "Planet"), sideCol, tr(s, "Attacker"),
		sideCol, tr(s, "Defender"), tr(s, "Outcome"), ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", dim(ansi.FgBrightRed), worldRule, ansi.Reset)
	for i := len(battles) - 1; i >= 0; i-- {
		b := battles[i]
		planet := b.Planet
		if planet == "" || planet == here {
			planet = tr(s, "here")
		}
		// The outcome is what a reader scans for, so it carries the colour --
		// and the WORDS differ too, so the plain file and a monochrome terminal
		// say the same thing colour alone would.
		outcome, color := worldReportOutcome(s, b)
		fmt.Fprintf(s, "  %s%-*s %-*s %-*s %s%s%s\n",
			ansi.FgWhite, planetCol, fitColumn(w.Term, planet, planetCol),
			sideCol, fitColumn(w.Term, b.Attacker, sideCol),
			sideCol, fitColumn(w.Term, b.Defender, sideCol),
			color, outcome, ansi.Reset)
	}
	fmt.Fprintf(s, "%s%s%s\n", dim(ansi.FgBrightRed), worldRule, ansi.Reset)
}

// worldReportOutcome words one battle. Colour never carries the meaning on its
// own -- the words differ too, so the report reads the same on a monochrome
// terminal and in the plain-text file.
func worldReportOutcome(s session.Session, b game.BattleLogEntry) (string, string) {
	switch {
	case b.Crushed:
		return tr(s, "CRUSHED"), ansi.FgBrightRed
	case b.Won && b.Land > 0:
		return fmt.Sprintf(tr(s, "took %d"), b.Land), ansi.FgBrightYellow
	case b.Won:
		return tr(s, "won"), ansi.FgBrightGreen
	}
	return tr(s, "held"), ansi.FgBrightCyan
}
