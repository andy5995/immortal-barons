package menu

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/session"
)

// GameLoop is the top-level session flow: show the Game Menu until the
// player quits. "Play Game" runs a turn pipeline. handle identifies the empire
// playing THIS session — per-session state, not shared World state, so the web
// front-end can run concurrent sessions against one World, and re-resolved each
// transaction so it survives the door's per-action world reload.
func GameLoop(s session.Session, w *game.World, handle string, utf8 bool) (err error) {
	// A prompt read failing mid-turn (idle boot or dropped connection) unwinds
	// the whole session via session.End; catch it here and report it as io.EOF,
	// which the caller (play.Session) treats as a clean save-and-exit end.
	defer session.GuardEnd(&err)
	c := &ctx{World: w, handle: handle, UTF8: utf8}
	// Seed the active-empire cache under the raw world lock so the first resolve
	// can't race a concurrent AddHuman (web onboarding). Later resolves are
	// cache hits until a reload bumps the generation.
	w.Lock()
	c.Player()
	w.Unlock()
	menus := BuildMenus()
	// Expand Ctrl-<letter> into the active player's saved macro keystrokes.
	// This single wrap covers every front-end, since all of them reach the
	// menu through GameLoop.
	ms := session.NewMacroExpander(s, func(letter string) (string, bool) {
		p := c.Player()
		if p == nil {
			return "", false
		}
		seq, ok := p.Macros[letter]
		return seq, ok
	})
	return Run(ms, c, menus.Game)
}

// showBulletinToday is the Today's News menu action.
func showBulletinToday(s session.Session, w *ctx) Result { return showBulletin(s, w, false) }

// showBulletinYesterday is the Yesterday's News menu action.
func showBulletinYesterday(s session.Session, w *ctx) Result { return showBulletin(s, w, true) }

// showBulletin prints the Daily Bulletin header plus that day's planetary
// news lines, or a note if there is no news. yesterday selects
// BulletinYesterday/NewsYesterday instead of today's.
func showBulletin(s session.Session, w *ctx, yesterday bool) Result {
	var bulletin game.DailyBulletin
	var news []string
	var boardID string
	w.With(func() {
		if yesterday {
			bulletin, news = w.BulletinYesterday, w.NewsYesterday
		} else {
			bulletin, news = w.BulletinToday, w.NewsToday
		}
		boardID = w.Config.BoardID
	})
	renderDailyBulletin(s, bulletin, boardID)
	if len(news) == 0 {
		fmt.Fprintf(s, "\n%s\n", tr(s, "No planetary bulletins."))
	} else {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Planetary Bulletin:"), ansi.Reset)
		for _, b := range news {
			fmt.Fprintf(s, "  %s\n", b)
		}
	}
	pause(s)
	return Stay
}

// askYesNo prompts msg with a "(Y/n)" or "(y/N)" hint (whichever matches
// defYes) and reads a single keypress — no Enter required. 'y'/'Y' returns
// true, 'n'/'N' returns false; Enter or any other key returns defYes.
func askYesNo(s session.Session, msg string, defYes bool) bool {
	hint := "(y/N)"
	if defYes {
		hint = "(Y/n)"
	}
	fmt.Fprintf(s, "\n%s %s ", i18n.T(sessionLang(s), msg), hint)
	for {
		r, err := readKey(s)
		if err != nil {
			return false // test stream ran out
		}
		switch r {
		case 'y', 'Y':
			fmt.Fprint(s, "y\n")
			return true
		case 'n', 'N':
			fmt.Fprint(s, "n\n")
			return false
		default:
			if defYes {
				fmt.Fprint(s, "y\n")
			} else {
				fmt.Fprint(s, "n\n")
			}
			return defYes
		}
	}
}

// withPlayer runs fn under w's lock against the FRESHLY-resolved active empire.
// It returns false when the empire has vanished (eliminated/abdicated by another
// node mid-turn), so a turn-pipeline caller can abort instead of mutating a
// stale *Empire that a reload may have rebound to a rival's data. Every mutating
// transaction in the turn pipeline goes through here rather than closing over a
// p captured across a w.With boundary.
func withPlayer(w *ctx, fn func(p *game.Empire)) bool {
	alive := false
	w.With(func() {
		if p := w.Player(); p != nil {
			fn(p)
			alive = true
		}
	})
	return alive
}

