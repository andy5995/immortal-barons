package game

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// Interplanetary Special Operations (#49) — the InterPlanetary Ops menu's
// Special Operations submenu, where every op is aimed at another planet: the
// four bombing ops at the planet as a whole, the three missiles at one baron
// on it.
//
// The original resolves these the same way IB already resolves terror ops: its
// menu handler (`run_bombing_operations_menu`, BRE.OVR 0x029ea9) calls
// `write_special_operation_packet` rather than touching the target, so the
// effect lands when the packet does. That is what this file implements — the
// launch side deducts the cost and books the op in flight, the target board
// applies it and answers, and the lost-forces timer hands the op back if the
// answer never arrives.
//
// The bombing EFFECTS live in bombing.go and the missiles' in specials.go and
// sabre.go. Neither is the local menu's: the original resolves an arriving op in
// receivers of its own, with their own rolls and bands, and IB follows those.
// The attacker is not present on the board that resolves it, so its score is
// awarded when the answer comes home rather than on the spot.

// SpecialOp names one interplanetary Special Operation. The values are IB's own
// wire strings, not the original's op codes: the packet format is a clean-room
// JSON design, so a readable discriminator costs nothing and survives a field
// being added beside it.
type SpecialOp string

const (
	OpBombFood   SpecialOp = "bomb-food"
	OpBombMarket SpecialOp = "bomb-market"
	OpBombRoutes SpecialOp = "bomb-routes"
	OpUndermine  SpecialOp = "undermine"
	OpNuclear    SpecialOp = "nuclear"
	OpChemical   SpecialOp = "chemical"
	// The wire string stays "slappenheimer", the name IB shipped the op under
	// before it took BRE's own name back: a renamed discriminator would strand
	// every op already in flight when a league updates board by board.
	OpSabre SpecialOp = "slappenheimer"
)

// SpecialOpLabel is the op's menu label, used in the reports both boards file.
// One table, so the news a target reads names the op the attacker chose.
func SpecialOpLabel(op SpecialOp) string {
	switch op {
	case OpBombFood:
		return "Bomb Food Market"
	case OpBombMarket:
		return "Bomb Trading Market"
	case OpBombRoutes:
		return "Bomb Trade Routes"
	case OpUndermine:
		return "Undermine Investments"
	case OpNuclear:
		return "Nuclear Assault"
	case OpChemical:
		return "Chemical Bombing"
	case OpSabre:
		return "S3-Sabre"
	}
	return string(op)
}

// isMissileOp reports whether op is one of the three the sysop's Missile Ops
// switch governs; the other four answer to Bombing Ops. Same split as the local
// menu, so one switch cannot disable an op on one menu and leave it on the
// other.
//
// It is also the line between the two kinds of target below, which is not a
// coincidence: the original's menu handler branches the same way. Keys '1'-'4'
// all jump to ONE shared handler differing only by an index into a price table,
// while '5', '6' and '7' each have their own branch (BRE.OVR 0x029ea9, the
// dispatch at 0x105a-0x124c).
func isMissileOp(op SpecialOp) bool {
	return op == OpNuclear || op == OpChemical || op == OpSabre
}

// TargetsPlanet reports whether op is aimed at the PLANET rather than at a named
// baron on it.
//
// The four bombing ops are: what they wreck belongs to the whole planet, not to
// one realm. The Food Market is the planet's, the Trading Market is the planet's
// (every realm lists on the one board), and the bank the investments sit in is
// the planet's. Aiming them at a single baron — which is what IB did at first,
// by copying the lettered submenu it wrongly gave the LOCAL Covert menu —
// misreads one menu as the model for the other. The three missiles are the other
// way round: they ruin land and kill people, so they must name the realm that
// owns them.
func (op SpecialOp) TargetsPlanet() bool { return !isMissileOp(op) }

