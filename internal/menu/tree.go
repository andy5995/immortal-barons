package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/game"
)

// Build constructs the full BRE menu tree and returns the main menu.
// Menus are created first, then wired, so submenus can reference each
// other (e.g. several menus offer "Visit Bank").
func Build() *Menu {
	main := &Menu{Title: "Immortal Barons — Main Menu"}
	buy := &Menu{Title: "Buy / Sell"}
	bank := &Menu{Title: "Bank"}
	attack := &Menu{Title: "War / Attack"}
	covert := &Menu{Title: "Covert Operations"}
	trading := &Menu{Title: "Trading"}
	diplomacy := &Menu{Title: "Diplomacy"}
	messages := &Menu{Title: "Messages"}
	display := &Menu{Title: "Display / Info"}
	prefs := &Menu{Title: "Preferences"}
	coord := &Menu{Title: "Sysop / Coordinator"}

	main.Status = statusBar
	main.Items = []Item{
		{Key: 'B', Label: "Buy / Sell", Do: gotoMenu(buy)},
		{Key: 'K', Label: "Bank", Do: gotoMenu(bank)},
		{Key: 'W', Label: "War / Attack", Do: gotoMenu(attack)},
		{Key: 'C', Label: "Covert Operations", Do: gotoMenu(covert)},
		{Key: 'T', Label: "Trading", Do: gotoMenu(trading)},
		{Key: 'R', Label: "Diplomacy (Relations)", Do: gotoMenu(diplomacy)},
		{Key: 'M', Label: "Messages", Do: gotoMenu(messages)},
		{Key: 'D', Label: "Display / Info", Do: gotoMenu(display)},
		{Key: 'P', Label: "Preferences", Do: gotoMenu(prefs)},
		{Key: 'Y', Label: "Sysop / Coordinator", Do: gotoMenu(coord),
			Hidden: func(w *game.World) bool { return !w.Coordinator }},
		{Key: 'N', Label: "Next Turn (end turn)", Do: nextTurn},
		{Key: '?', Label: "Show Instructions", Do: stubbed("Instructions")},
		{Key: 'Q', Label: "Quit", Do: quit},
	}

	buy.Items = []Item{
		{Key: 'L', Label: "Buy Land / Regions",
			Do: buy2("Buy Land", func(w *game.World) int { return w.Prices.Land }, (*game.World).BuyLand)},
		{Key: 'F', Label: "Buy Food",
			Do: buy2("Buy Food", func(w *game.World) int { return w.Prices.Food }, (*game.World).BuyFood)},
		{Key: 'T', Label: "Recruit Troopers",
			Do: buy2("Recruit Troopers", func(w *game.World) int { return w.Prices.Trooper }, (*game.World).Recruit)},
		{Key: 'J', Label: "Build Jets",
			Do: buy2("Build Jets", func(w *game.World) int { return w.Prices.Jet }, (*game.World).BuildJets)},
		{Key: 'U', Label: "Build Turrets (defense; shoots down jets)",
			Do: buy2("Build Turrets", func(w *game.World) int { return w.Prices.Turret }, (*game.World).BuildTurrets)},
		{Key: 'A', Label: "Build Tanks",
			Do: buy2("Build Tanks", func(w *game.World) int { return w.Prices.Tank }, (*game.World).BuildTanks)},
		{Key: 'C', Label: "Build Carriers (move jets to attack)",
			Do: buy2("Build Carriers", func(w *game.World) int { return w.Prices.Carrier }, (*game.World).BuildCarriers)},
		{Key: 'S', Label: "Sell Land",
			Do: buy2("Sell Land", func(w *game.World) int { return w.Prices.Land / 2 }, (*game.World).SellLand)},
		{Key: 'O', Label: "Build Bombers", Do: stubbed("Build Bombers")},
		{Key: 'H', Label: "Build HeadQuarters", Do: stubbed("Build HeadQuarters")},
		{Key: 'B', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: 'R', Label: "Return", Do: back},
	}

	bank.Items = []Item{
		{Key: 'D', Label: "Deposit", Do: money("Deposit", (*game.World).Deposit)},
		{Key: 'W', Label: "Withdraw", Do: money("Withdraw", (*game.World).Withdraw)},
		{Key: 'L', Label: "Take Loan", Do: money("Borrow", (*game.World).Loan)},
		{Key: 'P', Label: "Repay Loan", Do: money("Repay", (*game.World).Repay)},
		{Key: 'I', Label: "Invest", Do: money("Invest", (*game.World).Deposit)},
		{Key: 'R', Label: "Return", Do: back},
	}

	attack.Items = []Item{
		{Key: 'R', Label: "Regular Attack", Do: regularAttack},
		{Key: 'N', Label: "Nuclear Attack", Do: nuclearAttack},
		{Key: 'C', Label: "Chemical Attack", Do: chemicalAttack},
		{Key: 'B', Label: "Biological Attack", Do: biologicalAttack},
		{Key: 'P', Label: "Attack Pirates", Do: attackPirates},
		{Key: 'G', Label: "Create Group Attack", Do: stubbed("Create Group Attack")},
		{Key: 'J', Label: "Join Group Attack", Do: stubbed("Join Group Attack")},
		{Key: 'T', Label: "Terrorist Ops", Do: stubbed("Terrorist Ops")},
		{Key: 'K', Label: "Gooie Kablooie Ops", Do: stubbed("Gooie Kablooie Ops")},
		{Key: 'S', Label: "SDI Program", Do: stubbed("SDI Program")},
		{Key: 'V', Label: "Travel Times", Do: stubbed("Travel Times")},
		{Key: 'X', Label: "Return", Do: back},
	}

	covert.Items = []Item{
		{Key: 'S', Label: "Send Spy", Do: stubbed("Send Spy")},
		{Key: 'P', Label: "Spy on Relations", Do: stubbed("Spy on Relations")},
		{Key: 'D', Label: "Spy Database", Do: stubbed("Spy Database")},
		{Key: 'B', Label: "Bribery", Do: stubbed("Bribery")},
		{Key: 'O', Label: "Special Operations", Do: stubbed("Special Operations")},
		{Key: 'K', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: 'R', Label: "Return", Do: back},
	}

	trading.Items = []Item{
		{Key: 'F', Label: "Food Market", Do: stubbed("Food Market")},
		{Key: 'S', Label: "Send Trade Deal", Do: stubbed("Send Trade Deal")},
		{Key: 'V', Label: "View IPScores", Do: stubbed("View IPScores")},
		{Key: 'B', Label: "Buy / Sell", Do: gotoMenu(buy)},
		{Key: 'K', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: 'R', Label: "Return", Do: back},
	}

	diplomacy.Items = []Item{
		{Key: 'M', Label: "Modify Diplomacy", Do: stubbed("Modify Diplomacy")},
		{Key: 'V', Label: "View Diplomacy", Do: stubbed("View Diplomacy")},
		{Key: 'L', Label: "Diplomacy List", Do: stubbed("Diplomacy List")},
		{Key: 'R', Label: "Return", Do: back},
	}

	messages.Items = []Item{
		{Key: 'R', Label: "Read Messages", Do: stubbed("Read Messages")},
		{Key: 'S', Label: "Send Message", Do: stubbed("Send Message")},
		{Key: 'P', Label: "Planetary Post", Do: stubbed("Planetary Post")},
		{Key: 'X', Label: "Return", Do: back},
	}

	display.Items = []Item{
		{Key: 'E', Label: "Empire Status", Do: empireStatus},
		{Key: 'S', Label: "See Scores", Do: seeScores},
		{Key: 'A', Label: "Visit Advisors", Do: stubbed("Visit Advisors")},
		{Key: 'I', Label: "InterBBS Scores", Do: stubbed("InterBBS Scores")},
		{Key: 'D', Label: "Spy Database", Do: stubbed("Spy Database")},
		{Key: 'L', Label: "Diplomacy List", Do: stubbed("Diplomacy List")},
		{Key: 'T', Label: "Travel Times", Do: stubbed("Travel Times")},
		{Key: 'R', Label: "Return", Do: back},
	}

	prefs.Items = []Item{
		{Key: 'E', LabelFn: onOff("Enter exits Buy menu", func(w *game.World) *bool { return &w.EnterExitsBuy }),
			Do: toggle(func(w *game.World) *bool { return &w.EnterExitsBuy })},
		{Key: 'D', LabelFn: onOff("Deposit gold at end of turn", func(w *game.World) *bool { return &w.DepositEndTurn }),
			Do: toggle(func(w *game.World) *bool { return &w.DepositEndTurn })},
		{Key: 'M', LabelFn: onOff("Auto-pay maintenance", func(w *game.World) *bool { return &w.AutoPayMaint }),
			Do: toggle(func(w *game.World) *bool { return &w.AutoPayMaint })},
		{Key: 'F', LabelFn: onOff("Auto-feed people & army", func(w *game.World) *bool { return &w.AutoFeed }),
			Do: toggle(func(w *game.World) *bool { return &w.AutoFeed })},
		{Key: 'R', Label: "Return", Do: back},
	}

	coord.Items = []Item{
		{Key: 'C', Label: "Configuration Editor", Do: stubbed("Configuration Editor")},
		{Key: 'M', Label: "Modify League Diplomacy", Do: stubbed("Modify League Diplomacy")},
		{Key: 'P', Label: "Player List", Do: stubbed("Player List")},
		{Key: 'R', Label: "Return", Do: back},
	}

	return main
}

func statusBar(w *game.World) string {
	p := w.Player()
	return fmt.Sprintf("%s | Gold %d  Food %d  Land %d  Army %d | Turns left %d | Day %d",
		p.Name, p.Gold, p.Food, p.Land, p.Army(), p.TurnsLeft, w.GameDay)
}
