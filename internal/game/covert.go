package game

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// CovertOp names one item on the local Covert Operations menu. BRE keys BOTH the
// once-per-turn flag and the roll's difficulty divisor off the menu digit
// (`[es:di + 0xFD + digit]` and the divisor argument), so the two are held
// together on one type here rather than in parallel lists that could drift apart.
type CovertOp string

const (
	OpSendSpy            CovertOp = "Send Spy"
	OpStirRevolts        CovertOp = "Stir Revolts"
	OpSetUp              CovertOp = "Set Up"
	OpSupportDissensions CovertOp = "Support Dissensions"
	OpDemoralizeForces   CovertOp = "Demoralize Forces"
	OpSpyOnRelations     CovertOp = "Spy on Relations"
	OpBombEnemyTargets   CovertOp = "Bomb Enemy Targets"
	OpBribery            CovertOp = "Bribery"
)

// AllCovertOps is the whole set, in the order BRE's Covert Operations menu
// lists it. It is the canonical list: a screen that offers these operations
// names them through these constants rather than restating them as literals,
// because the same string is the CovertOpsUsed key the per-turn gate reads out
// of the save file (#208).
var AllCovertOps = []CovertOp{
	OpSendSpy,
	OpStirRevolts,
	OpSetUp,
	OpSupportDissensions,
	OpDemoralizeForces,
	OpSpyOnRelations,
	OpBombEnemyTargets,
	OpBribery,
}

// difficulty is the divisor this op applies to the attacker's own agent pool.
func (op CovertOp) difficulty() int {
	switch op {
	case OpDemoralizeForces:
		return CovertDifficultyDemoralizeForces
	case OpBombEnemyTargets:
		return CovertDifficultyBombEnemyTargets
	case OpBribery:
		return CovertDifficultyBribery
	case OpStirRevolts:
		return CovertDifficultyStirRevolts
	case OpSetUp:
		return CovertDifficultySetUp
	case OpSupportDissensions:
		return CovertDifficultySupportDissensions
	case OpSpyOnRelations:
		return CovertDifficultySpyOnRelations
	default:
		return CovertDifficultySendSpy
	}
}

// covertRoll is the one roll every local covert operation is resolved by, as BRE
// computes it (`send_spy`, BRE.OVR 0x04BA48) — including the defect that BOTH
// sides of the comparison are drawn from the ATTACKER. The bytes at 0x4BAE7 and
// 0x4BB03 are identical but for the mode argument, so the same empire is asked
// for its offensive pool and then its defensive one.
//
// The consequence is worth stating plainly, because it reads as a bug and is
// not: THE TARGET'S AGENTS NEVER ENTER THE ROLL. With no treaties in play the
// attacker's own count cancels, leaving `1/(1+difficulty)` plus the flat
// one-in-ten auto-success — 55% for an easy op however either realm is stocked.
// Agents defend against nothing local; what they buy is the attacking side of
// this roll once alliances are in play, and every other use the game makes of
// them.
//
// The order of the guards is BRE's own: a realm that has exposed you turns
// nine attempts in ten away before anything else is weighed, then the flat
// auto-success, then the pools.
func (w *World) covertRoll(a, d *Empire, op CovertOp) bool {
	if until, ok := d.ExposedFrom[a.Name]; ok && w.GameDay <= until {
		if w.rng.Intn(ExposeOpsSlipOdds) != 0 {
			return false
		}
	}
	if w.rng.Intn(CovertAutoSuccessOdds) == 0 {
		return true
	}
	offense := w.covertStrength(a, true) / op.difficulty()
	// A bribed agent inside the target is the ATTACKER's advantage: it doubles
	// the attacker's side of the roll, and buys the target nothing.
	if a.hasBribed(d.Name) {
		offense *= CovertBribeOffenseMultiplier
	}
	defense := w.covertStrength(a, false)
	total := offense + defense
	if total <= 0 {
		return false
	}
	return w.rng.Intn(total) < offense
}

// covertStrength is one side of a covert roll for e: its own agents plus a share
// of each ally's. An Intelligence Alliance lends to the attacking side, a
// Terrorist Prevention treaty to the defending one, and the two shares differ
// (BRE.OVR 0x4CAB7).
func (w *World) covertStrength(e *Empire, offense bool) int {
	if offense {
		return e.Agents + w.allyAgents(e, intelligenceAlliance)*CovertAllyOffensePct/100
	}
	return e.Agents + w.allyAgents(e, terroristPrevention)*CovertAllyDefensePct/100
}

