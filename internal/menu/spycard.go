package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// spycard.go — the target's spy record, printed once a strike has named its
// target. The original draws it from one routine, show_player_intelligence
// (BRE.OVR 0x02dbb0), and calls it from all three attack paths: Indiv. Attack
// Force and Create Group Attack after the target letter, Join Group Attack after
// the party is picked (cap/eots-ibbs-02.cap, cap/eots-ibbs-03.cap). With no
// record matching the realm it prints nothing and the attack goes on: the
// routine returns before its first line is drawn. The 2026-08-11 capture, which
// goes from the roster straight to the Attack Type menu, is consistent with that.

// spyCardWidth is the card's rule, 78 columns in the capture.
const spyCardWidth = 78

// showPlayerIntelligence prints the newest spy record on empire of board, if
// there is one with the military figures in it. A record filed before reports
// carried them (Protocol 3) holds none, and printing its zeros would read as a
// realm with no army at all.
func showPlayerIntelligence(s session.Session, w *ctx, board, empire string) {
	var rec game.SpyEntry
	var found bool
	letter := "?"
	w.Read(func() {
		for _, r := range w.SpyDatabase {
			if r.Board == board && r.Empire == empire {
				rec, found = r, true // newest last
			}
		}
		for _, b := range w.RemoteBoards {
			if b.BoardID != board {
				continue
			}
			for i, sc := range b.Scores {
				if sc.Empire == empire {
					if l, ok := remoteLetter(i); ok {
						letter = l
					}
				}
			}
		}
	})
	if !found || (rec.Troopers == 0 && rec.Jets == 0 && rec.Turrets == 0 && rec.Tanks == 0 && rec.Morale == 0) {
		return
	}
	when := rec.Date
	if !rec.Filed.IsZero() {
		when = rec.Filed.In(sessionZone(s)).Format("01/02/2006  15:04:05")
	}
	label, value := ansi.FgWhite, ansi.FgBrightYellow
	line := ansi.FgBrightBlack + insetRule(spyCardWidth, 15) + ansi.Reset
	fmt.Fprintf(s, "%s%s%s%-33s%s  %s%s%s%s\n", label, tr(s, "BBS Name: "), ansi.FgBrightWhite, board,
		label, tr(s, "Date: "), value, when, ansi.Reset)
	fmt.Fprintf(s, "%s%s%s%-30s%s  %s%s%s%s\n", label, tr(s, "Player Name: "), ansi.FgBrightWhite, empire,
		label, tr(s, "Player Letter: "), value, letter, ansi.Reset)
	fmt.Fprintln(s, line)
	fmt.Fprintf(s, "%s%-14s%s%s%s\n", label, tr(s, "Regions:"), value, comma(rec.Land), ansi.Reset)
	fmt.Fprintf(s, "%s[%s=%s%s%s]    [%s=%s%d%s%%]%s\n", label,
		tr(s, "Troopers"), value, comma(rec.Troopers), label,
		tr(s, "Morale"), value, rec.Morale, label, ansi.Reset)
	fmt.Fprintf(s, "%s[%s=%s%s%s]  [%s=%s%s%s]  [%s=%s%s%s]%s\n", label,
		tr(s, "Jets"), value, comma(rec.Jets), label,
		tr(s, "Turrets"), value, comma(rec.Turrets), label,
		tr(s, "Tanks"), value, comma(rec.Tanks), label, ansi.Reset)
	fmt.Fprintln(s, line)
}