// RemoteSpecialOp is one op in flight to another planet. FromEmpire travels
// because the target's event log names who struck it, and the target board has
// no other way to know.
type RemoteSpecialOp struct {
	ID           int
	FromBoard    string
	FromEmpire   string
	TargetEmpire string
	Op           SpecialOp
	// Dial is the S3-Sabre's aim, 0-10, and is meaningless for every other op.
	// Optional and omitted when zero, which is the shape the Protocol comment
	// calls a safe wire change: a board that predates it sends nothing and the
	// receiver reads 0, a dial the mapper defines.
	Dial int `json:",omitempty"`
}

// CanSpecialOp reports whether e may launch op right now, on the per-day rule
// that op is subject to: a missile is once a day each, a bombing op counts
// against MaxBombingOps. The menu asks so a spent missile can be left off the
// list, which is what the original does rather than refusing on selection.
func (w *World) CanSpecialOp(e *Empire, op SpecialOp) bool {
	if isMissileOp(op) {
		return !e.MissileUsedToday[op]
	}
	return w.CanBombingOp(e)
}

// RemoteLand is the last-known region count for a realm on another planet, from
// the scores this board has imported, or 0 when no packet has named it yet.
// That figure is what the original prices an interplanetary missile off — the
// Territory column of the IPScores screen, which is this field.
//
// It goes stale between packets, and that is the original's behavior too: it
// quotes from whatever the last scores said, so a realm that has grown since is
// nuked at the old price.
func (w *World) RemoteLand(board, empire string) int {
	for i := range w.RemoteBoards {
		if w.RemoteBoards[i].BoardID != board {
			continue
		}
		for _, s := range w.RemoteBoards[i].Scores {
			if s.Empire == empire {
				return s.Land
			}
		}
	}
	return 0
}

// SpecialOpGoldCost prices one op against a target holding targetLand regions.
//
// The four bombing ops carry the flat prices the original prints in the menu's
// own price column (captured live; see docs/dev/bre-screens.md) and ignore the
// target entirely — they are aimed at a planet, not a baron.
//
// The three missiles are priced off the TARGET's last-known territory, at a rate
// of their own per missile (IPNukeGoldPerRegion and friends, binary-verified and
// capture-confirmed; see balance.go). Uncapped: StrikeCostCap is the local
// path's ceiling and does not appear on this one.
//
// The sysop's Terror Costs dial scales the bombing ops only. The original
// applies no such knob to the missiles, and leaving it on them would have moved
// a price this now matches exactly.
func (w *World) SpecialOpGoldCost(e *Empire, op SpecialOp, targetLand int) int64 {
	switch op {
	case OpNuclear:
		return int64(targetLand) * IPNukeGoldPerRegion
	case OpChemical:
		return int64(targetLand) * IPChemGoldPerRegion
	case OpSabre:
		return int64(targetLand) * IPSabreGoldPerRegion
	}
	var cost int64
	switch op {
	case OpBombFood:
		cost = IPBombFoodCost
	case OpBombMarket:
		cost = IPBombMarketCost
	case OpBombRoutes:
		cost = IPBombRoutesCost
	case OpUndermine:
		cost = IPUndermineCost
	}
	return cost * int64(w.Config.TerrorCosts.CostPercent()) / 100
}