// covertFoiled files the target's event for a covert operation that was caught,
// naming the realm that sent the agent. attempt names the operation as the
// target's own security would describe it ("a bribery attempt").
//
// BINARY-VERIFIED: BRE tells a target who came after it only when an agent is
// caught; an operation that succeeds stays anonymous. Three independent paths
// in the original agree, so the split is the rule rather than one screen's
// wording:
//
//   - the local Send Spy / Spy on Relations report (BRE.OVR 0x016d67) files an
//     event on the target naming the caller's realm on the caught branch, and
//     files nothing at all when the spy gets away;
//   - a received interplanetary agent packet (BRE.OVR 0x04a96b) counts the
//     agents that failed and, only when that count is non-zero, files a line
//     naming the sending realm and its planet — the per-operation lines the
//     target sees for the agents that DID get through carry no source at all;
//   - a received interplanetary bombing run (BRE.OVR 0x04a09a) fails two rolls
//     in three and, on failure, files the one line in its template that names
//     the source; the four success lines name nobody.
//
// The Expose Enemy Ops guard in covertRoll routes to this same branch, so a
// realm you have exposed hands you the attacker's name nine times in ten — which
// is most of what the shield buys.
func covertFoiled(a, d *Empire, attempt string) {
	d.addEvent(fmt.Sprintf("Your security foiled %s — the agent was in %s's pay.", attempt, a.Name))
}

// covertStatLoss docks points off a morale or support figure and holds the
// result at CovertStatFloor, which is what the local covert resolver does to
// both stats after every successful op.
func covertStatLoss(stat, loss int) int {
	return max(stat-loss, CovertStatFloor)
}

// dissensionsPct is the share of a victim's Troopers a successful Support
// Dissensions sends fleeing. BRE draws two independent rolls and subtracts one
// from the other, so the spread is wide (1-19%) around a tenth rather than flat.
func (w *World) dissensionsPct() int {
	return DissensionsPctBase + w.rng.Intn(DissensionsPctSpread) - w.rng.Intn(DissensionsPctSpread)
}

// ErrCovertCapReached is returned when an EFFECT covert op is attempted a second
// time in one turn (BRE: "Limit one try per turn!"). BRE keeps the flag in a
// per-digit byte (`[es:di + 0xFD + digit]`, written at BRE.OVR 0x017C4F and read
// at 0x017AE0), so the limit is one try of EACH operation, not one operation
// overall. Info ops (Send Spy, Spy on Relations) are exempt: the menu skips the
// check for digits 1 and 6, and their path never sets the byte.
var ErrCovertCapReached = errors.New("You may run each covert operation only once per turn.")

// covertCost gates a covert op: the attacker must hold at least one agent and
// enough gold for the op's fee, which is charged up front (BRE charges per op).
// When capped, it also enforces BRE's once-per-turn limit on that ONE operation,
// marks it used, and SPENDS the agent — the three things BRE's commit_agent does
// together (BRE.OVR 0x01793C sets the per-digit byte, then decrements the agent
// count at +0x26F) before the record is queued. A successful operation hands the
// agent back when it resolves (covertReturned), so the net cost is still one
// agent per failure; between the two the attacker is genuinely one short.
//
// The agent check comes first so a broke-but-agentless caller still sees
// ErrNoAgents. No state changes when it returns an error.
func (w *World) covertCost(a *Empire, op CovertOp, cost int64, capped bool) error {
	if a.Agents < 1 {
		return ErrNoAgents
	}
	if capped && a.TurnProgress.CovertOpsUsed[op] {
		return ErrCovertCapReached
	}
	if a.Gold < cost {
		return ErrCantAfford
	}
	a.Gold -= cost
	if capped {
		if a.TurnProgress.CovertOpsUsed == nil {
			a.TurnProgress.CovertOpsUsed = make(map[CovertOp]bool, 1)
		}
		a.TurnProgress.CovertOpsUsed[op] = true
		a.Agents--
	}
	return nil
}

// SendSpy gathers military intel on d. Needs at least one agent. On failure
// the agent is caught (lost) and the victim is alerted.
func (w *World) SendSpy(a, d *Empire) (string, error) {
	if err := w.covertCost(a, OpSendSpy, CostSendSpy, false); err != nil {
		return "", err
	}
	if w.covertRoll(a, d, OpSendSpy) {
		return fmt.Sprintf("Intel on %s — Land %d, Troops %d, Turrets %d, Tanks %d, Offense %d, Defense %d, Gold %d, Agents %d",
			d.Name, d.Land, d.Troopers, d.Turrets, d.Tanks, d.Offense(), d.Defense(), d.Gold, d.Agents), nil
	}
	a.Agents--
	covertFoiled(a, d, "a spying attempt")
	return "Your spy was caught and did not return.", nil
}