// showTurnEvents prints and clears the active empire's accumulated events, if
// any. The read and clear happen together under w's lock so a concurrent
// maintenance tick or another session's action can't append between the two,
// and the empire is re-resolved inside the lock so a reload can't rebind it.
func showTurnEvents(s session.Session, w *ctx) {
	var events []string
	withPlayer(w, func(p *game.Empire) {
		events = p.Events
		p.Events = nil
	})
	if len(events) == 0 {
		return
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Since your last play, this has happened:"), ansi.Reset)
	for _, ev := range events {
		fmt.Fprintf(s, "  %s\n", ev)
	}
	pause(s)
}

// showUnreadMail is a pre-turn stop: when the player has unread mail, note the
// count and offer to read it inline. It lives in the shared pre-turn flow so
// every front-end (web + door) gets the same notice for free (#3). Declining
// leaves the mail for the Messages menu. Count/read-and-clear happen under w's
// lock so a concurrent sender can't slip a message between the check and the
// read.
func showUnreadMail(s session.Session, w *ctx) {
	var count int
	withPlayer(w, func(p *game.Empire) { count = len(p.Mail) })
	if count == 0 {
		return
	}
	if count == 1 {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "You have a new message."), ansi.Reset)
	} else {
		fmt.Fprintf(s, "\n%s"+tr(s, "You have %d new messages.")+"%s\n", ansi.FgBrightCyan, count, ansi.Reset)
	}
	if !askYesNo(s, "Read them now?", true) {
		return
	}
	var mail []string
	withPlayer(w, func(p *game.Empire) {
		mail = p.Mail
		p.Mail = nil
	})
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Your messages:"), ansi.Reset)
	for _, m := range mail {
		fmt.Fprintf(s, "  %s\n", m)
	}
	pause(s)
}

// incomeReport itemizes p's per-turn income by source. It shows exactly the
// values CollectIncome credits: both derive from World.IncomeThisTurn.
func incomeReport(s session.Session, w *ctx) {
	var b game.IncomeBreakdown
	var raids []string
	var madeTroopers, madeJets, madeTurrets, madeBombers, madeTanks, madeCarriers int
	if !withPlayer(w, func(p *game.Empire) {
		b = w.IncomeThisTurn(p)
		raids = p.PirateRaids
		p.PirateRaids = nil
		madeTroopers, madeJets, madeTurrets = p.MadeTroopers, p.MadeJets, p.MadeTurrets
		madeBombers, madeTanks, madeCarriers = p.MadeBombers, p.MadeTanks, p.MadeCarriers
	}) {
		return
	}

	golds := []struct {
		amount int
		text   string
	}{
		{b.Taxes, "gold was earned in taxes."},
		{b.Ore, "gold was produced from the Ore Mines."},
		{b.Tourism, "gold was earned in Tourism."},
		{b.Solar, "gold was earned by Solar Power Generators."},
		{b.Rivers, "gold was earned from Rivers."},
		{b.Industrial, "gold was earned from Industrial Zones."},
		{b.Trade, "gold was earned from Trade."},
	}
	// Right-align every amount to one column width so the report scans cleanly.
	total := 0
	width := len(comma(b.Food))
	for _, l := range golds {
		if l.amount > 0 {
			total += l.amount
			if w := len(comma(l.amount)); w > width {
				width = w
			}
		}
	}
	if w := len(comma(total)); w > width {
		width = w
	}

	titleBar(s, tr(s, "Income Report"))
	amt := func(color string, n int, text string) {
		fmt.Fprintf(s, "  %s%*s%s  %s\n", color, width, comma(n), ansi.Reset, i18n.T(sessionLang(s), text))
	}
	for _, l := range golds {
		if l.amount > 0 {
			amt(ansi.FgBrightCyan, l.amount, l.text)
		}
	}
	if total > 0 {
		fmt.Fprintf(s, "  %s%s%s\n", ansi.FgBlue, strings.Repeat("─", width), ansi.Reset)
		amt(ansi.FgBrightYellow, total, "gold earned this turn.")
	}
	if b.Food > 0 {
		amt(ansi.FgBrightCyan, b.Food, "Food units were grown.")
	}
	for _, r := range raids {
		fmt.Fprintf(s, "  %s%s%s\n", ansi.FgRed, r, ansi.Reset)
	}
	statLine(s, madeTroopers, "Troopers were trained by Industrial Zones.")
	statLine(s, madeJets, "Jets were manufactured by Industrial Zones.")
	statLine(s, madeTurrets, "Turrets were manufactured by Industrial Zones.")
	statLine(s, madeBombers, "Bombers were manufactured by Industrial Zones.")
	statLine(s, madeTanks, "Tanks were manufactured by Industrial Zones.")
	statLine(s, madeCarriers, "Carriers were manufactured by Industrial Zones.")
	pause(s)
}

