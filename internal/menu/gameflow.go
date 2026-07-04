package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// GameLoop is the top-level session flow: show the Game Menu until the
// player quits. "Play Game" runs a turn pipeline.
func GameLoop(s session.Session, w *game.World) error {
	menus := BuildMenus()
	// Expand Ctrl-<letter> into the active player's saved macro keystrokes.
	// This single wrap covers every front-end, since all of them reach the
	// menu through GameLoop.
	ms := session.NewMacroExpander(s, func(letter string) (string, bool) {
		p := w.Player()
		if p == nil {
			return "", false
		}
		seq, ok := p.Macros[letter]
		return seq, ok
	})
	return Run(ms, w, menus.Game)
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

// askYesNo prompts msg with a "(Y/n)" hint and reads a single keypress —
// 'y'/'Y' or Enter means yes (the default), 'n'/'N' means no. No Enter is
// required after the letter.
func askYesNo(s session.Session, msg string) bool {
	fmt.Fprintf(s, "\n%s (Y/n) ", msg)
	for {
		r, err := s.ReadKey()
		if err != nil {
			return false
		}
		switch r {
		case 'n', 'N':
			fmt.Fprint(s, "n\n")
			return false
		case 'y', 'Y', '\r', '\n':
			fmt.Fprint(s, "y\n")
			return true
		}
	}
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

// incomeReport itemizes p's per-turn income by source. It shows exactly the
// values CollectIncome credits: both derive from World.IncomeThisTurn.
func incomeReport(s session.Session, w *game.World, p *game.Empire) {
	b := w.IncomeThisTurn(p)

	fmt.Fprintf(s, "\n%sIncome Report:%s\n", ansi.FgBrightCyan, ansi.Reset)
	statLine(s, b.Taxes, "gold was earned in taxes.")
	statLine(s, b.Ore, "gold was produced from the Ore Mines.")
	statLine(s, b.Tourism, "gold was earned in Tourism.")
	statLine(s, b.Solar, "gold was earned by Solar Power Generators.")
	statLine(s, b.Rivers, "gold was earned from Rivers.")
	statLine(s, b.Urban, "gold was earned in Urban Centers.")
	statLine(s, b.Industrial, "gold was earned from Industrial Zones.")
	statLine(s, b.Technology, "gold was earned from Technology.")
	statLine(s, b.Trade, "gold was earned from Trade.")
	statLine(s, b.Food, "Food units were grown.")
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
		fmt.Fprintf(s, "  Your dominion gained %s%s%s people.\n", ansi.FgBrightCyan, comma(p.LastPopGrowth), ansi.Reset)
	}
	statLine(s, p.LastSpoiled, "units of food spoiled.")
	if p.LastRiot {
		fmt.Fprintf(s, "  %sRiots have broken out due to high tax rates!%s\n", ansi.FgRed, ansi.Reset)
	}
	statLine(s, p.IndustryGold, "gold was produced by your Industry.")
	statLine(s, p.MadeTroopers, "Troopers were trained by Industrial Zones.")
	statLine(s, p.MadeJets, "Jets were manufactured by Industrial Zones.")
	statLine(s, p.MadeTurrets, "Turrets were manufactured by Industrial Zones.")
	statLine(s, p.MadeBombers, "Bombers were manufactured by Industrial Zones.")
	statLine(s, p.MadeTanks, "Tanks were manufactured by Industrial Zones.")
	statLine(s, p.MadeCarriers, "Carriers were manufactured by Industrial Zones.")
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
	forces := w.ForcesDue(p)
	regions := w.RegionsDue(p)
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

		// Status and the maintenance results share one screen with a single
		// pause. If maintenance printed after the pause, the next menu's
		// clear-screen would wipe it before the player could read it.
		renderEmpireStatus(s, w)
		paymentStage(s, w, p)
		statLine(s, p.FoodUpkeep(), "units of Food consumed.")
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