// SupportDissensions agitates d's own troopers into fleeing. Queued; see
// resolveSupportDissensions for what happens when the agent gets there.
func (w *World) SupportDissensions(a, d *Empire) (string, error) {
	if err := w.covertCost(a, OpSupportDissensions, CostSupportDissensions, true); err != nil {
		return "", err
	}
	w.queueCovertOp(a, d, OpSupportDissensions, "")
	return covertSent(d), nil
}

// resolveSupportDissensions is the queued operation arriving. A successful op is
// anonymous; a caught agent gives the attacker away (see covertFoiled). The
// share that flees is BRE's own two-roll spread (dissensionsPct), which averages
// a tenth but ranges from a scratch to a fifth.
func (w *World) resolveSupportDissensions(a, d *Empire) string {
	if !w.covertRoll(a, d, OpSupportDissensions) {
		covertFoiled(a, d, "a sabotage attempt")
		return "The operation failed and your agent was lost."
	}
	covertReturned(a)
	lost := d.Troopers * w.dissensionsPct() / 100
	d.Troopers -= lost
	d.addEvent(fmt.Sprintf("Saboteurs struck your army — %d troopers lost.", lost))
	return fmt.Sprintf("Your agents sowed dissension in %s: %d troopers eliminated.", d.Name, lost)
}

// DemoralizeForces lowers d's military morale on success, weakening combat and
// risking desertion (see moraleFactor and moraleDesertion). Queued, which is
// what stops it being a pre-battle move: the agent lands at maintenance, so the
// attack it softens is the one you make the day after.
func (w *World) DemoralizeForces(a, d *Empire) (string, error) {
	if err := w.covertCost(a, OpDemoralizeForces, CostDemoralizeForces, true); err != nil {
		return "", err
	}
	w.queueCovertOp(a, d, OpDemoralizeForces, "")
	return covertSent(d), nil
}

// resolveDemoralizeForces is the queued operation arriving. The local resolver
// docks a few POINTS and holds the stat at CovertStatFloor; the x6/7 scaling IB
// used before is the inter-BBS packet resolver's figure, read against the wrong
// op enumeration.
func (w *World) resolveDemoralizeForces(a, d *Empire) string {
	if !w.covertRoll(a, d, OpDemoralizeForces) {
		covertFoiled(a, d, "an attempt to demoralize your forces")
		return "The operation failed and your agent was lost."
	}
	covertReturned(a)
	d.Morale = covertStatLoss(d.Morale, DemoralizeLossBase+w.rng.Intn(DemoralizeLossSpread))
	d.addEvent("Agents demoralized your forces — morale fell.")
	return fmt.Sprintf("You demoralized %s's forces, lowering their morale.", d.Name)
}

// SetUp tricks d and one of its treaty partners into believing the other
// declared war, voiding EVERY treaty between them — useful against a defense
// pact protecting a target you want to attack. Queued; the second court is
// chosen here and travels in the record, as BRE's menu picks it before it hands
// the record over (the digit-'3' branch at BRE.OVR 0x0175C2 fills that byte
// beside the target's).
func (w *World) SetUp(a, d *Empire) (string, error) {
	if err := w.covertCost(a, OpSetUp, CostSetUp, true); err != nil {
		return "", err
	}
	partner := ""
	if p := w.setUpPartner(a, d); p != nil {
		partner = p.Name
	}
	w.queueCovertOp(a, d, OpSetUp, partner)
	return covertSent(d), nil
}

// resolveSetUp is the queued operation arriving. The agent has to reach both
// courts, so it rolls once against each; either miss loses it. A partner that
// died between the queue and here leaves nothing to unravel, and the operation
// is rolled against the target alone for the agent's sake.
func (w *World) resolveSetUp(a, d, partner *Empire) string {
	if partner == nil || !partner.Alive {
		if !w.covertRoll(a, d, OpSetUp) {
			covertFoiled(a, d, "an attempt to set you up")
			return "The operation failed and your agent was lost."
		}
		covertReturned(a)
		return fmt.Sprintf("%s holds no treaty for us to unravel.", d.Name)
	}
	if w.covertRoll(a, d, OpSetUp) && w.covertRoll(a, partner, OpSetUp) {
		covertReturned(a)
		voided := w.TreatiesBetween(d, partner)
		for _, tt := range voided {
			w.BreakTreaty(d, partner, tt)
		}
		note := fmt.Sprintf("Agents tricked you and %s into believing you had declared war — every treaty between you is void.", partner.Name)
		d.addEvent(note)
		partner.addEvent(fmt.Sprintf("Agents tricked you and %s into believing you had declared war — every treaty between you is void.", d.Name))
		return fmt.Sprintf("You tricked %s and %s into voiding %d treaties.", d.Name, partner.Name, len(voided))
	}
	covertFoiled(a, d, "an attempt to set you up")
	return "The operation failed and your agent was lost."
}

