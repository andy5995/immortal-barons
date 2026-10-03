package game

import (
	"math"
)

// ibbs_terror.go — terrorist operations sent against another planet, and the
// damage each type does when its agents get through.

// TerrorOpType identifies which sub-operation was launched from the Terrorist
// Ops submenu. The packet carries it and the target board dispatches on it:
// each op lands on its own holding (applyTerrorOp).
type TerrorOpType int

const (
	TerrorOpSpy TerrorOpType = iota + 1
	TerrorOpBombIntel
	TerrorOpDemoralize
	TerrorOpDissensions
	TerrorOpBombAirBases
	TerrorOpEmigrations
	TerrorOpPropaganda
	TerrorOpBombFood
	TerrorOpSabotageHQ
)

// String is the BRE label for a terror sub-op.
func (t TerrorOpType) String() string {
	switch t {
	case TerrorOpSpy:
		return msgid("Send Spy")
	case TerrorOpBombIntel:
		return msgid("Bomb Intelligence")
	case TerrorOpDemoralize:
		return msgid("Demoralize")
	case TerrorOpDissensions:
		return msgid("Cause Dissensions")
	case TerrorOpBombAirBases:
		return msgid("Bomb AirBases")
	case TerrorOpEmigrations:
		return msgid("Stir Emigrations")
	case TerrorOpPropaganda:
		return msgid("Spread Propaganda")
	case TerrorOpBombFood:
		return msgid("Bomb Food Stores")
	case TerrorOpSabotageHQ:
		return msgid("Sabotage HQ")
	default:
		return msgid("Terrorist Ops")
	}
}

// RemoteTerror is a terror strike sent to an empire on another board: BRE's
// Terrorist Ops destroy the target's forces rather than capturing land.
type RemoteTerror struct {
	ID           int
	FromBoard    string
	TargetEmpire string
	Agents       int // agents committed; scales the forces destroyed
	// Op is which of the nine operations was launched; the target board resolves
	// each one differently (#166). Absent on a packet written before that, which
	// falls back to the blanket unit damage every op used to do.
	Op TerrorOpType `json:",omitempty"`
	// FromEmpire is the realm that sent the agents, carried so the target board
	// can name it on the one line the original names it: the count of agents its
	// security caught. Absent on a packet from a board that predates the field,
	// which then names the board alone.
	FromEmpire string `json:",omitempty"`
	// Strength is the sender's covert pool, measured at home and carried across
	// because the target board cannot see it. BRE does exactly this: the launcher
	// calls the covert-pool routine for its own realm and writes the figure into
	// the 18-byte record the packet carries (launch_terrorist_operation, BRE.OVR
	// 0x02B201, storing at record +0x0D). Absent on an older packet, which then
	// rolls against a pool of nothing and lands every agent, as it used to.
	Strength int `json:",omitempty"`
}

