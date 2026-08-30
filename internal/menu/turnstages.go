package menu

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// turnstages.go — the stages a turn runs through between its recap and its
// end: maintenance, decontamination, spoils, feeding, and the Queen's refund
// and lottery. runTurn in gameflow.go is what orders them.

// paymentStage runs BRE's start-of-turn maintenance prompts. With Auto-Pay
// Maintenance on and enough gold, everything is paid silently. Otherwise
// (pref off, can't afford a required cost, or the auto-pay bypass below
// fires) it runs BRE's manual flow: an optional bank visit (draw savings to
// cover upkeep), then the required armed-forces and region upkeep
// (underpayment causes desertion/revolt), then optional support/morale
// boosts. Underpaying a required cost warns of "disastrous results" and
// offers to reconsider, which loops back to the bank visit — matching BRE,
// where "reconsider" restarts the sequence.
// paymentStage reports whether it printed BRE's auto-pay summary, the one line
// the player has not already answered a prompt for. That decides the pause
// before the Food Market: every auto-pay market turn in the captures pauses
// there (22 across cap/eots-ibbs-01.cap and
// cap/121125-666H4H_Camembert_Public.cap), and the one manual market turn does
// not (cap/kd3-01.cap, market reached straight off the Queen Royale prompt).
func paymentStage(s session.Session, w *ctx, bankMenu *Menu) (summarised bool) {
	// Skip entirely on a replayed turn whose maintenance was already paid before an
	// idle-boot (#10). MaintPaid is set inside the charge transaction below, so
	// reaching here with it set means the required forces/regions charge already
	// committed — re-running would double-charge.
	alreadyPaid := false
	withPlayer(w, func(p *game.Empire) { alreadyPaid = p.TurnProgress.MaintPaid })
	if alreadyPaid {
		return false
	}

	// Every transaction below re-resolves the active empire via withPlayer; if it
	// has vanished (eliminated by another node mid-turn) the stage aborts cleanly
	// rather than paying maintenance for a rival's realm.
	if !withPlayer(w, func(p *game.Empire) { p.LastGoldPaid = 0 }) {
		return false
	}

	var forces, regions, sdi, crown, gold int64
	var autoPay bool
	var support, morale int
	// due is game's own total, not these four added up here: a charge added to
	// MaintenanceDue would otherwise be paid by Auto-Pay and left out of the
	// figure shown. The four parts are still gathered for the itemised path.
	var due int64
	// The gate is read inside the same gather, so it sees the state the four
	// charges below were figured from rather than whatever a concurrent node
	// leaves behind. The rule itself is game.AutoPayApplies.
	var autoPaySilent bool
	if !withPlayer(w, func(p *game.Empire) {
		forces = w.ForcesDue(p)
		regions = w.RegionsDue(p)
		sdi = w.SDIMaintenance(p)
		crown = w.World.CrownTax(p)
		gold = p.Gold
		autoPay = p.Prefs.AutoPayMaint
		due = w.World.MaintenanceDue(p)
		autoPaySilent = w.World.AutoPayApplies(p)
	}) {
		return false
	}

	if autoPaySilent {
		if !withPlayer(w, func(p *game.Empire) {
			w.World.PayForces(p, forces)
			w.World.PayRegions(p, regions)
			w.World.PaySDI(p, sdi)
			w.World.PayCrownTax(p, crown)
			p.TurnProgress.MaintPaid = true // record the charge atomically so a later boot can't replay it (#10)
		}) {
			return false
		}
		// BRE's auto-pay summary is a single comma-grouped total ("5,707,154 Gold
		// paid."), matching the food line under it; the per-item breakdown is what
		// the pay-by-hand path shows, prompt by prompt, exactly as the original
		// does (docs/dev/bre-screens.md).
		fmt.Fprint(s, "\n")
		statLine(s, due, "Gold paid.")
		decontaminateStage(s, w)
		return true
	}

	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Maintenance:"), ansi.Reset)
	if autoPay && gold < due {
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
			return false
		}

		fmt.Fprintf(s, "\n%s"+tr(s, "Your armed forces require %s gold.")+"%s",
			ansi.FgWhite, ansi.FgBrightCyan+comma(forces)+ansi.FgWhite, ansi.Reset)
		forcesGold = promptSuggested(s, "How much will you give?", min(forces, gold), min(forces, gold))

		fmt.Fprintf(s, "\n%s"+tr(s, "%s gold is required to maintain your regions.")+"%s",
			ansi.FgWhite, ansi.FgBrightCyan+comma(regions)+ansi.FgWhite, ansi.Reset)
		regionsGold = promptSuggested(s, "How much will you give?", min(regions, gold-forcesGold), min(regions, gold-forcesGold))

		if sdi > 0 {
			fmt.Fprintf(s, "\n%s"+tr(s, "Your SDI Program requires %s gold.")+"%s",
				ansi.FgWhite, ansi.FgBrightCyan+comma(sdi)+ansi.FgWhite, ansi.Reset)
			afford := min(sdi, gold-forcesGold-regionsGold)
			sdiGold = promptSuggested(s, "How much will you give?", afford, afford)
		}

		// The crown tax comes last, and unlike the two above its prompt maximum is
		// everything still in hand rather than the amount required — BRE lets a
		// baron hand the Queen more than she asked for.
		fmt.Fprintf(s, "\n%s"+tr(s, "The Queen Royale requires %s gold for Taxes.")+"%s",
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

	var forcesLost, regionsLost int
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
		return false
	}
	if forcesLost > 0 {
		fmt.Fprintf(s, "%s"+tr(s, "%d units deserted for lack of pay.")+"%s\n", ansi.FgBrightRed, forcesLost, ansi.Reset)
	}
	if regionsLost > 0 {
		fmt.Fprintf(s, "%s"+tr(s, "%d regions revolted for lack of upkeep.")+"%s\n", ansi.FgBrightRed, regionsLost, ansi.Reset)
	}

	decontaminateStage(s, w)
	if !withPlayer(w, func(p *game.Empire) { gold = p.Gold }) {
		return false
	}

	if support < 100 && gold > 0 {
		var cost, maxGive int64
		if !withPlayer(w, func(p *game.Empire) { cost, maxGive = p.SupportBoostCost(), min(p.SupportBoostMax(), p.Gold) }) {
			return false
		}
		fmt.Fprintf(s, "\n%s", hiNums(fmt.Sprintf(tr(s, "%d gold is requested to boost popular support."), cost)))
		supportGold := promptSuggested(s, "How much will you give?", min(cost, maxGive), maxGive)
		var pts int
		if !withPlayer(w, func(p *game.Empire) {
			pts = w.World.BoostSupport(p, supportGold)
			gold = p.Gold
		}) {
			return false
		}
		if pts > 0 {
			fmt.Fprintf(s, "%s\n", hiNums(fmt.Sprintf(tr(s, "Popular support rose %d points."), pts)))
		}
	}

	if morale < 100 && gold > 0 {
		var cost, maxGive int64
		if !withPlayer(w, func(p *game.Empire) {
			cost, maxGive = w.World.MoraleBoostCost(p), min(w.World.MoraleBoostMax(p), p.Gold)
		}) {
			return false
		}
		fmt.Fprintf(s, "\n%s", hiNums(fmt.Sprintf(tr(s, "%d gold is requested to improve military morale."), cost)))
		moraleGold := promptSuggested(s, "How much will you give?", min(cost, maxGive), maxGive)
		var pts int
		if !withPlayer(w, func(p *game.Empire) { pts = w.World.BoostMorale(p, moraleGold) }) {
			return false
		}
		if pts > 0 {
			fmt.Fprintf(s, "%s\n", hiNums(fmt.Sprintf(tr(s, "Military morale rose %d points."), pts)))
		}
	}
	// Paid by hand: every figure was read as its prompt was answered, so there is
	// no summary and the food stage that follows does not pause.
	return false
}