// setUpPartner picks the realm to turn against d: the one whose pact with d
// most protects it, preferring a Full Defense Alliance. The attacker is never
// picked — setting a realm against yourself buys nothing.
func (w *World) setUpPartner(a, d *Empire) *Empire {
	var any *Empire
	for _, other := range w.Empires {
		if other == d || other == a || !other.Alive {
			continue
		}
		treaties := w.TreatiesBetween(d, other)
		if len(treaties) == 0 {
			continue
		}
		for _, tt := range treaties {
			if tt == fullDefenseAlliance {
				return other
			}
		}
		if any == nil {
			any = other
		}
	}
	return any
}

// ErrNoBribedAgent is returned when Expose Enemy Ops is aimed at a realm the
// caller holds no bribed agent inside. BRE's screen lists only the realms you
// have bribed, so there is nothing else to aim it at.
var ErrNoBribedAgent = errors.New("You hold no bribed agent inside that realm.")

// ErrNoBribedAgents is returned when the caller holds no bribed agent anywhere,
// so Expose Enemy Ops has an empty list and nothing to do.
var ErrNoBribedAgents = errors.New("You hold no bribed agents to work through.")

// ExposeEnemyOps turns your bribed agent inside d against its own service: for
// ExposeOpsShieldDays it turns away nine of every ten covert operations d sends
// at you (covertRoll), and the ones it turns away name d as their source.
//
// BINARY-VERIFIED (`bribe_enemy_agents`, BRE.OVR 0x01701B). Three things about
// it are not what IB assumed, and all three are now matched:
//
//   - it shields against ONE realm, chosen from the realms you already hold a
//     bribed agent inside — never against everyone;
//   - it spends NO agent and takes no once-per-turn slot. The menu dispatches
//     digit 9 before it reaches either (BRE.OVR 0x017AB0), so this is the one
//     covert item you can run repeatedly in a turn;
//   - the block is not absolute: one attempt in ExposeOpsSlipOdds still lands.
//
// The one thing NOT matched is an off-by-one in the original (BRE.OVR 0x0172D4):
// BRE writes the expiry into the slot for the LAST realm letter its listing loop
// touched — always 'Y', the 25th — rather than the realm the player picked, so
// the shield lands where it was aimed only on a board with 25 realms in it.
// Reproducing that needs a permanent 25-entry letter table; IB's roster is
// unbounded and its letters are per-screen, so the same index would hit an
// arbitrary innocent realm instead of missing harmlessly. Reproducing the
// OUTCOME instead — charge the fee, shield nobody — would strand the verified
// half of the routine, since nothing else writes ExposedFrom. The reasoning is
// laid out in docs/mechanics-reference.md; IB shields the realm you picked.
func (w *World) ExposeEnemyOps(a, d *Empire) (string, error) {
	if !a.hasBribed(d.Name) {
		return "", ErrNoBribedAgent
	}
	// No agent is spent and no turn slot is taken; the fee is the whole cost.
	// BRE checks it too, in the menu rather than in this routine: the fee for
	// the pressed key is weighed against the caller's gold at BRE.OVR 0x01775C
	// ("Sorry!  You cannot afford that!") and digit 9 is dispatched after that
	// gate, so gold never goes negative there either.
	if a.Gold < CostExposeEnemyOps {
		return "", ErrCantAfford
	}
	a.Gold -= CostExposeEnemyOps
	if a.ExposedFrom == nil {
		a.ExposedFrom = make(map[string]int, 1)
	}
	a.ExposedFrom[d.Name] = w.GameDay + ExposeOpsShieldDays
	return fmt.Sprintf("Your agent inside %s will expose their operations against you for the next day.", d.Name), nil
}

// hasBribed reports whether e holds a bribed agent inside the named realm.
func (e *Empire) hasBribed(name string) bool {
	return slices.Contains(e.Bribed, name)
}