// SendTerror queues a terror op against targetEmpire on targetBoard, committing
// agents (deducted now). op is the sub-operation type from the Terrorist Ops
// submenu, and it decides what lands: the target board dispatches on it as the
// original does. It resolves on the target board's next packet run.
//
// A request larger than TerrorAgentsSendable is clamped to it, and the count
// actually sent is returned. The menu never asks for more than that, but agents
// can still fall while the prompt is open: an arriving Bomb Intelligence is
// resolved whenever packets are processed. Only a send of nothing is refused.
func (w *World) SendTerror(e *Empire, targetBoard, targetEmpire string, agents int, op TerrorOpType) (int, error) {
	// EACH AGENT IS ONE OPERATION. It pays its own share of the fee and takes its
	// own slot out of the day's allowance, which is what the original's prompt
	// counts down: `Send how many? (1; 15)` becomes `(1; 7)` after eight go out.
	// IB charged one fee for any number of agents and counted the send as a
	// single op until 2026-09-05.
	if agents < 1 {
		return 0, ErrNoAgents
	}
	switch {
	case w.Config.MaxTerrorOps > 0 && w.TerrorOpsLeft(e) < 1:
		return 0, ErrTerrorOpsExhausted
	case e.Agents < 1:
		return 0, ErrNoAgents
	}
	agents = min(agents, w.TerrorAgentsSendable(e))
	cost := w.TerrorOpGoldCost(e, agents)
	if e.Gold < cost {
		return 0, ErrCantAfford
	}
	e.Gold -= cost
	e.Agents -= agents
	e.TerrorOpsToday += agents
	w.NextAttackID++
	t := RemoteTerror{
		ID:           w.NextAttackID,
		FromBoard:    w.Config.BoardID,
		FromEmpire:   e.Name,
		TargetEmpire: targetEmpire,
		Agents:       agents,
		Op:           op,
		Strength:     w.covertStrength(e, true),
	}
	w.InFlight = append(w.InFlight, InFlightStrike{
		ID:           t.ID,
		Kind:         "terror",
		TargetBoard:  targetBoard,
		TargetEmpire: targetEmpire,
		LaunchedDay:  w.GameDay,
		Owner:        e.Owner,
		Agents:       agents,
		TerrorOp:     op,
	})
	p := w.outboxFor(targetBoard)
	p.Terrors = append(p.Terrors, t)
	return agents, nil
}

// resolveRemoteTerror applies a terror op on this board. A protected target is
// untouched and told nothing; otherwise each committed agent is rolled against
// the target's covert strength and, if it gets through, does what its
// operation does (applyTerrorOp).
func (w *World) resolveRemoteTerror(t RemoteTerror) AttackResult {
	res := AttackResult{ID: t.ID, TargetBoard: w.Config.BoardID, TargetEmpire: t.TargetEmpire, Kind: "terror"}
	target := w.remoteTarget(t.TargetEmpire)
	if target == nil {
		// Nobody of that name here: the sender is owed that answer rather than the
		// same "got nowhere" a repelled op gets (#165).
		res.Outcome = OutcomeNotFound
		return res
	}
	res.TargetEmpire = target.Name
	// BINARY-VERIFIED: the original tests protection (BRE.OVR 0x04a96b +0x373)
	// and jumps straight to writing the sender's result, filing nothing for the
	// target. IB told the target which board had tried until 2026-10-01.
	if target.Protection > 0 {
		res.Outcome = OutcomeProtected
		return res
	}
	if t.Op == 0 {
		return w.resolveLegacyTerror(t, target, res)
	}
	defense := w.covertStrength(target, false)
	// Each committed agent lands once, and what it does depends on the operation
	// the packet names (#166). BRE dispatches the same way, on the operation byte
	// it carried across (resolve_received_covert_operation, BRE.OVR 0x04a96b);
	// IB used to ignore it and destroy random units whichever of the nine was
	// sent, so eight menu items were priced and named for nothing. Each agent
	// is rolled against the target's covert strength, and only a winning one
	// lands (terrorAgentLands).
	hit, caught := 0, 0
	for i := 0; i < t.Agents; i++ {
		// A packet with no strength recorded is from a board that predates the
		// roll; its agents all land, which is what it was written expecting.
		if t.Strength > 0 && !w.terrorAgentLands(t.Strength, defense) {
			caught++
			continue
		}
		if w.applyTerrorOp(t.Op, target) {
			hit++
		}
		// The first spy that gets in ends the mission. BINARY-VERIFIED: the
		// resolver writes the spy report and returns at once (BRE.OVR 0x04a96b
		// +0x560), so the agents after it are never rolled, and the lines
		// below — the caught count and the per-operation line — are never
		// filed. The target of a spy that got in is told nothing at all.
		if t.Op == TerrorOpSpy {
			var fate agentCaught
			if caught > 0 {
				fate = w.pickAgentCaught()
			}
			res.Report = optMsg(terrorOpReport(t.Op, target.Name, w.Config.BoardID, t.Agents, hit, caught, fate))
			res.Won = true
			res.Outcome = OutcomeWon
			return res
		}
	}
	// A terror op is reported on the two recaps and nowhere else: the original's
	// received-op resolver (BRE.OVR 0x04a96b) and its returning-report routine
	// (process_terrorist_report, 0x04b38a) each file entries for their own
	// realm and neither calls the news writer. IB posted a line on both planets
	// until #285.
	//
	// The agents that did NOT get through are the only thing that gives the
	// sender away, and they give it away whatever the rest of the batch did.
	//
	// One caught-agent line is picked for both sides, so the target's event and
	// the sender's report tell the same story.
	var fate agentCaught
	if caught > 0 {
		fate = w.pickAgentCaught()
	}
	agentsCaught(target, t, caught, fate)
	res.Report = optMsg(terrorOpReport(t.Op, target.Name, w.Config.BoardID, t.Agents, hit, caught, fate))
	if hit == 0 {
		res.Outcome = OutcomeRepelled
		// Only an agent that got past security struck at anything; a batch that
		// was caught to the last agent is told by the line above alone.
		if caught < t.Agents {
			target.addEvent(say(terrorTextFor(t.Op).nowhere))
		}
		return res
	}
	text := terrorTextFor(t.Op)
	target.addEvent(sayIn(text.deed, "n", "n", hit))
	res.Won = true
	res.Outcome = OutcomeWon
	return res
}

