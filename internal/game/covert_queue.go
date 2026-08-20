package game

import "fmt"

// QueuedCovertOp is one covert agent in the field: an operation the menu has
// accepted and paid for, waiting on the next daily maintenance to resolve.
//
// BINARY-VERIFIED. BRE's local Covert Operations menu does not resolve an effect
// operation at all. `commit_agent` (BRE.OVR 0x01793C) sets the once-per-turn
// byte, decrements the attacker's agents, fills a 19-byte record and hands it to
// the queue writer at `BRE.OVR 0x04CA06`, which copies 0x13 bytes and appends it
// to the game's typed record list as type 7. `run_daily_maintenance` then calls
// `run_ai_covert_operations` (BRE.OVR 0x04BE9F) — a misnomer: it walks that list,
// takes every type-7 record and is the only resolver the local menu reaches. The
// list lives in the game data file (its helpers share a unit with
// `open_and_validate_game_data`), which it must: BRE's maintenance is a separate
// program run, so the record has to survive the door session that made it.
//
// BRE's own manual says as much from the other side — it singles Send Spy out
// with "This operation takes place immediately", which the five other effect
// operations are not told.
//
// The fields mirror the 19 bytes the original writes, minus the two 32-bit
// player IDs it uses to re-identify realms across a slot being reused: Attacker
// and Target are record +0x0C and +0x0D (the realm letters), Partner is the
// second court Set Up needs, and Op is the menu digit at +0x0F.
type QueuedCovertOp struct {
	Attacker string
	Target   string
	// Partner is Set Up's second court, chosen when the operation is queued.
	// Empty for every other operation.
	Partner string `json:",omitempty"`
	Op      CovertOp
	// Day is the game day the agent set out, kept for the recap line.
	Day int
}

// queueCovertOp puts an agent in the field. The fee, the agent and the
// once-per-turn slot have already been taken by covertCost; nothing else happens
// until resolveCovertQueue runs.
func (w *World) queueCovertOp(a, d *Empire, op CovertOp, partner string) {
	w.CovertQueue = append(w.CovertQueue, QueuedCovertOp{
		Attacker: a.Name, Target: d.Name, Partner: partner, Op: op, Day: w.GameDay,
	})
}

// covertSent is what the attacker is told at the menu, in place of a result: the
// agent has gone, and the outcome arrives on the recap after maintenance.
func covertSent(op CovertOp, d *Empire) string {
	return fmt.Sprintf("Your agent has set out for %s. Word of the %s will reach you on the next day's report.", d.Name, op)
}

// resolveCovertQueue drains every agent in the field, as BRE's daily maintenance
// does. The queue is taken whole and emptied first, so an operation that files
// events cannot re-enter the list it is being read from.
//
// A record whose attacker is gone is discarded silently — there is nobody left
// to report to. A record whose TARGET is gone is reported to the attacker and
// nothing else: BRE re-checks the target's stored player ID against the realm
// sitting in that slot and abandons the operation when they differ (BRE.OVR
// 0x04BF3D-0x04BF6A, which jumps clear of the whole resolution), and it refunds
// nothing. IB has no fixed letter slots to check, so it checks that the realm is
// still alive, and files a line rather than leaving the fee unexplained.
func (w *World) resolveCovertQueue() {
	queue := w.CovertQueue
	w.CovertQueue = nil
	for _, rec := range queue {
		a := w.FindByName(rec.Attacker)
		if a == nil || !a.Alive {
			continue
		}
		d := w.FindByName(rec.Target)
		if d == nil || !d.Alive {
			a.addEvent(fmt.Sprintf("Your agent reached %s to find the realm gone; the %s was abandoned.", rec.Target, rec.Op))
			continue
		}
		a.addEvent(w.resolveCovertOp(a, d, rec))
	}
}

// resolveCovertOp runs one queued operation and returns the attacker's report.
// The dispatch is on the operation, as BRE's resolver dispatches on the menu
// digit it stored (BRE.OVR 0x04BFD4 onward).
func (w *World) resolveCovertOp(a, d *Empire, rec QueuedCovertOp) string {
	switch rec.Op {
	case OpStirRevolts:
		return w.resolveStirRevolts(a, d)
	case OpSetUp:
		return w.resolveSetUp(a, d, w.FindByName(rec.Partner))
	case OpSupportDissensions:
		return w.resolveSupportDissensions(a, d)
	case OpDemoralizeForces:
		return w.resolveDemoralizeForces(a, d)
	case OpBombEnemyTargets:
		return w.resolveBombEnemyTargets(a, d)
	case OpBribery:
		return w.resolveBribery(a, d)
	}
	// An op this build does not know — a queue record written by a version that
	// spelled it differently. The agent is still gone, so the fee is still spent,
	// and a blank line on the recap would leave that unexplained.
	return fmt.Sprintf("Your agent's orders against %s could not be carried out.", d.Name)
}

// covertReturned hands the attacker its agent back, which is what every
// successful branch of BRE's resolver does (`agents += 1` at BRE.OVR 0x04C69D
// and its five siblings). The agent was spent when the operation was queued, so
// over the pair the attacker loses one only when the operation fails.
func covertReturned(a *Empire) {
	a.Agents++
}
