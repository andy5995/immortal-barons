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
	Spending       *Menu // the "Spending Menu" (formerly "buy")
	Sell           *Menu
	Bank           *Menu
	Attack         *Menu
	InterPlanetary *Menu
	Covert         *Menu
	Trading        *Menu
	Diplomacy      *Menu
	Messages       *Menu
	System         *Menu
	Game           *Menu
	Food           *Menu
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
	buy := &Menu{Title: "Spending Menu", Color: ansi.FgBrightRed, ExitOnEnter: true, Status: spendingStatus}
	sell := &Menu{Title: "Sell Menu", Color: ansi.FgBrightGreen}
	bank := &Menu{Title: "Goldie Luck's Bank", Color: ansi.FgBrightCyan, Columns: 2}
	attack := &Menu{Title: "War / Attack", Color: ansi.FgBrightMagenta, ExitOnEnter: true}
	interplanetary := &Menu{Title: "InterPlanetary Operations", Color: ansi.FgBrightYellow, ExitOnEnter: true, Columns: 2}
	covert := &Menu{Title: "Covert Operations", Color: ansi.FgBrightMagenta, ExitOnEnter: true}
	bombTargets := &Menu{Title: "Bomb Enemy Targets", Color: ansi.FgBrightMagenta, ExitOnEnter: true}
	ipSpecial := &Menu{Title: "Special Operations", Color: ansi.FgBrightYellow, ExitOnEnter: true, Columns: 2}
	trading := &Menu{Title: "Trading", Color: ansi.FgBrightRed, ExitOnEnter: true}
	diplomacy := &Menu{Title: "Diplomacy", Color: ansi.FgBrightGreen}
	messages := &Menu{Title: "Messages", Color: ansi.FgBrightCyan}
	prefs := &Menu{Title: "Preferences", Color: ansi.FgBrightCyan}
	coord := &Menu{Title: "Coordinator Menu", Color: ansi.FgBrightBlue}
	system := &Menu{Title: "System Menu", Color: ansi.FgBrightBlue, Columns: 3}
	food := &Menu{Title: "Chopper's Fair Market", Color: ansi.FgBrightCyan}

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
	// third is the sell (buy-back) price shown on the Sell menu: BRE sells units
	// at buy/3 (agents are the exception — a flat SellAgentPrice).
	third := func(f func(*ctx) int) func(*ctx) int {
		return func(w *ctx) int { return f(w) / 3 }
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
		{Key: '1', Label: "Troopers", Price: third(priceTrooper), Owned: owned(troopers),
			Do: sellUnit2("Sell Troopers", troopers, (*game.World).SellTroopers)},
		{Key: '2', Label: "Jets", Price: third(priceJet), Owned: owned(jets),
			Do: sellUnit2("Sell Jets", jets, (*game.World).SellJets)},
		{Key: '3', Label: "Turrets", Price: third(priceTurret), Owned: owned(turrets),
			Do: sellUnit2("Sell Turrets", turrets, (*game.World).SellTurrets)},
		{Key: '4', Label: "Bombers", Price: third(priceBomber), Owned: owned(bombers),
			Do: sellUnit2("Sell Bombers", bombers, (*game.World).SellBombers)},
		{Key: '6', Label: "Regions", Price: func(w *ctx) int { return 0 }, Owned: owned(land), Do: sellLand},
		{Key: '7', Label: "Covert Agents", Price: func(w *ctx) int { return game.SellAgentPrice }, Owned: owned(agents),
			Do: sellUnit2("Sell Covert Agents", agents, (*game.World).SellAgents)},
		{Key: '8', Label: "Tanks", Price: third(priceTank), Owned: owned(tanks),
			Do: sellUnit2("Sell Tanks", tanks, (*game.World).SellTanks)},
		{Key: '9', Label: "Carriers", Price: third(priceCarrier), Owned: owned(carriers),
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
		lang := playerLang(w)
		return fmt.Sprintf("You have %s%s%s gold in hand and %s%s%s gold in the bank.",
			ansi.FgBrightCyan, formatGold(p.Gold, lang), ansi.FgBrightYellow, ansi.FgBrightCyan, formatGold(p.Bank, lang), ansi.FgBrightYellow)
	}

	attack.Items = []Item{
		{Key: 'R', Label: "Regular Attack", Do: regularAttack},
		{Key: 'N', Label: "Nuclear Attack", Do: nuclearAttack},
		{Key: 'C', Label: "Chemical Attack", Do: chemicalAttack},
		{Key: 'B', Label: "Biological Attack", Do: biologicalAttack},
		{Key: 'P', Label: "Attack Pirates", Do: attackPirates},
		{Key: 'A', Label: "Alliance Strength", Do: allianceStrength},
		{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: '0', Label: "Quit", Do: back},
	}
	attack.DefaultOnEnter = quitOnEnter(attack)

	// InterPlanetary Operations: BRE gathers the cross-planet actions on their
	// own menu, "only for InterBBS Games", reached by (9) on the opening menu
	// and shown as a turn step after the Spending menu (both gated on IBBS). The
	// gated access carries the whole node, so its items need no per-item Hidden.
	// BRE renders this menu in amber, not the magenta of the war menus; the (9)
	// opening-menu entry is tinted to match.
	// Order and
	// hotkeys match BRE.OVR's full InterPlanetary Operations string table
	// (#75); Doomer Kaboomer Ops ('K') is IB's equivalent of the Gooie
	// Kablooie item BRE's own table carries but the observed board had
	// config-hidden. Send Trade Deal and Send Message reuse the same actions
	// as the Trading and Messages menus; Special Operations opens the separate
	// interplanetary Special Operations menu (BRE's cross-planet covert set, not
	// the local Covert menu). Indiv. Attack Force ('6') has no interplanetary
	// individual-attack mechanic behind it yet — see indivAttackForce's doc.
	interplanetary.Items = []Item{
		{Key: '1', Label: "View IPScores", Do: interbbsScores},
		{Key: '2', Label: "Terrorist Ops", Do: terroristOps},
		{Key: '3', Label: "Send Trade Deal", Do: sendTradeDeal},
		{Key: '4', Label: "Create Group Attack", Do: createGroupAttack},
		{Key: '5', Label: "Join Group Attack", Do: joinGroupAttack},
		{Key: '6', Label: "Indiv. Attack Force", Do: indivAttackForce},
		{Key: '7', Label: "Send Message", Do: sendMessage},
		{Key: '8', Label: "Special Operations", Do: gotoMenu(ipSpecial)},
		{Key: 'A', Label: "SDI Program", Do: sdiProgram},
		{Key: 'K', Label: "Doomer Kaboomer Ops", Do: doomerKaboomer},
		{Key: 'D', Label: "Diplomacy List", Do: planetaryTreaties},
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
		{Key: 'S', Label: "R5-Slappenheimer", Do: slappenheimerStrike},
		{Key: '0', Label: "Quit", Do: back},
	}
	bombTargets.DefaultOnEnter = quitOnEnter(bombTargets)

	// Interplanetary Special Operations: BRE's cross-planet covert menu — every
	// op targets an empire on another planet. Disassembly (BRE.OVR string table
	// @170012) shows the local Bomb Enemy Targets set and this IP menu draw from
	// ONE 8-item table; the local menu shows the first 7, the IP menu shows all 8
	// (it adds Send SpyGuy). Labels/order are binary-verified; the Send SpyGuy
	// hotkey ('G' here) wasn't recoverable from the overlay dispatch. Only Send
	// SpyGuy is wired; the bombing/WMD variants are recorded-but-inert until
	// interplanetary covert strikes are built. ('?'/'0' are IB's menu convention;
	// BRE exits via ESC/Q with no listed items.)
	ipSpecial.Items = []Item{
		{Key: 'F', Label: "Bomb Food Market", Do: ipSpecialStub},
		{Key: 'T', Label: "Bomb Trading Market", Do: ipSpecialStub},
		{Key: 'R', Label: "Bomb Trade Routes", Do: ipSpecialStub},
		{Key: 'U', Label: "Undermine Investments", Do: ipSpecialStub},
		{Key: 'N', Label: "Nuclear Assault", Do: ipSpecialStub},
		{Key: 'C', Label: "Chemical Bombing", Do: ipSpecialStub},
		{Key: 'S', Label: "R5-Slappenheimer", Do: ipSpecialStub},
		{Key: 'G', Label: "Send SpyGuy", Do: sendSpyGuy},
		{Key: '?', Label: "Help", Do: helpBrowse},
		{Key: '0', Label: "Quit", Do: back},
	}
	ipSpecial.DefaultOnEnter = quitOnEnter(ipSpecial)

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

	// Item set and order mirror BRE's Preferences submenu (the three "Visit …
	// Menu" skip-toggles, then the four behaviour toggles). The 'L' language
	// picker is IB's own addition, kept at the end.
	prefs.Items = []Item{
		{Key: '1', LabelFn: onOff("Visit Covert Menu", func(w *ctx) *bool { return &w.VisitCovert }),
			Do: toggle(func(w *ctx) *bool { return &w.VisitCovert })},
		{Key: '2', LabelFn: onOff("Visit Trading Menu", func(w *ctx) *bool { return &w.VisitTrading }),
			Do: toggle(func(w *ctx) *bool { return &w.VisitTrading })},
		{Key: '3', LabelFn: onOff("Visit Message Menu", func(w *ctx) *bool { return &w.VisitMessage }),
			Do: toggle(func(w *ctx) *bool { return &w.VisitMessage })},
		{Key: '4', LabelFn: onOff("Use Enter To Exit BUY Menu", func(w *ctx) *bool { return &w.EnterExitsBuy }),
			Do: toggle(func(w *ctx) *bool { return &w.EnterExitsBuy })},
		{Key: '5', LabelFn: onOff("Deposit gold at End of Turn", func(w *ctx) *bool { return &w.DepositEndTurn }),
			Do: toggle(func(w *ctx) *bool { return &w.DepositEndTurn })},
		{Key: '6', LabelFn: onOff("Auto-Pay Maintenance", func(w *ctx) *bool { return &w.AutoPayMaint }),
			Do: toggle(func(w *ctx) *bool { return &w.AutoPayMaint })},
		{Key: '7', LabelFn: onOff("Auto-Feed Empire", func(w *ctx) *bool { return &w.AutoFeed }),
			Do: toggle(func(w *ctx) *bool { return &w.AutoFeed })},
		{Key: 'L', LabelFn: func(w *ctx) string {
			// playerLang, not the raw stored code: on a CP437 session it names the
			// language actually rendering (the CP437-safe fallback), and its
			// endonym is displayable — a stored "Русский" would only mojibake.
			return i18n.T(playerLang(w), "Language") + ": " + languageName(playerLang(w))
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
		{LabelFn: func(w *ctx) string {
			lang := playerLang(w)
			avail := i18n.T(lang, "Unlimited supply today.")
			if !w.Config.FoodUnlimited {
				avail = fmt.Sprintf(i18n.T(lang, "%s units of food available today."), comma(w.FoodMarketSupply))
			}
			return fmt.Sprintf(i18n.T(lang, "%s You buy at %s, sell at %s per unit."), avail, comma(w.FoodBuyPrice()), comma(w.FoodSellPrice()))
		}},
		{Key: 'B', Label: "Buy Food", Do: buyFoodMarket},
		{Key: 'S', Label: "Sell Food", Do: sellFoodMarket},
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
		{Key: '3', Label: "Specialize Industry", Do: specializeIndustry,
			// Specializing is permanent, so hide the option once it's been done.
			Hidden: func(w *ctx) bool { return w.Player().Specialized != "" }},
		{Key: '?', Label: "Help", Do: helpBrowse},
		{Key: 'O', Label: "Vote for Coordinator", Do: voteCoordinator, Hidden: ibbsHidden},
		{Key: 'Y', Label: "Coordinator Menu", Do: gotoMenu(coord),
			Hidden: func(w *ctx) bool { return ibbsHidden(w) || w.BBSCoordinator() != w.Player() }},
		{Key: 'C', Label: "Configuration Editor", Do: configEditor,
			Hidden: func(w *ctx) bool { return !w.Coordinator }},
		{Key: '0', Label: "Quit", Do: back},
	}
	system.DefaultOnEnter = quitOnEnter(system)

	gameMenu := &Menu{Title: "Immortal Barons — Game Menu", Color: ansi.FgBrightMagenta, Columns: 2}
	gameMenu.Items = []Item{
		{Key: '1', Label: "Play", Do: runTurn},
		{Key: '2', Label: "See Status", Do: empireStatus},
		{Key: '3', Label: "See Scores", Do: seeScores},
		{Key: '4', Label: "Today's News", Do: showBulletinToday},
		{Key: '5', Label: "Yesterday's News", Do: showBulletinYesterday},
		{Key: '6', Label: "Messages", Do: gotoMenu(messages)},
		{Key: '8', Label: "Game Bulletins", Do: showBulletinToday},
		{Key: '9', Label: "InterPlanetary Ops", Do: gotoMenu(interplanetary), Hidden: ibbsHidden, Color: interplanetary.Color},
		{Key: 'A', Label: "Instructions", Do: helpBrowse},
		{Key: '?', Label: "Help", Do: helpBrowse},
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
		Spending:       buy,
		Sell:           sell,
		Bank:           bank,
		Attack:         attack,
		InterPlanetary: interplanetary,
		Covert:         covert,
		Trading:        trading,
		Diplomacy:      diplomacy,
		Messages:       messages,
		System:         system,
		Game:           gameMenu,
		Food:           food,
	}
}

// ibbsHidden hides interplanetary/inter-BBS menu items unless the game is
// configured for IBBS or league play.
func ibbsHidden(w *ctx) bool { return !w.Config.InterBBSEnabled() }

// spendingStatus is the Spending menu footer: gold on hand and turns left.
// It runs inside draw's world-lock section, so the reads need no separate
// locking.
func spendingStatus(w *ctx) string {
	p := w.Player()
	return fmt.Sprintf(i18n.T(playerLang(w), "You have %s gold and %d turns."), formatGold(p.Gold, playerLang(w)), p.TurnsLeft)
}