// agentsCaught files the one entry an interplanetary terror op gives its source
// away on: the agents the target's security stopped, named by realm and board.
//
// BINARY-VERIFIED, and the reason every other line above it names nobody: the
// original's received-op resolver (BRE.OVR 0x04a96b) counts the agents that
// failed and files this line only when that count is non-zero, while the
// per-operation lines for the agents that DID get through carry no source at
// all. A live capture shows both halves in one recap: this line carries a count,
// the realm and its planet, while the unit-loss and demoralization lines beside
// it name nobody. See covertFoiled for the local sibling of the same rule.
//
// A packet with no realm recorded names the board alone rather than dropping
// the line: the board is the part IB has always carried. fate is the entry of
// the caught-agent pool picked for this strike, whose other half the sender
// reads.
func agentsCaught(target *Empire, t RemoteTerror, caught int, fate agentCaught) {
	if caught <= 0 {
		return
	}
	who := sayN("{n} agent sent by {board}", "{n} agents sent by {board}", "n", "n", caught, "board", t.FromBoard)
	if t.FromEmpire != "" {
		who = sayN("{n} agent sent by {who} of {board}", "{n} agents sent by {who} of {board}", "n",
			"n", caught, "who", t.FromEmpire, "board", t.FromBoard)
	}
	target.addEvent(say(fate.Theirs, "who", who))
}

// terrorAgentLands is whether one committed agent gets through, weighing the
// sender's covert pool (carried in the packet) against the target's. It is
// calculate_combat_odds (BRE.OVR 0x04a7a9), which the received-op resolver calls
// once per agent before letting it act; see the constants for the shape.
func (w *World) terrorAgentLands(attack, defense int) bool {
	if attack < 0 {
		attack = 0
	}
	if defense < 0 {
		defense = 0
	}
	if w.rng.Intn(TerrorAutoLandOdds) == 0 {
		return true
	}
	if w.rng.Intn(TerrorAutoFoilOdds) == 0 {
		return false
	}
	a := roundedRoot(attack)
	d := roundedRoot(defense)
	weighted := func() int { return a + d*TerrorDefenseWeightNum/TerrorDefenseWeightDenom }
	for weighted() > TerrorOddsCeiling {
		a /= 2
		d /= 2
	}
	total := weighted()
	if total <= 0 {
		return false
	}
	return w.rng.Intn(total) < a
}

// roundedRoot is the square root the odds routine takes of each side, rounded
// as the original rounds it.
func roundedRoot(n int) int {
	return int(math.Round(math.Sqrt(float64(n))))
}

