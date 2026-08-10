package menu

import (
	"fmt"
	"strings"
	"time"

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
func GameLoop(s session.Session, w *game.World, handle string, t Term) (err error) {
	// A prompt read failing mid-turn (idle boot or dropped connection) unwinds
	// the whole session via session.End; catch it here and report it as io.EOF,
	// which the caller (play.Session) treats as a clean save-and-exit end.
	defer session.GuardEnd(&err)
	c := &ctx{World: w, handle: handle, Term: t}
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

// collectTurnIncome runs the turn-start block — industry production, income
// collection, and the per-turn region-cap reset — guarded so a turn REPLAYED
// after an idle-boot does not collect income (or reset the region cap) twice
// (#10). The mutation and the IncomeCollected flag commit in one transaction, so
// a boot leaves them consistent. Returns false if the empire vanished mid-turn
// (eliminated by another node), like every withPlayer transaction here.
func collectTurnIncome(w *ctx) bool {
	return withPlayer(w, func(p *game.Empire) {
		if p.TurnProgress.IncomeCollected {
			return
		}
		w.World.Manufacture(p)   // industry production at turn start, alongside income (#71)
		w.World.CollectIncome(p) // credit this turn's income up front, so maintenance and spending draw from it
		w.World.GrowFood(p)      // credit this turn's food at turn start too, so it can be sold this turn (matches BRE)
		p.RegionsBoughtThisTurn = 0
		p.TurnProgress.IncomeCollected = true
	})
}

// showTurnIntro prints the per-turn income report and the empire status pages.
// The income report covers THIS turn's production/income, so on a replay after an
// idle-boot (income already collected before the boot) it is skipped. The empire
// status is shown unconditionally — including on re-entry — so a player dropped
// back mid-turn sees their current state (gold, food, military) before acting,
// instead of landing cold at a submenu (#10).
func showTurnIntro(s session.Session, w *ctx, replaying bool) {
	if !replaying {
		incomeReport(s, w)
	}
	renderEmpireStatus(s, w)
}

// runStageOnce runs one turn stage unless it already completed this turn, in
// which case it is skipped — so a turn REPLAYED after an idle-boot does not walk
// the player back through a menu they already exited (#10). fn runs the stage
// (typically a nested Run); on a clean return the stage is marked done. If fn
// returns an error (a bare io.EOF stream end) the stage is left unmarked, and a
// session boot panics out of fn (via session.End) before the mark is written —
// both leave the stage to re-run on replay, resuming where the boot hit.
func runStageOnce(w *ctx, get func(game.TurnProgress) bool, set func(*game.TurnProgress), fn func() error) error {
	done := false
	withPlayer(w, func(p *game.Empire) { done = get(p.TurnProgress) })
	if done {
		return nil
	}
	if err := fn(); err != nil {
		return err
	}
	withPlayer(w, func(p *game.Empire) { set(&p.TurnProgress) })
	return nil
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
	var boardID, date string
	w.With(func() {
		if yesterday {
			bulletin, news = w.BulletinYesterday, w.NewsYesterday
		} else {
			bulletin, news = w.BulletinToday, w.NewsToday
		}
		// Only name the board in a league, where several planets file news into
		// one feed. On a stand-alone board the prefix is noise — and the default
		// name makes it read "local — Daily Bulletin" (#68).
		if w.Config.IBBS {
			boardID = w.Config.BoardID
		}
		date = w.LastMaintDate
	})
	if yesterday {
		date = prevISODate(date)
	}
	renderNewsMasthead(s, newsDate(date))
	renderDailyBulletin(s, bulletin, boardID)
	if len(news) == 0 {
		fmt.Fprintf(s, "\n%s\n", tr(s, "No planetary bulletins."))
	} else {
		// Each item led by the red arrow, blank-line separated, with empire /
		// faction / number highlights over a white body (BRE layout).
		terms := newsHighlightTerms(w)
		for _, b := range news {
			fmt.Fprintf(s, "\n%s %s\n", newsItemArrow, hiNewsItem(b, terms))
		}
	}
	pause(s)
	return Stay
}

// AskYesNo prompts msg with a "(Y/n)" or "(y/N)" hint (whichever matches
// defYes) and reads a single keypress — no Enter required. 'y'/'Y' returns
// true, 'n'/'N' returns false; Enter or any other key returns defYes.
func AskYesNo(s session.Session, msg string, defYes bool) bool {
	fmt.Fprint(s, "\n")
	return askYesNoHere(s, msg, defYes)
}

// askYesNoHere is AskYesNo without the leading newline, for a prompt BRE puts at
// the end of a line it has already started — the treaty-offer stats line ends
// "…; Score: N; Accept? (Y/n)".
func askYesNoHere(s session.Session, msg string, defYes bool) bool {
	letters := "y/N"
	if defYes {
		letters = "Y/n"
	}
	// BRE colors the y/n hint: the letters cyan, the parens a slightly darker blue.
	hint := ansi.FgBrightBlue + "(" + ansi.FgBrightCyan + letters + ansi.FgBrightBlue + ")" + ansi.Reset
	fmt.Fprintf(s, "%s %s ", i18n.T(sessionLang(s), msg), hint)
	for {
		r, err := readKey(s)
		if err != nil {
			return false // test stream ran out
		}
		drainInput(s) // drop a trailing Enter typed with the single-key answer
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
	var events []game.Event
	withPlayer(w, func(p *game.Empire) {
		events = p.Events
		p.Events = nil
		// The recap consumed everything; re-baseline the mid-session notice so an
		// event appended right after this clear still shows (takeSessionNews's
		// shrink-reset alone couldn't tell it from part of the cleared backlog).
		w.seenEvents, w.seenEventsSet = 0, true
	})
	if len(events) == 0 {
		return
	}
	fmt.Fprintf(s, "\n%s%s%s\n\n", ansi.FgWhite, tr(s, "Since your last play, this has happened:"), ansi.Reset)
	for i, ev := range events {
		fmt.Fprintf(s, "%s\n%s\n\n", eventRule(i+1, ev.When), hiNums(wrapReport(ev.Text)))
	}
	pause(s)
}

// Recap-rule geometry, measured off a live BRE capture: the rule runs 76 columns
// and the timestamp always starts at column 48.
const (
	eventRuleWidth   = 76
	eventStampColumn = 48
)

// eventRule is the numbered, timestamped rule BRE draws above each recap entry:
//
//	─────(1)────────────────────────────────────────07/31/2026  07:43:11────────
//
// The stamp sits at a fixed column so the times line up however many entries
// there are; a world saved before events carried one draws an unbroken rule.
// The capture only shows single-digit counters, so which of the two the original
// holds fixed past nine — the column or the 76-column width — is a guess.
func eventRule(n int, when time.Time) string {
	head := fmt.Sprintf("─────%s(%s%d%s)", dim(ansi.FgBrightYellow), ansi.FgBrightYellow, n, dim(ansi.FgBrightYellow))
	plain := len([]rune(fmt.Sprintf("─────(%d)", n)))
	stamp := ""
	if !when.IsZero() {
		stamp = when.Format(game.StampFormat)
	}
	fill := eventStampColumn - plain
	if fill < 1 {
		fill = 1
	}
	tail := eventRuleWidth - eventStampColumn - len([]rune(stamp))
	if tail < 0 {
		tail = 0
	}
	return ansi.FgBrightBlack + head + ansi.FgBrightBlack +
		strings.Repeat("─", fill) + stamp + strings.Repeat("─", tail) + ansi.Reset
}

// readTurnMail is the mail stop at the head of a turn. BRE asks nothing: the
// messages simply follow the recap, each read in its own box, and an empty inbox
// says so. IB used to state a count and gate the reader behind "Read them now?
// (Y/n)", which is not what the original does.
//
// announceEmpty is true only for the first turn of a session — the spot BRE
// prints "You have no messages." A later turn re-checks so mail arriving from
// another node mid-session is seen (#3), but stays quiet when there is none
// rather than repeating the line up to ten times a day.
func readTurnMail(s session.Session, w *ctx, announceEmpty bool) {
	var count int
	withPlayer(w, func(p *game.Empire) { count = len(p.Mail) })
	if count == 0 {
		if announceEmpty {
			fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgWhite, tr(s, "You have no messages."), ansi.Reset)
		}
		return
	}
	mailReader(s, w)
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
	if b.RiverFood > 0 {
		amt(ansi.FgBrightCyan, b.RiverFood, "Food units were fished from the rivers.")
	}
	for _, r := range raids {
		fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightRed, wrapIndented(r, "  "), ansi.Reset)
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
		fmt.Fprintf(s, "  "+tr(s, "Your dominion lost %s%s%s people.")+"\n", ansi.FgBrightRed, comma(-p.LastPopGrowth), ansi.Reset)
	}
	statLine(s, p.LastStarved, "people left your realm, unable to be fed.")
	statLine(s, p.LastSpoiled, "units of food spoiled.")
	if p.LastRiot {
		fmt.Fprintf(s, "  %s%s%s\n", ansi.FgBrightRed, tr(s, "Riots have broken out due to high tax rates!"), ansi.Reset)
	}
	statLine(s, p.LastMoraleDesertion, "troops deserted due to low morale.")
}

// paymentStage runs BRE's start-of-turn maintenance prompts. With Auto-Pay
// Maintenance on and enough gold, everything is paid silently. Otherwise
// (pref off, or can't afford a required cost) it runs BRE's manual flow: an
// optional bank visit (draw savings to cover upkeep), then the required
// armed-forces and region upkeep (underpayment causes desertion/revolt), then
// optional support/morale boosts. Underpaying a required cost warns of
// "disastrous results" and offers to reconsider, which loops back to the bank
// visit — matching BRE, where "reconsider" restarts the sequence.
func paymentStage(s session.Session, w *ctx, bankMenu *Menu) {
	// Skip entirely on a replayed turn whose maintenance was already paid before an
	// idle-boot (#10). MaintPaid is set inside the charge transaction below, so
	// reaching here with it set means the required forces/regions charge already
	// committed — re-running would double-charge.
	alreadyPaid := false
	withPlayer(w, func(p *game.Empire) { alreadyPaid = p.TurnProgress.MaintPaid })
	if alreadyPaid {
		return
	}

	// Every transaction below re-resolves the active empire via withPlayer; if it
	// has vanished (eliminated by another node mid-turn) the stage aborts cleanly
	// rather than paying maintenance for a rival's realm.
	if !withPlayer(w, func(p *game.Empire) { p.LastGoldPaid = 0 }) {
		return
	}

	var forces, regions, sdi, crown, gold int64
	var autoPay bool
	if !withPlayer(w, func(p *game.Empire) {
		forces = w.ForcesDue(p)
		regions = w.RegionsDue(p)
		sdi = w.SDIMaintenance(p)
		crown = w.World.CrownTax(p)
		gold = p.Gold
		autoPay = w.AutoPayMaint
	}) {
		return
	}
	due := forces + regions + sdi + crown

	if autoPay && gold >= due {
		if !withPlayer(w, func(p *game.Empire) {
			w.World.PayForces(p, forces)
			w.World.PayRegions(p, regions)
			w.World.PaySDI(p, sdi)
			w.World.PayCrownTax(p, crown)
			p.TurnProgress.MaintPaid = true // record the charge atomically so a later boot can't replay it (#10)
		}) {
			return
		}
		// BRE's auto-pay summary is a single comma-grouped total ("5,707,154 Gold
		// paid."), matching the food line under it; the per-item breakdown is what
		// the pay-by-hand path shows, prompt by prompt, exactly as the original
		// does (docs/dev/bre-screens.md).
		fmt.Fprint(s, "\n")
		statLine(s, due, "Gold paid.")
		return
	}

	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Maintenance:"), ansi.Reset)
	if autoPay {
		fmt.Fprintf(s, "%s%s%s\n", ansi.FgYellow, tr(s, "You cannot cover all your maintenance this turn."), ansi.Reset)
	}

	// Gather the player's intended payments without applying them, so that if
	// they underpay a required obligation we can warn ("DISASTEROUS results")
	// and let them reconsider before the consequences land.
	var forcesGold, regionsGold, sdiGold, crownGold int64
	for {
		// BRE opens the manual flow with a bank visit so a baron short on hand can
		// draw savings to cover upkeep; the reconsider below loops back here. Gold
		// is re-read after the visit, since a withdrawal changes it.
		if AskYesNo(s, "Do you wish to visit the bank?", false) {
			_ = Run(s, w, bankMenu) // a disconnect unwinds via the next read below
		}
		if !withPlayer(w, func(p *game.Empire) { gold = p.Gold }) {
			return
		}

		fmt.Fprintf(s, "\n%s"+tr(s, "Your armed forces require %s gold.")+"%s\n",
			ansi.FgWhite, ansi.FgBrightCyan+comma(forces)+ansi.FgWhite, ansi.Reset)
		forcesGold = promptSuggested(s, "How much will you give?", min(forces, gold), min(forces, gold))

		fmt.Fprintf(s, "\n%s"+tr(s, "%s gold is required to maintain your regions.")+"%s\n",
			ansi.FgWhite, ansi.FgBrightCyan+comma(regions)+ansi.FgWhite, ansi.Reset)
		regionsGold = promptSuggested(s, "How much will you give?", min(regions, gold-forcesGold), min(regions, gold-forcesGold))

		if sdi > 0 {
			fmt.Fprintf(s, "\n%s"+tr(s, "Your SDI Program requires %s gold.")+"%s\n",
				ansi.FgWhite, ansi.FgBrightCyan+comma(sdi)+ansi.FgWhite, ansi.Reset)
			afford := min(sdi, gold-forcesGold-regionsGold)
			sdiGold = promptSuggested(s, "How much will you give?", afford, afford)
		}

		// The crown tax comes last, and unlike the two above its prompt maximum is
		// everything still in hand rather than the amount required — BRE lets a
		// baron hand the Queen more than she asked for.
		fmt.Fprintf(s, "\n%s"+tr(s, "The Queen Royale requires %s gold for Taxes.")+"%s\n",
			ansi.FgWhite, ansi.FgBrightCyan+comma(crown)+ansi.FgWhite, ansi.Reset)
		left := gold - forcesGold - regionsGold - sdiGold
		crownGold = promptSuggested(s, "How much will you give?", min(crown, left), left)

		if forcesGold >= forces && regionsGold >= regions && sdiGold >= sdi && crownGold >= crown {
			break // fully paid — no warning
		}
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightRed, tr(s, "Your actions may lead to disastrous results."), ansi.Reset)
		if !AskYesNo(s, "Would you like to reconsider?", true) {
			break // proceed despite the shortfall
		}
		// Reconsider: loop back to the bank visit (gold is re-read at the top).
	}

	var forcesLost, regionsLost, support, morale int
	if !withPlayer(w, func(p *game.Empire) {
		forcesLost = w.World.PayForces(p, forcesGold)
		regionsLost = w.World.PayRegions(p, regionsGold)
		w.World.PaySDI(p, sdiGold)
		w.World.PayCrownTax(p, crownGold)
		p.TurnProgress.MaintPaid = true // required charge committed; a later boot must not replay it (#10)
		gold = p.Gold
		support = p.Support
		morale = p.Morale
	}) {
		return
	}
	if forcesLost > 0 {
		fmt.Fprintf(s, "%s"+tr(s, "%d units deserted for lack of pay.")+"%s\n", ansi.FgBrightRed, forcesLost, ansi.Reset)
	}
	if regionsLost > 0 {
		fmt.Fprintf(s, "%s"+tr(s, "%d regions revolted for lack of upkeep.")+"%s\n", ansi.FgBrightRed, regionsLost, ansi.Reset)
	}

	if support < 100 && gold > 0 {
		var cost, maxGive int64
		if !withPlayer(w, func(p *game.Empire) { cost, maxGive = p.SupportBoostCost(), min(p.SupportBoostMax(), p.Gold) }) {
			return
		}
		fmt.Fprintf(s, "\n%s\n", hiNums(fmt.Sprintf(tr(s, "%d gold is requested to boost popular support."), cost)))
		supportGold := promptSuggested(s, "How much will you give?", min(cost, maxGive), maxGive)
		var pts int
		if !withPlayer(w, func(p *game.Empire) {
			pts = w.World.BoostSupport(p, supportGold)
			gold = p.Gold
		}) {
			return
		}
		if pts > 0 {
			fmt.Fprintf(s, "%s\n", hiNums(fmt.Sprintf(tr(s, "Popular support rose %d points."), pts)))
		}
	}

	if morale < 100 && gold > 0 {
		fmt.Fprintf(s, "\n%s\n", hiNums(fmt.Sprintf(tr(s, "%d gold is requested to improve military morale."), (100-morale)*game.MoralePerBoostGold)))
		moraleGold := promptSuggested(s, "How much will you give?", 0, gold)
		var pts int
		if !withPlayer(w, func(p *game.Empire) { pts = w.World.BoostMorale(p, moraleGold) }) {
			return
		}
		if pts > 0 {
			fmt.Fprintf(s, "%s\n", hiNums(fmt.Sprintf(tr(s, "Military morale rose %d points."), pts)))
		}
	}
}

