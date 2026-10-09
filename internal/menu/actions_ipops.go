package menu

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// actions_ipops.go — the operations sent against another planet short of a
// battle: the spy guy, the InterPlanetary special ops, the spy database, the
// terror-bombing table, and global recon.

// needsTurnPlayed wraps an InterPlanetary item that the original refuses until
// the caller has begun a turn this entry, printing the refusal in place of the
// action. Five of the menu's items carry it, read out of the dispatch in
// run_interbbs_menu (BRE.OVR 0x020caf): Send Trade Deal, Create Group Attack,
// Indiv. Attack Force and Special Operations call
// enforce_interbbs_turn_requirement (0x020a12) at sites 0x021038, 0x021051,
// 0x021076 and 0x02109b, and Join Group Attack carries its own copy of the same
// refusal inside the routine (0x02d1c5). The other nine items — the scores, the
// terrorist ops, messages, the Gooie Kablooie, SDI, the diplomacy list, the spy
// database, travel times and the bank — are exempt, and the exemption of
// Terrorist Ops matches what play on a real board shows.
//
// Every refusal PRINTS: the helper's only job is to write the line, and the
// inline copy writes it too, so no gated item redraws silently.
func needsTurnPlayed(do Action) Action {
	return func(s session.Session, w *ctx) Result {
		if !turnPlayedThisEntry(s, w) {
			return Stay
		}
		return do(s, w)
	}
}

// turnPlayedThisEntry is the same gate as a check rather than a wrapper, for an
// action that has something to SHOW before it refuses. Join Group Attack is the
// one, and this is the ORIGINAL's shape rather than a liberty taken with it: its
// refusal for that item is inline rather than in the shared helper, after the
// party table is drawn (see #162 in docs/mechanics-reference.md).
func turnPlayedThisEntry(s session.Session, w *ctx) bool {
	if w.turnPlayed {
		return true
	}
	ok(s, "You must play at least one turn each entry into the game before you can use this option.")
	return false
}

