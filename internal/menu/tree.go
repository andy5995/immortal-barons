package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/i18n"
)

// Menus holds every top-level and sub menu built by BuildMenus, so the
// outer game flow (gameflow.go) can drive them as turn-pipeline stages
// without re-parsing the tree.
type Menus struct {
	Spending  *Menu // the "Spending Menu" (formerly "buy")
	Sell      *Menu
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

// quitOnEnter makes Enter activate a menu's own '0' Quit item, so the prompt
// shows and selects "Quit" uniformly across submenus (#62). The Spending
// menu is the one exception: its Enter-to-exit stays gated behind the
// EnterExitsBuy preference (see buy.ExitOnEnter below), so it does not use
// this hook.
func quitOnEnter(m *Menu) func(*ctx) *Item {
	return func(g *ctx) *Item { return m.byKey('0', g) }
}

// BuildMenus constructs the full BRE menu tree. Menus are created first,
// then wired, so submenus can reference each other (e.g. several menus
// offer "Visit Bank").
func BuildMenus() *Menus {
	buy := &Menu{Title: "Spending Menu", Color: ansi.FgBrightRed, ExitOnEnter: true, Status: spendingStatus,
		// NoClear: after a unit purchase the confirmation stays visible above
		// the redrawn menu (BRE-style), instead of being wiped by a clear.
		NoClear: true}
	sell := &Menu{Title: "Sell Menu", Color: ansi.FgBrightGreen}
	bank := &Menu{Title: "Goldie Luck's Bank", Color: ansi.FgBrightCyan}
	attack := &Menu{Title: "War / Attack", Color: ansi.FgBrightMagenta, ExitOnEnter: true}
	interplanetary := &Menu{Title: "InterPlanetary Operations", Color: ansi.FgBrightMagenta, ExitOnEnter: true}
	covert := &Menu{Title: "Covert Operations", Color: ansi.FgBrightMagenta, ExitOnEnter: true}
	bombTargets := &Menu{Title: "Bomb Enemy Targets", Color: ansi.FgBrightMagenta, ExitOnEnter: true}
	trading := &Menu{Title: "Trading", Color: ansi.FgBrightRed, ExitOnEnter: true}
	diplomacy := &Menu{Title: "Diplomacy", Color: ansi.FgBrightGreen}
	messages := &Menu{Title: "Messages", Color: ansi.FgBrightCyan}
	prefs := &Menu{Title: "Preferences", Color: ansi.FgBrightCyan}
	coord := &Menu{Title: "Coordinator Menu", Color: ansi.FgBrightBlue}
	system := &Menu{Title: "System Menu", Color: ansi.FgBrightBlue}
	food := &Menu{Title: "Food Market", Color: ansi.FgBrightCyan}

	// owned adapts a per-empire count into a menu column function.
	owned := func(f func(*game.Empire) int) func(*ctx) int {
		return func(w *ctx) int { return f(w.Player()) }
	}
	troopers := func(p *game.Empire) int { return p.Troopers }
	jets := func(p *game.Empire) int { return p.Jets }
	turrets := func(p *game.Empire) int { return p.Turrets }
	bombers := func(p *game.Empire) int { return p.Bombers }
	agents := func(p *game.Empire) int { return p.Agents }
	tanks := func(p *game.Empire) int { return p.Tanks }
	carriers := func(p *game.Empire) int { return p.Carriers }
	land := func(p *game.Empire) int { return p.Land }
	priceTrooper := func(w *ctx) int { return w.Prices.Trooper }
	priceJet := func(w *ctx) int { return w.Prices.Jet }
	priceTurret := func(w *ctx) int { return w.Prices.Turret }
	priceBomber := func(w *ctx) int { return w.Prices.Bomber }
	priceAgent := func(w *ctx) int { return w.Prices.Agent }
	priceTank := func(w *ctx) int { return w.Prices.Tank }
	priceCarrier := func(w *ctx) int { return w.Prices.Carrier }
	// half is the sell (buy-back) price shown on the Sell menu.
	half := func(f func(*ctx) int) func(*ctx) int {
		return func(w *ctx) int { return f(w) / 2 }
	}

	buy.Items = []Item{
		{Key: '*', Label: "System Menu", Do: gotoMenu(system)},
		{Key: '1', Label: "Troopers", Price: priceTrooper, Owned: owned(troopers),
			Do: buy2("Troopers", true, priceTrooper, (*game.World).Recruit)},
		{Key: '2', Label: "Jets", Price: priceJet, Owned: owned(jets),
			Do: buy2("Jets", true, priceJet, (*game.World).BuildJets)},
		{Key: '3', Label: "Turrets", Price: priceTurret, Owned: owned(turrets),
			Do: buy2("Turrets", true, priceTurret, (*game.World).BuildTurrets)},
		{Key: '4', Label: "Bombers", Price: priceBomber, Owned: owned(bombers),
			Do: buy2("Bombers", true, priceBomber, (*game.World).BuildBombers)},
		{Key: '5', Label: "HeadQuarters", Price: func(w *ctx) int { return game.HQCost }, Owned: owned(func(p *game.Empire) int { return p.HQ }), Do: buildHQ},
		{Key: '6', Label: "Regions", Price: func(w *ctx) int { return w.LandPrice(w.Player()) }, Owned: owned(land), Do: buyLand},
		{Key: '7', Label: "Covert Agents", Price: priceAgent, Owned: owned(agents),
			Do: buy2("Covert Agents", false, priceAgent, (*game.World).RecruitAgents)},
		{Key: '8', Label: "Tanks", Price: priceTank, Owned: owned(tanks),
			Do: buy2("Tanks", true, priceTank, (*game.World).BuildTanks)},
		{Key: '9', Label: "Carriers", Price: priceCarrier, Owned: owned(carriers),
			Do: buy2("Carriers", true, priceCarrier, (*game.World).BuildCarriers)},
		{Key: 'S', Label: "Sell", Do: gotoMenu(sell)},
		{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: '?', Label: "Help", Do: helpBrowse},
		{Key: '0', Label: "Quit", Do: back},
	}

	sell.Items = []Item{
		{Key: 'B', Label: "Buy", Do: back},
		{Key: '1', Label: "Troopers", Price: half(priceTrooper), Owned: owned(troopers),
			Do: sellUnit2("Sell Troopers", troopers, (*game.World).SellTroopers)},
		{Key: '2', Label: "Jets", Price: half(priceJet), Owned: owned(jets),
			Do: sellUnit2("Sell Jets", jets, (*game.World).SellJets)},
		{Key: '3', Label: "Turrets", Price: half(priceTurret), Owned: owned(turrets),
			Do: sellUnit2("Sell Turrets", turrets, (*game.World).SellTurrets)},
		{Key: '4', Label: "Bombers", Price: half(priceBomber), Owned: owned(bombers),
			Do: sellUnit2("Sell Bombers", bombers, (*game.World).SellBombers)},
		{Key: '6', Label: "Regions", Price: func(w *ctx) int { return 0 }, Owned: owned(land), Do: sellLand},
		{Key: '7', Label: "Covert Agents", Price: half(priceAgent), Owned: owned(agents),
			Do: sellUnit2("Sell Covert Agents", agents, (*game.World).SellAgents)},
		{Key: '8', Label: "Tanks", Price: half(priceTank), Owned: owned(tanks),
			Do: sellUnit2("Sell Tanks", tanks, (*game.World).SellTanks)},
		{Key: '9', Label: "Carriers", Price: half(priceCarrier), Owned: owned(carriers),
			Do: sellUnit2("Sell Carriers", carriers, (*game.World).SellCarriers)},
		{Key: '0', Label: "Quit", Do: back},
	}
	sell.DefaultOnEnter = quitOnEnter(sell)

	bank.Items = []Item{
		{Key: 'D', Label: "Deposit Funds", Do: money("Deposit", func(p *game.Empire) int { return p.Gold }, (*game.World).Deposit)},
		{Key: 'W', Label: "Withdraw Funds", Do: money("Withdraw", func(p *game.Empire) int { return p.Bank }, (*game.World).Withdraw)},
		// Loan cap is a v1 balance knob: 100 gold of credit per region owned,
		// measured against current debt so repeat visits can't exceed it.
		{Key: 'B', Label: "Take Loan", Do: money("Borrow", func(p *game.Empire) int { return max(0, p.Land*100-p.Debt) }, (*game.World).Loan)},
		{Key: 'R', Label: "Repay Loan", Do: money("Repay", func(p *game.Empire) int { return min(p.Gold, p.Debt) }, (*game.World).Repay)},
		{Key: 'I', Label: "Invest", Do: investFunds},
		{Key: 'L', Label: "List Investments", Do: listInvestments},
		{Key: 'V', Label: "View Bank Rates", Do: bankRates},
		{Key: '0', Label: "Quit", Do: back},
	}
	bank.DefaultOnEnter = quitOnEnter(bank)
	bank.Status = func(w *ctx) string {
		p := w.Player()
		return fmt.Sprintf("You have %s%s%s gold in hand and %s%s%s gold in the bank.",
			ansi.FgBrightCyan, comma(p.Gold), ansi.FgBrightYellow, ansi.FgBrightCyan, comma(p.Bank), ansi.FgBrightYellow)
	}

	attack.Items = []Item{
		{Key: 'R', Label: "Regular Attack", Do: regularAttack},
		{Key: 'N', Label: "Nuclear Attack", Do: nuclearAttack},
		{Key: 'C', Label: "Chemical Attack", Do: chemicalAttack},
		{Key: 'B', Label: "Biological Attack", Do: biologicalAttack},
		{Key: 'P', Label: "Attack Pirates", Do: attackPirates},
		{Key: 'A', Label: "Alliance Strength", Do: allianceStrength},
		{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: 'O', Label: "InterPlanetary Ops", Do: gotoMenu(interplanetary), Hidden: ibbsHidden},
		{Key: '0', Label: "Quit", Do: back},
	}
	attack.DefaultOnEnter = quitOnEnter(attack)

	// InterPlanetary Operations: BRE gathers the cross-planet actions on their
	// own menu, "only for InterBBS Games". The whole node hangs off a gated
	// entry on the War menu, so the items need no per-item Hidden. Keys mirror
	// BRE's InterPlanetary Operations menu; items whose cross-planet variants
	// don't exist yet here (Send Trade Deal, Indiv. Attack Force, Send Message,
	// Special Operations) are omitted until they are made interplanetary-aware.
	interplanetary.Items = []Item{
		{Key: '1', Label: "View IPScores", Do: interbbsScores},
		{Key: '2', Label: "Terrorist Ops", Do: terroristOps},
		{Key: '4', Label: "Create Group Attack", Do: createGroupAttack},
		{Key: '5', Label: "Join Group Attack", Do: joinGroupAttack},
		{Key: 'A', Label: "SDI Program", Do: sdiProgram},
		{Key: 'K', Label: "Doomer Kaboomer Ops", Do: doomerKaboomer},
		{Key: 'S', Label: "Spy Database", Do: spyDatabase},
		{Key: 'T', Label: "Travel Times", Do: travelTimes},
		{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: '?', Label: "Help", Do: helpBrowse},
		{Key: '0', Label: "Quit", Do: back},
	}
	interplanetary.DefaultOnEnter = quitOnEnter(interplanetary)

	// Order and hotkeys match BRE.OVR's string table for the Covert
	// Operations menu and its Bomb Enemy Targets submenu (#73).
	covert.Items = []Item{
		{Key: 'S', Label: "Send Spy", Do: sendSpy},
		{Key: 'R', Label: "Stir Revolts", Do: stirRevolts},
		{Key: 'U', Label: "Set Up", Do: setUp},
		{Key: 'D', Label: "Support Dissensions", Do: supportDissensions},
		{Key: 'M', Label: "Demoralize Forces", Do: demoralizeForces},
		{Key: 'P', Label: "Spy on Relations", Do: spyRelations},
		{Key: 'B', Label: "Bomb Enemy Targets", Do: gotoMenu(bombTargets)},
		{Key: 'Y', Label: "Bribery", Do: briberyOp},
		{Key: 'X', Label: "Expose Enemy Ops", Do: exposeEnemyOps},
		{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: '0', Label: "Quit", Do: back},
	}
	covert.DefaultOnEnter = quitOnEnter(covert)

	bombTargets.Items = []Item{
		{Key: 'F', Label: "Bomb Food Market", Do: bombFoodMarket},
		{Key: 'T', Label: "Bomb Trading Market", Do: bombTradingMarket},
		{Key: 'R', Label: "Bomb Trade Routes", Do: bombTradeRoutes},
		{Key: 'U', Label: "Undermine Investments", Do: undermineInvestments},
		{Key: 'N', Label: "Nuclear Assault", Do: nuclearAssault},
		{Key: 'C', Label: "Chemical Bombing", Do: chemicalBombing},
		{Key: 'S', Label: "S3-Sabre", Do: sabreStrike},
		{Key: '0', Label: "Quit", Do: back},
	}
	bombTargets.DefaultOnEnter = quitOnEnter(bombTargets)

	trading.Items = []Item{
		{Key: 'F', Label: "Food Market", Do: gotoMenu(food)},
		{Key: '1', Label: "Send Trade Deal", Do: sendTradeDeal},
		{Key: 'B', Label: "Buy / Sell", Do: gotoMenu(buy)},
		{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: '0', Label: "Quit", Do: back},
	}
	trading.DefaultOnEnter = quitOnEnter(trading)

	// Treaty types are direct menu items, matching BRE's Diplomacy menu
	// layout (#68) instead of hiding behind a single "Modify Diplomacy"
	// item. Order and hotkeys match BRE.OVR's string table.
	diplomacy.Items = []Item{
		{Key: '1', Label: "Tariff Trade Agreement", Do: negotiateTreaty("Tariff Trade Agreement")},
		{Key: '2', Label: "Protective Trade", Do: negotiateTreaty("Protective Trade")},
		{Key: '3', Label: "Free Trade Agreement", Do: negotiateTreaty("Free Trade Agreement")},
		{Key: '4', Label: "Terrorist Prevention", Do: negotiateTreaty("Terrorist Prevention")},
		{Key: '5', Label: "Intelligence Alliance", Do: negotiateTreaty("Intelligence Alliance")},
		{Key: '6', Label: "Technology Agreement", Do: negotiateTreaty("Technology Agreement")},
		{Key: '7', Label: "Full Defense Alliance", Do: negotiateTreaty("Full Defense Alliance")},
		{Key: '8', Label: "Declaration Of War", Do: declareWar},
		{Key: '9', Label: "View Treaties", Do: viewDiplomacy},
		{Key: '?', Label: "Help", Do: helpBrowse},
		{Key: '0', Label: "Quit", Do: back},
	}
	diplomacy.DefaultOnEnter = quitOnEnter(diplomacy)

	messages.Items = []Item{
		{Key: 'R', Label: "Read Messages", Do: readMessages},
		{Key: 'S', Label: "Send Message", Do: sendMessage},
		{Key: 'P', Label: "Planetary Post", Do: planetaryPost},
		{Key: '0', Label: "Quit", Do: back},
	}
	messages.DefaultOnEnter = quitOnEnter(messages)

	prefs.Items = []Item{
		{Key: 'E', LabelFn: onOff("Enter exits Buy menu", func(w *ctx) *bool { return &w.EnterExitsBuy }),
			Do: toggle(func(w *ctx) *bool { return &w.EnterExitsBuy })},
		{Key: 'D', LabelFn: onOff("Deposit gold at end of turn", func(w *ctx) *bool { return &w.DepositEndTurn }),
			Do: toggle(func(w *ctx) *bool { return &w.DepositEndTurn })},
		{Key: 'M', LabelFn: onOff("Auto-pay maintenance", func(w *ctx) *bool { return &w.AutoPayMaint }),
			Do: toggle(func(w *ctx) *bool { return &w.AutoPayMaint })},
		{Key: 'F', LabelFn: onOff("Auto-feed people & army", func(w *ctx) *bool { return &w.AutoFeed }),
			Do: toggle(func(w *ctx) *bool { return &w.AutoFeed })},
		{Key: 'C', LabelFn: onOff("Visit Covert Menu", func(w *ctx) *bool { return &w.VisitCovert }),
			Do: toggle(func(w *ctx) *bool { return &w.VisitCovert })},
		{Key: 'T', LabelFn: onOff("Visit Trading Menu", func(w *ctx) *bool { return &w.VisitTrading }),
			Do: toggle(func(w *ctx) *bool { return &w.VisitTrading })},
		{Key: 'G', LabelFn: onOff("Visit Message Menu", func(w *ctx) *bool { return &w.VisitMessage }),
			Do: toggle(func(w *ctx) *bool { return &w.VisitMessage })},
		{Key: 'L', LabelFn: func(w *ctx) string {
			return i18n.T(playerLang(w), "Language") + ": " + languageName(w.Player().Language)
		}, Do: pickLanguage},
		{Key: '0', Label: "Quit", Do: back},
	}
	prefs.DefaultOnEnter = quitOnEnter(prefs)

	// The Coordinator Menu belongs to the elected BBS Coordinator (see the
	// System menu gate below); it holds the planet-coordination functions.
	coord.Items = []Item{
		{Key: 'M', Label: "Modify League Diplomacy", Do: modifyLeagueDiplomacy},
		{Key: 'P', Label: "Player List", Do: playerList},
		{Key: '0', Label: "Quit", Do: back},
	}
	coord.DefaultOnEnter = quitOnEnter(coord)

	food.Items = []Item{
		{Label: fmt.Sprintf("The market buys food for %d and sells for %d.", game.FoodSellPrice, game.FoodBuyPrice)},
		{Key: 'B', Label: "Buy Food", Do: buyFoodMarket},
		{Key: 'S', Label: "Food", Do: sellFoodMarket},
		{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: '0', Label: "Quit", Do: back},
	}
	food.DefaultOnEnter = quitOnEnter(food)

	system.Items = []Item{
		{Key: '#', Label: "Abdicate", Do: abdicate},
		{Key: 'A', Label: "Visit Advisors", Do: visitAdvisors},
		{Key: 'D', Label: "Diplomacy", Do: gotoMenu(diplomacy)},
		{Key: 'E', Label: "Empire Status", Do: empireStatus},
		{Key: 'F', Label: "Food Market", Do: gotoMenu(food)},
		{Key: 'G', Label: "Game Setup", Do: gameSetup},
		{Key: 'I', Label: "About", Do: about},
		{Key: 'M', Label: "Messages", Do: gotoMenu(messages)},
		{Key: 'P', Label: "Preferences", Do: gotoMenu(prefs)},
		{Key: 'R', Label: "Set Tax Rate", Do: setTaxRate},
		{Key: 'S', Label: "See Scores", Do: seeScores},
		{Key: 'T', Label: "Trading", Do: gotoMenu(trading)},
		{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: 'W', Label: "Write Macros", Do: writeMacros},
		{Key: '1', Label: "Set Industries", Do: setIndustries},
		{Key: '2', Label: "Show Instructions", Do: helpBrowse},
		{Key: '3', Label: "Specialize Industry", Do: specializeIndustry},
		{Key: 'O', Label: "Vote for Coordinator", Do: voteCoordinator, Hidden: ibbsHidden},
		{Key: 'Y', Label: "Coordinator Menu", Do: gotoMenu(coord),
			Hidden: func(w *ctx) bool { return ibbsHidden(w) || w.BBSCoordinator() != w.Player() }},
		{Key: 'C', Label: "Configuration Editor", Do: configEditor,
			Hidden: func(w *ctx) bool { return !w.Coordinator }},
		{Key: '0', Label: "Quit", Do: back},
	}
	system.DefaultOnEnter = quitOnEnter(system)

	gameMenu := &Menu{Title: "Immortal Barons — Game Menu", Color: ansi.FgBrightMagenta, Status: statusBar}
	gameMenu.Items = []Item{
		{Key: '1', Label: "Play", Do: runTurn},
		{Key: '2', Label: "See Status", Do: empireStatus},
		{Key: '3', Label: "See Scores", Do: seeScores},
		{Key: '4', Label: "Today's News", Do: showBulletinToday},
		{Key: '5', Label: "Yesterday's News", Do: showBulletinYesterday},
		{Key: '6', Label: "Read Messages", Do: readMessages},
		{Key: '7', Label: "Send Message", Do: sendMessage},
		{Key: '8', Label: "Game Bulletins", Do: showBulletinToday},
		{Key: 'A', Label: "Instructions", Do: helpBrowse},
		{Key: 'B', Label: "Help Database", Do: helpBrowse},
		{Key: 'I', Label: "About", Do: about},
		{Key: 'P', Label: "Preferences", Do: gotoMenu(prefs)},
		{Key: '0', Label: "Quit", Do: quit},
	}
	// Enter picks "Play" while turns remain, else "Quit" — mutually
	// exclusive defaults on the opening menu (#56/#57).
	gameMenu.DefaultOnEnter = func(w *ctx) *Item {
		key := byte('0')
		if p := w.Player(); p != nil && p.TurnsLeft > 0 {
			key = '1'
		}
		return gameMenu.byKey(rune(key), w)
	}

	return &Menus{
		Spending:  buy,
		Sell:      sell,
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

// ibbsHidden hides interplanetary/inter-BBS menu items unless the game is
// configured for IBBS or league play.
func ibbsHidden(w *ctx) bool { return !w.Config.InterBBSEnabled() }

func statusBar(w *ctx) string {
	p := w.Player()
	return fmt.Sprintf(i18n.T(playerLang(w), "%s | Gold %d  Food %d  Land %d  Army %d | Turns left %d | Day %d"),
		p.Name, p.Gold, p.Food, p.Land, p.Army(), p.TurnsLeft, w.GameDay)
}

// spendingStatus is the Spending menu footer: gold on hand and turns left.
// It runs inside draw's world-lock section (like statusBar), so the reads
// need no separate locking.
func spendingStatus(w *ctx) string {
	p := w.Player()
	return fmt.Sprintf(i18n.T(playerLang(w), "You have %s gold and %d turns."), comma(p.Gold), p.TurnsLeft)
}