// resolveLegacyTerror is the blanket effect every terror op had before the nine
// were separated: a fraction of one randomly chosen unit type per agent. It is
// reached only by a packet from a board too old to name its operation.
func (w *World) resolveLegacyTerror(t RemoteTerror, target *Empire, res AttackResult) AttackResult {
	fields := []*int{&target.Troopers, &target.Jets, &target.Turrets, &target.Tanks, &target.Bombers, &target.Carriers}
	destroyed := 0
	for i := 0; i < t.Agents; i++ {
		f := fields[w.rng.Intn(len(fields))]
		loss := *f / TerrorUnitLossDenom
		*f -= loss
		destroyed += loss
	}
	if destroyed == 0 {
		target.addEvent(say("Terrorists struck but destroyed nothing."))
		return res
	}
	target.addEvent(say("Terrorists destroyed {n} of your forces!", "n", comma(destroyed)))
	res.LandTaken = destroyed
	res.Won = true
	return res
}

// applyTerrorOp lands one agent's operation on target and reports whether it
// changed anything. Send Spy costs the target nothing, so it always "lands":
// what it takes is intelligence, carried home beside the result.
func (w *World) applyTerrorOp(op TerrorOpType, target *Empire) bool {
	if band, ok := TerrorOpLosses[op]; ok {
		field := terrorOpField(op, target)
		loss := *field * (band.Base + w.rng.Intn(band.Spread)) / 100
		*field -= loss
		return loss > 0
	}
	switch op {
	case TerrorOpSpy:
		return true
	case TerrorOpDemoralize:
		before := target.Morale
		target.Morale = target.Morale * TerrorMoraleKeepNumerator / TerrorMoraleKeepDenominator
		return target.Morale < before
	case TerrorOpPropaganda:
		before := target.Support
		target.Support = target.Support * TerrorSupportKeepNumerator / TerrorSupportKeepDenominator
		return target.Support < before
	case TerrorOpSabotageHQ:
		if target.HQ <= 0 {
			return false
		}
		target.HQ -= TerrorHQSabotagePoints
		if target.HQ < 0 {
			target.HQ = 0
		}
		return true
	}
	return false
}

// terrorOpField is the count each percentage-based operation eats into.
func terrorOpField(op TerrorOpType, e *Empire) *int {
	switch op {
	case TerrorOpBombIntel:
		return &e.Agents
	case TerrorOpDissensions:
		return &e.Troopers
	case TerrorOpBombAirBases:
		return &e.Jets
	case TerrorOpEmigrations:
		return &e.People
	case TerrorOpBombFood:
		return &e.Food
	}
	return new(int) // unreachable for anything in TerrorOpLosses
}

// terrorText is every sentence one operation is reported with, each whole so
// it translates on its own (#297). BRE reports a batch the same way on both
// sides — `ipreport.dat` holds a SINGLE_ and a MULTI_ template for each of the
// eight damaging operations, the multi form counting the agents that got
// through ("... %N times!") rather than repeating the line, and the single
// form used when exactly one did (resolve_received_covert_operation and
// process_terrorist_report both branch on that count being 1). The words are
// IB's own.
type terrorText struct {
	// The target's lines: the agents that got through ({n}), or some got past
	// security and found nothing to damage.
	deed    forms
	nowhere string
	// The sender's: a lone agent's strike, a batch's ({n} the agents that got
	// through), and a lone agent or a batch that got through to nothing left
	// to damage. {target} and {board} name the realm hit.
	lone             string
	batch            forms
	emptyLone, empty string
}

