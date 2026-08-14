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
	buy := &Menu{Title: "Spending", Color: ansi.FgBrightRed, ExitOnEnter: true, Status: spendingStatus, Width: 44}
	sell := &Menu{Title: "Sell", Color: ansi.FgBrightGreen}
	bank := &Menu{Title: "Goldie Luck's Bank", Color: ansi.FgBrightCyan, Columns: 2}
	// Width: BRE's captured Attack box is 23 columns and IB's content measures the
	// same, so the box fits it exactly (docs/dev/bre-screens.md).
	attack := &Menu{Title: "Attack", Color: ansi.FgBrightMagenta, ExitOnEnter: true, Width: 23}
	interplanetary := &Menu{Title: "InterPlanetary Operations", Color: ansi.FgBrightYellow, ExitOnEnter: true, Columns: 2}
	covert := &Menu{Title: "Covert Operations", Color: ansi.FgBrightGreen, ExitOnEnter: true, Status: covertStatus}
	bombTargets := &Menu{Title: "Bomb Enemy Targets", Color: ansi.FgBrightGreen, ExitOnEnter: true}
	ipSpecial := &Menu{Title: "Special Operations", Color: ansi.FgBrightYellow, ExitOnEnter: true, Columns: 2}
	// BRE draws IP Messages as a narrow single-column box (25 columns, from a
	// live capture) rather than at the full menu width.
	ipMessages := &Menu{Title: "IP Messages", Color: ansi.FgBrightCyan, ExitOnEnter: true, Width: 25}
	trading := &Menu{Title: "Trading", Color: ansi.FgBrightRed, ExitOnEnter: true}
	diplomacy := &Menu{Title: "Diplomacy", Color: ansi.FgBrightGreen}
	messages := &Menu{Title: "Messages", Color: ansi.FgBrightCyan}
	prefs := &Menu{Title: "Preferences", Color: ansi.FgBrightCyan}
	coord := &Menu{Title: "Coordinator", Color: ansi.FgBrightBlue}
	// Two columns, not BRE's three: several labels ("Configuration Editor",
	// "Specialize Industry") are already wide in English and grow when translated,
	// so a 3-column row overflows 80 cols. The wider 2-column cell fits them.
	// Width: sized to IB's own content, which is the rule BRE follows. Its own
	// captures are 69 and 75 because it lays this menu out in THREE columns and
	// its width tracks whichever items are showing; IB uses two, so it measures
	// 59 at its widest (57 once Specialize Industry is spent).
	system := &Menu{Title: "System", Color: ansi.FgBrightBlue, Columns: 2, Width: 59}
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
	// Prices fluctuate per empire per turn (#30); the display closures call the
	// same accessors the buy/sell paths charge through, so shown == charged.
	priceTrooper := func(w *ctx) int { return w.TrooperPrice(w.Player()) }
	priceJet := func(w *ctx) int { return w.JetPrice(w.Player()) }
	priceTurret := func(w *ctx) int { return w.TurretPrice(w.Player()) }
	priceBomber := func(w *ctx) int { return w.BomberPrice(w.Player()) }
	priceAgent := func(w *ctx) int { return w.AgentPrice(w.Player()) }
	priceTank := func(w *ctx) int { return w.TankPrice(w.Player()) }
	priceCarrier := func(w *ctx) int { return w.CarrierPrice(w.Player()) }
	// third is the sell (buy-back) price shown on the Sell menu: BRE sells units
	// at buy/3 (agents are the exception — a flat SellAgentPrice).
	third := func(f func(*ctx) int) func(*ctx) int {
		return func(w *ctx) int { return f(w) / 3 }
	}

	buy.Items = []Item{
		{Key: '*', Label: "System Menu", Do: gotoMenu(system)},
		{Key: '1', Label: "Troopers", Price: priceTrooper, Owned: owned(troopers),
			Do: buyUnit("Troopers", true, priceTrooper, (*game.World).Recruit)},
		{Key: '2', Label: "Jets", Price: priceJet, Owned: owned(jets),
			Do: buyUnit("Jets", true, priceJet, (*game.World).BuildJets)},
		{Key: '3', Label: "Turrets", Price: priceTurret, Owned: owned(turrets),
			Do: buyUnit("Turrets", true, priceTurret, (*game.World).BuildTurrets)},
		{Key: '4', Label: "Bombers", Price: priceBomber, Owned: owned(bombers),
			Do: buyUnit("Bombers", true, priceBomber, (*game.World).BuildBombers)},
		{Key: '5', Label: "HeadQuarters", Price: func(w *ctx) int { return w.HQPrice(w.Player()) }, Owned: owned(func(p *game.Empire) int { return p.HQ }), Do: buildHQ},
		{Key: '6', Label: "Regions", Price: func(w *ctx) int { return w.LandPrice(w.Player()) }, Owned: owned(land), Do: buyLand},
		{Key: '7', Label: "Covert Agents", Price: priceAgent, Owned: owned(agents),
			Do: buyUnit("Covert Agents", false, priceAgent, (*game.World).RecruitAgents)},
		{Key: '8', Label: "Tanks", Price: priceTank, Owned: owned(tanks),
			Do: buyUnit("Tanks", true, priceTank, (*game.World).BuildTanks)},
		{Key: '9', Label: "Carriers", Price: priceCarrier, Owned: owned(carriers),
			Do: buyUnit("Carriers", true, priceCarrier, (*game.World).BuildCarriers)},
		{Key: 'S', Label: "Sell", Do: gotoMenu(sell)},
		{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: '?', Label: "Help", Do: helpBrowse},
		{Key: '0', Label: "Quit", Do: back},
	}

	sell.Items = []Item{
		{Key: 'B', Label: "Buy", Do: back},
		{Key: '1', Label: "Troopers", Price: third(priceTrooper), Owned: owned(troopers),
			Do: sellUnit("Sell Troopers", troopers, (*game.World).SellTroopers)},
		{Key: '2', Label: "Jets", Price: third(priceJet), Owned: owned(jets),
			Do: sellUnit("Sell Jets", jets, (*game.World).SellJets)},
		{Key: '3', Label: "Turrets", Price: third(priceTurret), Owned: owned(turrets),
			Do: sellUnit("Sell Turrets", turrets, (*game.World).SellTurrets)},
		{Key: '4', Label: "Bombers", Price: third(priceBomber), Owned: owned(bombers),
			Do: sellUnit("Sell Bombers", bombers, (*game.World).SellBombers)},
		{Key: '6', Label: "Regions", Price: func(w *ctx) int { return 0 }, Owned: owned(land), Do: sellLand},
		{Key: '7', Label: "Covert Agents", Price: func(w *ctx) int { return game.SellAgentPrice }, Owned: owned(agents),
			Do: sellUnit("Sell Covert Agents", agents, (*game.World).SellAgents)},
		{Key: '8', Label: "Tanks", Price: third(priceTank), Owned: owned(tanks),
			Do: sellUnit("Sell Tanks", tanks, (*game.World).SellTanks)},
		{Key: '9', Label: "Carriers", Price: third(priceCarrier), Owned: owned(carriers),
			Do: sellUnit("Sell Carriers", carriers, (*game.World).SellCarriers)},
		{Key: '0', Label: "Quit", Do: back},
	}
	sell.DefaultOnEnter = quitOnEnter(sell)

	bank.Items = []Item{
		{Key: 'C', Label: "Cash Relief / Loans", Do: cashRelief},
		{Key: 'D', Label: "Deposit Funds", Do: money("Deposit", func(p *game.Empire) int64 { return p.Gold }, (*game.World).Deposit)},
		{Key: 'W', Label: "Withdraw Funds", Do: money("Withdraw", func(p *game.Empire) int64 { return p.Bank }, (*game.World).Withdraw)},
		{Key: 'I', Label: "Investments", Do: investFunds},
		{Key: 'L', Label: "List Investments / Loans", Do: listInvestments},
		{Key: 'V', Label: "View Bank Rates", Do: bankRates},
		{Key: '0', Label: "Quit", Do: back},
	}
	bank.DefaultOnEnter = quitOnEnter(bank)
	bank.Status = func(w *ctx) string {
		p := w.Player()
		lang := playerLang(w)
		// Plain text: the menu footer highlights the figures itself.
		return fmt.Sprintf("You have %s gold in hand and %s gold in the bank.",
			formatGold(p.Gold, lang), formatGold(p.Bank, lang))
	}

	attack.Items = []Item{
		{Key: 'R', Label: "Regular Attack", Do: regularAttack},
		{Key: 'N', Label: "Nuclear Attack", Do: nuclearAttack},
		{Key: 'C', Label: "Chemical Attack", Do: chemicalAttack},
		{Key: 'B', Label: "Biological Attack", Do: biologicalAttack},
		{Key: 'P', Label: "Attack Pirates", Do: attackPirates,
			// Hidden while under new-realm protection — a protected realm can't raid.
			Hidden: func(w *ctx) bool { return w.Player().Protection > 0 }},
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
	// (#75); Clingy Annihilator Ops is IB's equivalent of BRE's Gooie Kablooie
	// item, on its '9' as a later capture showed it. Send Trade Deal and Send Message reuse the same actions
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
		{Key: '7', Label: "Send Message", Do: gotoMenu(ipMessages)},
		{Key: '8', Label: "Special Operations", Do: gotoMenu(ipSpecial)},
		{Key: '9', Label: "Clingy Annihilator Ops", Do: clingyAnnihilator},
		{Key: 'A', Label: "SDI Program", Do: sdiProgram},
		{Key: 'D', Label: "Diplomacy List", Do: planetaryTreaties},
		{Key: 'S', Label: "Spy Database", Do: spyDatabase},
		{Key: 'T', Label: "Travel Times", Do: travelTimes},
		{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: '?', Label: "Help", Do: helpBrowse},
		{Key: '0', Label: "Quit", Do: back},
	}
	interplanetary.Status = goldStatus
	interplanetary.DefaultOnEnter = quitOnEnter(interplanetary)

	// IP Messages: BRE's interplanetary mail, addressed to planets rather than
	// to barons. Items, order and hotkeys are the original's.
	ipMessages.Items = []Item{
		{Key: '1', Label: "Single Planet", Do: ipMessageSingle},
		{Key: '2', Label: "Select Planets", Do: ipMessageSelect},
		{Key: '3', Label: "All Planets", Do: ipMessageAll},
		{Key: '4', Label: "Allied Planets", Do: ipMessageAllied},
		{Key: '5', Label: "Planet Coordinator", Do: ipMessageCoordinator},
		{Key: '0', Label: "Quit", Do: back},
	}
	ipMessages.DefaultOnEnter = quitOnEnter(ipMessages)

	// Order, numeric hotkeys, and per-op gold costs match BRE's live Covert
	// Operations menu (2026-07-21); costs are the balance.go Cost* constants
	// (#73). costOf shows a fixed gold price in the menu's cost column.
	costOf := func(n int) func(*ctx) int { return func(*ctx) int { return n } }
	covert.Items = []Item{
		{Key: '1', Label: "Send Spy", Price: costOf(game.CostSendSpy), Do: sendSpy},
		{Key: '2', Label: "Stir Revolts", Price: costOf(game.CostStirRevolts), Do: stirRevolts},
		{Key: '3', Label: "Set Up", Price: costOf(game.CostSetUp), Do: setUp},
		{Key: '4', Label: "Support Dissensions", Price: costOf(game.CostSupportDissensions), Do: supportDissensions},
		{Key: '5', Label: "Demoralize Forces", Price: costOf(game.CostDemoralizeForces), Do: demoralizeForces},
		{Key: '6', Label: "Spy on Relations", Price: costOf(game.CostSpyOnRelations), Do: spyRelations},
		{Key: '7', Label: "Bomb Enemy Targets", Price: costOf(game.CostBombEnemyTargets), Do: gotoMenu(bombTargets)},
		{Key: '8', Label: "Bribery", Price: costOf(game.CostBribery), Do: briberyOp},
		{Key: '9', Label: "Expose Enemy Ops", Price: costOf(game.CostExposeEnemyOps), Do: exposeEnemyOps},
		{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: '?', Label: "Help", Do: helpBrowse},
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
	// Numbered 1-8 with no Help item, as the live capture draws it — unlike the
	// local Bomb Enemy Targets submenu above, which is lettered.
	ipSpecial.Items = []Item{
		{Key: '1', Label: "Bomb Food Market", Do: ipSpecialStub},
		{Key: '2', Label: "Bomb Trading Market", Do: ipSpecialStub},
		{Key: '3', Label: "Bomb Trade Routes", Do: ipSpecialStub},
		{Key: '4', Label: "Undermine Investments", Do: ipSpecialStub},
		{Key: '5', Label: "Nuclear Assault", Do: ipSpecialStub},
		{Key: '6', Label: "Chemical Bombing", Do: ipSpecialStub},
		{Key: '7', Label: "R5-Slappenheimer", Do: ipSpecialStub},
		{Key: '8', Label: "Send SpyGuy", Do: sendSpyGuy},
		{Key: '0', Label: "Quit", Do: back},
	}
	ipSpecial.Status = goldStatus
	ipSpecial.DefaultOnEnter = quitOnEnter(ipSpecial)

	trading.Items = []Item{
		{Key: '1', Label: "Send Trade Deal", Do: sendTradeDeal},
		{Key: '2', Label: "Trading Market", Do: tradingMarket},
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
		{Key: 'D', Label: "Modify Diplomacy", Do: diplomacyModification},
		{Key: 'P', Label: "Player List", Do: playerList},
		{Key: '0', Label: "Quit", Do: back},
	}
	coord.DefaultOnEnter = quitOnEnter(coord)

	// BRE-style Price / # Owned columns: the buy price on Buy Food, the sell price
	// on Sell Food, and the caller's current food holdings on both — so the player
	// can see what they hold before buying or selling. The daily supply and gold on
	// hand go in the status line below (see foodMarketStatus).
	foodOwned := owned(func(p *game.Empire) int { return p.Food })
	food.Items = []Item{
		{Key: 'B', Label: "Buy Food", Do: buyFoodMarket,
			Price: func(w *ctx) int { return w.FoodBuyPrice() }, Owned: foodOwned},
		{Key: 'S', Label: "Sell Food", Do: sellFoodMarket,
			Price: func(w *ctx) int { return w.FoodSellPrice() }, Owned: foodOwned},
		{Key: 'A', Label: "Visit Advisors", Do: visitAdvisors},
		{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: '0', Label: "Quit", Do: back},
	}
	food.Status = foodMarketStatus
	food.DefaultOnEnter = quitOnEnter(food)

	system.Items = []Item{
		{Key: '#', Label: "Abdicate", Do: abdicate},
		{Key: 'X', Label: "End Protection", Do: endProtection,
			// Only relevant while the realm is still under new-realm protection.
			Hidden: func(w *ctx) bool { return w.Player().Protection <= 0 }},
		{Key: 'A', Label: "Visit Advisors", Do: visitAdvisors},
		// BRE lists Covert Operations here too, and hides it until the realm holds
		// an agent — the same gate as the per-turn covert step, and independent of
		// the Visit Covert Menu preference (captures show it listed with the
		// preference off). Verified in cap/: the item appears the moment agents go
		// 0 -> nonzero, in three separate sessions.
		{Key: 'C', Label: "Covert Operations", Do: gotoMenu(covert),
			Hidden: func(w *ctx) bool { return w.Player().Agents < 1 }},
		{Key: 'D', Label: "Diplomacy", Do: gotoMenu(diplomacy)},
		{Key: 'E', Label: "Empire Status", Do: empireStatus},
		{Key: 'F', Label: "Food Market", Do: gotoMenu(food)},
		{Key: 'G', Label: "Game Setup", Do: gameSetup},
		// BRE reserves 'I' on the System Menu for InterBBS Scores; About (an IB
		// addition) now lives in the Help browser, so the two don't collide (#17).
		{Key: 'I', Label: "InterBBS Scores", Do: interbbsScores, Hidden: ibbsHidden},
		{Key: 'M', Label: "Messages", Do: gotoMenu(messages)},
		{Key: 'P', Label: "Preferences", Do: gotoMenu(prefs)},
		{Key: 'R', Label: "Set Tax Rate", Do: setTaxRate},
		{Key: 'S', Label: "See Scores", Do: seeScores},
		{Key: 'T', Label: "Trading", Do: gotoMenu(trading)},
		{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: 'W', Label: "Write Macros", Do: writeMacros},
		{Key: '1', Label: "Set Industries", Do: setIndustries},
		{Key: '2', Label: "Show Instructions", Do: showInstructions},
		{Key: '3', Label: "Specialize Industry", Do: specializeIndustry,
			// Specializing is permanent, so hide the option once it's been done.
			Hidden: func(w *ctx) bool { return w.Player().Specialized != "" }},
		// BRE lists the Spy Database on the System Menu ('4') as well as on
		// InterPlanetary Ops ('S'); it holds cross-board intel, so IBBS-only.
		{Key: '4', Label: "Spy Database", Do: spyDatabase, Hidden: ibbsHidden},
		{Key: '?', Label: "Help", Do: helpBrowse},
		// BRE: Coordinator Vote is '5', shown only in IBBS games and only once the
		// voter's new-realm protection ends (a protected realm can't vote and isn't
		// a candidate). The Coordinator Menu is '*' and appears only for the empire
		// currently elected Coordinator (most votes).
		{Key: '5', Label: "Coordinator Vote", Do: voteCoordinator,
			Hidden: func(w *ctx) bool { return ibbsHidden(w) || w.Player().Protection > 0 }},
		{Key: '*', Label: "Coordinator Menu", Do: gotoMenu(coord),
			Hidden: func(w *ctx) bool { return ibbsHidden(w) || w.BBSCoordinator() != w.Player() }},
		{Key: '0', Label: "Quit", Do: back},
	}
	system.DefaultOnEnter = quitOnEnter(system)

	gameMenu := &Menu{Title: "Entry", Color: ansi.FgBrightMagenta, Columns: 2}
	gameMenu.Items = []Item{
		{Key: '1', Label: "Play", Do: runTurn},
		{Key: '2', Label: "See Status", Do: empireStatus},
		{Key: '3', Label: "See Scores", Do: seeScores},
		{Key: '4', Label: "Today's News", Do: showBulletinToday},
		{Key: '5', Label: "Yesterday's News", Do: showBulletinYesterday},
		{Key: '6', Label: "Messages", Do: gotoMenu(messages)},
		{Key: '8', Label: "Game Bulletins", Do: showBulletinToday},
		{Key: '9', Label: "InterPlanetary Ops", Do: gotoMenu(interplanetary), Hidden: ibbsHidden, Color: interplanetary.Color},
		{Key: 'A', Label: "Instructions", Do: showInstructions},
		// Also on the System menu, as Messages is. BRE has no Diplomacy item here
		// at all — it sits two menus in, under System — but a treaty is answered
		// between turns, like mail, so it belongs beside it on the way in (#70
		// took it out of the pre-turn flow; this is where it lands instead).
		{Key: 'D', Label: "Diplomacy", Do: gotoMenu(diplomacy)},
		// Also on the System menu, where BRE keeps it. Listed here as well because
		// a player wants the board's rules — turns per day, protection, how long
		// they may stay away — before starting a turn, not two menus in.
		{Key: 'G', Label: "Game Setup", Do: gameSetup},
		{Key: '?', Label: "Help", Do: helpBrowse}, // About lives inside Help now (#17)
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
	gameMenu.Header = gameMenuStatus

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
// locking. Like BRE, the count is the turns remaining AFTER the current one, so
// the last turn of the day reads 0 (Empire Status still shows the inclusive
// count). The Spending menu only renders during a turn, so TurnsLeft >= 1 here.
func spendingStatus(w *ctx) string {
	p := w.Player()
	lang := playerLang(w)
	turns := p.TurnsLeft - 1
	if turns == 1 {
		return fmt.Sprintf(i18n.T(lang, "You have %s gold and 1 turn."), formatGold(p.Gold, lang))
	}
	return fmt.Sprintf(i18n.T(lang, "You have %s gold and %d turns."), formatGold(p.Gold, lang), turns)
}

// covertStatus is the Covert Operations footer: gold on hand and agents held
// (BRE: "You have <gold> gold and <N> agents.").
// goldStatus is BRE's plainest footer, "You have N gold." — what the
// InterPlanetary and Special Operations menus carry.
func goldStatus(w *ctx) string {
	lang := playerLang(w)
	return fmt.Sprintf(i18n.T(lang, "You have %s gold."), formatGold(w.Player().Gold, lang))
}

func covertStatus(w *ctx) string {
	p := w.Player()
	return fmt.Sprintf(i18n.T(playerLang(w), "You have %s gold and %d agents."), formatGold(p.Gold, playerLang(w)), p.Agents)
}

// foodMarketStatus is the Food Market status line: today's planet-wide supply
// (or "Unlimited" when the sysop toggled it) plus the caller's gold on hand.
func foodMarketStatus(w *ctx) string {
	lang := playerLang(w)
	supply := i18n.T(lang, "Unlimited supply today.")
	if !w.Config.FoodUnlimited {
		supply = fmt.Sprintf(i18n.T(lang, "%s units of food available today."), comma(w.FoodMarketSupply))
	}
	return fmt.Sprintf(i18n.T(lang, "%s  You have %s gold."), supply, formatGold(w.Player().Gold, lang))
}

// gameMenuStatus is the line ABOVE the opening menu's title rule, where BRE puts
// it. The game day is deliberately absent — BRE does not show it there, and it
// used to sit in a footer under the menu.
//
// It reports World.StartedDate, the day the game actually began, NOT
// Config.GameStartDate: the latter is the date a sysop scheduled the game for
// and is empty on the common "start immediately" setup, which left the line
// blank on any default board. Before a scheduled start arrives it announces when
// the game begins instead.
func gameMenuStatus(w *ctx) string {
	lang := playerLang(w)
	if d := w.Config.GameStartDate; d != "" && w.Today != "" && !w.Config.GameStarted(w.Today) {
		return fmt.Sprintf(i18n.T(lang, "The game begins %s."), d)
	}
	if w.StartedDate == "" {
		return "" // no maintenance has run yet, so the game has not begun
	}
	return fmt.Sprintf(i18n.T(lang, "Game started on %s"), w.StartedDate)
}