// BribedRealms lists the realms e holds a bribed agent inside that are still
// alive, in world order — the realms Expose Enemy Ops can be aimed at.
func (w *World) BribedRealms(e *Empire) []*Empire {
	var out []*Empire
	for _, other := range w.Empires {
		if other != e && other.Alive && e.hasBribed(other.Name) {
			out = append(out, other)
		}
	}
	return out
}

// SpyOnRelations reveals every treaty d holds with other empires — useful
// pre-war intelligence on alliance networks and trade partners. On failure the
// agent is lost.
//
// DELIBERATE DIVERGENCE: it costs the CostSpyOnRelations its menu advertises.
// BRE advertises the same 100,000 and gates on it, then charges 5,000 —
// report_spy_result serves both info ops and subtracts the slot-'1' Send Spy fee
// whichever one called it (BRE.OVR 0x016E73). IB charges the advertised price on
// purpose; do not "correct" this to the Send Spy fee.
func (w *World) SpyOnRelations(a, d *Empire) (string, error) {
	if err := w.covertCost(a, OpSpyOnRelations, CostSpyOnRelations, false); err != nil {
		return "", err
	}
	if w.covertRoll(a, d, OpSpyOnRelations) {
		var lines []string
		for _, other := range w.Empires {
			if other == d || !other.Alive {
				continue
			}
			for _, tt := range w.TreatiesBetween(d, other) {
				lines = append(lines, fmt.Sprintf("  %s with %s: %s", d.Name, other.Name, tt))
			}
		}
		if len(lines) == 0 {
			return fmt.Sprintf("%s holds no treaties.", d.Name), nil
		}
		return fmt.Sprintf("Treaties of %s:\n%s", d.Name, strings.Join(lines, "\n")), nil
	}
	a.Agents--
	covertFoiled(a, d, "an attempt to spy on your relations")
	return "Your spy was caught and did not return.", nil
}

// Bribery buys an agent inside d over to your side. The bought agent is an
// OFFENSIVE holding: it doubles your own side of every later covert roll against
// d (covertRoll), and it is what Expose Enemy Ops needs in place before it can
// shield you from that realm. On failure your own agent is lost.
//
// BINARY-VERIFIED (BRE.OVR 0x04BA48, at +0x165): the flag is read from the
// ATTACKER's record indexed by the target, and doubles the attacker's numerator.
// IB read the same flag backwards until this was checked, as a shield that made
// the bribed realm's ops against YOU fail — which is not a thing BRE does.
func (w *World) Bribery(a, d *Empire) (string, error) {
	// BRE refuses the op at the menu when a bribed agent is already in place,
	// before it charges the fee or spends the try (BRE.OVR 0x01790A), so a
	// second attempt costs nothing.
	if a.hasBribed(d.Name) {
		return "", fmt.Errorf("You already hold a bribed agent inside %s.", d.Name)
	}
	if err := w.covertCost(a, OpBribery, CostBribery, true); err != nil {
		return "", err
	}
	w.queueCovertOp(a, d, OpBribery, "")
	return covertSent(d), nil
}

// resolveBribery is the queued operation arriving. Because the bribe only lands
// at maintenance, Expose Enemy Ops — which needs the agent already in place —
// cannot be run against that realm until the day after the bribe was paid for.
func (w *World) resolveBribery(a, d *Empire) string {
	if !w.covertRoll(a, d, OpBribery) {
		covertFoiled(a, d, "a bribery attempt")
		return "The operation failed and your agent was lost."
	}
	covertReturned(a)
	if !a.hasBribed(d.Name) {
		a.Bribed = append(a.Bribed, d.Name)
	}
	d.addEvent("A rival power bribed one of your agents.")
	return fmt.Sprintf("You bribed an agent in %s. Your operations against them are twice as likely to land.", d.Name)
}

// StirRevolts spreads propaganda that lowers d's popular support (rioting and
// revolt), weakening its economy and its troopers. Queued.
func (w *World) StirRevolts(a, d *Empire) (string, error) {
	if err := w.covertCost(a, OpStirRevolts, CostStirRevolts, true); err != nil {
		return "", err
	}
	w.queueCovertOp(a, d, OpStirRevolts, "")
	return covertSent(d), nil
}

// resolveStirRevolts is the queued operation arriving. Support is docked in
// points and floored, as Demoralize Forces is.
func (w *World) resolveStirRevolts(a, d *Empire) string {
	if !w.covertRoll(a, d, OpStirRevolts) {
		covertFoiled(a, d, "an agitation attempt")
		return "The operation failed and your agent was lost."
	}
	covertReturned(a)
	d.Support = covertStatLoss(d.Support, StirRevoltsLossBase+w.rng.Intn(StirRevoltsLossSpread))
	d.addEvent("Agitators stirred revolts — your popular support fell.")
	return fmt.Sprintf("You stirred revolts in %s, lowering its popular support.", d.Name)
}