// SendSpecialOp queues an op against targetEmpire on targetBoard. The gold goes
// now and the op is booked in flight, so a packet that never comes back is
// swept by the same lost-forces timer that returns an attack.
func (w *World) SendSpecialOp(e *Empire, targetBoard, targetEmpire string, op SpecialOp, dial int) error {
	if isMissileOp(op) {
		if !w.Config.MissileOps {
			return ErrMissileOpsDisabled
		}
	} else if !w.Config.BombingOps {
		return ErrBombingOpsDisabled
	}
	if isMissileOp(op) {
		// Each missile is its own once-a-day gate, not a share of the bombing
		// allowance — see Empire.MissileUsedToday.
		if e.MissileUsedToday[op] {
			return ErrMissileSpentToday
		}
	} else if !w.CanBombingOp(e) {
		return ErrBombingOpsExhausted
	}
	// The original requires the bombers for every op on this menu, missiles
	// included: "All missiles and bombs require 500 Bombers to deliver their
	// payloads". The local menu enforces the same floor.
	if e.Bombers < BombingBombersRequired {
		return ErrNeedBombers
	}
	// Priced off what the last scores said the target holds, which is what the
	// original quotes from; a planet-wide op ignores it.
	targetLand := w.RemoteLand(targetBoard, targetEmpire)
	if isMissileOp(op) && targetLand <= 0 {
		// No scores for that realm, so no price — and a missile that costs
		// nothing is worse than one that cannot be sent. The menu only offers
		// barons this board holds scores for, so this is the engine refusing a
		// call the menu would not have made.
		return ErrNoTargetSize
	}
	cost := w.SpecialOpGoldCost(e, op, targetLand)
	if e.Gold < cost {
		return ErrCantAfford
	}
	if op.TargetsPlanet() {
		targetEmpire = "" // nothing on the far side should look for a realm
	}
	if op == OpSabre {
		dial = w.sabreDialFor(dial)
	}
	e.Gold -= cost
	if isMissileOp(op) {
		if e.MissileUsedToday == nil {
			e.MissileUsedToday = map[SpecialOp]bool{}
		}
		e.MissileUsedToday[op] = true
	} else {
		e.BombingOpsToday++
	}
	w.NextAttackID++
	w.InFlight = append(w.InFlight, InFlightStrike{
		ID:           w.NextAttackID,
		Kind:         "special",
		TargetBoard:  targetBoard,
		TargetEmpire: targetEmpire,
		LaunchedDay:  w.GameDay,
		Owner:        e.Owner,
		Op:           op,
	})
	p := w.outboxFor(targetBoard)
	p.SpecialOps = append(p.SpecialOps, RemoteSpecialOp{
		ID:           w.NextAttackID,
		FromBoard:    w.Config.BoardID,
		FromEmpire:   e.Name,
		TargetEmpire: targetEmpire,
		Op:           op,
		Dial:         dial,
	})
	return nil
}

// specialOutcome is how one arriving Special Operation ended on the board it
// landed on. The target board's single news line is chosen by it (#288), so
// the planet reads the same outcome the firer's report carries home.
type specialOutcome int

const (
	specialHit         specialOutcome = iota // it landed and did damage
	specialNothing                           // it landed on nothing worth damaging
	specialMisfire                           // a missile that failed on its own
	specialIntercepted                       // a missile the target's SDI shot down
	specialBackfire                          // an S3-Sabre that developed land for its target
	specialDrivenOff                         // a bombing run that never reached what it was sent at

	// specialOutcomeCount is not an outcome but the number of them, so a test
	// can hold both boards' news lines to every one.
	specialOutcomeCount
)