// decontaminateStage offers to clean waste regions, in the maintenance slot the
// original puts it in — after the SDI upkeep and before the popular-support and
// morale boosts. Like those two it is optional: paying nothing leaves the ruined
// land ruined, costing upkeep and earning nothing, so it competes for gold
// rather than being billed.
func decontaminateStage(s session.Session, w *ctx) {
	var waste, allowance int
	var cost, gold int64
	if !withPlayer(w, func(p *game.Empire) {
		waste, allowance = p.Regions.Waste, w.World.DecontaminateAllowance(p)
		cost, gold = w.World.DecontaminateCost(p), p.Gold
	}) {
		return
	}
	if waste <= 0 || allowance <= 0 || gold <= 0 {
		return
	}
	fmt.Fprintf(s, "\n%s", hiNums(fmt.Sprintf(
		tr(s, "%s gold will decontaminate %s of your %s waste regions."),
		comma(cost), comma(int64(allowance)), comma(int64(waste)))))
	give := promptSuggested(s, "How much will you give?", min(cost, gold), min(cost, gold))
	var cleaned int
	if !withPlayer(w, func(p *game.Empire) { cleaned = w.World.Decontaminate(p, give) }) {
		return
	}
	// Decontaminate restored the land as Coastal so the payment is never in
	// limbo; the picker then re-types it, since cleaned land has no type of its
	// own until the owner names one.
	if cleaned > 0 {
		allocateDecontaminated(s, w, cleaned)
	}
}

