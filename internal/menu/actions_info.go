package menu

import (
	"fmt"
	"sort"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// abdicate deletes the player's empire from the game (BRE.DOC: "immediately
// delete your empire from the game so you may start over the next day"). It
// is irreversible, so the player must retype their realm name to confirm.
// Removing the empire and quitting is enough: play.go persists the world on
// exit, and the caller's next visit finds no empire and onboards a fresh one.
func abdicate(s session.Session, w *ctx) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%s"+tr(s, "Abdicating deletes %s permanently. This cannot be undone.")+"%s\n",
		ansi.FgBrightRed, p.Name, ansi.Reset)
	typed := prompt(s, fmt.Sprintf(tr(s, "Type your realm name (%s) to confirm, or anything else to cancel"), p.Name))
	if strings.TrimSpace(typed) != p.Name {
		fmt.Fprintf(s, "\n%s\n", tr(s, "Abdication cancelled."))
		pause(s)
		return Stay
	}
	w.With(func() { w.World.RemoveEmpire(p) })
	w.active = nil
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow, tr(s, "Your empire is no more. Fare thee well."), ansi.Reset)
	pause(s)
	return Quit
}

// renderAdvisors prints the Advisors screen: contextual advice based on the
// empire's current state — the sort of nudges the original's advisors
// offered (military readiness, food, support/morale, taxes, treasury,
// technology). Split from visitAdvisors so tests can render without a pause.
func renderAdvisors(s session.Session, w *ctx) {
	// Snapshot so the report reflects one consistent moment even if another
	// session mutates the world mid-render (same reasoning as Empire Status).
	var p game.Empire
	w.With(func() { p = *w.Player() })

	titleBar(s, tr(s, "Visit Advisors"))
	var tips []string
	switch {
	case p.HQ == 0:
		tips = append(tips, tr(s, "We have no HeadQuarters, Sire. Building one would strengthen our tanks."))
	case p.HQ < 100:
		tips = append(tips, tr(s, "Our HeadQuarters is still under construction."))
	}
	if p.Carriers*100 < p.Jets {
		tips = append(tips, tr(s, "We have more jets than our carriers can carry into battle. Build more carriers."))
	}
	if p.Morale < 50 {
		tips = append(tips, tr(s, "Morale is low among our troops. Desertion is a real risk before our next battle."))
	}
	if p.Food < w.FoodNeededNextTurn(&p) {
		tips = append(tips, tr(s, "Our food will not last the turn. Buy or grow more."))
	}
	if p.Support < 50 {
		tips = append(tips, tr(s, "The people grow restless. Lower taxes or spend on their support."))
	}
	// Riots are possible above RiotTaxFloor(10), but the chance (tax^2/10000)
	// is trivial near the floor; only advise once the risk is worth acting on.
	if p.Tax > 20 {
		tips = append(tips, tr(s, "Taxes are set high enough to risk riots. Consider lowering them."))
	}
	if p.TechFactor() == 0 {
		tips = append(tips, tr(s, "We have no Technology infrastructure. Such regions would sharpen our efficiency."))
	}
	if p.Gold <= 0 && p.Bank <= 0 {
		tips = append(tips, tr(s, "Our treasury is empty, Sire. We should raise gold before it costs us dearly."))
	}
	if p.Debt > 0 {
		tips = append(tips, tr(s, "We carry debt that grows each turn. Repay it soon."))
	}
	if p.Agents == 0 {
		tips = append(tips, tr(s, "We have no covert agents. Recruit some for spying and sabotage."))
	}
	if len(tips) == 0 {
		tips = append(tips, tr(s, "The realm is in good order, Sire. Press the attack."))
	}
	for _, t := range tips {
		fmt.Fprintf(s, "  - %s\n", t)
	}
}

// visitAdvisors is the System menu's "Visit Advisors" action.
func visitAdvisors(s session.Session, w *ctx) Result {
	renderAdvisors(s, w)
	pause(s)
	return Stay
}

// about shows a short project panel: name, version, website, and the BRE
// heritage note (reachable from both the Game and System menus, #66).
func about(s session.Session, w *ctx) Result {
	titleBar(s, tr(s, "About"))
	fmt.Fprintf(s, "  %s\n", "Immortal Barons v"+game.Version)
	fmt.Fprintf(s, "  %s\n", "https://andy5995.github.io/immortal-barons/")
	fmt.Fprintf(s, "  %s\n", tr(s, "An independent tribute to Barren Realms Elite (BRE), created by Mehul Patel and later maintained by John Dailey. No original BRE code, text, or art is used."))
	pause(s)
	return Stay
}