// resolveRemoteSpecialOp applies an inbound op to this board's target and
// returns the answer that rides home.
//
// A realm under New Realm Protection is untouched, as it is for a terror op and
// an attack: the protection is the point, and an op that could slip past it
// would make the strongest weapon the one that ignores the only shield a new
// player has.
//
// Every outcome but a missing realm posts one line to this planet's news, and
// the line is chosen by the outcome. BINARY-VERIFIED: the original's two
// receivers (resolve_received_sabre_strike, BRE.OVR 0x04546e, for the three
// missiles; resolve_received_bombing, 0x04a09a, for the four bombing ops) each
// make one call to the news writer, reached by every outcome past the realm
// lookup, and pick a failure or a success line from ipreport.dat's
// SPECIAL_OPERATIONS and BOMBING_HITS sections. IB posted "X struck Y" for
// every outcome until #288, so a missile that broke up read as a hit here while
// the firer's own report said it failed.
func (w *World) resolveRemoteSpecialOp(op RemoteSpecialOp) AttackResult {
	res := AttackResult{
		ID:           op.ID,
		TargetBoard:  w.Config.BoardID,
		TargetEmpire: op.TargetEmpire,
		Kind:         string(op.Op),
	}
	label := SpecialOpLabel(op.Op)
	from := fmt.Sprintf("%s of %s", op.FromEmpire, op.FromBoard)

	// A planet op wrecks what the whole planet shares, so there is no realm to
	// look up and New Realm Protection does not enter into it — a new realm is
	// shielded from being singled out, not from the planet's market burning
	// down around it. BINARY-VERIFIED: resolve_received_bombing (BRE.OVR
	// 0x04a09a) calls no protection test, and its per-realm loops (+0x1d7,
	// +0x308) skip only an empty slot, so a protected realm's listings and
	// investments are hit with everyone else's.
	if op.Op.TargetsPlanet() {
		report, outcome := w.applyPlanetOp(op.Op, from)
		res.Report = report
		res.Won = outcome == specialHit
		res.Outcome = planetOpOutcome(outcome)
		w.postNews(planetOpNews(op.Op, from, outcome))
		return res
	}

	target := w.remoteTarget(op.TargetEmpire)
	if target == nil {
		res.Outcome = OutcomeNotFound
		return res
	}
	res.TargetEmpire = target.Name
	if target.Protection > 0 {
		res.Outcome = OutcomeProtected
		target.addEvent(fmt.Sprintf("A %s from %s broke on your New Realm Protection.", label, from))
		w.postNews(fmt.Sprintf("%s's New Realm Protection turned aside a %s from %s.",
			target.Name, label, op.FromBoard))
		return res
	}
	report, score, outcome := w.applySpecialOp(op.Op, target, from, op.Dial)
	res.Report = report
	res.Score = score
	res.Won = outcome == specialHit
	res.Backfired = outcome == specialBackfire
	res.Outcome = missileOutcome(outcome)
	w.postNews(missileNews(label, from, target.Name, outcome))
	return res
}

// missileOutcome is the verdict an arriving missile's answer carries home, so
// the firer's planet can word its line by the same outcome this one did. A
// backfire travels as a failure with Backfired set, the shape it had before.
func missileOutcome(outcome specialOutcome) AttackOutcome {
	switch outcome {
	case specialHit:
		return OutcomeWon
	case specialMisfire:
		return OutcomeMisfire
	case specialIntercepted:
		return OutcomeIntercepted
	case specialNothing:
		return OutcomeNegligible
	}
	return OutcomeRepelled
}

// planetOpOutcome is the verdict a bombing run's answer carries home, so the
// firer's planet can word its line by the same outcome this one did.
func planetOpOutcome(outcome specialOutcome) AttackOutcome {
	switch outcome {
	case specialHit:
		return OutcomeWon
	case specialDrivenOff:
		return OutcomeDrivenOff
	}
	return OutcomeNothing
}

// missileNews is the line this planet reads about a Special Operation aimed at
// one of its realms, worded by how it ended. The original has a failure and a
// success form per missile, and appends a sentence naming the SDI to the
// failure form when the shield was what stopped it; IB has one line per
// outcome, and its own words.
//
// A backfire is the one place IB's line departs from the original's choice
// rather than its wording: the original's receiver takes the success form for
// it (the backfire is a row of the damage mapper, not a failure), so its planet
// reads that the S3-Sabre struck. IB says what happened, which is what the
// firer's report and the target's own recap already say.
func missileNews(label, from, target string, outcome specialOutcome) string {
	switch outcome {
	case specialMisfire:
		return fmt.Sprintf("The %s from %s misfired on its way to %s.", label, from, target)
	case specialIntercepted:
		return fmt.Sprintf("%s's SDI shot down the %s from %s.", target, label, from)
	case specialBackfire:
		return fmt.Sprintf("The %s from %s broke up over %s.", label, from, target)
	case specialNothing:
		return fmt.Sprintf("The %s from %s reached %s and did little harm.", label, from, target)
	}
	return fmt.Sprintf("The %s from %s hit %s.", label, from, target)
}

