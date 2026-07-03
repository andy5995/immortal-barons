package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/game"
)

// Menus holds every top-level and sub menu built by BuildMenus, so the
// outer game flow (gameflow.go) can drive them as turn-pipeline stages
// without re-parsing the tree.
type Menus struct {
	Spending  *Menu // the "Spending Menu" (formerly "buy")
	Bank      *Menu
	Attack    *Menu
	Covert    *Menu
	Trading   *Menu
	Diplomacy *Menu
	Messages  *Menu
	System    *Menu
	Game      *Menu
	Food      *Menu
}

// BuildMenus constructs the full BRE menu tree. Menus are created first,
// then wired, so submenus can reference each other (e.g. several menus
// offer "Visit Bank").
func BuildMenus() *Menus {
	buy := &Menu{Title: "Spending Menu"}
	bank := &Menu{Title: "Bank"}
	attack := &Menu{Title: "War / Attack"}
	covert := &Menu{Title: "Covert Operations"}
	trading := &Menu{Title: "Trading"}
	diplomacy := &Menu{Title: "Diplomacy"}
	messages := &Menu{Title: "Messages"}
	prefs := &Menu{Title: "Preferences"}
	coord := &Menu{Title: "Sysop / Coordinator"}
	system := &Menu{Title: "System Menu"}
	food := &Menu{Title: "Food Market"}

	buy.Items = []Item{
		{Key: 'L', Label: "Buy Land / Regions", Do: buyLand},
		{Key: 'F', Label: "Buy Food",
			Do: buy2("Buy Food", func(w *game.World) int { return game.FoodBuyPrice }, (*game.World).BuyFoodMarket)},
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
		{Key: 'S', Label: "Sell Land", Do: sellLand},
		{Key: 'G', Label: "Recruit Agents",
			Do: buy2("Recruit Agents", func(w *game.World) int { return w.Prices.Agent }, (*game.World).RecruitAgents)},
		{Key: 'O', Label: "Build Bombers", Do: stubbed("Build Bombers")},
		{Key: 'H', Label: "Build HeadQuarters", Do: stubbed("Build HeadQuarters")},
		{Key: 'B', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: '*', Label: "System Menu", Do: gotoMenu(system)},
		{Key: 'R', Label: "Return", Do: back},
	}

	bank.Items = []Item{
		{Key: 'D', Label: "Deposit", Do: money("Deposit", func(p *game.Empire) int { return p.Gold }, (*game.World).Deposit)},
		{Key: 'W', Label: "Withdraw", Do: money("Withdraw", func(p *game.Empire) int { return p.Bank }, (*game.World).Withdraw)},
		// Loan cap is a v1 balance knob: 100 gold of credit per region owned.
		{Key: 'L', Label: "Take Loan", Do: money("Borrow", func(p *game.Empire) int { return p.Land * 100 }, (*game.World).Loan)},
		{Key: 'P', Label: "Repay Loan", Do: money("Repay", func(p *game.Empire) int { return min(p.Gold, p.Debt) }, (*game.World).Repay)},
		{Key: 'I', Label: "Invest", Do: money("Invest", func(p *game.Empire) int { return p.Gold }, (*game.World).Deposit)},
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
		{Key: 'K', Label: "Gooie Kablooie Ops", Do: gooieKablooie},
		{Key: 'S', Label: "SDI Program", Do: sdiProgram},
		{Key: 'V', Label: "Travel Times", Do: stubbed("Travel Times")},
		{Key: 'X', Label: "Return", Do: back},
	}

	covert.Items = []Item{
		{Key: 'S', Label: "Send Spy", Do: sendSpy},
		{Key: 'P', Label: "Spy on Relations", Do: stubbed("Spy on Relations")},
		{Key: 'D', Label: "Spy Database", Do: stubbed("Spy Database")},
		{Key: 'B', Label: "Bribery", Do: stubbed("Bribery")},
		{Key: 'O', Label: "Special Operations", Do: specialOps},
		{Key: 'K', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: 'R', Label: "Return", Do: back},
	}

	trading.Items = []Item{
		{Key: 'F', Label: "Food Market", Do: gotoMenu(food)},
		{Key: 'S', Label: "Send Trade Deal", Do: sendTradeDeal},
		{Key: 'V', Label: "View IPScores", Do: interbbsScores},
		{Key: 'B', Label: "Buy / Sell", Do: gotoMenu(buy)},
		{Key: 'K', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: 'R', Label: "Return", Do: back},
	}

	diplomacy.Items = []Item{
		{Key: 'M', Label: "Modify Diplomacy", Do: modifyDiplomacy},
		{Key: 'V', Label: "View Diplomacy", Do: viewDiplomacy},
		{Key: 'L', Label: "Diplomacy List", Do: viewDiplomacy},
		{Key: 'R', Label: "Return", Do: back},
	}

	messages.Items = []Item{
		{Key: 'R', Label: "Read Messages", Do: readMessages},
		{Key: 'S', Label: "Send Message", Do: sendMessage},
		{Key: 'P', Label: "Planetary Post", Do: planetaryPost},
		{Key: 'X', Label: "Return", Do: back},
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
		{Key: 'C', LabelFn: onOff("Visit Covert Menu", func(w *game.World) *bool { return &w.VisitCovert }),
			Do: toggle(func(w *game.World) *bool { return &w.VisitCovert })},
		{Key: 'T', LabelFn: onOff("Visit Trading Menu", func(w *game.World) *bool { return &w.VisitTrading }),
			Do: toggle(func(w *game.World) *bool { return &w.VisitTrading })},
		{Key: 'G', LabelFn: onOff("Visit Message Menu", func(w *game.World) *bool { return &w.VisitMessage }),
			Do: toggle(func(w *game.World) *bool { return &w.VisitMessage })},
		{Key: 'R', Label: "Return", Do: back},
	}

	coord.Items = []Item{
		{Key: 'C', Label: "Configuration Editor", Do: stubbed("Configuration Editor")},
		{Key: 'M', Label: "Modify League Diplomacy", Do: stubbed("Modify League Diplomacy")},
		{Key: 'P', Label: "Player List", Do: stubbed("Player List")},
		{Key: 'R', Label: "Return", Do: back},
	}

	food.Items = []Item{
		{Label: fmt.Sprintf("The market buys food for %d and sells for %d.", game.FoodSellPrice, game.FoodBuyPrice)},
		{Key: 'B', Label: "Buy Food", Do: buyFoodMarket},
		{Key: 'S', Label: "Sell Food", Do: sellFoodMarket},
		{Key: 'K', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: 'R', Label: "Return", Do: back},
	}

	system.Items = []Item{
		{Key: 'E', Label: "Empire Status", Do: empireStatus},
		{Key: 'S', Label: "See Scores", Do: seeScores},
		{Key: 'V', Label: "Advisors", Do: stubbed("Advisors")},
		{Key: 'I', Label: "Set Industries", Do: stubbed("Set Industries")},
		{Key: 'Z', Label: "Specialize", Do: stubbed("Specialize")},
		{Key: 'U', Label: "Game Setup", Do: stubbed("Game Setup")},
		{Key: 'D', Label: "Diplomacy", Do: gotoMenu(diplomacy)},
		{Key: 'T', Label: "Trading", Do: gotoMenu(trading)},
		{Key: 'M', Label: "Messages", Do: gotoMenu(messages)},
		{Key: 'P', Label: "Preferences", Do: gotoMenu(prefs)},
		{Key: 'K', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: 'X', Label: "Set Tax Rate", Do: setTaxRate},
		{Key: 'F', Label: "Food Market", Do: gotoMenu(food)},
		{Key: 'Y', Label: "Sysop / Coordinator", Do: gotoMenu(coord),
			Hidden: func(w *game.World) bool { return !w.Coordinator }},
		{Key: 'R', Label: "Return", Do: back},
		{Key: 'Q', Label: "Quit", Do: quit},
	}

	gameMenu := &Menu{Title: "Immortal Barons — Game Menu", Status: statusBar}
	gameMenu.Items = []Item{
		{Key: '1', Label: "Play Game", Do: runTurn},
		{Key: '2', Label: "See Status", Do: empireStatus},
		{Key: '3', Label: "See Scores", Do: seeScores},
		{Key: '6', Label: "Read Messages", Do: readMessages},
		{Key: '7', Label: "Send Message", Do: sendMessage},
		{Key: '8', Label: "Game Bulletins", Do: showBulletin},
		{Key: 'P', Label: "Preferences", Do: gotoMenu(prefs)},
		{Key: 'Q', Label: "Quit", Do: quit},
	}

	return &Menus{
		Spending:  buy,
		Bank:      bank,
		Attack:    attack,
		Covert:    covert,
		Trading:   trading,
		Diplomacy: diplomacy,
		Messages:  messages,
		System:    system,
		Game:      gameMenu,
		Food:      food,
	}
}

func statusBar(w *game.World) string {
	p := w.Player()
	return fmt.Sprintf("%s | Gold %d  Food %d  Land %d  Army %d | Turns left %d | Day %d",
		p.Name, p.Gold, p.Food, p.Land, p.Army(), p.TurnsLeft, w.GameDay)
}
