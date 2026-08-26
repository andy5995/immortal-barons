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
	IPSpecial      *Menu // the interplanetary "Special Operations" submenu
	TerrorOps      *Menu
	Covert         *Menu
	Trading        *Menu
	Diplomacy      *Menu
	Messages       *Menu
	System         *Menu
	Game           *Menu
	Food           *Menu
}

// noBombingOps, noMissileOps and noAnnihilator are the sysop's three special-
// attack switches as menu gates. A disabled class leaves the menu rather than
// refusing at the prompt; byKey skips a hidden item, so the hotkey goes too.
//
// BINARY-VERIFIED reach (2026-08-23). Each byte has exactly TWO testers in the
// whole product — the Configuration Editor that sets it, and one menu that reads
// it: Gooie Kablooies (cfg+0x18a) in run_interbbs_menu, Bombing Ops (cfg+0x18b)
// and Missile Ops (cfg+0x18c) both in run_bombing_operations_menu, which is
// Special Operations. **Nothing local is gated by any of them** — these are
// inter-BBS switches about what may be sent to another board, exactly as
// game/reset.hlp words them. IB used to hide the local Attack menu's WMDs behind
// Missile Ops and Bomb Enemy Targets behind Bombing Ops; neither is BRE's, and
// both are gone.
func noBombingOps(w *ctx) bool  { return !w.Config.BombingOps }
func noMissileOps(w *ctx) bool  { return !w.Config.MissileOps }
func noAnnihilator(w *ctx) bool { return !w.Config.GooieKablooie }

// noIPTrading hides Trading when the league has turned it off. It is IB's own
// feature, so a league that wants the original's shape says so and the menu
// entry goes with it, hotkey included.
func noIPTrading(w *ctx) bool { return !w.Config.IPTrading }

