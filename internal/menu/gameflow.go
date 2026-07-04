package menu

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// GameLoop is the top-level session flow: show the Game Menu until the
// player quits. "Play Game" runs a turn pipeline.
func GameLoop(s session.Session, w *game.World) error {
	menus := BuildMenus()
	return Run(s, w, menus.Game)
}

// showBulletin prints the planetary bulletin, or a note if there is none.
func showBulletin(s session.Session, w *game.World) Result {
	if len(w.Bulletin) == 0 {
		fmt.Fprint(s, "\nNo planetary bulletins.\n")
	} else {
		fmt.Fprintf(s, "\n%sPlanetary Bulletin:%s\n", ansi.FgBrightCyan, ansi.Reset)
		for _, b := range w.Bulletin {
			fmt.Fprintf(s, "  %s\n", b)
		}
	}
	pause(s)
	return Stay
}

// askYesNo prompts msg with a "(Y/n)" hint, defaulting to yes: only an
// answer starting with 'n'/'N' returns false.
func askYesNo(s session.Session, msg string) bool {
	line := prompt(s, msg+" (Y/n)")
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	return line[0] != 'n' && line[0] != 'N'
}

// showTurnEvents prints and clears p's accumulated events, if any.
func showTurnEvents(s session.Session, p *game.Empire) {
	if len(p.Events) == 0 {
		return
	}
	fmt.Fprintf(s, "\n%sSince your last play, this has happened:%s\n", ansi.FgBrightCyan, ansi.Reset)
	for _, ev := range p.Events {
		fmt.Fprintf(s, "  %s\n", ev)
	}
	p.Events = nil
	pause(s)
}

// incomeReport itemizes p's per-turn income by source, using the same
// per-type multipliers RegionMix.income()/foodProduced() apply.
func incomeReport(s session.Session, w *game.World, p *game.Empire) {
	taxes := p.People * p.Tax / 100 * 8
	ore := p.Regions.Mountain * 12
	tourism := p.Regions.Coastal * 25
	solar := p.Regions.Desert * 20
	rivers := p.Regions.River * 30
	food := p.Regions.Agricultural*250 + p.Regions.Total()*15

	fmt.Fprintf(s, "\n%sIncome Report:%s\n", ansi.FgBrightCyan, ansi.Reset)
	fmt.Fprintf(s, "  %d gold earned in taxes.\n", taxes)
	fmt.Fprintf(s, "  %d gold from Ore Mines.\n", ore)
	fmt.Fprintf(s, "  %d gold in Tourism.\n", tourism)
	fmt.Fprintf(s, "  %d gold by Solar Power.\n", solar)
	fmt.Fprintf(s, "  %d gold from Rivers.\n", rivers)
	fmt.Fprintf(s, "  %d food units grown.\n", food)
	for _, r := range p.PirateRaids {
		fmt.Fprintf(s, "  %s%s%s\n", ansi.FgRed, r, ansi.Reset)
	}
	p.PirateRaids = nil
	pause(s)
}

// endOfTurnStats prints a short flavor line and the remaining turns.
func endOfTurnStats(s session.Session, w *game.World, p *game.Empire) {
	fmt.Fprintf(s, "\n%sEnd of Turn Statistics:%s\n", ansi.FgBrightCyan, ansi.Reset)
	fmt.Fprintf(s, "  The people of %s go about their business.\n", p.Name)
	if p.LastPopGrowth > 0 {
		fmt.Fprintf(s, "  Your dominion gained %d people.\n", p.LastPopGrowth)
	}
	if p.LastSpoiled > 0 {
		fmt.Fprintf(s, "  %d units of food spoiled.\n", p.LastSpoiled)
	}
	if p.LastRiot {
		fmt.Fprintf(s, "  %sRiots have broken out due to high tax rates!%s\n", ansi.FgRed, ansi.Reset)
	}
	if p.IndustryGold > 0 {
		fmt.Fprintf(s, "  %d gold was produced by your Industry.\n", p.IndustryGold)
	}
	if p.MadeTroopers > 0 {
		fmt.Fprintf(s, "  %d Troopers were trained by Industrial Zones.\n", p.MadeTroopers)
	}
	if p.MadeJets > 0 {
		fmt.Fprintf(s, "  %d Jets were manufactured by Industrial Zones.\n", p.MadeJets)
	}
	if p.MadeTurrets > 0 {
		fmt.Fprintf(s, "  %d Turrets were manufactured by Industrial Zones.\n", p.MadeTurrets)
	}
	if p.MadeBombers > 0 {
		fmt.Fprintf(s, "  %d Bombers were manufactured by Industrial Zones.\n", p.MadeBombers)
	}
	if p.MadeTanks > 0 {
		fmt.Fprintf(s, "  %d Tanks were manufactured by Industrial Zones.\n", p.MadeTanks)
	}
	if p.MadeCarriers > 0 {
		fmt.Fprintf(s, "  %d Carriers were manufactured by Industrial Zones.\n", p.MadeCarriers)
	}
	if p.LastGoldPaid > 0 {
		fmt.Fprintf(s, "  %d Gold paid.\n", p.LastGoldPaid)
	}
	if p.LastFoodConsumed > 0 {
		fmt.Fprintf(s, "  %d units of Food consumed.\n", p.LastFoodConsumed)
	}
	fmt.Fprintf(s, "  Turns left today: %d\n", p.TurnsLeft)
	pause(s)
}