// peopleMood returns an end-of-turn flavor line keyed to popular support, in
// the spirit of BRE's tiered "how your people feel" message (original wording).
func peopleMood(support int) string {
	switch {
	case support < 10:
		return "The mob is at your gates — your people would be rid of you by any means."
	case support < 20:
		return "Your people seethe with open hatred for your rule."
	case support < 30:
		return "Riots flare through the streets almost daily."
	case support < 40:
		return "Unrest simmers; angry crowds gather against your decrees."
	case support < 50:
		return "Discontent runs deep — your people grumble at every order."
	case support < 60:
		return "Your people endure your rule, but take little joy in it."
	case support < 70:
		return "Your people go about their business, content enough."
	case support < 80:
		return "Your people are glad to live under your banner."
	case support < 90:
		return "Your people admire your leadership and prosper gladly."
	default:
		return "Your people revere you — faith in your rule has never been higher."
	}
}

// endOfTurnStats prints a short flavor line and the remaining turns. It does
// not pause — the caller (runTurn) immediately follows it with the "Continue
// to your next turn?" prompt, and a pause here plus that prompt were two
// consecutive single-key reads that could cross a fast typist's input.
// endOfTurnStats snapshots p under the lock first, since the daily-
// maintenance ticker (or another session) can mutate these same fields
// concurrently.
func endOfTurnStats(s session.Session, w *ctx) {
	var snap game.Empire
	if !withPlayer(w, func(p *game.Empire) { snap = *p }) {
		return
	}
	p := &snap
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "End of Turn Statistics:"), ansi.Reset)
	fmt.Fprintf(s, "  %s\n", tr(s, peopleMood(p.Support)))
	if p.LastPopGrowth > 0 {
		fmt.Fprintf(s, "  "+tr(s, "Your dominion gained %s%s%s people.")+"\n", ansi.FgBrightCyan, comma(p.LastPopGrowth), ansi.Reset)
	} else if p.LastPopGrowth < 0 {
		fmt.Fprintf(s, "  "+tr(s, "Your dominion lost %s%s%s people.")+"\n", ansi.FgRed, comma(-p.LastPopGrowth), ansi.Reset)
	}
	statLine(s, p.LastSpoiled, "units of food spoiled.")
	if p.LastRiot {
		fmt.Fprintf(s, "  %s%s%s\n", ansi.FgRed, tr(s, "Riots have broken out due to high tax rates!"), ansi.Reset)
	}
	statLine(s, p.LastMoraleDesertion, "troops deserted due to low morale.")
	fmt.Fprintf(s, "  "+tr(s, "Turns left today: %d")+"\n", p.TurnsLeft)
}