// sendSpyGuy is the Special Operations "Send SpyGuy" item: post a watcher on
// another PLANET for a paid number of days. He is not a covert agent — no agent
// is spent, he cannot be caught, and he brings back no intelligence. What he
// does is send word home the moment his hosts aim a group attack or a Gooie
// Kablooie at his own planet, and that word arrives as planet news there.
// The model is BRE's, read out of the binary; see internal/game/spyguy.go.
func sendSpyGuy(s session.Session, w *ctx) Result {
	// The original gates the whole Special Operations node on the caller's own
	// protection — the InterPlanetary menu tests it when '8' is pressed, with no
	// exemption for any item inside (`BRE.OVR 0x020F88`). IB gates the items
	// instead, and this one had been missed.
	if blockedByIPProtection(s, w) {
		return Stay
	}
	var planets []string
	var perDay int64
	var out []game.SpyGuyPosted
	w.Read(func() {
		planets = w.KnownBoards()
		perDay = w.SpyGuyCostPerDay()
		out = w.SpyGuysOut()
	})
	// A planet this board cannot route to is not offered: the man would be
	// discarded on the way out (game.ErrSpyGuyNoRoute).
	planets = addressable(w, planets)
	if noPlanets(s, len(planets)) {
		return Stay
	}
	showSpyGuysOut(s, out)
	// The price is quoted before the target is picked, as BRE quotes it: it is
	// the same on every planet, being drawn from the sender's own size.
	fmt.Fprintf(s, "\n%s"+tr(s, "A SpyGuy costs %s%s%s gold per day.")+"%s\n",
		ansi.FgWhite, ansi.FgBrightCyan, comma(perDay), ansi.FgWhite, ansi.Reset)
	board := pickAddressee(s, w, planets)
	if board == "" {
		return Stay
	}
	// The whole stay the man can be paid for, not the part this baron happens to
	// have the gold for. The original offers only what the gold covers, which
	// tells a short baron nothing about what the office is actually for; IB
	// offers the length and deals with the money afterward, where the bank is.
	days := promptSuggested(s, "How many days would you like him to remain?",
		game.SpyGuyDefaultDays, game.SpyGuyMaxDays)
	if days < 1 {
		return Stay
	}
	// The same helper every other op on this menu uses: the refusal, the bank,
	// and then the stay if they came back with the gold for it.
	if !affordOrBank(s, w, perDay*int64(days), game.ErrCantAfford) {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		return w.World.SendSpyGuy(p, board, days)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Your SpyGuy leaves for %s, and will watch it for %d days.", board, days)
	return Stay
}

// bombersOrBuy reports whether the caller holds the bombers a Special
// Operation needs, offering the missing ones when they do not rather than
// sending the player off to the Spending menu and back. IB's own: the original
// refuses and stops. It runs once the op is priced, so the quote can show what
// the bombers and the op come to together, and the bank, when it is needed, is
// offered for both: bombers bought for an op the gold then cannot send are
// bombers the player did not want.
func bombersOrBuy(s session.Session, w *ctx, opCost int64) bool {
	var need int
	var cost int64
	w.Read(func() {
		if p := w.Player(); p != nil {
			need, cost = w.BomberShortfall(p)
		}
	})
	if need == 0 {
		return true
	}
	failNoPause(s, game.ErrNeedBombers)
	bombers := plural(s, float64(need), "%.0f Bomber", "%.0f Bombers")
	total := cost + opCost
	okNoPause(s, "Price of %s: %s gold, %s with the operation.", bombers, comma(cost), comma(total))
	if short, present := shortOf(w, total); present && short > 0 &&
		!bankTheShortfall(s, w, total, short, game.ErrCantAfford) {
		return false
	}
	drawOfferBox(s, ansi.FgBrightRed, tr(s, "Bombers Needed"),
		[]string{fmt.Sprintf(tr(s, "Buy %s for %s gold"), bombers, comma(cost))})
	if ChoiceQuit(s, 1) != 1 {
		return false
	}
	if err := w.mutatePlayer(func(p *game.Empire) error {
		return w.World.BuyBomberShortfall(p, need, cost)
	}); err != nil {
		fail(s, err)
		return false
	}
	okNoPause(s, "%s purchased.", bombers)
	return true
}

// showSpyGuysOut lists the watchers this planet already has out, so any baron
// can see them, and so a sender can see that a planet is already watched: the
// far board keeps only the longer of two stays, and a shorter one buys nothing.
// BRE keeps no such list.
func showSpyGuysOut(s session.Session, out []game.SpyGuyPosted) {
	if len(out) == 0 {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgWhite, tr(s, "This planet has no SpyGuys out."), ansi.Reset)
		return
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgWhite, tr(s, "This planet's SpyGuys:"), ansi.Reset)
	for _, o := range out {
		line := fmt.Sprintf(tr(s, "%s: %s left, sent by %s"), o.Board,
			plural(s, float64(o.Days), "%.0f day", "%.0f days"), o.By)
		// Wrapped before the names are colored: the escapes are invisible on
		// screen but would count against the margin.
		fmt.Fprintf(s, "%s\n", hiTokens(WrapIndented(line, "  "), []string{o.Board, o.By}, ansi.FgBrightCyan))
	}
}