// spoilsStage hands over land captured on another planet. The strike resolved
// on the target's board while its owner was not in a session, so the regions
// have been parked with no type since (game.Empire.PendingRegions); this is the
// first point in the turn there is anybody to ask, and it sits right after the
// recap that told them the strike had come home (#107).
func spoilsStage(s session.Session, w *ctx) {
	var pending int
	if !withPlayer(w, func(p *game.Empire) { pending = p.PendingRegions }) {
		return
	}
	if pending <= 0 {
		return
	}
	allocateSpoils(s, w, pending)
}

// feedStage is BRE's Food Market slot in the turn (Payment -> Food Market ->
// Covert). The gate is not in BRE's food routine at all — allocate_food opens
// the market with an unconditional call twelve instructions in — but one level
// up, in run_player_turn's stage dispatch: it sums the two obligations, compares
// them against the realm's stored food, and only when the realm can cover them
// AND the Auto-Feed byte (empire +0x33a) is set does it feed silently and skip
// the routine. Short of food, or Auto-Feed off, the player gets the market and
// both prompts. So Auto-Feed means "feed me without asking, if I can afford it",
// not "open the market for me" — IB had the two halves the other way round, and
// with Auto-Feed off never showed the market at all.
// feedStage reports whether the realm was fed silently, which is what decides
// the "units of Food consumed." summary the caller prints: BRE shows that line
// and its pause only on the silent path, and goes straight from the food
// prompts to the bank when the market ran (cap/eots-ibbs-01.cap, 130 silent
// turns against 8 market ones).
func feedStage(s session.Session, w *ctx, food *Menu, pauseFirst bool) (silent bool, err error) {
	people, forces, have, autoFeed := 0, 0, 0, false
	if !withPlayer(w, func(p *game.Empire) {
		people, forces, have, autoFeed = p.PeopleFoodUpkeep(), w.ForcesFoodDue(p), p.Food, p.Prefs.AutoFeed
	}) {
		return false, nil // realm gone; the caller's next withPlayer aborts the turn
	}
	if have >= people+forces && autoFeed {
		return true, nil // fed automatically — the one path BRE runs silently
	}
	// BRE pauses between the auto-pay total and the market, with no blank line
	// between the two ("Gold paid.\r\n─»>Paused<«─\r\n"), so the player reads what
	// upkeep cost before the market takes the screen. It does NOT pause when the
	// maintenance was paid by hand — those prompts were read as they were
	// answered. Outside the loop: the reconsider returns to the market, not to
	// another pause.
	if pauseFirst {
		pauseTight(s)
	}
	// The Food Market first, then the two obligations. BRE asks TWICE — the people
	// and the armed forces are separate obligations, each with its own prompt — and
	// raises the disastrous-results reconsider once at the end if either was
	// underfed, looping back above the market call. (IB consumes food
	// automatically, so the given amounts gate the reconsider; buying enough food
	// is the real fix.)
	for {
		if err := Run(s, w, food); err != nil {
			return false, err
		}
		if !withPlayer(w, func(p *game.Empire) {
			people, forces, have = p.PeopleFoodUpkeep(), w.ForcesFoodDue(p), p.Food
		}) {
			return false, nil
		}
		short := askFoodGift(s, tr(s, "Your people need %s units of food."), people, &have)
		short = askFoodGift(s, tr(s, "Your armed forces require %s units of food."), forces, &have) || short
		if !short {
			return false, nil // both obligations met — proceed
		}
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightRed, tr(s, "Your actions may lead to disastrous results."), ansi.Reset)
		if !AskYesNo(s, "Would you like to reconsider?", true) {
			return false, nil // proceed despite underfeeding
		}
	}
}