// paymentStage runs BRE's start-of-turn maintenance prompts. With Auto-Pay
// Maintenance on and enough gold, everything is paid silently. Otherwise
// (pref off, or can't afford a required cost) the player is prompted for
// each obligation: armed-forces upkeep and region maintenance (required,
// underpayment causes desertion/revolt), then an optional support boost.
func paymentStage(s session.Session, w *ctx) {
	// Every transaction below re-resolves the active empire via withPlayer; if it
	// has vanished (eliminated by another node mid-turn) the stage aborts cleanly
	// rather than paying maintenance for a rival's realm.
	if !withPlayer(w, func(p *game.Empire) { p.LastGoldPaid = 0 }) {
		return
	}

	var forces, regions, gold, bank int
	var autoPay bool
	if !withPlayer(w, func(p *game.Empire) {
		forces = w.ForcesDue(p)
		regions = w.RegionsDue(p)
		gold = p.Gold
		bank = p.Bank
		autoPay = w.AutoPayMaint
	}) {
		return
	}
	due := forces + regions

	// If on-hand gold can't cover maintenance but savings can, offer to draw
	// from the bank before paying (BRE lets you visit the bank to make upkeep).
	if gold < due && bank > 0 &&
		askYesNo(s, fmt.Sprintf(tr(s, "Maintenance is %d but you hold only %d gold. Withdraw from your bank (balance %d)?"), due, gold, bank), true) {
		n := promptSuggested(s, "Withdraw how much?", min(due-gold, bank), bank)
		if !withPlayer(w, func(p *game.Empire) {
			w.World.Withdraw(p, n)
			gold = p.Gold
		}) {
			return
		}
	}

	if autoPay && gold >= forces+regions {
		if !withPlayer(w, func(p *game.Empire) {
			w.World.PayForces(p, forces)
			w.World.PayRegions(p, regions)
		}) {
			return
		}
		fmt.Fprintf(s, "\n"+tr(s, "Maintenance paid: %d gold to your forces, %d to your regions.")+"\n", forces, regions)
		return
	}

	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Maintenance:"), ansi.Reset)
	if autoPay {
		fmt.Fprintf(s, "%s%s%s\n", ansi.FgYellow, tr(s, "You cannot cover all your maintenance this turn."), ansi.Reset)
	}

	// Gather the player's intended payments without applying them, so that if
	// they underpay a required obligation we can warn ("DISASTEROUS results")
	// and let them reconsider before the consequences land.
	var forcesGold, regionsGold int
	for {
		fmt.Fprintf(s, "\n"+tr(s, "Your armed forces require %d gold.")+"\n", forces)
		forcesGold = promptSuggested(s, "How much will you give?", min(forces, gold), gold)

		fmt.Fprintf(s, "\n"+tr(s, "%d gold is required to maintain your regions.")+"\n", regions)
		regionsGold = promptSuggested(s, "How much will you give?", min(regions, gold-forcesGold), gold-forcesGold)

		if forcesGold >= forces && regionsGold >= regions {
			break // fully paid — no warning
		}
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgRed, tr(s, "Your actions may lead to disastrous results."), ansi.Reset)
		if !askYesNo(s, "Would you like to reconsider?", true) {
			break // proceed despite the shortfall
		}
		// Reconsider: re-read current gold (it is unchanged; nothing applied yet)
		// and prompt again.
		if !withPlayer(w, func(p *game.Empire) { gold = p.Gold }) {
			return
		}
	}

	var forcesLost, regionsLost, support, morale int
	if !withPlayer(w, func(p *game.Empire) {
		forcesLost = w.World.PayForces(p, forcesGold)
		regionsLost = w.World.PayRegions(p, regionsGold)
		gold = p.Gold
		support = p.Support
		morale = p.Morale
	}) {
		return
	}
	if forcesLost > 0 {
		fmt.Fprintf(s, "%s"+tr(s, "%d units deserted for lack of pay.")+"%s\n", ansi.FgRed, forcesLost, ansi.Reset)
	}
	if regionsLost > 0 {
		fmt.Fprintf(s, "%s"+tr(s, "%d regions revolted for lack of upkeep.")+"%s\n", ansi.FgRed, regionsLost, ansi.Reset)
	}

	if support < 100 && gold > 0 {
		fmt.Fprintf(s, "\n"+tr(s, "%d gold is requested to boost popular support.")+"\n", (100-support)*game.SupportPerBoostGold)
		supportGold := promptSuggested(s, "How much will you give?", 0, gold)
		var pts int
		if !withPlayer(w, func(p *game.Empire) {
			pts = w.World.BoostSupport(p, supportGold)
			gold = p.Gold
		}) {
			return
		}
		if pts > 0 {
			fmt.Fprintf(s, tr(s, "Popular support rose %d points.")+"\n", pts)
		}
	}

	if morale < 100 && gold > 0 {
		fmt.Fprintf(s, "\n"+tr(s, "%d gold is requested to improve military morale.")+"\n", (100-morale)*game.MoralePerBoostGold)
		moraleGold := promptSuggested(s, "How much will you give?", 0, gold)
		var pts int
		if !withPlayer(w, func(p *game.Empire) { pts = w.World.BoostMorale(p, moraleGold) }) {
			return
		}
		if pts > 0 {
			fmt.Fprintf(s, tr(s, "Military morale rose %d points.")+"\n", pts)
		}
	}
}