// ipSpecialOp drives every item on the interplanetary Special Operations menu
// (#49): choose a target, quote the price, confirm, and send. What "a target"
// means depends on the op — a planet for the four bombing ops, a named baron for
// the three missiles — which is why the choice branches below.
//
// The strike itself happens on the target's board when the packet lands, so the
// screen promises a report rather than printing an outcome — the same shape as
// Terrorist Ops below, and for the same reason.
func ipSpecialOp(op game.SpecialOp) func(session.Session, *ctx) Result {
	return func(s session.Session, w *ctx) Result {
		if blockedByIPProtection(s, w) {
			return Stay
		}
		// Checked before a planet is picked so a baron who cannot deliver a
		// payload, and cannot buy the means, is not walked through choosing a
		// target first. One who can buy them is offered them once the op is
		// priced (bombersOrBuy).
		var short bool
		w.Read(func() {
			if p := w.Player(); p != nil {
				short = p.Bombers < game.BombingBombersRequired
			}
		})
		if short && !militaryForSale(w) {
			fail(s, game.ErrNeedBombers)
			return Stay
		}
		// The S3-Sabre's handling mode is the sysop's, and this is the only
		// menu that fires one: in BRE the missile is an interplanetary Special
		// Operation, so the local Covert menu never had it. Under None the item
		// is not on the menu at all (sabreUnavailable).
		dial := 0
		if op == game.OpSabre {
			var mode game.SabreMode
			w.Read(func() { mode = w.Config.SabreHandling })
			if mode == game.SabreUserSelect {
				// The dial aims the missile: it picks which of the target's assets
				// the payload goes for. It is nudged by one either way in flight,
				// so a setting is a tendency rather than a promise.
				dial = min(max(promptInt(s, "Set the S3-Sabre dial (0-10)"), game.SabreDialMin), game.SabreDialMax)
			}
			// Random and Constant handling settle the dial in the game package,
			// where the config and the RNG live; the value passed here is ignored
			// under those modes.
		}
		label := game.SpecialOpLabel(op)
		var board, baron string
		if op.TargetsPlanet() {
			// The four bombing ops wreck what the whole planet shares — its food
			// market, its trading market, its bank — so there is no baron to
			// name and asking for one would be asking a question with no answer.
			var boards []string
			w.Read(func() { boards = w.ScoredBoards() })
			if noPlanets(s, len(boards)) {
				return Stay
			}
			fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightYellow, fmt.Sprintf(tr(s, "%s against which planet?"), label), ansi.Reset)
			if board = pickAddressee(s, w, boards); board == "" {
				return Stay
			}
		} else {
			var found bool
			board, baron, _, found = pickRemoteTarget(s, w,
				fmt.Sprintf(tr(s, "%s against which planet?"), label),
				fmt.Sprintf(tr(s, "%s against which baron?"), label))
			if !found {
				return Stay
			}
		}
		// After the target is chosen, because the three missiles are priced off
		// what that realm holds — which is also where the original quotes it
		// ("Cost: N Gold", prepare_bombing_attack).
		cost := game.SpecialOpGoldCost(op, w.World.RemoteLand(board, baron))
		okNoPause(s, "This operation will cost %s gold.", comma(cost))
		if !askYesNoHere(s, "Send this Operation?", true) {
			return Stay
		}
		if !bombersOrBuy(s, w, cost) {
			return Stay
		}
		// Accepted but short: the refusal, then the bank, rather than the send
		// simply failing at a price the player has just agreed to.
		if !affordOrBank(s, w, cost, game.ErrCantAfford) {
			return Stay
		}
		err := w.mutatePlayer(func(p *game.Empire) error {
			return w.World.SendSpecialOp(p, board, baron, op, dial)
		})
		if err != nil {
			fail(s, err)
			return Stay
		}
		if baron == "" {
			ok(s, "Your %s is away against %s. Word will come back with the next packet.", label, board)
			return Stay
		}
		ok(s, "Your %s is away against %s of %s. Word will come back with the next packet.", label, baron, board)
		return Stay
	}
}

// spyDatabase is the read-only Spy Database viewer (sending is Special
// Operations → Send SpyGuy, matching BRE). Reports are grouped by realm, oldest
// first, and a realm with two or more ends with the change since the one before,
// which is the before-and-after a spy sent around a strike is for.
func spyDatabase(s session.Session, w *ctx) Result {
	if len(w.SpyDatabase) == 0 {
		ok(s, "The spy database is empty. Spy on empires on other planets to fill it.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Spy Database:"), ansi.Reset)
	rows := recapHeaderRows
	for _, reports := range spyReportsByRealm(w.SpyDatabase) {
		// A blank line, the realm, the head and its rule, a row per report, and
		// a change row.
		block := 4 + len(reports)
		if len(reports) > 1 {
			block++
		}
		if rows+block > recapPageRows {
			pauseTight(s)
			rows = 0
		}
		rows += block
		r := reports[0]
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, fmt.Sprintf(tr(s, "%s of %s"), r.Empire, r.Board), ansi.Reset)
		printTableHead(s, w.Term, spyColumns)
		for _, r := range reports {
			// Where it is known, when the report arrived: two reports on one realm
			// often share a game day.
			when := r.Date
			if !r.Filed.IsZero() {
				when = r.Filed.In(sessionZone(s)).Format("01/02 15:04")
			}
			printSpyRow(s, w.Term, ansi.FgWhite+gaPad(w.Term, when, spyColumns[0].width, false),
				[]string{comma(r.Land), comma(r.Offense), comma(r.Defense), comma(r.Gold)})
		}
		if n := len(reports); n > 1 {
			a, b := reports[n-2], reports[n-1]
			printSpyRow(s, w.Term, ansi.FgBrightWhite+gaPad(w.Term, tr(s, "Change"), spyColumns[0].width, false),
				[]string{signed(int64(b.Land - a.Land)), signed(int64(b.Offense - a.Offense)),
					signed(int64(b.Defense - a.Defense)), signed(b.Gold - a.Gold)})
		}
	}
	pause(s)
	return Stay
}