// askFoodGift runs one of BRE's two food prompts: it states the obligation,
// offers as much of the remaining stock as the obligation asks for, and draws
// what the player gives out of that stock so the second prompt sees what the
// first spent. Reports whether the obligation went unmet. A zero obligation is
// skipped, as in BRE, where the prompt only appears when something is owed.
func askFoodGift(s session.Session, label string, need int, stock *int) bool {
	if need <= 0 {
		return false
	}
	fmt.Fprintf(s, "\n%s", hiNums(fmt.Sprintf(label, comma(need))))
	offer := min(*stock, need)
	give := min(promptSuggested(s, "How much will you give?", offer, offer), *stock)
	*stock -= max(give, 0)
	return give < need
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
	fmt.Fprintf(s, "\n%s"+tr(s, "The Queen Royale opens her coffers and refunds you %s gold!")+"%s\n",
		ansi.FgWhite, ansi.FgBrightYellow+comma(paid)+ansi.FgWhite, ansi.Reset)
	pause(s)
}

// showCoordinatorNotice tells an inter-BBS caller where they stand on the BBS
// Coordinator: that they hold the office, or who their vote is for. BRE prints
// one of the two the moment Play Game is chosen, ahead of the "since your last
// play" recap, and only in an InterBBS game (`run_door_session`, BRE.EXE
// 013a:0cf7, behind its InterBBS flag). Without it a baron who never opens the
// vote screen never learns either fact.
//
// A baron who has not voted is named "no one", as the original does: its
// formatter (`format_no_recipient`, BRE.OVR 0x0176f) falls back to that literal
// when the stored vote is not a realm letter, or names a realm whose net worth
// has gone to zero. So a dead or departed choice reads the same as never having
// chosen, and the line is always the same one sentence.
func showCoordinatorNotice(s session.Session, w *ctx) {
	if !w.Config.IBBS {
		return
	}
	var isCoordinator bool
	var voteName string
	var protection int
	withPlayer(w, func(p *game.Empire) {
		if co := w.World.BBSCoordinator(); co != nil && co == p {
			isCoordinator = true
			return
		}
		protection = p.Protection
		if v := w.World.FindByOwner(p.CoordinatorVote); v != nil && v.Alive {
			voteName = v.Name
		}
	})

	// Wrapped before colouring, so a translation longer than the English does
	// not break past column 80 — then the one name the line is about is
	// brightened, as BRE brightens it. A name the wrap has split across two
	// lines simply stays unhighlighted.
	say := func(msg, highlight string) {
		msg = WrapIndented(msg, "")
		if highlight != "" {
			msg = strings.Replace(msg, highlight, ansi.FgBrightWhite+highlight+ansi.FgWhite, 1)
		}
		fmt.Fprintf(s, "%s%s%s\n", ansi.FgWhite, msg, ansi.Reset)
	}

	fmt.Fprint(s, "\n")
	if isCoordinator {
		office := tr(s, "BBS Coordinator")
		say(fmt.Sprintf(tr(s, "You hold the office of %s."), office), office)
		return
	}
	if voteName == "" {
		voteName = tr(s, "no one")
	}
	say(fmt.Sprintf(tr(s, "Your vote for BBS Coordinator is %s."), voteName), voteName)
	// The original sends a protected realm to the System menu for an item the
	// same routine hides from it while protection lasts (see the Coordinator Vote
	// item in tree.go). IB says which of the two is true instead of copying the
	// contradiction — a deliberate divergence (docs/dev/bre-screens.md).
	if protection > 0 {
		say(fmt.Sprintf(tr(s, "You cannot change it until your new-realm protection ends, %d turns from now."), protection), "")
		return
	}
	say(tr(s, "You can change it from the System menu."), "")
}

