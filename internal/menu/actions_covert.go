package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// blockedByCovertProtection reports whether the player's own New Realm
// Protection bars them from an operation that acts on another realm, printing
// the refusal when it does. The reason is the reverse of the one an attack
// prints: there the shield is the TARGET's, here it is the caller's own, and
// the two must not read alike.
//
// The original tests this at the menu, immediately before the affordability
// gate (`BRE.OVR 0x017716`, refusal string loaded at `0x01772E`), so a refused
// operation costs no gold, no agent and none of the day’s allowance. IB keeps that
// placement.
func blockedByCovertProtection(s session.Session, w *ctx) bool {
	if w.Player().Protection > 0 {
		ok(s, "Your New Realm Protection shelters you, and while it lasts your agents may not move against another realm.")
		return true
	}
	return false
}

// covertRow is one operation on the local Covert Operations menu: the typed op
// the engine keys its daily allowance off, the hotkey and fee the screen
// draws, and the resolver behind it. The menu's label is string(Op), so a
// screen cannot name an operation except through the constant — the same string
// is the CovertOpsToday key persisted in the save file, and a menu literal that
// drifted from it wrote a key nothing read, silently retiring the gate
// (#208). This is the units.go treatment (#134) applied to the covert set.
type covertRow struct {
	Op   game.CovertOp
	Key  rune
	Cost int
	// Info marks the two operations that only gather information — the menu
	// digits 1 and 6, which the original jumps its protection test over, so a
	// sheltered realm can still look before it can touch.
	Info   bool
	Strike func(w *ctx, a, d *game.Empire) (string, error)
}

// covertRows are the eight operations, in BRE's menu order. Expose Enemy Ops is
// deliberately absent: it takes no allowance and no CovertOp, picks from its
// own list of bribed realms, and is wired by hand in tree.go.
var covertRows = []covertRow{
	{Op: game.OpSendSpy, Key: '1', Cost: game.CostSendSpy, Info: true,
		Strike: func(w *ctx, a, d *game.Empire) (string, error) { return w.SendSpy(a, d) }},
	{Op: game.OpStirRevolts, Key: '2', Cost: game.CostStirRevolts,
		Strike: func(w *ctx, a, d *game.Empire) (string, error) { return w.StirRevolts(a, d) }},
	{Op: game.OpSetUp, Key: '3', Cost: game.CostSetUp,
		Strike: func(w *ctx, a, d *game.Empire) (string, error) { return w.SetUp(a, d) }},
	{Op: game.OpSupportDissensions, Key: '4', Cost: game.CostSupportDissensions,
		Strike: func(w *ctx, a, d *game.Empire) (string, error) { return w.SupportDissensions(a, d) }},
	{Op: game.OpDemoralizeForces, Key: '5', Cost: game.CostDemoralizeForces,
		Strike: func(w *ctx, a, d *game.Empire) (string, error) { return w.DemoralizeForces(a, d) }},
	{Op: game.OpSpyOnRelations, Key: '6', Cost: game.CostSpyOnRelations, Info: true,
		Strike: func(w *ctx, a, d *game.Empire) (string, error) { return w.SpyOnRelations(a, d) }},
	{Op: game.OpBombEnemyTargets, Key: '7', Cost: game.CostBombEnemyTargets,
		Strike: func(w *ctx, a, d *game.Empire) (string, error) { return w.BombEnemyTargets(a, d) }},
	{Op: game.OpBribery, Key: '8', Cost: game.CostBribery,
		Strike: func(w *ctx, a, d *game.Empire) (string, error) { return w.Bribery(a, d) }},
}

// action is the menu Action for this operation: the caller's own New Realm
// Protection gate (except for the two info ops), then the shared target picker.
//
// An EFFECT op then asks how many agents to send, as the interplanetary
// Terrorist Ops screen does, and sends that many at the one target. The
// allowance being a day's rather than a turn's is worth nothing if the only way
// to spend it is to walk back through the menu for each agent (#257's sibling
// complaint) — and nothing on the screen would say the allowance existed.
//
// The two INFO ops are unchanged: they are uncapped, they answer immediately,
// and a second look at the same realm in the same breath tells you nothing.
func (row covertRow) action() Action {
	return func(s session.Session, w *ctx) Result {
		if row.Info {
			return pickAndStrike(s, w, string(row.Op), nil, false, false,
				func(a, d *game.Empire) (string, error) { return row.Strike(w, a, d) })
		}
		if blockedByCovertProtection(s, w) {
			return Stay
		}
		return sendAgents(s, w, row)
	}
}

// agentsAvailable is how many agents the player could send on this operation
// right now: the agents held, what today's allowance still permits, and what the
// gold in hand can pay the fee for, whichever runs out first.
func agentsAvailable(w *ctx, row covertRow) int {
	p := w.Player()
	n := min(p.Agents, w.CovertOpsLeft(p, row.Op))
	if row.Cost > 0 {
		n = min(n, int(p.Gold/int64(row.Cost)))
	}
	return max(n, 0)
}