// noLocalAttacks hides the ways of striking a baron on this board. With Local
// Attacks disabled BRE's Attack Menu collapses to the pirate and alliance
// entries, captured live in docs/dev/bre-screens.md ("Attack Menu (InterBBS,
// local attacks OFF)") — Regular, Nuclear, Chemical and Biological all go.
func noLocalAttacks(w *ctx) bool { return !w.LocalAttacksAllowed() }

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
	// Width: BRE's box is 32 columns (re-captured live 2026-08-17,
	// cap/covert-menu-20260817.cap), sized to its widest row. IB's same row runs
	// 34: the engine indents every menu item two columns, and IB comma-groups
	// figures BRE prints bare — a recorded divergence. So 34 follows BRE's rule,
	// sizing the box to its own content, where 32 would clip every priced row.
	covert := &Menu{Title: "Covert Operations", Color: ansi.FgBrightGreen, ExitOnEnter: true, Status: covertStatus, Width: 34}
	ipSpecial := &Menu{Title: "Special Operations", Color: ansi.FgBrightYellow, ExitOnEnter: true, Columns: 2}
	// BRE's Terrorist Ops submenu (IP Operations item '2') shows 9 named
	// operations; all share the same mechanical effect (each agent destroys
	// 1/7 of a random unit type) but BRE carries the op type in the packet
	// so the result report can name it. Order, labels and hotkeys match the
	// binary-verified string table.
	terrorOps := &Menu{Title: "Terrorist Ops", Color: ansi.FgBrightYellow, ExitOnEnter: true}
	// BRE draws IP Messages as a narrow single-column box (25 columns, from a
	// live capture) rather than at the full menu width.
	ipMessages := &Menu{Title: "IP Messages", Color: ansi.FgBrightCyan, ExitOnEnter: true, Width: 25}
	ipTrading := &Menu{Title: "Trading", Color: ansi.FgBrightYellow, ExitOnEnter: true, Header: tradingHeader}
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
	// sellPrice is the buy-back price shown on the Sell menu; game.UnitSellPrice
	// is the same rule the sell itself is paid at (agents are the exception — a
	// flat SellAgentPrice).
	sellPrice := func(f func(*ctx) int) func(*ctx) int {
		return func(w *ctx) int { return game.UnitSellPrice(f(w)) }
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
		{Key: '1', Label: "Troopers", Price: sellPrice(priceTrooper), Owned: owned(troopers),
			Do: sellUnit("Sell Troopers", troopers, (*game.World).SellTroopers)},
		{Key: '2', Label: "Jets", Price: sellPrice(priceJet), Owned: owned(jets),
			Do: sellUnit("Sell Jets", jets, (*game.World).SellJets)},
		{Key: '3', Label: "Turrets", Price: sellPrice(priceTurret), Owned: owned(turrets),
			Do: sellUnit("Sell Turrets", turrets, (*game.World).SellTurrets)},
		{Key: '4', Label: "Bombers", Price: sellPrice(priceBomber), Owned: owned(bombers),
			Do: sellUnit("Sell Bombers", bombers, (*game.World).SellBombers)},
		{Key: '6', Label: "Regions", Price: func(w *ctx) int { return 0 }, Owned: owned(land), Do: sellLand},
		{Key: '7', Label: "Covert Agents", Price: func(w *ctx) int { return game.SellAgentPrice }, Owned: owned(agents),
			Do: sellUnit("Sell Covert Agents", agents, (*game.World).SellAgents)},
		{Key: '8', Label: "Tanks", Price: sellPrice(priceTank), Owned: owned(tanks),
			Do: sellUnit("Sell Tanks", tanks, (*game.World).SellTanks)},
		{Key: '9', Label: "Carriers", Price: sellPrice(priceCarrier), Owned: owned(carriers),
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

	ipTrading.Items = []Item{
		{Key: 'M', Label: "Markets", Do: ipMarkets},
		{Key: 'B', Label: "Bids Out", Do: ipPendingBids},
		{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		{Key: '0', Label: "Quit", Do: back},
	}
	ipTrading.DefaultOnEnter = quitOnEnter(ipTrading)

	attack.Items = []Item{
		{Key: 'R', Label: "Regular Attack", Do: regularAttack, Hidden: noLocalAttacks},
		{Key: 'N', Label: "Nuclear Attack", Do: nuclearAttack, Hidden: noLocalAttacks},
		{Key: 'C', Label: "Chemical Attack", Do: chemicalAttack, Hidden: noLocalAttacks},
		{Key: 'B', Label: "Biological Attack", Do: biologicalAttack, Hidden: noLocalAttacks},
		{Key: 'P', Label: "Attack Pirates", Do: attackPirates,
			// Hidden while under new-realm protection — a protected realm can't
			// raid — and on a board whose sysop has turned the pirates off.
			Hidden: func(w *ctx) bool { return !w.Config.Pirates || w.Player().Protection > 0 }},
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
	// (#75); Gooie Kablooie Ops is IB's equivalent of BRE's Gooie Kablooie
	// item, on its '9' as a later capture showed it. Send Trade Deal and Send Message reuse the same actions
	// as the Trading and Messages menus; Special Operations opens the separate
	// interplanetary Special Operations menu (BRE's cross-planet covert set, not
	// the local Covert menu). Indiv. Attack Force ('6') has no interplanetary
	// individual-attack mechanic behind it yet — see indivAttackForce's doc.
	interplanetary.Items = []Item{
		{Key: '1', Label: "View IPScores", Do: interbbsScores},
		{Key: '2', Label: "Terrorist Ops", Do: gotoMenu(terrorOps)},
		{Key: '3', Label: "Send Trade Deal", Do: sendTradeDeal},
		{Key: '4', Label: "Create Group Attack", Do: createGroupAttack},
		{Key: '5', Label: "Join Group Attack", Do: joinGroupAttack},
		{Key: '6', Label: "Indiv. Attack Force", Do: indivAttackForce},
		{Key: '7', Label: "Send Message", Do: gotoMenu(ipMessages)},
		// Send SpyGuy is the one item here the two switches do not govern, so the
		// menu stays reachable even with both off.
		{Key: '8', Label: "Special Operations", Do: gotoMenu(ipSpecial)},
		{Key: '9', Label: "Gooie Kablooie Ops", Do: gooieKablooie, Hidden: noAnnihilator},
		{Key: 'A', Label: "SDI Program", Do: sdiProgram},
		{Key: 'B', Label: "Trading", Do: gotoMenu(ipTrading), Hidden: noIPTrading},
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
	// The eight operations come from covertRows, which carries each one's
	// game.CovertOp: the label IS the constant, so the screen cannot name an op
	// by a string of its own (#208). Their msgids are still the same English
	// text, so the .po catalogs are untouched.
	for _, row := range covertRows {
		covert.Items = append(covert.Items, Item{
			Key: row.Key, Label: string(row.Op), Price: costOf(row.Cost), Do: row.action(),
		})
	}
	covert.Items = append(covert.Items,
		Item{Key: '9', Label: "Expose Enemy Ops", Price: costOf(game.CostExposeEnemyOps), Do: exposeEnemyOps},
		Item{Key: 'V', Label: "Visit Bank", Do: gotoMenu(bank)},
		Item{Key: '?', Label: "Help", Do: helpBrowse},
		Item{Key: '0', Label: "Quit", Do: back},
	)
	covert.DefaultOnEnter = quitOnEnter(covert)
	// BRE re-reads the agent count at the head of the covert menu's own loop
	// (enter_covert_operations_menu, BRE.OVR 0x0179db, testing the 32-bit count
	// at record +0x26f) and returns when it hits zero — so the operation that
	// spends the last agent closes the menu and the turn moves on. IB tested it
	// only on the way in, and went on drawing a menu that could do nothing but
	// refuse. The same wrapper serves BRE's System-menu entry, so this applies
	// there too.
	covert.ExitWhen = func(w *ctx) bool {
		p := w.Player()
		return p == nil || p.Agents < 1
	}

	// Interplanetary Special Operations: BRE's cross-planet covert menu — every
	// op targets an empire on another planet. The 8-item table at BRE.OVR 170011
	// is read by `run_bombing_operations_menu` (0x029EA9) alone, whose only caller
	// is the InterBBS menu, so the table belongs to THIS menu and to no other.
	// Labels/order are binary-verified; the Send SpyGuy hotkey wasn't recoverable
	// from the overlay dispatch, so IB numbers it 8 with the rest. Only Send SpyGuy is wired; the
	// bombing/WMD variants are recorded-but-inert until interplanetary covert
	// strikes are built. ('?'/'0' are IB's menu convention; BRE exits via ESC/Q
	// with no listed items.) Numbered 1-8 with no Help item, as the live capture
	// draws it.
	ipSpecial.Items = []Item{
		{Key: '1', Label: "Bomb Food Market", Do: ipSpecialOp(game.OpBombFood), Hidden: noBombingOps},
		{Key: '2', Label: "Bomb Trading Market", Do: ipSpecialOp(game.OpBombMarket), Hidden: noBombingOps},
		{Key: '3', Label: "Bomb Trade Routes", Do: ipSpecialOp(game.OpBombRoutes), Hidden: noBombingOps},
		{Key: '4', Label: "Undermine Investments", Do: ipSpecialOp(game.OpUndermine), Hidden: noBombingOps},
		{Key: '5', Label: "Nuclear Assault", Do: ipSpecialOp(game.OpNuclear), Hidden: noMissileOps},
		{Key: '6', Label: "Chemical Bombing", Do: ipSpecialOp(game.OpChemical), Hidden: noMissileOps},
		{Key: '7', Label: "S3-Sabre", Do: ipSpecialOp(game.OpSabre), Hidden: noMissileOps},
		{Key: '8', Label: "Send SpyGuy", Do: sendSpyGuy},
		{Key: '0', Label: "Quit", Do: back},
	}
	ipSpecial.Status = goldStatus
	ipSpecial.DefaultOnEnter = quitOnEnter(ipSpecial)

	// Terrorist Ops submenu: BRE's 9-item sub-menu under IP Operations item '2'.
	// All sub-ops share the same mechanical effect (each agent destroys 1/7 of
	// one random unit type) and the same gold cost; the labels are cosmetic flavor.
	// Order, labels and hotkeys match the binary-verified string table. Numbered
	// 1-9 with no Help item, as BRE draws it.
	terrorOps.Items = []Item{
		{Key: '1', Label: "Send Spy", Do: terrorOp(game.TerrorOpSpy)},
		{Key: '2', Label: "Bomb Intelligence", Do: terrorOp(game.TerrorOpBombIntel)},
		{Key: '3', Label: "Demoralize", Do: terrorOp(game.TerrorOpDemoralize)},
		{Key: '4', Label: "Cause Dissensions", Do: terrorOp(game.TerrorOpDissensions)},
		{Key: '5', Label: "Bomb AirBases", Do: terrorOp(game.TerrorOpBombAirBases)},
		{Key: '6', Label: "Stir Emigrations", Do: terrorOp(game.TerrorOpEmigrations)},
		{Key: '7', Label: "Spread Propaganda", Do: terrorOp(game.TerrorOpPropaganda)},
		{Key: '8', Label: "Bomb Food Stores", Do: terrorOp(game.TerrorOpBombFood)},
		{Key: '9', Label: "Sabotage HQ", Do: terrorOp(game.TerrorOpSabotageHQ)},
		{Key: '0', Label: "Quit", Do: back},
	}
	terrorOps.DefaultOnEnter = quitOnEnter(terrorOps)

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
		{Key: '1', LabelFn: onOff("Visit Covert Menu", func(w *ctx) *bool { return &w.prefs().VisitCovert }),
			Do: toggle(func(w *ctx) *bool { return &w.prefs().VisitCovert })},
		{Key: '2', LabelFn: onOff("Visit Trading Menu", func(w *ctx) *bool { return &w.prefs().VisitTrading }),
			Do: toggle(func(w *ctx) *bool { return &w.prefs().VisitTrading })},
		{Key: '3', LabelFn: onOff("Visit Message Menu", func(w *ctx) *bool { return &w.prefs().VisitMessage }),
			Do: toggle(func(w *ctx) *bool { return &w.prefs().VisitMessage })},
		{Key: '4', LabelFn: onOff("Use Enter To Exit BUY Menu", func(w *ctx) *bool { return &w.prefs().EnterExitsBuy }),
			Do: toggle(func(w *ctx) *bool { return &w.prefs().EnterExitsBuy })},
		{Key: '5', LabelFn: onOff("Deposit gold at End of Turn", func(w *ctx) *bool { return &w.prefs().DepositEndTurn }),
			Do: toggle(func(w *ctx) *bool { return &w.prefs().DepositEndTurn })},
		{Key: '6', LabelFn: onOff("Auto-Pay Maintenance", func(w *ctx) *bool { return &w.prefs().AutoPayMaint }),
			Do: toggle(func(w *ctx) *bool { return &w.prefs().AutoPayMaint })},
		{Key: '7', LabelFn: onOff("Auto-Feed Empire", func(w *ctx) *bool { return &w.prefs().AutoFeed }),
			Do: toggle(func(w *ctx) *bool { return &w.prefs().AutoFeed })},
		{Key: 'L', LabelFn: func(w *ctx) string {
			// playerLang, not the raw stored code: on a CP437 session it names the
			// language actually rendering (the CP437-safe fallback), and its
			// endonym is displayable — a stored "Русский" would only mojibake.
			return i18n.T(playerLang(w), "Language") + ": " + languageName(playerLang(w))
		}, Do: pickLanguage},
		// IB's own, and once spent it is gone for good — the way Specialize
		// Industry goes from the System menu. A realm still under New Realm
		// Protection sees it greyed rather than missing, so it can be found again
		// when protection ends; choosing it then says why it is refused.
		{Key: 'N', Label: "Change Realm Name", Do: changeRealmName,
			Hidden: func(w *ctx) bool { return w.Player().FormerName != "" },
			Dimmed: func(w *ctx) bool { return w.Player().Protection > 0 }},
		{Key: '0', Label: "Quit", Do: back},
	}
	prefs.DefaultOnEnter = quitOnEnter(prefs)

	// The Coordinator Menu belongs to the elected BBS Coordinator (see the
	// System menu gate below); it holds the planet-coordination functions.
	// BRE's own Coordinator Ops menu, from its handler (run_interbbs_operations_menu,
	// BRE.OVR 0x015e3a): four items keyed '1'-'4', dispatched by cmp al,'1'..'4',
	// drawn from the label offsets 0x00/0x11/0x22/0x37 in that order — Dismantle
	// Gooie, Modify Diplomacy, Global Recon Request, View Diplomacy. See
	// docs/dev/bre-screens.md, which has carried that table since the eight-
	// lettered-hotkey reading was corrected.
	//
	// Player List is IB's own, keyed past the original's four rather than
	// displacing one. IB numbered its three built items 1-3 until 2026-08-18,
	// which put every one of them on the original's key for its neighbour.
	coord.Items = []Item{
		{Key: '1', Label: "Dismantle Gooie Kablooie", Do: dismantleAnnihilator,
			Hidden: noAnnihilator},
		{Key: '2', Label: "Modify Diplomacy", Do: diplomacyModification},
		{Key: '3', Label: "Global Recon Request", Do: globalReconRequest},
		{Key: '4', Label: "View Diplomacy", Do: planetaryTreaties},
		{Key: '5', Label: "Player List", Do: playerList},
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
		// voter's new-realm protection ends — the guard at show_game_settings
		// +0x082f. The Coordinator Menu is '*' and appears only for the empire
		// currently elected Coordinator (most votes).
		//
		// The original contradicts itself here and IB copies both halves: the
		// turn-opening notice tells a protected realm to change its vote in this
		// menu, which is the one menu the same routine has just left the item out
		// of. Out of protection the item is there and the vote can change freely.
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
		{Key: '8', Label: "Game Bulletins", Do: gameBulletins},
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
		IPSpecial:      ipSpecial,
		TerrorOps:      terrorOps,
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