var terrorTexts = map[TerrorOpType]terrorText{
	TerrorOpSpy: {
		nowhere:   msgid("Terrorists went after your secrets and got nowhere."),
		emptyLone: msgid("Your agent reached {target}'s secrets on {board} and found nothing left to damage."),
		empty:     msgid("Your agents reached {target}'s secrets on {board} and found nothing left to damage."),
	},
	TerrorOpBombIntel: {
		msgidN("Terrorists bombed your intelligence agencies.", "Terrorists bombed your intelligence agencies {n} times."),
		msgid("Terrorists went after your intelligence agencies and got nowhere."),
		msgid("Your agent bombed {target}'s intelligence agencies on {board} once."),
		msgidN("Your agents bombed {target}'s intelligence agencies on {board} once.",
			"Your agents bombed {target}'s intelligence agencies on {board} {n} times."),
		msgid("Your agent reached {target}'s intelligence agencies on {board} and found nothing left to damage."),
		msgid("Your agents reached {target}'s intelligence agencies on {board} and found nothing left to damage."),
	},
	TerrorOpDemoralize: {
		msgidN("Terrorists demoralized your forces.", "Terrorists demoralized your forces {n} times."),
		msgid("Terrorists went after your forces and got nowhere."),
		msgid("Your agent sank {target}'s morale on {board} once."),
		msgidN("Your agents sank {target}'s morale on {board} once.",
			"Your agents sank {target}'s morale on {board} {n} times."),
		msgid("Your agent reached {target}'s forces on {board} and found nothing left to damage."),
		msgid("Your agents reached {target}'s forces on {board} and found nothing left to damage."),
	},
	TerrorOpDissensions: {
		msgidN("Terrorists stirred dissent in your ranks.", "Terrorists stirred dissent in your ranks {n} times."),
		msgid("Terrorists went after your ranks and got nowhere."),
		msgid("Your agent stirred up dissent in {target}'s ranks on {board} once."),
		msgidN("Your agents stirred up dissent in {target}'s ranks on {board} once.",
			"Your agents stirred up dissent in {target}'s ranks on {board} {n} times."),
		msgid("Your agent reached {target}'s ranks on {board} and found nothing left to damage."),
		msgid("Your agents reached {target}'s ranks on {board} and found nothing left to damage."),
	},
	TerrorOpBombAirBases: {
		msgidN("Terrorists bombed your air bases.", "Terrorists bombed your air bases {n} times."),
		msgid("Terrorists went after your air bases and got nowhere."),
		msgid("Your agent bombed {target}'s air bases on {board} once."),
		msgidN("Your agents bombed {target}'s air bases on {board} once.",
			"Your agents bombed {target}'s air bases on {board} {n} times."),
		msgid("Your agent reached {target}'s air bases on {board} and found nothing left to damage."),
		msgid("Your agents reached {target}'s air bases on {board} and found nothing left to damage."),
	},
	TerrorOpEmigrations: {
		msgidN("Terrorists drove your people into exile.", "Terrorists drove your people into exile {n} times."),
		msgid("Terrorists went after your people and got nowhere."),
		msgid("Your agent drove {target}'s people into exile on {board} once."),
		msgidN("Your agents drove {target}'s people into exile on {board} once.",
			"Your agents drove {target}'s people into exile on {board} {n} times."),
		msgid("Your agent reached {target}'s people on {board} and found nothing left to damage."),
		msgid("Your agents reached {target}'s people on {board} and found nothing left to damage."),
	},
	TerrorOpPropaganda: {
		msgidN("Terrorists spread false rumors through your realm.", "Terrorists spread false rumors through your realm {n} times."),
		msgid("Terrorists went after your streets and got nowhere."),
		msgid("Your agent spread rumors through {target}'s realm on {board} once."),
		msgidN("Your agents spread rumors through {target}'s realm on {board} once.",
			"Your agents spread rumors through {target}'s realm on {board} {n} times."),
		msgid("Your agent reached {target}'s streets on {board} and found nothing left to damage."),
		msgid("Your agents reached {target}'s streets on {board} and found nothing left to damage."),
	},
	TerrorOpBombFood: {
		msgidN("Terrorists bombed your food stores.", "Terrorists bombed your food stores {n} times."),
		msgid("Terrorists went after your food stores and got nowhere."),
		msgid("Your agent bombed {target}'s food stores on {board} once."),
		msgidN("Your agents bombed {target}'s food stores on {board} once.",
			"Your agents bombed {target}'s food stores on {board} {n} times."),
		msgid("Your agent reached {target}'s food stores on {board} and found nothing left to damage."),
		msgid("Your agents reached {target}'s food stores on {board} and found nothing left to damage."),
	},
	TerrorOpSabotageHQ: {
		msgidN("Terrorists sabotaged your headquarters.", "Terrorists sabotaged your headquarters {n} times."),
		msgid("Terrorists went after your headquarters and got nowhere."),
		msgid("Your agent sabotaged {target}'s headquarters on {board} once."),
		msgidN("Your agents sabotaged {target}'s headquarters on {board} once.",
			"Your agents sabotaged {target}'s headquarters on {board} {n} times."),
		msgid("Your agent reached {target}'s headquarters on {board} and found nothing left to damage."),
		msgid("Your agents reached {target}'s headquarters on {board} and found nothing left to damage."),
	},
}