// planetOpNews is the line this planet reads about a bombing op aimed at the
// whole of it, worded by how it ended. The original always posts one — the
// run that fails its landing roll names the realm that sent it, and each
// operation has a success line of its own — but has no form for a run that
// landed on nothing, because its receiver applies the percentage whatever is
// there. IB says so, as its report home does.
func planetOpNews(op SpecialOp, from string, outcome specialOutcome) string {
	if outcome == specialDrivenOff {
		return fmt.Sprintf("Bombers from %s were driven off before they reached %s.", from, planetOpObject(op, "the planet's"))
	}
	switch op {
	case OpBombFood:
		if outcome == specialHit {
			return fmt.Sprintf("Bombers from %s hit the planet's food market.", from)
		}
		return fmt.Sprintf("Bombers from %s hit the planet's food market and found it bare.", from)
	case OpBombMarket:
		if outcome == specialHit {
			return fmt.Sprintf("Bombers from %s wrecked the planet's trading market.", from)
		}
		return fmt.Sprintf("Bombers from %s hit the planet's trading market and found nothing listed there.", from)
	case OpBombRoutes:
		if outcome == specialHit {
			return fmt.Sprintf("Bombers from %s hit trade routes across the planet.", from)
		}
		return fmt.Sprintf("Bombers from %s found nothing moving on the planet's trade routes.", from)
	case OpUndermine:
		if outcome == specialHit {
			return fmt.Sprintf("Bombers from %s undermined investments across the planet.", from)
		}
		return fmt.Sprintf("Bombers from %s found nothing invested in the planet's bank to undermine.", from)
	}
	return fmt.Sprintf("An operation from %s against this planet came to nothing.", from)
}

// planetOpObject names what a bombing op was sent at, owned by whose — "the
// planet's" for this planet's news, "that planet's" for the firer's report.
func planetOpObject(op SpecialOp, whose string) string {
	switch op {
	case OpBombFood:
		return whose + " food market"
	case OpBombMarket:
		return whose + " trading market"
	case OpBombRoutes:
		return whose + " trade routes"
	case OpUndermine:
		return whose + " bank"
	}
	return "the planet"
}

