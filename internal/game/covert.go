package game

import (
	"errors"
	"fmt"
	"slices"
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
// because the same string is the CovertOpsToday key the per-day gate reads out
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
//
// The line is one entry of the caught-agent pool, picked here and used on both
// sides: the target's event, and the returned report the caller reads.
func (w *World) covertFoiled(a, d *Empire, attempt Msg) Msg {
	fate := w.pickAgentCaught()
	d.addEvent(say(fate.Theirs, "who", say("an agent behind {attempt}, in {who}'s pay", "attempt", attempt, "who", a.Name)))
	return say(fate.Yours)
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

// ErrCovertCapReached is returned when an EFFECT covert op has already been run
// its allowed number of times today. The allowance is per OPERATION and equals
// Config.TurnsPerDay, so a day holds as many Stir Revolts as it holds turns, and
// as many Set Ups beside them. Info ops (Send Spy, Spy on Relations) are exempt,
// as they are in the original.
//
// BRE spells the same ceiling as one try of each operation per TURN, keyed by a
// per-digit byte (`[es:di + 0xFD + digit]`, written at BRE.OVR 0x017C4F and read
// at 0x017AE0). With no banked turns the two come to the same number of
// operations a day; counting the day lets a baron send them when it suits rather
// than one per visit to the menu. See Empire.CovertOpsToday.
var ErrCovertCapReached = errors.New("You have run that covert operation as often as you may today.")

// CovertOpsAllowed is the number of times one EFFECT covert operation may be run
// per day: one for each turn of the day, which is what BRE's per-turn gate came
// to. A TurnsPerDay of zero or less means no cap, the underDailyCap convention.
func (w *World) CovertOpsAllowed() int { return w.Config.TurnsPerDay }

// CanRunCovertOp reports whether a still has an allowance left for op today.
// Info ops are uncapped and always report true.
func (w *World) CanRunCovertOp(a *Empire, op CovertOp) bool {
	return underDailyCap(a.CovertOpsToday[op], w.CovertOpsAllowed())
}

// CovertOpsLeft is how many more times a may run op today. It is what the menu
// suggests when it asks how many agents to send, alongside the agents held and
// the gold in hand. An allowance of zero or less is no cap, and reports the
// agents held instead — the only other thing bounding a send.
func (w *World) CovertOpsLeft(a *Empire, op CovertOp) int {
	allowed := w.CovertOpsAllowed()
	if allowed <= 0 {
		return a.Agents
	}
	return max(allowed-a.CovertOpsToday[op], 0)
}

// covertCost gates a covert op: the attacker must hold at least one agent and
// enough gold for the op's fee, which is charged up front (BRE charges per op).
// When capped, it also enforces the day's allowance for that ONE operation,
// counts the run, and SPENDS the agent — the three things BRE's commit_agent does
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
	if capped && !w.CanRunCovertOp(a, op) {
		return ErrCovertCapReached
	}
	if a.Gold < cost {
		return ErrCantAfford
	}
	a.Gold -= cost
	if capped {
		if a.CovertOpsToday == nil {
			a.CovertOpsToday = make(map[CovertOp]int, 1)
		}
		a.CovertOpsToday[op]++
		a.Agents--
	}
	return nil
}

// SendSpy gathers military intel on d. Needs at least one agent. On failure
// the agent is caught (lost) and the victim is alerted.
func (w *World) SendSpy(a, d *Empire) (Msg, error) {
	if err := w.covertCost(a, OpSendSpy, CostSendSpy, false); err != nil {
		return Msg{}, err
	}
	if w.covertRoll(a, d, OpSendSpy) {
		return say("Intel on {who} — Land {land}, Troops {troops}, Turrets {turrets}, Tanks {tanks}, Offense {offense}, Defense {defense}, Gold {gold}, Agents {agents}",
			"who", d.Name, "land", d.Land, "troops", d.Troopers, "turrets", d.Turrets, "tanks", d.Tanks,
			"offense", d.Offense(), "defense", d.Defense(), "gold", d.Gold, "agents", d.Agents), nil
	}
	a.Agents--
	return w.covertFoiled(a, d, say("a spying attempt")), nil
}

// SupportDissensions agitates d's own troopers into fleeing. Queued; see
// resolveSupportDissensions for what happens when the agent gets there.
func (w *World) SupportDissensions(a, d *Empire) (Msg, error) {
	if err := w.covertCost(a, OpSupportDissensions, CostSupportDissensions, true); err != nil {
		return Msg{}, err
	}
	w.queueCovertOp(a, d, OpSupportDissensions, "")
	return covertSent(d), nil
}

// resolveSupportDissensions is the queued operation arriving. A successful op is
// anonymous; a caught agent gives the attacker away (see covertFoiled). The
// share that flees is BRE's own two-roll spread (dissensionsPct), which averages
// a tenth but ranges from a scratch to a fifth.
func (w *World) resolveSupportDissensions(a, d *Empire) Msg {
	if !w.covertRoll(a, d, OpSupportDissensions) {
		return w.covertFoiled(a, d, say("a sabotage attempt"))
	}
	covertReturned(a)
	lost := pctOf(d.Troopers, w.dissensionsPct())
	d.Troopers -= lost
	d.addEvent(say("Agitators stirred dissent in your army, and {n} troopers deserted.", "n", lost))
	return say("Your agents stirred dissent in {who}'s army, and {n} troopers deserted.", "who", d.Name, "n", lost)
}

// DemoralizeForces lowers d's military morale on success, weakening combat and
// risking desertion (see moraleFactor and moraleDesertion). Queued, which is
// what stops it being a pre-battle move: the agent lands at maintenance, so the
// attack it softens is the one you make the day after.
func (w *World) DemoralizeForces(a, d *Empire) (Msg, error) {
	if err := w.covertCost(a, OpDemoralizeForces, CostDemoralizeForces, true); err != nil {
		return Msg{}, err
	}
	w.queueCovertOp(a, d, OpDemoralizeForces, "")
	return covertSent(d), nil
}

// resolveDemoralizeForces is the queued operation arriving. The local resolver
// docks a few POINTS and holds the stat at CovertStatFloor; the x6/7 scaling IB
// used before is the inter-BBS packet resolver's figure, read against the wrong
// op enumeration.
func (w *World) resolveDemoralizeForces(a, d *Empire) Msg {
	if !w.covertRoll(a, d, OpDemoralizeForces) {
		return w.covertFoiled(a, d, say("an attempt to demoralize your forces"))
	}
	covertReturned(a)
	d.Morale = covertStatLoss(d.Morale, DemoralizeLossBase+w.rng.Intn(DemoralizeLossSpread))
	d.addEvent(say("Agents found your forces' weaknesses, and their self-esteem dropped."))
	return say("Your agents found weaknesses and caused {who}'s self-esteem to drop.", "who", d.Name)
}

// SetUp tricks d and one of its treaty partners into believing the other
// declared war, voiding EVERY treaty between them — useful against a defense
// pact protecting a target you want to attack. Queued; the second court is
// chosen here and travels in the record, as BRE's menu picks it before it hands
// the record over (the digit-'3' branch at BRE.OVR 0x0175C2 fills that byte
// beside the target's).
func (w *World) SetUp(a, d *Empire) (Msg, error) {
	if err := w.covertCost(a, OpSetUp, CostSetUp, true); err != nil {
		return Msg{}, err
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
func (w *World) resolveSetUp(a, d, partner *Empire) Msg {
	if partner == nil || !partner.Alive {
		if !w.covertRoll(a, d, OpSetUp) {
			return w.covertFoiled(a, d, say("an attempt to set you up"))
		}
		covertReturned(a)
		return say("{who} holds no treaty for you to unravel.", "who", d.Name)
	}
	if w.covertRoll(a, d, OpSetUp) && w.covertRoll(a, partner, OpSetUp) {
		covertReturned(a)
		voided := w.TreatiesBetween(d, partner)
		for _, tt := range voided {
			w.BreakTreaty(d, partner, tt)
		}
		forged := msgid("Forged papers convinced you and {who} that war had been declared; every treaty between you is void.")
		d.addEvent(say(forged, "who", partner.Name))
		partner.addEvent(say(forged, "who", d.Name))
		return say("Your forged declaration of war fooled {a} and {b} into tearing up {n} treaties.",
			"a", d.Name, "b", partner.Name, "n", len(voided))
	}
	return w.covertFoiled(a, d, say("an attempt to set you up"))
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
func (w *World) ExposeEnemyOps(a, d *Empire) (Msg, error) {
	if !a.hasBribed(d.Name) {
		return Msg{}, ErrNoBribedAgent
	}
	// No agent is spent and no turn slot is taken; the fee is the whole cost.
	// BRE checks it too, in the menu rather than in this routine: the fee for
	// the pressed key is weighed against the caller's gold at BRE.OVR 0x01775C
	// ("Sorry!  You cannot afford that!") and digit 9 is dispatched after that
	// gate, so gold never goes negative there either.
	if a.Gold < CostExposeEnemyOps {
		return Msg{}, ErrCantAfford
	}
	a.Gold -= CostExposeEnemyOps
	if a.ExposedFrom == nil {
		a.ExposedFrom = make(map[string]int, 1)
	}
	a.ExposedFrom[d.Name] = w.GameDay + ExposeOpsShieldDays
	return say("Your agent inside {who} will report on their operations against you for the next day.", "who", d.Name), nil
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
func (w *World) SpyOnRelations(a, d *Empire) (Msg, error) {
	if err := w.covertCost(a, OpSpyOnRelations, CostSpyOnRelations, false); err != nil {
		return Msg{}, err
	}
	if w.covertRoll(a, d, OpSpyOnRelations) {
		rows := []Msg{say("Treaties of {who}:", "who", d.Name)}
		for _, other := range w.Empires {
			if other == d || !other.Alive {
				continue
			}
			for _, tt := range w.TreatiesBetween(d, other) {
				rows = append(rows, say("  {who} with {other}: {treaty}", "who", d.Name, "other", other.Name, "treaty", say(tt)))
			}
		}
		if len(rows) == 1 {
			return say("{who} holds no treaties.", "who", d.Name), nil
		}
		return Msg{R: rows}, nil
	}
	a.Agents--
	return w.covertFoiled(a, d, say("an attempt to spy on your relations")), nil
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
func (w *World) Bribery(a, d *Empire) (Msg, error) {
	// BRE refuses the op at the menu when a bribed agent is already in place,
	// before it charges the fee or spends the try (BRE.OVR 0x01790A), so a
	// second attempt costs nothing.
	if a.hasBribed(d.Name) {
		return Msg{}, fmt.Errorf("You already hold a bribed agent inside %s.", d.Name)
	}
	if err := w.covertCost(a, OpBribery, CostBribery, true); err != nil {
		return Msg{}, err
	}
	w.queueCovertOp(a, d, OpBribery, "")
	return covertSent(d), nil
}

// resolveBribery is the queued operation arriving. Because the bribe only lands
// at maintenance, Expose Enemy Ops — which needs the agent already in place —
// cannot be run against that realm until the day after the bribe was paid for.
func (w *World) resolveBribery(a, d *Empire) Msg {
	if !w.covertRoll(a, d, OpBribery) {
		return w.covertFoiled(a, d, say("a bribery attempt"))
	}
	covertReturned(a)
	if !a.hasBribed(d.Name) {
		a.Bribed = append(a.Bribed, d.Name)
	}
	d.addEvent(say("One of your agents took a rival's bribe."))
	return say("You bribed one of {who}'s agents, so your operations against them now land more often.", "who", d.Name)
}

// StirRevolts spreads propaganda that lowers d's popular support (rioting and
// revolt), weakening its economy and its troopers. Queued.
func (w *World) StirRevolts(a, d *Empire) (Msg, error) {
	if err := w.covertCost(a, OpStirRevolts, CostStirRevolts, true); err != nil {
		return Msg{}, err
	}
	w.queueCovertOp(a, d, OpStirRevolts, "")
	return covertSent(d), nil
}

// resolveStirRevolts is the queued operation arriving. Support is docked in
// points and floored, as Demoralize Forces is.
func (w *World) resolveStirRevolts(a, d *Empire) Msg {
	if !w.covertRoll(a, d, OpStirRevolts) {
		return w.covertFoiled(a, d, say("an agitation attempt"))
	}
	covertReturned(a)
	d.Support = covertStatLoss(d.Support, StirRevoltsLossBase+w.rng.Intn(StirRevoltsLossSpread))
	d.addEvent(say("Agitators stirred revolts in your realm, and your popular support fell."))
	return say("Your agitators stirred revolts in {who}, and their popular support fell.", "who", d.Name)
}

// bombTarget is one holding a Bomb Enemy Targets strike can find, with the
// percentage band that holding loses.
//
// theirs and yours are the two sides' lines when it is hit, {n} the count lost
// and {who} the target.
type bombTarget struct {
	theirs   string
	yours    string
	base     int
	spread   int
	get      func(*Empire) int
	set      func(*Empire, int)
	roundsUp bool
}

// bombGood is a target that is one of the canonical goods (#134), so its name
// and its accessor come from the table rather than being spelled out again.
func bombGood(g *Good, theirs, yours string, base, spread int, roundsUp bool) bombTarget {
	return bombTarget{theirs, yours, base, spread,
		func(e *Empire) int { return *g.Count(e) },
		func(e *Empire, v int) { *g.Count(e) = v }, roundsUp}
}

// bombTargets is BRE's six-slot table, in its roll order: Random(6)+1 indexes
// straight into it. The player picks nothing.
func bombTargets() []bombTarget {
	return []bombTarget{
		{msgid("Terrorists bombed your people and destroyed {n} of them."),
			msgid("Your agents bombed {who}'s people, destroying {n} of them."),
			BombTargetPeoplePctBase, BombTargetPeoplePctSpread,
			func(e *Empire) int { return e.People }, func(e *Empire, v int) { e.People = v }, false},
		bombGood(Trooper, msgid("Terrorists bombed your troopers and destroyed {n} of them."),
			msgid("Your agents bombed {who}'s troopers, destroying {n} of them."),
			BombTargetTrooperPctBase, BombTargetTrooperPctSpread, false),
		bombGood(Agent, msgid("Terrorists bombed your agents and destroyed {n} of them."),
			msgid("Your agents bombed {who}'s agents, destroying {n} of them."),
			BombTargetAgentPctBase, BombTargetAgentPctSpread, false),
		bombGood(Tank, msgid("Terrorists bombed your tanks and destroyed {n} of them."),
			msgid("Your agents bombed {who}'s tanks, destroying {n} of them."),
			BombTargetTankPctBase, BombTargetTankPctSpread, false),
		bombGood(Jet, msgid("Terrorists bombed your jets and destroyed {n} of them."),
			msgid("Your agents bombed {who}'s jets, destroying {n} of them."),
			BombTargetJetPctBase, BombTargetJetPctSpread, false),
		bombGood(Food, msgid("Terrorists bombed your food and destroyed {n} of them."),
			msgid("Your agents bombed {who}'s food, destroying {n} of them."),
			BombTargetFoodPctBase, BombTargetFoodPctSpread, true),
	}
}

// BombEnemyTargets is BRE's single local terrorism op: on success the agency
// picks ONE of six holdings at random and destroys a slice of it. Neither side
// chooses the target, which is what the original's "randomly bomb targets"
// means. Queued.
func (w *World) BombEnemyTargets(a, d *Empire) (Msg, error) {
	if err := w.covertCost(a, OpBombEnemyTargets, CostBombEnemyTargets, true); err != nil {
		return Msg{}, err
	}
	w.queueCovertOp(a, d, OpBombEnemyTargets, "")
	return covertSent(d), nil
}

// resolveBombEnemyTargets is the queued operation arriving.
func (w *World) resolveBombEnemyTargets(a, d *Empire) Msg {
	if !w.covertRoll(a, d, OpBombEnemyTargets) {
		return w.covertFoiled(a, d, say("a terror bombing"))
	}
	covertReturned(a)
	t := bombTargets()[w.rng.Intn(BombTargetPickCount)]
	pct := t.base + w.rng.Intn(t.spread)
	held := t.get(d)
	lost := pctOf(held, pct)
	// Food is the one slot BRE rounds rather than truncates (BRE.OVR 0x04C6C6
	// calls the rounding helper where the other five call Trunc).
	if t.roundsUp && int64(held)*int64(pct)%100 >= 50 {
		lost++
	}
	if lost <= 0 {
		return say("Your agents found nothing worth bombing in {who}.", "who", d.Name)
	}
	t.set(d, held-lost)
	d.addEvent(say(t.theirs, "n", lost))
	return say(t.yours, "who", d.Name, "n", lost)
}