// paymentStage runs BRE's start-of-turn maintenance prompts. With Auto-Pay
// Maintenance on and enough gold, everything is paid silently. Otherwise
// (pref off, or can't afford a required cost) the player is prompted for
// each obligation: armed-forces upkeep and region maintenance (required,
// underpayment causes desertion/revolt), then an optional support boost.
func paymentStage(s session.Session, w *game.World, p *game.Empire) {
	p.LastGoldPaid = 0
	forces := p.ForcesUpkeep()
	regions := p.RegionUpkeep()
	due := forces + regions

	// If on-hand gold can't cover maintenance but savings can, offer to draw
	// from the bank before paying (BRE lets you visit the bank to make upkeep).
	if p.Gold < due && p.Bank > 0 &&
		askYesNo(s, fmt.Sprintf("Maintenance is %d but you hold only %d gold. Withdraw from your bank (balance %d)?", due, p.Gold, p.Bank)) {
		n := promptSuggested(s, "Withdraw how much?", min(due-p.Gold, p.Bank), p.Bank)
		w.Withdraw(p, n)
	}

	if w.AutoPayMaint && p.Gold >= forces+regions {
		w.PayForces(p, forces)
		w.PayRegions(p, regions)
		fmt.Fprintf(s, "\nMaintenance paid: %d gold to your forces, %d to your regions.\n", forces, regions)
		return
	}

	fmt.Fprintf(s, "\n%sMaintenance:%s\n", ansi.FgBrightCyan, ansi.Reset)
	if w.AutoPayMaint {
		fmt.Fprintf(s, "%sYou cannot cover all your maintenance this turn.%s\n", ansi.FgYellow, ansi.Reset)
	}

	fmt.Fprintf(s, "\nYour armed forces require %d gold.\n", forces)
	if lost := w.PayForces(p, promptSuggested(s, "How much will you give?", min(forces, p.Gold), p.Gold)); lost > 0 {
		fmt.Fprintf(s, "%s%d units deserted for lack of pay.%s\n", ansi.FgRed, lost, ansi.Reset)
	}

	fmt.Fprintf(s, "\n%d gold is required to maintain your regions.\n", regions)
	if lost := w.PayRegions(p, promptSuggested(s, "How much will you give?", min(regions, p.Gold), p.Gold)); lost > 0 {
		fmt.Fprintf(s, "%s%d regions revolted for lack of upkeep.%s\n", ansi.FgRed, lost, ansi.Reset)
	}

	if p.Support < 100 && p.Gold > 0 && askYesNo(s, "Spend gold to boost popular support?") {
		if pts := w.BoostSupport(p, promptSuggested(s, "How much will you give?", 0, p.Gold)); pts > 0 {
			fmt.Fprintf(s, "Popular support rose %d points.\n", pts)
		}
	}
}

// runTurn is the "Play Game" action: it walks the turn pipeline (event
// log, income report, status, spending/attack/covert/trading/message
// stages, then end-of-turn) for as many turns as the player wants to play.
func runTurn(s session.Session, w *game.World) Result {
	menus := BuildMenus()
	for {
		p := w.Player()
		if p.TurnsLeft <= 0 {
			ok(s, "Sorry, you have used all of your turns today.")
			seeScores(s, w)
			return Stay
		}

		w.CollectIncome(p) // credit this turn's income up front, so maintenance and spending draw from it
		showTurnEvents(s, p)
		incomeReport(s, w, p)
		empireStatus(s, w)

		paymentStage(s, w, p)

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
			if askYesNo(s, "Send a message?") {
				sendMessage(s, w)
			}
		}

		w.PlayTurn(p, w.Today)
		if w.DepositEndTurn && p.Gold > 0 {
			w.Deposit(p, p.Gold)
		}

		endOfTurnStats(s, w, p)

		if p.TurnsLeft <= 0 || !askYesNo(s, "Continue to your next turn?") {
			return Stay
		}
	}
}