// sendAgents picks the target, asks how many agents to send, and sends them one
// at a time — each is its own operation with its own fee, its own agent and its
// own roll when it lands, exactly as sending them one per visit to the menu
// would be. The count is a convenience, not a new mechanic.
func sendAgents(s session.Session, w *ctx, row covertRow) Result {
	rows := warTargets.rows(w)
	if len(rows) == 0 {
		ok(s, "There are no rival empires left to attack.")
		return Stay
	}
	name, chosen := pickAttackTarget(s, w.Term, rows,
		warTargets.prompts(tr(s, "Choose a target (letter, RETURN to abort)")))
	if !chosen {
		return Stay
	}
	// A pact is no shelter from an agent, in the original or here: the same
	// question the war menu asks, and the same charge for answering yes.
	if !confirmBreach(s, w, rows, name) {
		return Stay
	}
	// Asking before the count is what makes the refusal say which of the three
	// ran out, rather than offering a prompt whose only honest answer is zero.
	avail := agentsAvailable(w, row)
	if avail < 1 {
		p := w.Player()
		switch {
		case p.Agents < 1:
			fail(s, game.ErrNoAgents)
		case w.CovertOpsLeft(p, row.Op) < 1:
			fail(s, game.ErrCovertCapReached)
		default:
			fail(s, game.ErrCantAfford)
		}
		return Stay
	}
	n := promptSuggested(s, "How many agents to send?", avail, avail)
	if n <= 0 {
		return Stay
	}

	var report string
	var sent int
	var err error
	for ; sent < n; sent++ {
		e := w.mutatePlayer(func(p *game.Empire) error {
			d := warTargets.find(w, p, name)
			if d == nil {
				return errTargetGone
			}
			var inner error
			report, inner = row.Strike(w, p, d)
			return inner
		})
		if e != nil {
			err = e
			break
		}
	}
	// Whatever went, went: a run stopped part way by a price rise or a realm
	// that died mid-send still sent the agents ahead of it, so the count is
	// reported rather than the whole thing failing.
	if sent == 0 {
		fail(s, err)
		return Stay
	}
	if sent > 1 {
		report = fmt.Sprintf(tr(s, "%d agents have set out for %s."), sent, name)
	}
	fmt.Fprintf(s, "\n%s\n", hiNums(wrapReport(report)))
	if err != nil {
		fail(s, err)
	}
	pause(s)
	return Stay
}

// covertAction is the menu action for op. It panics on an op with no row, which
// only an edit to covertRows can cause.
func covertAction(op game.CovertOp) Action {
	for _, row := range covertRows {
		if row.Op == op {
			return row.action()
		}
	}
	panic("no covert menu row for " + string(op))
}

// exposeEnemyOps aims the shield at ONE realm, chosen from the realms the
// player already holds a bribed agent inside — BRE lists no others, because the
// bribed agent is what does the exposing. It does not go through localAttack:
// that lists every living rival and refuses realms under New Realm Protection or
// in an alliance, neither of which has anything to do with an agent already on
// the payroll.
func exposeEnemyOps(s session.Session, w *ctx) Result {
	if blockedByCovertProtection(s, w) {
		return Stay
	}
	rows := snapshotBribedTargets(w)
	if len(rows) == 0 {
		fail(s, game.ErrNoBribedAgents)
		return Stay
	}
	name, chosen := pickAttackTarget(s, w.Term, rows, warTargets.prompts(tr(s, "Whose operations should your agent expose (letter, RETURN to abort)")))
	if !chosen {
		return Stay
	}
	var report string
	err := w.mutatePlayer(func(p *game.Empire) error {
		d := w.FindByName(name)
		if d == nil || !d.Alive {
			return errTargetGone
		}
		var e error
		report, e = w.ExposeEnemyOps(p, d)
		return e
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", hiNums(wrapReport(report)))
	pause(s)
	return Stay
}

// snapshotBribedTargets is the Expose Enemy Ops list: the living realms the
// player holds a bribed agent inside, every one of them selectable — an agent
// already inside is not stopped by the target's own shield. The flag is still
// drawn, because it is a fact about the realm.
func snapshotBribedTargets(w *ctx) []targetRow {
	var rows []targetRow
	w.Read(func() {
		p := w.Player()
		if p == nil {
			return
		}
		for _, e := range w.BribedRealms(p) {
			rows = append(rows, targetRow{
				// The letter is the realm's permanent slot, as everywhere else — this
				// picker is newer than the slot work and was still lettering by row.
				name: e.Name, letter: e.Letter(),
				land: e.Land, score: e.Score, netWorth: w.NetWorth(e),
				people: e.People, troopers: e.Troopers,
				attackable: true, protected: e.Protection > 0,
				presence: presenceOf(w, e, false, w.Today),
			})
		}
	})
	return rows
}