// BombingBombersRequired is the Bombers an empire must hold to run an
// interplanetary Special Operation (BRE.OVR: "All missiles and bombs require
// 500 Bombers to deliver their payloads"). The gate — and the 500 Bombers each
// launch consumes — belong to `run_bombing_operations_menu` (BRE.OVR 0x02AEBE
// onward), which only the InterPlanetary menu reaches. The LOCAL covert op has
// no bomber requirement at all.
const BombingBombersRequired = 500

// bombTarget is one holding a Bomb Enemy Targets strike can find, with the
// percentage band that holding loses.
type bombTarget struct {
	name     string
	base     int
	spread   int
	get      func(*Empire) int
	set      func(*Empire, int)
	roundsUp bool
}

// bombGood is a target that is one of the canonical goods (#134), so its name
// and its accessor come from the table rather than being spelled out again.
func bombGood(g *Good, base, spread int, roundsUp bool) bombTarget {
	return bombTarget{g.Plural, base, spread,
		func(e *Empire) int { return *g.Count(e) },
		func(e *Empire, v int) { *g.Count(e) = v }, roundsUp}
}

// bombTargets is BRE's six-slot table, in its roll order: Random(6)+1 indexes
// straight into it. The player picks nothing.
func bombTargets() []bombTarget {
	return []bombTarget{
		{"People", BombTargetPeoplePctBase, BombTargetPeoplePctSpread,
			func(e *Empire) int { return e.People }, func(e *Empire, v int) { e.People = v }, false},
		bombGood(Trooper, BombTargetTrooperPctBase, BombTargetTrooperPctSpread, false),
		bombGood(Agent, BombTargetAgentPctBase, BombTargetAgentPctSpread, false),
		bombGood(Tank, BombTargetTankPctBase, BombTargetTankPctSpread, false),
		bombGood(Jet, BombTargetJetPctBase, BombTargetJetPctSpread, false),
		bombGood(Food, BombTargetFoodPctBase, BombTargetFoodPctSpread, true),
	}
}

// BombEnemyTargets is BRE's single local terrorism op: on success the agency
// picks ONE of six holdings at random and destroys a slice of it. Neither side
// chooses the target, which is what the original's "randomly bomb targets"
// means. Queued.
func (w *World) BombEnemyTargets(a, d *Empire) (string, error) {
	if err := w.covertCost(a, OpBombEnemyTargets, CostBombEnemyTargets, true); err != nil {
		return "", err
	}
	w.queueCovertOp(a, d, OpBombEnemyTargets, "")
	return covertSent(d), nil
}

// resolveBombEnemyTargets is the queued operation arriving.
func (w *World) resolveBombEnemyTargets(a, d *Empire) string {
	if !w.covertRoll(a, d, OpBombEnemyTargets) {
		covertFoiled(a, d, "a terror bombing")
		return "The operation failed and your agent was lost."
	}
	covertReturned(a)
	t := bombTargets()[w.rng.Intn(BombTargetPickCount)]
	pct := t.base + w.rng.Intn(t.spread)
	held := t.get(d)
	lost := held * pct / 100
	// Food is the one slot BRE rounds rather than truncates (BRE.OVR 0x04C6C6
	// calls the rounding helper where the other five call Trunc).
	if t.roundsUp && held*pct%100 >= 50 {
		lost++
	}
	if lost <= 0 {
		return fmt.Sprintf("Your agents found nothing worth bombing in %s.", d.Name)
	}
	t.set(d, held-lost)
	d.addEvent(fmt.Sprintf("Terrorists bombed targets across your realm — %d %s destroyed.", lost, t.name))
	return fmt.Sprintf("Your agents bombed targets in %s: %d %s destroyed.", d.Name, lost, t.name)
}

// S3-Sabre tuning. In BRE only 3 of the missile's 11 dial settings
// (1, 2, 3) did anything and the rest fizzled; the manual never said which
// number did what. IB keeps the unpredictability but makes it honest: the dial
// (0-10) is a bluff that changes nothing — every launch is the same random
// gamble. Only about SabreEffectHits launches in SabreEffectRange
// (3 in 10) deliver a payload; the rest fizzle. The sysop's None handling mode
// disables the weapon entirely (gated in the menu).
const (
	SabreEffectHits    = 3   // landing launches per SabreEffectRange...
	SabreEffectRange   = 10  // ...i.e. a 3-in-10 chance to deliver a payload
	SabreBaseDamagePct = 5   // a landed hit always removes at least this %
	SabreDamageSpread  = 26  // random % headroom on top of the base (5-30% total)
	SabreBackfireScale = 200 // target Troopers / this = backfire chance (percent)
	SabreMultiHitOdds  = 10  // 1-in-this a hit strafes several assets at once
)

