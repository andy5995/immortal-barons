package game

import (
	"fmt"
	"math"
	"strings"

	"github.com/andy5995/immortal-barons/internal/numfmt"
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
		return "Send Spy"
	case TerrorOpBombIntel:
		return "Bomb Intelligence"
	case TerrorOpDemoralize:
		return "Demoralize"
	case TerrorOpDissensions:
		return "Cause Dissensions"
	case TerrorOpBombAirBases:
		return "Bomb AirBases"
	case TerrorOpEmigrations:
		return "Stir Emigrations"
	case TerrorOpPropaganda:
		return "Spread Propaganda"
	case TerrorOpBombFood:
		return "Bomb Food Stores"
	case TerrorOpSabotageHQ:
		return "Sabotage HQ"
	default:
		return "Terrorist Ops"
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
			res.Report = terrorOpReport(t.Op, target.Name, w.Config.BoardID, t.Agents, hit, caught, fate)
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
	res.Report = terrorOpReport(t.Op, target.Name, w.Config.BoardID, t.Agents, hit, caught, fate)
	if hit == 0 {
		res.Outcome = OutcomeRepelled
		// Only an agent that got past security struck at anything; a batch that
		// was caught to the last agent is told by the line above alone.
		if caught < t.Agents {
			target.addEvent(fmt.Sprintf("Terrorists went after your %s and got nowhere.", terrorOpTargetName(t.Op)))
		}
		return res
	}
	if hit == 1 {
		target.addEvent(fmt.Sprintf("Terrorists %s.", terrorOpDeed(t.Op)))
	} else {
		target.addEvent(fmt.Sprintf("Terrorists %s %d times.", terrorOpDeed(t.Op), hit))
	}
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
	from := t.FromBoard
	if t.FromEmpire != "" {
		from = fmt.Sprintf("%s of %s", t.FromEmpire, t.FromBoard)
	}
	target.addEvent(fill(fate.Theirs, "who", agentCount(caught)+" sent by "+from))
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
		target.addEvent("Terrorists struck but destroyed nothing.")
		return res
	}
	target.addEvent(fmt.Sprintf("Terrorists destroyed %s of your forces!", numfmt.Comma(int64(destroyed))))
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

// terrorOpTargetName is what an operation aims at, for the line the target
// reads when agents got through and found nothing there to damage.
func terrorOpTargetName(op TerrorOpType) string {
	switch op {
	case TerrorOpSpy:
		return "secrets"
	case TerrorOpBombIntel:
		return "intelligence agencies"
	case TerrorOpDemoralize:
		return "forces"
	case TerrorOpDissensions:
		return "ranks"
	case TerrorOpBombAirBases:
		return "air bases"
	case TerrorOpEmigrations:
		return "people"
	case TerrorOpPropaganda:
		return "streets"
	case TerrorOpBombFood:
		return "food stores"
	case TerrorOpSabotageHQ:
		return "headquarters"
	}
	return "realm"
}

// terrorOpDeed is what one operation does, as the realm it landed on reads
// it. BRE reports a batch the same way on both sides — `ipreport.dat` holds a
// SINGLE_ and a MULTI_ template for each of the eight damaging operations, the
// multi form counting the agents that got through ("... %N times!") rather
// than repeating the line, and the single form used when exactly one did
// (resolve_received_covert_operation and process_terrorist_report both branch
// on that count being 1). The phrases are IB's own.
func terrorOpDeed(op TerrorOpType) string {
	switch op {
	case TerrorOpBombIntel:
		return "bombed your intelligence agencies"
	case TerrorOpDemoralize:
		return "demoralized your forces"
	case TerrorOpDissensions:
		return "stirred dissent in your ranks"
	case TerrorOpBombAirBases:
		return "bombed your air bases"
	case TerrorOpEmigrations:
		return "drove your people into exile"
	case TerrorOpPropaganda:
		return "spread false rumors through your realm"
	case TerrorOpBombFood:
		return "bombed your food stores"
	case TerrorOpSabotageHQ:
		return "sabotaged your headquarters"
	}
	return "struck your realm"
}

// terrorOpStrike is what one operation does to target on board, for the
// sender's report: "bombed Victim's air bases on boardB". It is separate from
// terrorOpDeed on purpose: the wording differs by side.
func terrorOpStrike(op TerrorOpType, target, board string) string {
	var f string
	switch op {
	case TerrorOpBombIntel:
		f = "bombed %s's intelligence agencies on %s"
	case TerrorOpDemoralize:
		f = "sank %s's morale on %s"
	case TerrorOpDissensions:
		f = "stirred up dissent in %s's ranks on %s"
	case TerrorOpBombAirBases:
		f = "bombed %s's air bases on %s"
	case TerrorOpEmigrations:
		f = "drove %s's people into exile on %s"
	case TerrorOpPropaganda:
		f = "spread rumors through %s's realm on %s"
	case TerrorOpBombFood:
		f = "bombed %s's food stores on %s"
	case TerrorOpSabotageHQ:
		f = "sabotaged %s's headquarters on %s"
	default:
		f = "struck %s's realm on %s"
	}
	return fmt.Sprintf(f, target, board)
}

// terrorOpReport is what the launching realm reads when the strike comes home:
// one sentence, written here on the target's board because only it knows the
// target and the board both, and printed by the sender as it stands. It
// accounts for every agent — what the ones that got through did, then, when it
// applies, the ones caught (fate, the pool entry whose other half the target
// read) and the ones that got through to nothing left to damage. An agent that
// got through to a target already at zero costs the same gold as one that did
// damage, so IB says how many landed on nothing. The original's report has the
// same parts (process_terrorist_report, BRE.OVR 0x04b38a); the words are IB's.
func terrorOpReport(op TerrorOpType, target, board string, sent, hit, caught int, fate agentCaught) string {
	if caught >= sent {
		if sent == 1 {
			return fmt.Sprintf("%s's security on %s caught your agent.", target, board)
		}
		return fmt.Sprintf("%s's security on %s caught every one of your %d agents.", target, board, sent)
	}
	agents := "Your agents"
	if sent == 1 {
		agents = "Your agent"
	}
	// A spy batch stops at the first agent in, so the rest never went anywhere.
	wasted := sent - hit - caught
	var b strings.Builder
	switch {
	case op == TerrorOpSpy:
		wasted = 0
		fmt.Fprintf(&b, "Your spy slipped into %s's files on %s and came home with a full report", target, board)
	case hit > 0:
		fmt.Fprintf(&b, "%s %s %s", agents, terrorOpStrike(op, target, board), times(hit))
	default:
		fmt.Fprintf(&b, "%s reached %s's %s on %s and found nothing left to damage",
			agents, target, terrorOpTargetName(op), board)
		wasted = 0
	}
	them := "of them"
	if op == TerrorOpSpy {
		them = "of your other agents"
	}
	switch {
	case caught == 1:
		fmt.Fprintf(&b, "; one %s %s", them, fate.Singular)
	case caught > 1:
		fmt.Fprintf(&b, "; %d %s %s", caught, them, fate.Plural)
	}
	if wasted > 0 {
		fmt.Fprintf(&b, "; %d more found nothing left to damage", wasted)
	}
	b.WriteString(".")
	return b.String()
}

// times is "once" for one and "N times" otherwise.
// agentCount is n agents, in the singular for one.
func agentCount(n int) string {
	if n == 1 {
		return "1 agent"
	}
	return fmt.Sprintf("%d agents", n)
}

func times(n int) string {
	if n == 1 {
		return "once"
	}
	return fmt.Sprintf("%d times", n)
}