// unknownTerrorText reports an operation this build does not know, from a
// newer board.
var unknownTerrorText = terrorText{
	msgidN("Terrorists struck your realm.", "Terrorists struck your realm {n} times."),
	msgid("Terrorists went after your realm and got nowhere."),
	msgid("Your agent struck {target}'s realm on {board} once."),
	msgidN("Your agents struck {target}'s realm on {board} once.",
		"Your agents struck {target}'s realm on {board} {n} times."),
	msgid("Your agent reached {target}'s realm on {board} and found nothing left to damage."),
	msgid("Your agents reached {target}'s realm on {board} and found nothing left to damage."),
}

func terrorTextFor(op TerrorOpType) terrorText {
	if t, ok := terrorTexts[op]; ok {
		return t
	}
	return unknownTerrorText
}

// terrorOpReport is what the launching realm reads when the strike comes home,
// written here on the target's board because only it knows the target and the
// board both, and put into words by the sender's board. It accounts for every
// agent — what the ones that got through did, then, when it applies, the ones
// caught (fate, the pool entry whose other half the target read) and the ones
// that got through to nothing left to damage, each in a sentence of its own. An
// agent that got through to a target already at zero costs the same gold as one
// that did damage, so IB says how many landed on nothing. The original's report
// has the same parts (process_terrorist_report, BRE.OVR 0x04b38a); the words
// are IB's.
func terrorOpReport(op TerrorOpType, target, board string, sent, hit, caught int, fate agentCaught) Msg {
	if caught >= sent {
		if sent == 1 {
			return say("{target}'s security on {board} caught your agent.", "target", target, "board", board)
		}
		return say("{target}'s security on {board} caught every one of your {n} agents.",
			"target", target, "board", board, "n", sent)
	}
	text := terrorTextFor(op)
	where := []any{"target", target, "board", board}
	// A spy batch stops at the first agent in, so the rest never went anywhere.
	wasted := sent - hit - caught
	var did Msg
	switch {
	case op == TerrorOpSpy:
		wasted = 0
		did = say("Your spy slipped into {target}'s files on {board} and came home with a full report.", where...)
	case hit > 0 && sent == 1:
		did = say(text.lone, where...)
	case hit > 0:
		did = sayIn(text.batch, "n", append(where, "n", hit)...)
	case sent == 1:
		did = say(text.emptyLone, where...)
		wasted = 0
	default:
		did = say(text.empty, where...)
		wasted = 0
	}
	var lost Msg
	if caught > 0 {
		f := fate.Caught
		if op == TerrorOpSpy {
			f = fate.Other
		}
		lost = sayIn(f, "n", "n", caught)
	}
	var idle Msg
	if wasted > 0 {
		idle = sayN("One more found nothing left to damage.", "{n} more found nothing left to damage.", "n", "n", wasted)
	}
	return sentences(did, lost, idle)
}