// showLottery offers the Queen's lottery, once a game day, immediately after
// her tax refund — the original settles the two as one first-play block.
//
// Three details are the original's and look like bugs if you do not know them.
// The 5,000-gold price is charged the moment the offer is accepted and is never
// shown; a realm that cannot pay is not offered a ticket at all; and either way
// the day's offer is spent, so there is no second chance after a refusal or
// after spending down. The price is documented in the Taxes help topic.
func showLottery(s session.Session, w *ctx) {
	offered := false
	withPlayer(w, func(p *game.Empire) {
		offered = w.World.LotteryOffered(p)
		// Spent for the day either way. The original runs this as part of one
		// first-play block, so a realm too poor to be offered a ticket does not
		// get another chance after selling something.
		p.LotteryTaken = true
	})
	if !offered {
		return
	}
	if !AskYesNo(s, "Care to try your luck on the Queen's lottery?", true) {
		return
	}
	bought := false
	withPlayer(w, func(p *game.Empire) { bought = w.World.BuyLotteryTicket(p) })
	if !bought {
		return
	}

	ticket := readLotteryTicket(s, w)
	var draw []byte
	var hit []bool
	var matches int
	var prize int64
	withPlayer(w, func(p *game.Empire) {
		draw, hit, matches = w.World.DrawLottery(ticket)
		prize = w.World.PayLotteryPrize(p, matches)
	})

	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgWhite, tr(s, "Drawn:"), ansi.Reset)
	for i, c := range draw {
		// A matched letter is bright yellow and an unmatched one bright red, but
		// the count below says the same thing in words: colour alone must not be
		// what tells a win from a miss. Bright red rather than the original's
		// dark red, which sits at 2:1 against black.
		color := ansi.FgBrightRed
		if hit[i] {
			color = ansi.FgBrightYellow
		}
		fmt.Fprintf(s, "%s%c%s", color, c, ansi.Reset)
	}
	fmt.Fprint(s, "\n")
	if prize <= 0 {
		ok(s, "No matches. The Queen thanks you for your contribution.")
		return
	}
	ok(s, "%d matched! You win %s gold, paid straight into your bank.", matches, comma(prize))
}

// readLotteryTicket reads the six letters of a ticket, one keypress each,
// accepting only A-Z. Enter takes a random letter for that slot, so a player who
// does not care can hold it down and still hold a playable ticket — the
// original's behaviour, and the reason its prompt takes no Enter to finish.
func readLotteryTicket(s session.Session, w *ctx) []byte {
	// The random fallbacks are drawn in one go, under the world lock, rather than
	// one at a time while the player types: the draw needs the lock and nothing
	// about it depends on which slot it lands in.
	fallback := make([]byte, game.LotteryLetters)
	withPlayer(w, func(*game.Empire) {
		for i := range fallback {
			fallback[i] = w.World.RandomLotteryLetter()
		}
	})

	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgWhite, tr(s, "Pick your six letters:"), ansi.Reset)
	ticket := make([]byte, 0, game.LotteryLetters)
	for len(ticket) < game.LotteryLetters {
		r, err := readKey(s)
		var c byte
		switch {
		case err != nil, r == '\r', r == '\n':
			c = fallback[len(ticket)]
		case r >= 'a' && r <= 'z':
			c = byte(r) - 'a' + 'A'
		case r >= 'A' && r <= 'Z':
			c = byte(r)
		default:
			continue
		}
		ticket = append(ticket, c)
		fmt.Fprintf(s, "%s%c%s", ansi.FgBrightCyan, c, ansi.Reset)
	}
	fmt.Fprint(s, "\n")
	return ticket
}