// spyColumns is a Spy Database table read left to right, drawn in the Join
// Group Attack table's shape (printTableHead). With its sign, Land holds a
// figure to 99,999,999, Offense and Defense one to two billion, and Gold one to
// 99 billion; the five come to 75 with their separators, inside an 80-column
// screen.
var spyColumns = []tableColumn{
	{"Arrived", 13},
	{"Land", 12},
	{"Offense", 15},
	{"Defense", 15},
	{"Gold", 16},
}

// printSpyRow draws one Spy Database row: the first cell already laid out, then
// each figure right-justified in its column. Figures are bright cyan and a
// rise's "+" bright green, as the news screen colors a change: the sign carries
// the direction, so color is never the only cue.
func printSpyRow(s session.Session, t Term, first string, figures []string) {
	cells := []string{first}
	for i, f := range figures {
		cell := ansi.FgBrightCyan + gaFigure(f, spyColumns[i+1].width)
		if strings.HasPrefix(f, "+") {
			cell = strings.Replace(cell, "+", ansi.FgBrightGreen+"+"+ansi.FgBrightCyan, 1)
		}
		cells = append(cells, cell)
	}
	fmt.Fprintf(s, "%s%s\n", strings.TrimRight(strings.Join(cells, ansi.FgBrightBlack+gaSep), " "), ansi.Reset)
}

// spyReportsByRealm groups the database by realm, keeping each realm's reports
// in arrival order and the realms in the order they first appear.
func spyReportsByRealm(db []game.SpyEntry) [][]game.SpyEntry {
	type key struct{ board, empire string }
	at := map[key]int{}
	var out [][]game.SpyEntry
	for _, r := range db {
		k := key{r.Board, r.Empire}
		i, seen := at[k]
		if !seen {
			i = len(out)
			at[k] = i
			out = append(out, nil)
		}
		out[i] = append(out[i], r)
	}
	return out
}

// signed renders a change with its sign, so a rise and a fall read apart without
// relying on color.
func signed(n int64) string {
	if n > 0 {
		return "+" + comma(n)
	}
	return comma(n)
}

// terrorTarget is the realm the Terrorist Ops menu is aimed at.
type terrorTarget struct{ board, baron string }

// terroristOps is InterPlanetary item 2. It follows the original's order
// (launch_terrorist_operation, BRE.OVR 0x2afbf; captured in
// cap/eots-ibbs-02.cap): the refusals that need no target, then the planet,
// then the baron, and only then the ops menu, which runs against that one
// baron until the agents or the day's allowance are gone. Leaving the ops menu
// goes back to the baron prompt on the same planet, and Enter there leaves.
//
// Gold is not among the up-front refusals, though the original's is: IB
// offers the bank once the count is chosen, as it does everywhere else.
func terroristOps(ops *Menu) Action {
	return func(s session.Session, w *ctx) Result {
		if blockedByIPProtection(s, w) {
			return Stay
		}
		if w.Player().Agents < 1 {
			fail(s, game.ErrNoAgents)
			return Stay
		}
		var spent bool
		w.Read(func() { spent = !w.CanTerrorOp(w.Player()) })
		if spent {
			fail(s, game.ErrTerrorOpsExhausted)
			return Stay
		}
		board, barons := pickRemotePlanet(s, w, "Terrorize which planet?")
		if board == "" {
			return Stay
		}
		for terrorAgentsLeft(w) > 0 {
			baron := pickRemoteBaronFrom(s, w.Term, barons, tr(s, "Terrorize which baron?"), protectedNoStrike)
			if baron == "" {
				return Stay
			}
			w.terror = terrorTarget{board: board, baron: baron}
			err := Run(s, w, ops)
			w.terror = terrorTarget{}
			if err != nil {
				session.End(err)
			}
		}
		return Stay
	}
}