// runTurn is the "Play Game" action. It first runs BRE's pre-turn stops
// once per Play session — the event log, Diplomacy (act on treaties, or
// Quit out), and Change Production (Set Industries, or decline) (#63) — then
// walks the per-turn pipeline (industry production, income report, status,
// spending/attack/covert/trading/message stages, then end-of-turn) for as
// many turns as the player wants to play (#70).
func runTurn(s session.Session, w *ctx) Result {
	menus := BuildMenus()
	// abort ends the turn cleanly when the active empire has been eliminated by
	// another node mid-turn (withPlayer returned false). The active *Empire is
	// re-resolved inside every transaction below, never captured across a w.With,
	// so a reload that reshapes the empire set can't rebind it to a rival's data.
	abort := func() Result {
		ok(s, "Your realm is no longer active.")
		return Stay
	}
	var turnsLeft int
	if !withPlayer(w, func(p *game.Empire) { turnsLeft = p.TurnsLeft }) {
		return abort()
	}
	if turnsLeft <= 0 {
		ok(s, "Sorry, you have used all of your turns today.")
		seeScores(s, w)
		return Stay
	}

	showTurnEvents(s, w)
	showUnreadMail(s, w)
	if err := Run(s, w, menus.Diplomacy); err != nil {
		return Stay
	}
	setIndustries(s, w)

	for {
		if !withPlayer(w, func(p *game.Empire) { turnsLeft = p.TurnsLeft }) {
			return abort()
		}
		if turnsLeft <= 0 {
			ok(s, "Sorry, you have used all of your turns today.")
			seeScores(s, w)
			return Stay
		}

		if !withPlayer(w, func(p *game.Empire) {
			w.World.Manufacture(p)   // industry production happens at turn start, alongside income (#71)
			w.World.CollectIncome(p) // credit this turn's income up front, so maintenance and spending draw from it
			p.RegionsBoughtThisTurn = 0
		}) {
			return abort()
		}

		incomeReport(s, w)

		// The second status page and the maintenance results share one screen
		// with a single pause (renderEmpireStatus already paused after page 1).
		// If maintenance printed after the pause, the next menu's clear-screen
		// would wipe it before the player could read it.
		renderEmpireStatus(s, w)
		paymentStage(s, w)
		var foodUpkeep int
		if !withPlayer(w, func(p *game.Empire) { foodUpkeep = p.FoodUpkeep() }) {
			return abort()
		}
		statLine(s, foodUpkeep, "units of Food consumed.")
		pause(s)

		if err := Run(s, w, menus.Spending); err != nil {
			return Stay
		}
		if err := Run(s, w, menus.Attack); err != nil {
			return Stay
		}
		if w.VisitCovert {
			if err := Run(s, w, menus.Covert); err != nil {
				return Stay
			}
		}
		if w.VisitTrading {
			if err := Run(s, w, menus.Trading); err != nil {
				return Stay
			}
		}
		if w.VisitMessage {
			if askYesNo(s, "Send a message?", false) {
				sendMessage(s, w)
			}
		}

		if !withPlayer(w, func(p *game.Empire) {
			w.World.PlayTurn(p, w.Today)
			if w.DepositEndTurn && p.Gold > 0 {
				w.World.Deposit(p, p.Gold)
			}
		}) {
			return abort()
		}

		endOfTurnStats(s, w)

		if !withPlayer(w, func(p *game.Empire) { turnsLeft = p.TurnsLeft }) {
			return abort()
		}
		if turnsLeft <= 0 || !askYesNo(s, "Continue to your next turn?", true) {
			return Stay
		}
	}
}