// sabreResource pairs a strikeable field with its display name. BRE hid
// which asset each effect hit, so IB picks its own spread of targets. Land and
// Gold are not in this list — Land has to move through the RegionMix and Gold is
// held in money width — so sabreDamage handles both beside the loop.
type sabreResource struct {
	name string
	val  *int
}

// sabreResources are the plain-count assets the missile can hit, taken
// from the canonical table (#134) rather than listed again here. Land and gold
// are handled apart by sabreDamage — one goes through the RegionMix and
// the other is money-width.
func sabreResources(e *Empire) []sabreResource {
	res := make([]sabreResource, 0, len(AllGoods))
	for _, g := range AllGoods {
		res = append(res, sabreResource{g.Plural, g.Count(e)})
	}
	return res
}

// sabreDamage applies a landed S3-Sabre hit to e and returns a
// human-readable list of what was destroyed (empty if the roll removed
// nothing). Each hit removes a random 5-30% of one asset; usually a single
// asset, but occasionally the missile strafes several at once — BRE's
// "extremely devastating" outcome. Land is one of the targets, but must be
// removed through the RegionMix (whose Total must always equal e.Land) rather
// than by touching e.Land directly.
func (w *World) sabreDamage(e *Empire) string {
	res := sabreResources(e)
	landIdx := len(res)    // one past the plain-int assets: Land
	goldIdx := landIdx + 1 // and Gold, in money width
	hits := 1
	if w.rng.Intn(SabreMultiHitOdds) == 0 {
		hits = 2 + w.rng.Intn(3) // 2-4 assets at once
	}
	seen := make(map[int]bool, hits)
	var parts []string
	for k := 0; k < hits; k++ {
		i := w.rng.Intn(goldIdx + 1)
		if seen[i] {
			continue
		}
		seen[i] = true
		pct := SabreBaseDamagePct + w.rng.Intn(SabreDamageSpread)
		if i == landIdx {
			lost := e.Land * pct / 100
			if lost <= 0 {
				continue
			}
			e.Regions.remove(lost)
			e.syncLand()
			parts = append(parts, fmt.Sprintf("%d Regions", lost))
			continue
		}
		if i == goldIdx {
			lost := pctOf(e.Gold, pct)
			if lost <= 0 {
				continue
			}
			e.Gold -= lost
			parts = append(parts, fmt.Sprintf("%d Gold", lost))
			continue
		}
		lost := pctOf(*res[i].val, pct)
		if lost <= 0 {
			continue
		}
		*res[i].val -= lost
		parts = append(parts, fmt.Sprintf("%d %s", lost, res[i].name))
	}
	return strings.Join(parts, ", ")
}

// sabreBackfires reports whether the missile turns back on whoever
// fired it: the more Troopers the target garrisons, the likelier it does.
func (w *World) sabreBackfires(d *Empire) bool {
	return w.rng.Intn(100) < d.Troopers/SabreBackfireScale
}

// sabreEffect is the target-side half of the missile, for a strike that
// arrived from another planet (#49). It runs the same shield, fizzle and
// backfire rolls the local strike runs, in the same order.
//
// A backfire cannot be applied here — the realm it would hurt is on the board
// that fired — so it is reported instead, and the launching board takes the
// damage when the answer gets home. That is the one thing the two versions do
// differently, and the delay is the packet's, not a rule of its own.
//
// This is the one covert event in this file that names the source on SUCCESS,
// and it is deliberate: BRE treats a Sabre that lands from another planet as a
// missile impact rather than an agent op and reports it with the firing realm
// and its planet, the same as an incoming nuclear or chemical strike. Agent ops
// stay anonymous unless the agent is caught (see covertFoiled).
func (w *World) sabreEffect(d *Empire, from string) (report string, hit, backfired bool) {
	if w.rng.Intn(100)*100 <= d.SDI*SDIMissileInterceptPct {
		return fmt.Sprintf("%s's SDI intercepted your S3-Sabre.", d.Name), false, false
	}
	if w.rng.Intn(SabreEffectRange) >= SabreEffectHits {
		return "The S3-Sabre fizzled and did no damage.", false, false
	}
	if w.sabreBackfires(d) {
		return "The S3-Sabre backfired on the way out!", false, true
	}
	lost := w.sabreDamage(d)
	if lost == "" {
		return fmt.Sprintf("Your S3-Sabre reached %s but did negligible damage.", d.Name), false, false
	}
	d.addEvent(fmt.Sprintf("An S3-Sabre from %s struck your empire — lost %s.", from, lost))
	return fmt.Sprintf("Your S3-Sabre hit %s: %s destroyed.", d.Name, lost), true, false
}

