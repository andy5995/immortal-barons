package menu

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/help"
	"github.com/andy5995/immortal-barons/internal/session"
)

// actions_info.go — the screens that only tell the player something: About,
// the Game Setup sheet of the sysop's settings, the player list, and the two
// entry points to the status and scores screens.

// about shows a short project panel: name, version, website, and the BRE
// heritage note (reachable from both the Game and System menus, #66).
func about(s session.Session, w *ctx) Result {
	// BRE frames its status blocks with a dashes-and-double-line separator
	// (─────═════…, docs/dev/bre-screens.md) rather than a title panel; the About
	// screen follows that look — no blue title bar, the name/version below is the
	// only headline.
	fmt.Fprintf(s, "\n%s%s%s\n", dim(ansi.FgBrightRed), InsetRule, ansi.Reset)
	centered(s, ansi.FgBrightWhite, "Immortal Barons  v"+game.VersionString())
	centered(s, ansi.FgBrightCyan, "https://andy5995.github.io/immortal-barons/")
	fmt.Fprint(s, "\n")
	// Body prose is off-white so the name/version headline above stays the only
	// bright thing on the panel, per docs/dev/bre-screens.md. Wrap to the rule's
	// width less the 2-space indent.
	for _, wl := range strings.Split(help.Wrap(tr(s, "An independent tribute to Barren Realms Elite (BRE), created by Mehul Patel and now owned by John Dailey Software. It is maintained by Andy Alt, with contributions and advice from the BBS and Barren Realms Elite community. This project is not affiliated with, nor endorsed by, the current or past owner(s) of Barren Realms Elite."), len([]rune(rule))-2), "\n") {
		fmt.Fprintf(s, "  %s%s%s\n", ansi.FgWhite, wl, ansi.Reset)
	}
	fmt.Fprint(s, "\n")
	fmt.Fprintf(s, "  %s%s%s\n", ansi.FgWhite, tr(s, "Free software under the MIT License."), ansi.Reset)
	fmt.Fprintf(s, "  %s%s%s\n", ansi.FgWhite, "Copyright (c) 2026 Andy Alt", ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", dim(ansi.FgBrightRed), rule, ansi.Reset)
	pause(s)
	return Stay
}

// gameSetup shows the current game rules (read-only; the sysop edits them with
// the -reset Configuration Editor). Two screens with a pause between them.
//
// Settings are paired two to a line, as BRE's own setup screen does
// ("Maintenance Costs:   Medium          Region Cost Change:  Medium", 78
// columns wide — cap/shsbbs.cap). Pairing is what keeps a ruleset IB has grown
// well past the original's inside two pages: one per line ran to 37 lines on the
// second page of a league game, so its first dozen rows scrolled off an 80x25
// terminal unread. A pair whose labels or values are too wide for the half
// column falls back to two full-width lines by itself, which is what makes the
// layout safe for a translated catalog — German runs longer than the English it
// came from and nothing here needs to know that.
func gameSetup(s session.Session, w *ctx) Result {
	c := w.Config
	// BRE's own setup screen is 78 columns; the 62-column `rule` is the menu
	// engine's box and would leave these dividers short of the rows they head.
	const setupRule = 78
	const labelW = 22
	const halfValueW = 14

	pad := func(text string, width int) string {
		if n := len([]rune(text)); n < width {
			return text + strings.Repeat(" ", width-n)
		}
		return text
	}
	// Label in body white, figure in bright white — the highlight convention from
	// docs/dev/bre-screens.md, so the numbers a player is scanning for stand out.
	cell := func(label, value string, valueW int) string {
		return ansi.FgWhite + pad(label, labelW) + ansi.Reset + " " +
			ansi.FgBrightWhite + pad(value, valueW) + ansi.Reset
	}
	row := func(label, value string) {
		fmt.Fprintf(s, "  %s\n", cell(tr(s, label), value, 0))
	}
	// pair puts two settings on one line, or gives up and prints both full width
	// when either would be truncated.
	pair := func(l1, v1, l2, v2 string) {
		t1, t2 := tr(s, l1), tr(s, l2)
		if len([]rune(t1)) > labelW || len([]rune(t2)) > labelW ||
			len([]rune(v1)) > halfValueW || len([]rune(v2)) > halfValueW {
			row(l1, v1)
			row(l2, v2)
			return
		}
		fmt.Fprintf(s, "  %s  %s\n", cell(t1, v1, halfValueW), cell(t2, v2, 0))
	}
	// countOr renders a limit, naming what its zero means rather than printing a
	// bare 0 the player has to interpret.
	countOr := func(n int, unlimited string) string {
		if n <= 0 {
			return tr(s, unlimited)
		}
		return comma(n)
	}
	daysOr := func(n int, forever string) string {
		if n <= 0 {
			return tr(s, forever)
		}
		return fmt.Sprintf(tr(s, "%s days"), comma(n))
	}
	// group heads a run of related settings with a captioned divider. BRE's own
	// Configuration Editor is one long ungrouped list; these dividers are IB's,
	// and they are what makes a screen this dense scannable. Dim, so the figures
	// stay the brightest thing on the panel.
	group := func(caption string) {
		text := tr(s, caption)
		fill := setupRule - len([]rune(text)) - 6
		if fill < 0 {
			fill = 0
		}
		// The leading blank line is what separates one group from the rows above
		// it — and it is also what puts the first group of a panel clear of the
		// pause prompt the previous panel ended on.
		fmt.Fprintf(s, "\n%s── %s %s%s\n",
			dim(ansi.FgBrightCyan), text, strings.Repeat("─", fill), ansi.Reset)
	}

	// The league comes first, because on a league board it is the frame for
	// everything under it: none of these rules are the local sysop's, and a
	// player who reads the ruleset without knowing whose it is asks the wrong
	// person to change it.
	if c.IBBS {
		var boards int
		var declaration, coordBoard string
		w.With(func() {
			boards = len(w.LeagueNodes)
			declaration = w.LeagueDiplomacy
			coordBoard = w.CoordinatorBoardID()
		})
		group("The league")
		// The league is identified by its NUMBER and the board that coordinates
		// it, and by nothing else — a league has no name of its own, because
		// hardly any league ever set one. The number is what keeps two leagues
		// sharing an inbound directory apart, so a sysop diagnosing one reads it
		// here rather than in bbs.cfg.
		leagueNo := tr(s, "Not set")
		if c.LeagueNumber > 0 {
			leagueNo = strconv.Itoa(c.LeagueNumber)
		}
		pair("League number", leagueNo, "Planets in the league", countOr(boards, "Roster not loaded"))
		// Which board holds the office, because the rules below are not this
		// sysop's to set once a board joins a league: the Coordinator broadcasts
		// the whole ruleset (see LeagueConfig), so a player asking their own sysop
		// to change one is asking the wrong person. "League Coordinator" is BRE's
		// own name for this office and IB's everywhere else — not to be confused
		// with the elected BBS Coordinator, which is a different job. A board with
		// no roster to look the name up in says so instead.
		coordinator := tr(s, "Not known — no roster yet")
		if coordBoard != "" {
			coordinator = coordBoard
		}
		row("League Coordinator", fitColumn(w.Term, coordinator, 50))
		// Last, so the group narrows from the whole league to the board the
		// player is standing on.
		row("This planet", fitColumn(w.Term, c.BoardID, 50))
		if declaration != "" {
			row("League declaration", declaration)
		}
	}

	group("The day and the game")
	pair("Turns per day", comma(c.TurnsPerDay),
		"New realm protection", fmt.Sprintf(tr(s, "%s turns"), comma(c.ProtectionTurns)))
	pair("Game length", daysOr(c.GameLength, "Endless"),
		"Removed if unplayed", daysOr(c.IdleDaysRemove, "Never"))
	group("Land")
	pair("Maximum regions", countOr(c.MaxRegions, "Unlimited"),
		"Land at game start", comma(c.InitialMarketLand))
	pair("New land per realm/day", comma(c.LandPerDay),
		"Region costs", tr(s, c.RegionCosts.String()))
	group("Money")
	pair("Maximum tax rate", fmt.Sprintf("%d%%", c.MaxTaxRate),
		"Crown tax on income", fmt.Sprintf("%d%%", c.PlanetaryTaxRate))
	pair("Bank interest", fmt.Sprintf(tr(s, "%d%% over 10 days"), c.InterestRate),
		"Investment rate", investRateStr(s, c))
	row("Food market", foodMarketStr(s, c))
	// Worth a line of its own: gold earned past this is destroyed, and a player
	// who does not know the figure only finds out by losing some.
	// Whole billions, not the 4-decimal display form: the cap is SET in billions,
	// so the fraction is always zeros.
	row("Most gold you can hold", fmt.Sprintf(tr(s, "%sB in hand, and again in the bank"),
		comma(w.MoneyCapBillions())))
	pause(s)

	group("Forces and trade")
	pair("Buy military", tr(s, c.BuyMilitary.String()),
		"Maintenance costs", tr(s, c.MaintCosts.String()))
	pair("Trade costs", tr(s, c.TradeCosts.String()),
		"Pirates", onOffStr(c.Pirates))
	group("Attacking")
	pair("Attack damage", tr(s, c.AttackDamage.String()),
		"Attack rewards", tr(s, c.AttackRewards.String()))
	pair("Attacks per day", countOr(c.MaxIndividualAttacks, "Unlimited"),
		"Attack costs", tr(s, c.AttackCosts.String()))
	row("R5-Slappenheimer", tr(s, c.SlappenheimerHandling.String()))
	if !c.MissileOps || !c.BombingOps {
		pair("Missile ops", onOffStr(c.MissileOps), "Bombing ops", onOffStr(c.BombingOps))
	}

	group("This board")
	pair("Players per board", countOr(c.MaxPlayers, "Unlimited"),
		"Inter-BBS play", onOffStr(c.IBBS))
	if c.GameStartDate != "" || c.JoinDate != "" {
		pair("Game starts", orDash(c.GameStartDate), "Joining closes", orDash(c.JoinDate))
	}

	if c.IBBS {
		// The interplanetary rules only mean anything once this board is in a
		// league, so they stay hidden on a stand-alone board.
		group("Interplanetary")
		pair("Group attacks per day", countOr(c.MaxGroupAttacks, "Unlimited"),
			"Terrorist ops per day", countOr(c.MaxTerrorOps, "Unlimited"))
		pair("Bombing ops per day", countOr(c.MaxBombingOps, "Unlimited"),
			"Terrorism costs", tr(s, c.TerrorCosts.String()))
		pair("Lost forces return", lostForcesStr(s, c),
			"Clingy Annihilator", onOffStr(c.ClingyAnnihilator))
		pair("Allied market trading", onOffStr(c.IPTrading),
			"Local attack scoring", onOffStr(c.LocalAttackScoring))
		row("Local attacks", onOffStr(c.LocalAttacks))
	}
	pause(s)
	return Stay
}

// orDash renders an unset date as a dash, so a pair keeps its shape when only
// one of the two dates is configured.
func orDash(v string) string {
	if v == "" {
		return "—"
	}
	return v
}

// lostForcesStr says how long a strike waits for a result before its forces are
// given back, or that they never are.
func lostForcesStr(s session.Session, c game.Config) string {
	if c.LostForcesDays <= 0 {
		return tr(s, "Never")
	}
	return fmt.Sprintf(tr(s, "%d days"), c.LostForcesDays)
}

// investRateStr names the investment rate and whether it is pinned there.
func investRateStr(s session.Session, c game.Config) string {
	rate := fmt.Sprintf(tr(s, "%d%% over 10 days"), c.StdInvestRate)
	if c.SteadyInvest {
		return rate + tr(s, " (steady)")
	}
	return rate + tr(s, " (floating)")
}

// foodMarketStr says whether the market's daily supply runs out.
func foodMarketStr(s session.Session, c game.Config) string {
	if c.FoodUnlimited {
		return tr(s, "Unlimited")
	}
	return tr(s, "Limited daily supply")
}

// playerList shows every living empire (Coordinator tool).
// playerListNameWidth holds a realm name AND the online suffix. The column was
// 16 before the suffix existed; keeping it there would have clipped ordinary
// names down to seven characters to make room.
const playerListNameWidth = 25

// It names realms, never the callers behind them. BRE prints a player's BBS
// account name in exactly one place — PLAYERS.LST, written by the PLAYERLIST
// command line, which its manual restricts to the League Coordinator. That is
// a BOARD's operator, running a DOS command, and need not be playing at all;
// the BBS Coordinator this menu belongs to is an elected PLAYER with no
// standing on anyone's system. No screen in the game shows one: the empire record
// carries the BBS name at +0x00 and the realm name at +0x1f, and See Scores,
// the coordinator vote and the interplanetary Player Information screen all
// print +0x1f, as does the recon packet's own name array. This screen listed
// an Owner column of handles until 2026-08-18.
func playerList(s session.Session, w *ctx) Result {
	type row struct {
		name     string
		presence string
		land, nw int
	}
	var rows []row
	w.With(func() {
		for _, e := range w.Empires {
			if !e.Alive {
				continue
			}
			self := e == w.Player()
			rows = append(rows, row{e.Name, presenceOf(e, self, w.Today), e.Land, w.NetWorth(e)})
		}
	})
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightBlue, tr(s, "Player List"), ansi.Reset)
	fmt.Fprintf(s, "  %-*s %-8s %s\n", playerListNameWidth, tr(s, "Empire"), tr(s, "Land"), tr(s, "Net Worth"))
	for _, r := range rows {
		// The suffix rides in the name column, so the roster says who is on
		// without moving the columns beside it.
		fmt.Fprintf(s, "  %s %-8d %d\n", nameCell(s, r.name, "", r.presence, playerListNameWidth), r.land, r.nw)
	}
	pause(s)
	return Stay
}

// empireStatus is the standalone status action (System menu): draw the Empire
// Status block, then pause so it is read before the menu redraws over it.
func empireStatus(s session.Session, w *ctx) Result {
	renderEmpireStatus(s, w)
	pause(s)
	return Stay
}

func seeScores(s session.Session, w *ctx) Result {
	printScores(s, w)
	pause(s)
	return Stay
}