// gameSetup shows the current game rules (read-only; the sysop edits them from
// the Coordinator menu's Configuration Editor).
func gameSetup(s session.Session, w *ctx) Result {
	c := w.Config
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Game Rules"), ansi.Reset)
	fmt.Fprintf(s, "  "+tr(s, "Turns per day:      %d")+"\n", c.TurnsPerDay)
	fmt.Fprintf(s, "  "+tr(s, "Protection turns:   %d")+"\n", c.ProtectionTurns)
	fmt.Fprintf(s, "  "+tr(s, "Game length (days): %d  (0 = endless)")+"\n", c.GameLength)
	fmt.Fprintf(s, "  "+tr(s, "Inter-BBS play:     %s")+"\n", onOffStr(c.IBBS))
	pause(s)
	return Stay
}

// playerList shows every living empire (Coordinator tool).
func playerList(s session.Session, w *ctx) Result {
	type row struct {
		name, owner string
		land, nw    int
	}
	var rows []row
	w.With(func() {
		for _, e := range w.Empires {
			if !e.Alive {
				continue
			}
			rows = append(rows, row{e.Name, e.Owner, e.Land, w.NetWorth(e)})
		}
	})
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightBlue, tr(s, "Player List"), ansi.Reset)
	fmt.Fprintf(s, "  %-16s %-14s %-8s %s\n", tr(s, "Empire"), tr(s, "Owner"), tr(s, "Land"), tr(s, "Net Worth"))
	for _, r := range rows {
		owner := r.owner
		if owner == "" {
			owner = tr(s, "(AI)")
		}
		fmt.Fprintf(s, "  %-16s %-14s %-8d %d\n", r.name, owner, r.land, r.nw)
	}
	pause(s)
	return Stay
}

// renderDailyBulletin draws the boxed Daily Bulletin header: planet-wide
// totals with day-over-day change, colored green/red/neutral for +/-/0.
// title is Config.BoardID, or "" to show just "Daily Bulletin".
func renderDailyBulletin(s session.Session, b game.DailyBulletin, title string) {
	head := tr(s, "Daily Bulletin")
	if title != "" {
		head = title + " — " + head
	}
	titleBar(s, head)

	row := func(label string, total, change int, fmtNum func(int) string) {
		clr := ansi.FgWhite
		switch {
		case change > 0:
			clr = ansi.FgGreen
		case change < 0:
			clr = ansi.FgRed
		}
		sign := "+"
		abs := change
		if change < 0 {
			sign = "-"
			abs = -change
		}
		fmt.Fprintf(s, "  %s%s:%s %s    %s%s:%s %s%s%s%s\n",
			ansi.FgWhite, tr(s, label), ansi.Reset, fmtNum(total),
			ansi.FgWhite, tr(s, "Change"), ansi.Reset, clr, sign, fmtNum(abs), ansi.Reset)
	}

	row("Total Population", b.Totals.Population, b.Change.Population, comma)
	row("Total Regions", b.Totals.Regions, b.Change.Regions, comma)
	row("Total Net Worth", b.Totals.NetWorth, b.Change.NetWorth, abbrevMoney)
}

// empireStatus is the standalone status action (System menu): page through the
// Empire Status screens, pausing on each so a wide screen is not scrolled past.
func empireStatus(s session.Session, w *ctx) Result {
	renderEmpireStatus(s, w)
	pause(s)
	return Stay
}

func hqStatus(p *game.Empire) string {
	switch {
	case p.HQ == 0:
		return "None"
	case p.HQ >= 100:
		return "Complete"
	default:
		return fmt.Sprintf("%d%%", p.HQ)
	}
}

func seeScores(s session.Session, w *ctx) Result {
	printScores(s, w)
	pause(s)
	return Stay
}

func printScores(s session.Session, w *ctx) {
	// Snapshot every empire's rank inputs together so the board reflects one
	// consistent moment, even if another session mutates the world mid-render.
	type row struct {
		name     string
		alive    bool
		isPlayer bool
		land, nw int
	}
	var rows []row
	var lastMaster string
	w.With(func() {
		rows = make([]row, 0, len(w.Empires))
		for _, e := range w.Empires {
			rows = append(rows, row{e.Name, e.Alive, e == w.Player(), e.Land, w.NetWorth(e)})
		}
		lastMaster = w.LastMaster
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i].nw > rows[j].nw })

	fmt.Fprintf(s, "\n%s%-4s %-18s %-8s %-10s%s\n",
		ansi.FgBrightCyan, tr(s, "Rank"), tr(s, "Empire"), tr(s, "Land"), tr(s, "Net Worth"), ansi.Reset)
	for i, r := range rows {
		name := r.name
		if !r.alive {
			name += " " + tr(s, "(dead)")
		}
		mark := "  "
		if r.isPlayer {
			mark = "->"
		}
		fmt.Fprintf(s, "%s%2d %-18s %-8d %-10d\n", mark, i+1, name, r.land, r.nw)
	}
	if lastMaster != "" {
		fmt.Fprintf(s, "\n"+tr(s, "Last Planetary Master: %s")+"\n", lastMaster)
	}
}