// The effects the local Bomb Enemy Targets ops and their interplanetary
// counterparts share (#49). Each one is the part of an op that touches the
// TARGET only — no attacker, no success roll, no fee — which is exactly the
// part a board can run for a strike that arrived in a packet. Keeping them here
// rather than duplicating the arithmetic is what stops the two menus drifting
// apart when a number is tuned.

// bombFoodEffect burns half of d's food reserve and reports what was lost.
func bombFoodEffect(d *Empire) int {
	lost := d.Food / 2
	d.Food -= lost
	return lost
}

// bombRoutesLands reports whether a trade-route strike comes to anything at
// all. BINARY-VERIFIED (BRE.OVR 0x04a09a): the original rolls this once at the
// top of the routine that resolves a received bombing op, before it looks at a
// single deal, and two strikes in three end there.
func (w *World) bombRoutesLands() bool {
	return w.rng.Intn(BombRoutesLandOdds) == 0
}

// bombRoutesEffect wrecks the goods riding in pending trade deals and reports
// how many deals it hit: every deal on the planet when only is nil, otherwise
// every deal `only` is a party to, whichever side of it that realm is on.
//
// BINARY-VERIFIED against BRE.OVR 0x051077, which walks the pending deals and,
// for each, rolls a `random(3)` that lets one deal in three escape, skips a deal
// whose own two parties hold Protective Trade, and otherwise cuts every one of
// its goods quantities to bombRoutesKeptPct. Nothing is refunded and no
// per-deal message is filed.
//
// The Protective Trade guard is a property of the DEAL, not of the attacker: it
// reads the relation between the deal's sender and recipient (record fields +8
// and +9), never the firing realm's own. So a deal between a pair who hold the
// pact survives a strike from anyone, and holding it with the victim buys the
// attacker nothing.
//
// Scope is IB's own call, not BRE's. The original's Bomb Trade Routes is
// interplanetary and planet-wide, so it never had to say which deals a strike
// against ONE realm reaches, and its local Covert item 7 is a different op
// entirely. IB takes both sides of a deal — a realm's trade routes run in both
// directions, and counting only inbound deals would let a realm dodge the op by
// never accepting one. The local covert op and its interplanetary counterpart
// call this same helper, so neither menu can become the cheaper way to do it.
func (w *World) bombRoutesEffect(only *Empire) (hit int) {
	for _, to := range w.Empires {
		if !to.Alive {
			continue
		}
		for i := range to.TradeDeals {
			deal := &to.TradeDeals[i]
			from := w.FindByName(deal.From)
			if only != nil && to != only && from != only {
				continue
			}
			if w.rng.Intn(BombRoutesDealHitOdds) != 0 {
				continue
			}
			if from != nil && w.HasTreaty(from, to, protectiveTrade) {
				continue
			}
			w.bombDealBasket(&deal.Send)
			w.bombDealBasket(&deal.Demand)
			hit++
		}
	}
	return hit
}

// bombDealBasket cuts every good in b to the sliver a bombed deal keeps. The
// share is rolled per good, as the original rolls it inside its own loop over
// the deal's quantities.
func (w *World) bombDealBasket(b *TradeBasket) {
	for _, p := range basketPtrs(b) {
		*p = pctOf(*p, w.bombRoutesKeptPct())
	}
	b.Gold = pctOf(b.Gold, w.bombRoutesKeptPct())
}

// bombRoutesKeptPct is the percentage of one good a bombed deal keeps: 5-9%.
func (w *World) bombRoutesKeptPct() int {
	return BombRoutesKeptPctMin + w.rng.Intn(BombRoutesKeptPctSpread)
}

// undermineEffect trims a quarter off the principal and matching return of each
// of d's pending investments, and reports the principal lost. Zero means d had
// nothing invested.
func undermineEffect(d *Empire) int64 {
	var lost int64
	for i := range d.Investments {
		cut := d.Investments[i].Amount / UndermineInvestmentDivisor
		d.Investments[i].Amount -= cut
		d.Investments[i].Return -= cut
		lost += cut
	}
	return lost
}