// terrorAgentsLeft is what the Terrorist Ops menu can still send; it closes
// the menu at zero, as the original's loop does (BRE.OVR 0x2afbf, the test
// after each send).
func terrorAgentsLeft(w *ctx) int {
	p := w.Player()
	if p == nil {
		return 0
	}
	return w.TerrorAgentsSendable(p)
}

// terrorOp is one item of the Terrorist Ops menu: send agents on op against
// the baron terroristOps chose.
func terrorOp(op game.TerrorOpType) Action {
	return func(s session.Session, w *ctx) Result {
		return sendTerrorOp(s, w, w.terror, op)
	}
}

// sendTerrorOp asks how many agents, confirms the price, and sends them. The
// strike resolves on the target board's next packet run.
func sendTerrorOp(s session.Session, w *ctx, t terrorTarget, op game.TerrorOpType) Result {
	most := terrorAgentsLeft(w)
	if most < 1 {
		return Back
	}
	// The original's own wording for the three lines of a dispatch — the count,
	// the price, and what went. They are prompts and labels rather than prose:
	// short, functional, and dictated by what is being asked (see AGENTS.md on
	// where that line falls). The suggested count is 1, as the original's
	// number reader returns its lower bound on Enter (056d:01bf → 0851:0bd9).
	agents := promptSuggested(s, "Send how many?", 1, most)
	if agents <= 0 {
		return Stay
	}
	// The whole charge is quoted only for two or more agents: the original
	// sends a lone agent without asking (BRE.OVR 0x2afbf, unit ovr_02aca8
	// +0x79f), its price being the one already on the InterPlanetary menu.
	cost := w.TerrorOpGoldCost(w.Player(), agents)
	if agents > 1 && !askYesNoHere(s, fmt.Sprintf(tr(s, "This will cost you %s gold.  Accept?"), comma(cost)), true) {
		return Stay
	}
	// Short: the bank is opened here rather than the send simply being
	// refused, and the refusal follows only if they come back no richer.
	if !affordOrBank(s, w, cost, game.ErrCantAfford) {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		var err error
		agents, err = w.World.SendTerror(p, t.board, t.baron, agents, op)
		return err
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	// No pause: the original goes straight back to the ops menu (capture).
	okNoPause(s, "%s %s sent out.", comma(agents), agentWord(s, agents))
	return Stay
}

// globalReconRequest is Coordinator Ops item 3: one scouting sweep of the whole
// league, answered with a report on every realm every other board holds. The
// answers land in the planet-wide Spy Database, so the sweep is the Coordinator
// spending their agent on everyone's behalf.
func globalReconRequest(s session.Session, w *ctx) Result {
	var boards int
	err := w.mutatePlayer(func(p *game.Empire) error {
		n, e := w.World.GlobalReconRequest(p)
		boards = n
		return e
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	if boards == 0 {
		ok(s, "No other planets are known yet, so there is nobody to scout.")
		return Stay
	}
	ok(s, "Recon requests created to all %d planets. The reports will reach the Spy Database.", boards)
	return Stay
}

// opPrice is a special operation's menu price: the op's fixed cost, from the
// same function the bill uses, so the column and the bill agree.
//
// Only the four bombing ops carry one. A missile's price depends on the target,
// which nobody has picked yet at the time the menu is drawn — which is exactly
// why the original leaves those three cells of its price column blank and quotes
// the figure after target selection instead.
func opPrice(op game.SpecialOp) func(*ctx) int {
	return func(*ctx) int { return int(game.SpecialOpGoldCost(op, 0)) }
}

// agentWord is "agent" or "agents", which the original picks the same way: the
// singular is the stem and the plural takes the "s".
func agentWord(s session.Session, n int) string {
	if n == 1 {
		return tr(s, "agent")
	}
	return tr(s, "agents")
}