// feedStage is BRE's Food Market slot in the turn (Payment -> Food Market ->
// Covert). When the realm's stored food can't cover this turn's consumption it
// warns the player; with Auto-Feed on it also brings the Food Market up
// automatically so they can buy food before their people starve (BRE's "the food
// bank comes up automatically" behaviour). With Auto-Feed off it only warns — the
// player manages food from the menu themselves. Fixes the old silent starvation:
// Auto-Feed was a dead no-op and nothing ever flagged a food shortfall.
func feedStage(s session.Session, w *ctx, food *Menu) error {
	need, have, autoFeed := 0, 0, false
	if !withPlayer(w, func(p *game.Empire) {
		need, have, autoFeed = p.FoodUpkeep(), p.Food, w.AutoFeed
	}) {
		return nil // realm gone; the caller's next withPlayer aborts the turn
	}
	if have >= need {
		return nil // fed — no notice
	}
	if !autoFeed {
		// Auto-Feed off: just warn (no pause) — the player manages food from the menu.
		fmt.Fprintf(s, "\n%s"+tr(s, "Your people need %s units of food and you have only %s.")+"%s\n",
			ansi.FgYellow, comma(need), comma(have), ansi.Reset)
		fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightRed, tr(s, "Visit the Food Market to feed them, or they will starve."), ansi.Reset)
		return nil
	}
	// Auto-Feed on and short: bring up the Food Market so the player can buy food,
	// then ask how much to give (BRE's "How much will you give?"). Giving LESS than
	// required triggers the disastrous-results reconsider, which loops back to the
	// market — exactly like IB's maintenance-underpayment guard; giving the full
	// amount proceeds silently. (IB consumes food automatically, so the given amount
	// gates the reconsider; buying enough food is the real fix.)
	for {
		if err := Run(s, w, food); err != nil {
			return err
		}
		if !withPlayer(w, func(p *game.Empire) { need, have = p.FoodUpkeep(), p.Food }) {
			return nil
		}
		fmt.Fprintf(s, "\n%s\n", hiNums(fmt.Sprintf(tr(s, "Your people need %s units of food."), comma(need))))
		give := promptSuggested(s, "How much will you give?", min(have, need), min(have, need))
		if give >= need {
			return nil // fed the full amount — proceed
		}
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightRed, tr(s, "Your actions may lead to disastrous results."), ansi.Reset)
		if !AskYesNo(s, "Would you like to reconsider?", true) {
			return nil // proceed despite underfeeding
		}
	}
}

