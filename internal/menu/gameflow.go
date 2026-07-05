package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/session"
)

// GameLoop is the top-level session flow: show the Game Menu until the
// player quits. "Play Game" runs a turn pipeline. active is the empire
// playing THIS session — per-session state, not shared World state, so the
// web front-end can run concurrent sessions against one World.
func GameLoop(s session.Session, w *game.World, active *game.Empire) error {
	c := &ctx{World: w, active: active}
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

// showBulletin prints the planetary bulletin, or a note if there is none.
func showBulletin(s session.Session, w *ctx) Result {
	if len(w.Bulletin) == 0 {
		fmt.Fprintf(s, "\n%s\n", tr(s, "No planetary bulletins."))
	} else {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Planetary Bulletin:"), ansi.Reset)
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
	fmt.Fprintf(s, "\n%s (Y/n) ", i18n.T(sessionLang(s), msg))
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

// askYesNoDefaultNo prompts "(y/N)" and defaults to No, so Enter declines.
func askYesNoDefaultNo(s session.Session, msg string) bool {
	fmt.Fprintf(s, "\n%s (y/N) ", i18n.T(sessionLang(s), msg))
	for {
		r, err := s.ReadKey()
		if err != nil {
			return false
		}
		switch r {
		case 'y', 'Y':
			fmt.Fprint(s, "y\n")
			return true
		case 'n', 'N', '\r', '\n':
			fmt.Fprint(s, "n\n")
			return false
		}
	}
}

// showTurnEvents prints and clears p's accumulated events, if any.
func showTurnEvents(s session.Session, p *game.Empire) {
	if len(p.Events) == 0 {
		return
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Since your last play, this has happened:"), ansi.Reset)
	for _, ev := range p.Events {
		fmt.Fprintf(s, "  %s\n", ev)
	}
	p.Events = nil
	pause(s)
}

// incomeReport itemizes p's per-turn income by source. It shows exactly the
// values CollectIncome credits: both derive from World.IncomeThisTurn.
func incomeReport(s session.Session, w *ctx, p *game.Empire) {
	b := w.IncomeThisTurn(p)

	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Income Report:"), ansi.Reset)
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
func endOfTurnStats(s session.Session, w *ctx, p *game.Empire) {
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "End of Turn Statistics:"), ansi.Reset)
	fmt.Fprintf(s, "  "+tr(s, "The people of %s go about their business.")+"\n", p.Name)
	if p.LastPopGrowth > 0 {
		fmt.Fprintf(s, "  "+tr(s, "Your dominion gained %s%s%s people.")+"\n", ansi.FgBrightCyan, comma(p.LastPopGrowth), ansi.Reset)
	}
	statLine(s, p.LastSpoiled, "units of food spoiled.")
	if p.LastRiot {
		fmt.Fprintf(s, "  %s%s%s\n", ansi.FgRed, tr(s, "Riots have broken out due to high tax rates!"), ansi.Reset)
	}
	statLine(s, p.IndustryGold, "gold was produced by your Industry.")
	statLine(s, p.MadeTroopers, "Troopers were trained by Industrial Zones.")
	statLine(s, p.MadeJets, "Jets were manufactured by Industrial Zones.")
	statLine(s, p.MadeTurrets, "Turrets were manufactured by Industrial Zones.")
	statLine(s, p.MadeBombers, "Bombers were manufactured by Industrial Zones.")
	statLine(s, p.MadeTanks, "Tanks were manufactured by Industrial Zones.")
	statLine(s, p.MadeCarriers, "Carriers were manufactured by Industrial Zones.")
	fmt.Fprintf(s, "  "+tr(s, "Turns left today: %d")+"\n", p.TurnsLeft)
	pause(s)
}

// paymentStage runs BRE's start-of-turn maintenance prompts. With Auto-Pay
// Maintenance on and enough gold, everything is paid silently. Otherwise
// (pref off, or can't afford a required cost) the player is prompted for
// each obligation: armed-forces upkeep and region maintenance (required,
// underpayment causes desertion/revolt), then an optional support boost.
func paymentStage(s session.Session, w *ctx, p *game.Empire) {
	w.With(func() { p.LastGoldPaid = 0 })
	forces := w.ForcesDue(p)
	regions := w.RegionsDue(p)
	due := forces + regions

	// If on-hand gold can't cover maintenance but savings can, offer to draw
	// from the bank before paying (BRE lets you visit the bank to make upkeep).
	if p.Gold < due && p.Bank > 0 &&
		askYesNo(s, fmt.Sprintf(tr(s, "Maintenance is %d but you hold only %d gold. Withdraw from your bank (balance %d)?"), due, p.Gold, p.Bank)) {
		n := promptSuggested(s, "Withdraw how much?", min(due-p.Gold, p.Bank), p.Bank)
		w.With(func() { w.World.Withdraw(p, n) })
	}

	if w.AutoPayMaint && p.Gold >= forces+regions {
		w.With(func() {
			w.World.PayForces(p, forces)
			w.World.PayRegions(p, regions)
		})
		fmt.Fprintf(s, "\n"+tr(s, "Maintenance paid: %d gold to your forces, %d to your regions.")+"\n", forces, regions)
		return
	}

	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Maintenance:"), ansi.Reset)
	if w.AutoPayMaint {
		fmt.Fprintf(s, "%s%s%s\n", ansi.FgYellow, tr(s, "You cannot cover all your maintenance this turn."), ansi.Reset)
	}

	fmt.Fprintf(s, "\n"+tr(s, "Your armed forces require %d gold.")+"\n", forces)
	forcesGold := promptSuggested(s, "How much will you give?", min(forces, p.Gold), p.Gold)
	var forcesLost int
	w.With(func() { forcesLost = w.World.PayForces(p, forcesGold) })
	if forcesLost > 0 {
		fmt.Fprintf(s, "%s"+tr(s, "%d units deserted for lack of pay.")+"%s\n", ansi.FgRed, forcesLost, ansi.Reset)
	}

	fmt.Fprintf(s, "\n"+tr(s, "%d gold is required to maintain your regions.")+"\n", regions)
	regionsGold := promptSuggested(s, "How much will you give?", min(regions, p.Gold), p.Gold)
	var regionsLost int
	w.With(func() { regionsLost = w.World.PayRegions(p, regionsGold) })
	if regionsLost > 0 {
		fmt.Fprintf(s, "%s"+tr(s, "%d regions revolted for lack of upkeep.")+"%s\n", ansi.FgRed, regionsLost, ansi.Reset)
	}

	if p.Support < 100 && p.Gold > 0 && askYesNo(s, "Spend gold to boost popular support?") {
		supportGold := promptSuggested(s, "How much will you give?", 0, p.Gold)
		var pts int
		w.With(func() { pts = w.World.BoostSupport(p, supportGold) })
		if pts > 0 {
			fmt.Fprintf(s, tr(s, "Popular support rose %d points.")+"\n", pts)
		}
	}
}

// runTurn is the "Play Game" action: it walks the turn pipeline (event
// log, income report, status, spending/attack/covert/trading/message
// stages, then end-of-turn) for as many turns as the player wants to play.
func runTurn(s session.Session, w *ctx) Result {
	menus := BuildMenus()
	for {
		p := w.Player()
		if p.TurnsLeft <= 0 {
			ok(s, "Sorry, you have used all of your turns today.")
			seeScores(s, w)
			return Stay
		}

		w.With(func() { w.World.CollectIncome(p) }) // credit this turn's income up front, so maintenance and spending draw from it
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

		w.With(func() {
			w.World.PlayTurn(p, w.Today)
			if w.DepositEndTurn && p.Gold > 0 {
				w.World.Deposit(p, p.Gold)
			}
		})

		endOfTurnStats(s, w, p)

		if p.TurnsLeft <= 0 || !askYesNo(s, "Continue to your next turn?") {
			return Stay
		}
	}
}
