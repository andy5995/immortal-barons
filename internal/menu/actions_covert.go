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
// operation costs no gold, no agent and no per-turn slot. IB keeps that
// placement.
func blockedByCovertProtection(s session.Session, w *ctx) bool {
	if w.Player().Protection > 0 {
		ok(s, "Your New Realm Protection shelters you, and while it lasts your agents may not move against another realm.")
		return true
	}
	return false
}

// covertOp runs a covert operation that has an effect on its target, refusing
// it while the caller is sheltered.
func covertOp(s session.Session, w *ctx, label string, strike func(a, d *game.Empire) (string, error)) Result {
	if blockedByCovertProtection(s, w) {
		return Stay
	}
	return pickAndStrike(s, w, label, nil, false, strike)
}

// covertInfoOp runs one of the two operations that only gather information.
// The original's protection test is jumped over for exactly these two — the
// menu digits 1 and 6 — so a sheltered realm can still look before it can
// touch.
func covertInfoOp(s session.Session, w *ctx, label string, strike func(a, d *game.Empire) (string, error)) Result {
	return pickAndStrike(s, w, label, nil, false, strike)
}

func sendSpy(s session.Session, w *ctx) Result {
	return covertInfoOp(s, w, "Send Spy", func(a, d *game.Empire) (string, error) { return w.SendSpy(a, d) })
}

func supportDissensions(s session.Session, w *ctx) Result {
	return covertOp(s, w, "Support Dissensions", func(a, d *game.Empire) (string, error) { return w.SupportDissensions(a, d) })
}

func demoralizeForces(s session.Session, w *ctx) Result {
	return covertOp(s, w, "Demoralize Forces", func(a, d *game.Empire) (string, error) { return w.DemoralizeForces(a, d) })
}

func setUp(s session.Session, w *ctx) Result {
	return covertOp(s, w, "Set Up", func(a, d *game.Empire) (string, error) { return w.SetUp(a, d) })
}

// exposeEnemyOps aims the shield at ONE realm, chosen from the realms the
// player already holds a bribed agent inside — BRE lists no others, because the
// bribed agent is what does the exposing. It does not go through localAttack:
// that lists every living rival and withholds a selection letter from realms
// under New Realm Protection or in an alliance, neither of which has anything to
// do with an agent already on the payroll.
func exposeEnemyOps(s session.Session, w *ctx) Result {
	if blockedByCovertProtection(s, w) {
		return Stay
	}
	rows := snapshotBribedTargets(w)
	if len(rows) == 0 {
		fail(s, game.ErrNoBribedAgents)
		return Stay
	}
	name, chosen := pickAttackTarget(s, rows, tr(s, "Whose operations should your agent expose (letter, RETURN to abort)"))
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
// player holds a bribed agent inside, every one of them selectable.
func snapshotBribedTargets(w *ctx) []targetRow {
	var rows []targetRow
	w.With(func() {
		p := w.Player()
		if p == nil {
			return
		}
		for _, e := range w.BribedRealms(p) {
			rows = append(rows, targetRow{
				name: e.Name, land: e.Land, score: e.Score, netWorth: w.NetWorth(e),
				people: e.People, troopers: e.Troopers,
				attackable: true, online: e.Online(),
			})
		}
	})
	return rows
}

func stirRevolts(s session.Session, w *ctx) Result {
	return covertOp(s, w, "Stir Revolts", func(a, d *game.Empire) (string, error) { return w.StirRevolts(a, d) })
}

func bombEnemyTargets(s session.Session, w *ctx) Result {
	return covertOp(s, w, "Bomb Enemy Targets", func(a, d *game.Empire) (string, error) { return w.BombEnemyTargets(a, d) })
}

func spyRelations(s session.Session, w *ctx) Result {
	return covertInfoOp(s, w, "Spy on Relations", func(a, d *game.Empire) (string, error) { return w.SpyOnRelations(a, d) })
}

func briberyOp(s session.Session, w *ctx) Result {
	return covertOp(s, w, "Bribery", func(a, d *game.Empire) (string, error) { return w.Bribery(a, d) })
}