// showQueenRefund hands the realm its share of the Queen's purse, once per game
// day, and announces it. BRE prints this in the opening recap right after the
// mail, which is where this sits. Silent when the realm has already drawn today
// or the purse has nothing in it — a fresh planet that has collected no tax yet.
func showQueenRefund(s session.Session, w *ctx) {
	var paid int64
	withPlayer(w, func(p *game.Empire) {
		if p.RefundTaken {
			return
		}
		p.RefundTaken = true
		paid = w.World.QueenRefund(p)
	})
	if paid <= 0 {
		return
	}
	fmt.Fprintf(s, "\n%s"+tr(s, "The Queen Royale opens her coffers and refunds you %s gold in taxes!")+"%s\n",
		ansi.FgWhite, ansi.FgBrightYellow+comma(paid)+ansi.FgWhite, ansi.Reset)
}

// runTurn is the "Play Game" action. It shows the event log, then walks the
// per-turn pipeline (industry production, income report, status,
// spending/attack/covert/trading/message stages, then end-of-turn) for as
// many turns as the player wants to play (#70). Diplomacy and Change Production
// are no longer pre-turn stops here — they stay reachable from the System menu.
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
	reviewTreatyOffers(s, w) // BRE surfaces pending treaty offers here, with proposer stats + accept/decline
	reviewTradeDeals(s, w)   // and pending trade-deal barters (accept/decline)

	firstTurn := true
	for {
		if !withPlayer(w, func(p *game.Empire) { turnsLeft = p.TurnsLeft }) {
			return abort()
		}
		if turnsLeft <= 0 {
			ok(s, "Sorry, you have used all of your turns today.")
			seeScores(s, w)
			return Stay
		}

		// New mail may arrive between turns (from another node), so check each
		// turn, not just once in the pre-turn flow (#3).
		readTurnMail(s, w, firstTurn)
		if firstTurn {
			showQueenRefund(s, w)
		}
		firstTurn = false

		// Capture whether this is a replay BEFORE collecting income (which sets the
		// flag), so the intro screens can be skipped on a booted-turn replay.
		replaying := false
		withPlayer(w, func(p *game.Empire) { replaying = p.TurnProgress.IncomeCollected })

		if !collectTurnIncome(w) {
			return abort()
		}

		showTurnIntro(s, w, replaying)
		paymentStage(s, w, menus.Bank)
		// Was the realm already fed before this pass? On a replay it may have been
		// (fed before the boot), in which case the feed stage and its "food consumed"
		// summary are both skipped — so the replay lands at the first unfinished
		// stage without re-showing a screen the player already saw (#10).
		fedBefore := false
		withPlayer(w, func(p *game.Empire) { fedBefore = p.TurnProgress.Fed })
		if err := runStageOnce(w,
			func(tp game.TurnProgress) bool { return tp.Fed },
			func(tp *game.TurnProgress) { tp.Fed = true },
			func() error { return feedStage(s, w, menus.Food) }); err != nil {
			return Stay
		}
		if !fedBefore {
			var foodUpkeep int
			if !withPlayer(w, func(p *game.Empire) { foodUpkeep = p.FoodUpkeep() }) {
				return abort()
			}
			statLine(s, foodUpkeep, "units of Food consumed.")
			pause(s)
		}

		// Covert Operations runs right after maintenance and before Spending, per
		// BRE's turn order (Payment/Food Market -> Covert -> Spending). Shown only
		// when the player keeps the step on (Preferences) AND holds at least one
		// covert agent to act with — a fresh realm starts with none.
		if w.VisitCovert {
			var agents int
			if !withPlayer(w, func(p *game.Empire) { agents = p.Agents }) {
				return abort()
			}
			if agents >= 1 {
				if err := runStageOnce(w,
					func(tp game.TurnProgress) bool { return tp.CovertDone },
					func(tp *game.TurnProgress) { tp.CovertDone = true },
					func() error { return Run(s, w, menus.Covert) }); err != nil {
					return Stay
				}
			}
		}

		if err := runStageOnce(w,
			func(tp game.TurnProgress) bool { return tp.SpendingDone },
			func(tp *game.TurnProgress) { tp.SpendingDone = true },
			func() error { return Run(s, w, menus.Spending) }); err != nil {
			return Stay
		}
		if err := runStageOnce(w,
			func(tp game.TurnProgress) bool { return tp.AttackDone },
			func(tp *game.TurnProgress) { tp.AttackDone = true },
			func() error { return Run(s, w, menus.Attack) }); err != nil {
			return Stay
		}
		if w.VisitTrading {
			if err := runStageOnce(w,
				func(tp game.TurnProgress) bool { return tp.TradingDone },
				func(tp *game.TurnProgress) { tp.TradingDone = true },
				func() error { return Run(s, w, menus.Trading) }); err != nil {
				return Stay
			}
		}
		// InterPlanetary Ops is an InterBBS-only step, shown after Trading.
		if w.Config.InterBBSEnabled() {
			if err := runStageOnce(w,
				func(tp game.TurnProgress) bool { return tp.InterPlanetaryDone },
				func(tp *game.TurnProgress) { tp.InterPlanetaryDone = true },
				func() error { return Run(s, w, menus.InterPlanetary) }); err != nil {
				return Stay
			}
		}
		if w.VisitMessage {
			_ = runStageOnce(w,
				func(tp game.TurnProgress) bool { return tp.MessageDone },
				func(tp *game.TurnProgress) { tp.MessageDone = true },
				func() error {
					if AskYesNo(s, "Send a message?", false) {
						sendMessage(s, w)
					}
					return nil
				})
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
		if turnsLeft <= 0 || !AskYesNo(s, "Continue to your next turn?", true) {
			return Stay
		}
	}
}
