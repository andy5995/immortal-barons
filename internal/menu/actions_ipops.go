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

// sendSpyGuy is the Special Operations "Send SpyGuy" item: post a watcher on
// another PLANET for a paid number of days. He is not a covert agent — no agent
// is spent, he cannot be caught, and he brings back no intelligence. What he
// does is send word home the moment his hosts aim a group attack or a Clingy
// Annihilator at his own planet, and that word arrives as planet news there.
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
	var maxDays int
	w.With(func() {
		planets = w.KnownBoards()
		perDay = w.SpyGuyCostPerDay()
		maxDays = w.SpyGuyDaysAffordable(w.Player())
	})
	if len(planets) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	// The price is quoted before the target is picked, as BRE quotes it: it is
	// the same on every planet, being drawn from the sender's own size.
	fmt.Fprintf(s, "\n%s"+tr(s, "A SpyGuy costs %s%s%s gold per day.")+"%s\n",
		ansi.FgWhite, ansi.FgBrightCyan, comma(perDay), ansi.FgWhite, ansi.Reset)
	if maxDays < 1 {
		fail(s, game.ErrCantAfford)
		return Stay
	}
	board := pickAddressee(s, w, planets)
	if board == "" {
		return Stay
	}
	suggested := game.SpyGuyDefaultDays
	if suggested > maxDays {
		suggested = maxDays
	}
	days := promptSuggested(s, "How many days would you like him to remain?", suggested, maxDays)
	if days < 1 {
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
		// The R5-Slappenheimer's handling mode is the sysop's, and this is the
		// only menu that fires one: BRE's S3-Sabre is an interplanetary Special
		// Operation, so the local Covert menu never had it.
		if op == game.OpSlappenheimer {
			var mode game.SlappenheimerMode
			w.With(func() { mode = w.Config.SlappenheimerHandling })
			if mode == game.SlappenheimerNone {
				ok(s, "The R5-Slappenheimer is disabled.")
				return Stay
			}
			// Under User Select handling the player dials the missile in (0-10).
			// The dial is BRE's bluff — it changes nothing about the outcome —
			// but we still prompt for it to keep the original's feel.
			if mode == game.SlappenheimerUserSelect {
				promptInt(s, "Set the R5-Slappenheimer dial (0-10)")
			}
		}
		label := game.SpecialOpLabel(op)
		var board, baron string
		if op.TargetsPlanet() {
			// The four bombing ops wreck what the whole planet shares — its food
			// market, its trading market, its bank — so there is no baron to
			// name and asking for one would be asking a question with no answer.
			var boards []string
			w.With(func() {
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
				fmt.Sprintf(tr(s, "%s against which baron?"), label), hostile)
			if !found {
				return Stay
			}
		}
		cost := w.World.SpecialOpGoldCost(w.Player(), op)
		okNoPause(s, "This operation will cost %s gold.", comma(cost))
		if !askYesNoHere(s, "Send this Operation?", true) {
			return Stay
		}
		err := w.mutatePlayer(func(p *game.Empire) error {
			return w.World.SendSpecialOp(p, board, baron, op)
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
	board, baron, _, found := pickRemoteTarget(s, w, "Terrorize which planet?", "Terrorize which baron?", hostile)
	if !found {
		return Stay
	}
	agents := promptSuggested(s, "How many agents to send?", w.Player().Agents, w.Player().Agents)
	if agents <= 0 {
		return Stay
	}
	// BRE prices the op on the menu itself; quote it here too, since the price
	// climbs with the launcher's own region count and is easy to be surprised by.
	okNoPause(s, "This operation will cost %s gold.", comma(w.TerrorOpGoldCost(w.Player())))
	if !askYesNoHere(s, "Send this Operation?", true) {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		return w.World.SendTerror(p, board, baron, agents, op)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Your %s agents depart for %s on %s.", op, baron, board)
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
