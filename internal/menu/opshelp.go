package menu

import "github.com/andy5995/immortal-barons/internal/session"

// opshelp.go — the (?) Help browsers on the two InterPlanetary submenus.
//
// DELIBERATE DIVERGENCE. The original lists these ops and nothing else: the
// Terrorist Ops menu is nine numbered items and a Quit, the Special Operations
// menu eight and a Quit, both confirmed across the captures. Its parent
// InterPlanetary menu DOES carry a Help item, so dropping into either submenu
// is the one place the help key stops answering.
//
// IB adds one, in the shape the Attack Type menu already uses (showTopicHelp):
// a browser the reader stays inside, so nine ops can be read one after another
// without returning to the menu each time.
//
// The reason is that the ops are NOT interchangeable, whatever a stale comment
// in tree.go used to claim. Each lands on a different holding at its own rate,
// all binary-verified (TerrorOpLosses and applyTerrorOp), so choosing between
// them is a real decision and the menu gave the reader nothing to make it with.
//
// The prose is IB's own and the figures are IB's constants; nothing here is the
// original's text.

// terrorOpTopics documents the nine Terrorist Ops. The percentages are the
// bands in game.TerrorOpLosses, written as the reader sees them rather than as
// Base/Spread.
var terrorOpTopics = []attackTypeTopic{
	{name: "Send Spy", body: "Costs the target nothing and takes nothing from it. What it brings home is intelligence: the realm's forces and standing, filed in your Spy Database."},
	{name: "Bomb Intelligence", body: "Destroys 2 to 4 percent of the target's covert agents, which is what it can send against you in return."},
	{name: "Demoralize", body: "Cuts the target's military morale by a fixed fraction. Morale scales everything its army does, in attack and in defence."},
	{name: "Cause Dissensions", body: "Destroys 2 to 4 percent of the target's troopers."},
	{name: "Bomb AirBases", body: "Destroys 3 to 7 percent of the target's jets. The widest band of the unit strikes, and jets are the most expensive thing it can take."},
	{name: "Stir Emigrations", body: "Drives off 4 to 10 percent of the target's population, which is its tax base and its food bill at once."},
	{name: "Spread Propaganda", body: "Cuts the target's popular support by a fixed fraction. Low support costs it income and invites unrest."},
	{name: "Bomb Food Stores", body: "Destroys anything from nothing at all to 29 percent of the target's food. The wildest of the operations: it can be wasted entirely or gut a season's stores."},
	{name: "Sabotage HQ", body: "Knocks the target's HeadQuarters back. HQ is what makes its tanks worth more than three troopers each, so this weakens every tank it owns."},
}

// ipSpecialOpTopics documents the eight InterPlanetary Special Operations.
// Several are recorded-but-inert pending the interplanetary covert strikes, and
// the topics say so rather than describing something that will not happen.
var ipSpecialOpTopics = []attackTypeTopic{
	{name: "Bomb Food Market", body: "Aimed at the food market of another planet. Recorded but not yet built: sending it costs you nothing and does nothing."},
	{name: "Bomb Trading Market", body: "Aimed at another planet's trading market. Recorded but not yet built."},
	{name: "Bomb Trade Routes", body: "Aimed at the trade routes between planets. Recorded but not yet built."},
	{name: "Undermine Investments", body: "Aimed at another planet's invested gold. Recorded but not yet built."},
	{name: "Nuclear Assault", body: "A nuclear missile at a realm on another planet. One of each missile per game day, and the target's SDI may stop it."},
	{name: "Chemical Bombing", body: "A chemical missile at a realm on another planet. It kills population rather than ruining land."},
	{name: "S3-Sabre", body: "A variable-return missile. What it destroys depends on a dial, and whether you are asked for that dial at all is a setting your sysop chooses."},
	{name: "Send SpyGuy", body: "Posts a watcher on another planet for a number of days you pay for. He gathers no intelligence; he reports the group attacks and Gooie Kablooies being readied against your planet, as planet news."},
}

// Both are menu Items, so they return Stay: the reader lands back on the
// menu they asked from, which is where they were choosing an op.
func showTerrorOpHelp(s session.Session, w *ctx) Result {
	showTopicHelp(s, terrorOpTopics)
	return Stay
}

func showIPSpecialOpHelp(s session.Session, w *ctx) Result {
	showTopicHelp(s, ipSpecialOpTopics)
	return Stay
}
