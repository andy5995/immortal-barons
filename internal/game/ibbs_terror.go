package game

import (
	"fmt"
	"math"

	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// ibbs_terror.go — terrorist operations sent against another planet, and the
// damage each type does when its agents get through.

// TerrorOpType identifies which sub-operation was launched from the Terrorist
// Ops submenu. All nine types have the same mechanical effect (each agent
// destroys 1/TerrorUnitLossDenom of one random unit type), but BRE carries the
// type in the packet so the result report can name it.
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
func (w *World) SendTerror(e *Empire, targetBoard, targetEmpire string, agents int, op TerrorOpType) error {
	if !w.CanTerrorOp(e) {
		return ErrTerrorOpsExhausted
	}
	if e.Agents < agents {
		return ErrNoAgents
	}
	cost := w.TerrorOpGoldCost(e)
	if e.Gold < cost {
		return ErrCantAfford
	}
	e.Gold -= cost
	e.Agents -= agents
	e.TerrorOpsToday++
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
	return nil
}

// resolveRemoteTerror applies a terror op on this board: a protected target is
// untouched (the op fails); otherwise it destroys TerrorTrooperKill troopers per
// committed agent, capped at the target's troopers.
func (w *World) resolveRemoteTerror(t RemoteTerror) AttackResult {
	res := AttackResult{ID: t.ID, TargetBoard: w.Config.BoardID, TargetEmpire: t.TargetEmpire, Kind: "terror"}
	target := w.remoteTarget(t.TargetEmpire)
	if target == nil {
		// Nobody of that name here: the sender is owed that answer rather than the
		// same "achieved nothing" a repelled op gets (#165).
		res.Outcome = OutcomeNotFound
		return res
	}
	res.TargetEmpire = target.Name
	if target.Protection > 0 {
		target.addEvent(fmt.Sprintf("Terrorists from %s were stopped by your New Realm Protection.", t.FromBoard))
		w.postNews(fmt.Sprintf("Terrorists from %s broke on %s's New Realm Protection.", t.FromBoard, target.Name))
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
	// sent, so eight menu items were priced and named for nothing.
	//
	// NOT modelled: the original rolls each agent against the covert odds and
	// only a winning agent lands. IB lands them all, as it always has here.
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
	}
	// The agents that did NOT get through are the only thing that gives the
	// sender away, and they give it away whatever the rest of the batch did.
	agentsCaught(target, t, caught)
	res.Report = terrorOpReport(t.Op, hit)
	if hit == 0 {
		res.Outcome = OutcomeRepelled
		target.addEvent(fmt.Sprintf("Terrorists struck at your %s and achieved nothing.", terrorOpTargetName(t.Op)))
		w.postNews(fmt.Sprintf("Terrorists reached %s and achieved nothing.", target.Name))
		return res
	}
	target.addEvent(fmt.Sprintf("Terrorists %s %s", terrorOpDamage(t.Op), timesSuffix(hit)))
	w.postNews(fmt.Sprintf("Terrorists struck %s's %s.", target.Name, terrorOpTargetName(t.Op)))
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
// the line: the board is the part IB has always carried.
func agentsCaught(target *Empire, t RemoteTerror, caught int) {
	if caught <= 0 {
		return
	}
	who := t.FromBoard
	if t.FromEmpire != "" {
		who = fmt.Sprintf("%s of %s", t.FromEmpire, t.FromBoard)
	}
	agents := "agents"
	if caught == 1 {
		agents = "agent"
	}
	target.addEvent(fmt.Sprintf("Your security caught %d %s from %s.", caught, agents, who))
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
		w.postNews(fmt.Sprintf("Terrorists reached %s and achieved nothing.", target.Name))
		return res
	}
	target.addEvent(fmt.Sprintf("Terrorists destroyed %d of your forces!", destroyed))
	w.postNews(fmt.Sprintf("Terrorists destroyed %s of %s's forces!",
		numfmt.Comma(int64(destroyed)), target.Name))
	res.LandTaken = destroyed
	res.Won = true
	return res
}

// applyTerrorOp lands one agent's operation on target and reports whether it
// changed anything. Send Spy costs the target nothing, so it always "lands":
// what it takes is intelligence, carried home in the result's report.
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

// terrorOpTargetName, terrorOpDamage and timesSuffix write the two sentences an
// operation produces: what the target reads, and the line the result carries
// home. BRE reports a batch the same way — `ipreport.dat` holds a MULTI_ and a
// SINGLE_ template for each of the eight damaging operations, the multi form
// counting the successes ("... %N times!") rather than repeating the line.
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

func terrorOpDamage(op TerrorOpType) string {
	switch op {
	case TerrorOpSpy:
		return "went through your files"
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

// terrorOpReport is what the launching realm reads when the strike comes home.
func terrorOpReport(op TerrorOpType, hit int) string {
	if hit == 0 {
		return fmt.Sprintf("Your agents reached their target and achieved nothing (%s).", op)
	}
	return fmt.Sprintf("%s: your agents got through %s", op, timesSuffix(hit))
}

// timesSuffix closes a report line with how many agents landed, counting rather
// than repeating the sentence.
func timesSuffix(n int) string {
	if n == 1 {
		return "once."
	}
	return fmt.Sprintf("%d times.", n)
}
