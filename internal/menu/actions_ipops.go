package menu

import (
	"fmt"

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
		if !w.turnPlayed {
			ok(s, "You must play at least one turn each entry into the game before you can use this option.")
			return Stay
		}
		return do(s, w)
	}
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
	if blockedByCovertProtection(s, w) {
		return Stay
	}
	var planets []string
	var perDay int64
	w.Read(func() {
		planets = w.KnownBoards()
		perDay = w.SpyGuyCostPerDay()
	})
	if len(planets) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
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
	// offers the length and deals with the money afterwards, where the bank is.
	days := promptSuggested(s, "How many days would you like him to remain?",
		game.SpyGuyDefaultDays, game.SpyGuyMaxDays)
	if days < 1 {
		return Stay
	}
	// The same helper every other op on this menu uses: the refusal, the bank,
	// and then the stay if they came back with the gold for it.
	if !affordOrBank(s, w, perDay*int64(days), game.ErrCantAffordOp) {
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

// ipSpecialOp drives every item on the interplanetary Special Operations menu
// (#49): choose a target, quote the price, confirm, and send. What "a target"
// means depends on the op — a planet for the four bombing ops, a named baron for
// the three missiles — which is why the choice branches below.
//
// The strike itself happens on the target's board when the packet lands, so the
// screen promises a report rather than printing an outcome — the same shape as
// Terrorist Ops above, and for the same reason.
func ipSpecialOp(op game.SpecialOp) func(session.Session, *ctx) Result {
	return func(s session.Session, w *ctx) Result {
		if blockedByProtection(s, w) {
			return Stay
		}
		// Checked before a planet is picked so a baron who cannot deliver a
		// payload is not walked through choosing a target first.
		if w.Player().Bombers < game.BombingBombersRequired {
			fail(s, game.ErrNeedBombers)
			return Stay
		}
		// The S3-Sabre's handling mode is the sysop's, and this is the only
		// menu that fires one: in BRE the missile is an interplanetary Special
		// Operation, so the local Covert menu never had it.
		dial := 0
		if op == game.OpSabre {
			var mode game.SabreMode
			w.Read(func() { mode = w.Config.SabreHandling })
			switch mode {
			case game.SabreNone:
				ok(s, "The S3-Sabre is disabled.")
				return Stay
			case game.SabreUserSelect:
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
			w.Read(func() {
				for _, b := range w.World.RemoteBoards {
					boards = append(boards, b.BoardID)
				}
			})
			if len(boards) == 0 {
				ok(s, "No other planets are known yet.")
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
		cost := w.World.SpecialOpGoldCost(w.Player(), op)
		okNoPause(s, "This operation will cost %s gold.", comma(cost))
		if !askYesNoHere(s, "Send this Operation?", true) {
			return Stay
		}
		// Accepted but short: the refusal, then the bank, rather than the send
		// simply failing at a price the player has just agreed to.
		if !affordOrBank(s, w, cost, game.ErrCantAffordOp) {
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
// Operations → Send SpyGuy, matching BRE).
func spyDatabase(s session.Session, w *ctx) Result {
	if len(w.SpyDatabase) == 0 {
		ok(s, "The spy database is empty. Spy on empires on other planets to fill it.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Spy Database:"), ansi.Reset)
	for _, r := range w.SpyDatabase {
		fmt.Fprintf(s, "  "+tr(s, "%s @ %s (%s): Land %s  Off %s  Def %s  Gold %s")+"\n",
			r.Empire, r.Board, r.Date, comma(r.Land), comma(r.Offense), comma(r.Defense), comma(r.Gold))
	}
	pause(s)
	return Stay
}

// terrorOp returns a handler that sends agents to perform a specific terror
// sub-operation on an enemy baron on another planet. All nine sub-ops share the
// same mechanical effect (each agent destroys 1/7 of a random unit type) but BRE
// carries the op type in the packet so the result report can name it.
func terrorOp(op game.TerrorOpType) Action {
	return func(s session.Session, w *ctx) Result {
		return doTerrorOp(s, w, op)
	}
}

// doTerrorOp is the shared implementation behind the Terrorist Ops submenu.
// The strike is queued and resolves on the target board's next packet run; New
// Realm Protection blocks it.
func doTerrorOp(s session.Session, w *ctx, op game.TerrorOpType) Result {
	if blockedByProtection(s, w) {
		return Stay
	}
	if w.Player().Agents < 1 {
		fail(s, game.ErrNoAgents)
		return Stay
	}
	board, baron, _, found := pickRemoteTarget(s, w, "Terrorize which planet?", "Terrorize which baron?")
	if !found {
		return Stay
	}
	// Each agent is one operation, so the maximum the prompt offers is the day's
	// remaining ALLOWANCE, not the agents held — the original counts it down,
	// `(1; 15)` then `(1; 7)` after eight have gone. Whichever of the two runs
	// out first bounds it.
	most := w.Player().Agents
	if left := w.TerrorOpsLeft(w.Player()); left > 0 && left < most {
		most = left
	}
	if most < 1 {
		fail(s, game.ErrTerrorOpsExhausted)
		return Stay
	}
	// The original's own wording for the three lines of a dispatch — the count,
	// the price, and what went. They are prompts and labels rather than prose:
	// short, functional, and dictated by what is being asked (see AGENTS.md on
	// where that line falls).
	agents := promptSuggested(s, "Send how many?", most, most)
	if agents <= 0 {
		return Stay
	}
	// BRE prices the op on the menu itself and quotes the whole charge here too,
	// since it climbs with the launcher's own region count and with the ops
	// already sent today, and is easy to be surprised by.
	cost := w.TerrorOpGoldCost(w.Player(), agents)
	if !askYesNoHere(s, fmt.Sprintf(tr(s, "This will cost you %s gold.  Accept?"), comma(cost)), true) {
		return Stay
	}
	// Accepted but short: the bank is opened here rather than the send simply
	// being refused, and the refusal follows only if they come back no richer.
	if !affordOrBank(s, w, cost, game.ErrCantAffordOp) {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		return w.World.SendTerror(p, board, baron, agents, op)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "%s %s sent out.", comma(agents), agentWord(s, agents))
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

// opPrice is a special operation's menu price: what this board would charge the
// caller today, sysop cost dial included, so the column and the bill agree.
func opPrice(op game.SpecialOp) func(*ctx) int {
	return func(w *ctx) int {
		p := w.Player()
		if p == nil {
			return 0
		}
		return int(w.SpecialOpGoldCost(p, op))
	}
}

// agentWord is "agent" or "agents", which the original picks the same way: the
// singular is the stem and the plural takes the "s".
func agentWord(s session.Session, n int) string {
	if n == 1 {
		return tr(s, "agent")
	}
	return tr(s, "agents")
}