// applyPlanetOp runs one of the four bombing ops against the whole planet.
//
// Each reaches what the whole planet holds, as the original's receiver does: the
// food market's supply, every listing on the Trading Market, every pending
// trade deal, every realm's investments near maturity. The landing roll comes
// first and covers all four.
func (w *World) applyPlanetOp(op SpecialOp, from string) (report string, outcome specialOutcome) {
	living := func() []*Empire {
		var out []*Empire
		for _, e := range w.Empires {
			if e.Alive {
				out = append(out, e)
			}
		}
		return out
	}
	tell := func(text string) {
		for _, e := range living() {
			e.addEvent(text)
		}
	}

	// One landing roll for the whole run, ahead of the op switch, as the
	// original rolls it; a run that fails it touches nothing on the planet.
	if !w.bombingLands() {
		return fmt.Sprintf("Your bombers were driven off before they reached %s.", planetOpObject(op, "that planet's")), specialDrivenOff
	}

	switch op {
	case OpBombFood:
		lost := w.bombFoodMarketEffect()
		if lost <= 0 {
			return "The food market on that planet was already bare.", specialNothing
		}
		tell(fmt.Sprintf("Bombers from %s hit the planet's food market — %s units of supply destroyed.",
			from, numfmt.Comma(int64(lost))))
		return fmt.Sprintf("You destroyed %d units of the planet's food supply.", lost), specialHit

	case OpBombMarket:
		goods, pct := 0, w.bombMarketLossPct()
		for _, e := range living() {
			goods += w.bombMarketPosition(e, pct)
		}
		if goods == 0 {
			return "Nothing was listed on that planet's trading market.", specialNothing
		}
		tell(fmt.Sprintf("Bombers from %s wrecked the planet's trading market — %s listed goods destroyed.",
			from, numfmt.Comma(int64(goods))))
		return fmt.Sprintf("You wrecked the planet's trading market: %d listed goods destroyed.", goods), specialHit

	case OpBombRoutes:
		hit := w.bombRoutesEffect()
		if hit == 0 {
			return "Nothing worth hitting was moving on that planet's trade routes.", specialNothing
		}
		tell(fmt.Sprintf("Bombers from %s hit trade routes across the planet — the goods in %d deals in transit were all but destroyed.", from, hit))
		return fmt.Sprintf("You wrecked the goods in %d trade deals on that planet.", hit), specialHit

	case OpUndermine:
		var lost int64
		pct := w.undermineLossPct()
		for _, e := range living() {
			lost += w.undermineEffect(e, pct)
		}
		if lost == 0 {
			return "Nothing was invested on that planet to undermine.", specialNothing
		}
		tell(fmt.Sprintf("Bombers from %s undermined the planet's bank — %s gold in principal lost.",
			from, numfmt.Comma(lost)))
		return fmt.Sprintf("You undermined the planet's investments: %d gold lost.", lost), specialHit
	}
	return "Nothing came of the operation.", specialNothing
}

// applySpecialOp runs an arriving missile's effect against d and reports what it
// did, what the attacker earned, and how it ended.
//
// Only the three missiles reach it: resolveRemoteSpecialOp sends every op that
// TargetsPlanet down applyPlanetOp first. The four bombing ops had per-realm
// branches here, from when they were aimed at one baron, which no packet could
// reach once they were aimed at the planet; they were removed on 2026-09-23.
func (w *World) applySpecialOp(op SpecialOp, d *Empire, from string, dial int) (report string, score int, outcome specialOutcome) {
	switch op {
	// The three missiles do NOT run the local helpers of the same name (#255).
	// The receiving board resolves all three in one routine with its own gates
	// and its own bands (BRE.OVR ovr_0450a9 +0x3c5) — an arriving nuclear strike
	// ruins a wider swathe than a neighbor's, and an arriving chemical strike is
	// a population weapon that touches no land at all.
	case OpNuclear:
		if stopped, why := w.arrivingMissileStopped(d, "nuclear strike"); stopped != "" {
			d.addEvent(fmt.Sprintf("A nuclear strike from %s never reached your empire.", from))
			return stopped, 0, why
		}
		regions := w.arrivingNuclearEffect(d)
		score = w.rng.Intn(NukeScoreRoll)
		d.addEvent(fmt.Sprintf("%s hit you with a nuclear strike: %d regions reduced to waste.", from, regions))
		return fmt.Sprintf("Nuclear strike! %d regions of %s are now waste.", regions, d.Name), score, specialHit

	case OpChemical:
		if stopped, why := w.arrivingMissileStopped(d, "chemical strike"); stopped != "" {
			d.addEvent(fmt.Sprintf("A chemical strike from %s never reached your empire.", from))
			return stopped, 0, why
		}
		people := w.arrivingChemicalEffect(d)
		score = w.rng.Intn(ChemScoreRoll)
		d.addEvent(fmt.Sprintf("%s hit you with a chemical strike: %d of your people are dead.", from, people))
		return fmt.Sprintf("Chemical strike! %d of %s's people are dead.", people, d.Name), score, specialHit

	case OpSabre:
		report, outcome = w.sabreEffect(d, from, dial)
		return report, 0, outcome
	}
	return fmt.Sprintf("Nothing came of the operation against %s.", d.Name), 0, specialNothing
}
