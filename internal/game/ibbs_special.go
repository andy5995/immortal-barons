package game

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// Interplanetary Special Operations (#49) — the InterPlanetary Ops menu's
// Special Operations submenu, where every op is aimed at a baron on another
// planet.
//
// The original resolves these the same way IB already resolves terror ops: its
// menu handler (`run_bombing_operations_menu`, BRE.OVR 0x029ea9) calls
// `write_special_operation_packet` rather than touching the target, so the
// effect lands when the packet does. That is what this file implements — the
// launch side deducts the cost and books the op in flight, the target board
// applies it and answers, and the lost-forces timer hands the op back if the
// answer never arrives.
//
// The EFFECTS are deliberately not written here: each one calls the same helper
// the local Bomb Enemy Targets op calls, so a tuning change lands on both. What
// differs cross-planet is only what cannot travel — the attacker is not present
// to roll a covert success against, and its score is awarded when the answer
// comes home rather than on the spot.

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
}

// SpecialOpGoldCost prices one op for e.
//
// The four bombing ops carry the flat prices the original prints in the menu's
// own price column (captured live; see docs/dev/bre-screens.md). The three
// missiles show NO price there, and cannot: the local versions price off the
// TARGET's size, which is exactly what a board does not know about a realm on
// another planet. So they are priced off the launcher instead, the way terror
// ops already are — an IB decision, recorded in docs/mechanics-reference.md.
func (w *World) SpecialOpGoldCost(e *Empire, op SpecialOp) int64 {
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
	default:
		cost = int64(e.Land) * IPMissileGoldPerRegion
	}
	return cost * int64(w.Config.TerrorCosts.CostPercent()) / 100
}

// SendSpecialOp queues an op against targetEmpire on targetBoard. The gold goes
// now and the op is booked in flight, so a packet that never comes back is
// swept by the same lost-forces timer that returns an attack.
func (w *World) SendSpecialOp(e *Empire, targetBoard, targetEmpire string, op SpecialOp) error {
	if isMissileOp(op) {
		if !w.Config.MissileOps {
			return ErrMissileOpsDisabled
		}
	} else if !w.Config.BombingOps {
		return ErrBombingOpsDisabled
	}
	if !w.CanBombingOp(e) {
		return ErrBombingOpsExhausted
	}
	// The original requires the bombers for every op on this menu, missiles
	// included: "All missiles and bombs require 500 Bombers to deliver their
	// payloads". The local menu enforces the same floor.
	if e.Bombers < BombingBombersRequired {
		return ErrNeedBombers
	}
	cost := w.SpecialOpGoldCost(e, op)
	if e.Gold < cost {
		return ErrCantAfford
	}
	if op.TargetsPlanet() {
		targetEmpire = "" // nothing on the far side should look for a realm
	}
	e.Gold -= cost
	e.BombingOpsToday++
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
	})
	return nil
}

// resolveRemoteSpecialOp applies an inbound op to this board's target and
// returns the answer that rides home.
//
// A realm under New Realm Protection is untouched, as it is for a terror op and
// an attack: the protection is the point, and an op that could slip past it
// would make the strongest weapon the one that ignores the only shield a new
// player has.
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
	// down around it.
	if op.Op.TargetsPlanet() {
		report, hit := w.applyPlanetOp(op.Op, from)
		res.Report = report
		res.Won = hit
		if hit {
			res.Outcome = OutcomeWon
			w.postNews(fmt.Sprintf("%s struck this planet: %s.", from, label))
		} else {
			res.Outcome = OutcomeRepelled
		}
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
	report, score, hit, backfired := w.applySpecialOp(op.Op, target, from)
	res.Report = report
	res.Score = score
	res.Won = hit
	res.Backfired = backfired
	if hit {
		res.Outcome = OutcomeWon
	} else {
		res.Outcome = OutcomeRepelled
	}
	w.postNews(fmt.Sprintf("%s struck %s from %s.", from, target.Name, w.Config.BoardID))
	return res
}

// applyPlanetOp runs one of the four bombing ops against the whole planet.
//
// Each is the planet-wide reading of the local op's effect: the food market's
// own supply rather than one realm's stores, every listing on the Trading Market
// rather than one realm's position, every realm's trade agreements, every
// realm's investments. The per-realm helpers are still the ones doing the work,
// so the local menu and this one stay in step.
func (w *World) applyPlanetOp(op SpecialOp, from string) (report string, hit bool) {
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

	switch op {
	case OpBombFood:
		lost := w.FoodMarketSupply / 2
		if lost <= 0 {
			return "The food market on that planet was already bare.", false
		}
		w.FoodMarketSupply -= lost
		tell(fmt.Sprintf("Bombers from %s hit the planet's food market — %s units of supply destroyed.",
			from, numfmt.Comma(int64(lost))))
		return fmt.Sprintf("You destroyed %d units of the planet's food supply.", lost), true

	case OpBombMarket:
		goods, proceeds := 0, int64(0)
		for _, e := range living() {
			g, p := w.bombMarketPosition(e, BombMarketLossPct)
			goods, proceeds = goods+g, proceeds+p
		}
		if goods == 0 && proceeds == 0 {
			return "Nothing was listed on that planet's trading market.", false
		}
		tell(fmt.Sprintf("Bombers from %s wrecked the planet's trading market — %s listed goods and %s gold in proceeds destroyed.",
			from, numfmt.Comma(int64(goods)), numfmt.Comma(proceeds)))
		return fmt.Sprintf("You wrecked the planet's trading market: %d goods and %d gold in proceeds.", goods, proceeds), true

	case OpBombRoutes:
		// nil takes every deal on the planet, and the strike's one landing roll
		// covers the whole planet rather than being rolled per realm.
		hit := 0
		if w.bombRoutesLands() {
			hit = w.bombRoutesEffect(nil)
		}
		if hit == 0 {
			return "Nothing worth hitting was moving on that planet's trade routes.", false
		}
		tell(fmt.Sprintf("Bombers from %s hit trade routes across the planet — the goods in %d deals in transit were all but destroyed.", from, hit))
		return fmt.Sprintf("You wrecked the goods in %d trade deals on that planet.", hit), true

	case OpUndermine:
		var lost int64
		for _, e := range living() {
			lost += undermineEffect(e)
		}
		if lost == 0 {
			return "Nothing was invested on that planet to undermine.", false
		}
		tell(fmt.Sprintf("Agents from %s undermined the planet's bank — %s gold in principal lost.",
			from, numfmt.Comma(lost)))
		return fmt.Sprintf("You undermined the planet's investments: %d gold lost.", lost), true
	}
	return "Nothing came of the operation.", false
}

// applySpecialOp runs one op's effect against d and reports what it did, what
// the attacker earned, and whether anything landed.
//
// Every branch calls the helper the LOCAL op calls, so the two menus cannot
// drift apart: a change to what bombing a food market does lands on both.
func (w *World) applySpecialOp(op SpecialOp, d *Empire, from string) (report string, score int, hit, backfired bool) {
	switch op {
	case OpBombFood:
		lost := bombFoodEffect(d)
		d.addEvent(fmt.Sprintf("Bombers from %s torched your food stores — %d units lost.", from, lost))
		return fmt.Sprintf("You destroyed %d units of %s's food.", lost, d.Name), 0, lost > 0, false

	case OpBombMarket:
		goods, proceeds := w.bombMarketPosition(d, BombMarketLossPct)
		if goods == 0 && proceeds == 0 {
			d.addEvent(fmt.Sprintf("Bombers from %s hit your trading market, which stood empty.", from))
			return fmt.Sprintf("%s had nothing on the market to destroy.", d.Name), 0, false, false
		}
		d.addEvent(fmt.Sprintf("Bombers from %s wrecked your trading market — %d listed goods and %d gold in proceeds destroyed.", from, goods, proceeds))
		return fmt.Sprintf("You wrecked %s's trading market: %d goods and %d gold in proceeds.", d.Name, goods, proceeds), 0, true, false

	case OpBombRoutes:
		hit := 0
		if w.bombRoutesLands() {
			hit = w.bombRoutesEffect(d)
		}
		if hit == 0 {
			d.addEvent(fmt.Sprintf("Bombers from %s struck at your trade routes and found nothing.", from))
			return fmt.Sprintf("%s had no trade deals in transit worth hitting.", d.Name), 0, false, false
		}
		d.addEvent(fmt.Sprintf("Bombers from %s hit your trade routes — the goods in %d deals in transit were all but destroyed.", from, hit))
		return fmt.Sprintf("You wrecked the goods in %d of %s's trade deals in transit.", hit, d.Name), 0, true, false

	case OpUndermine:
		lost := undermineEffect(d)
		if lost == 0 {
			d.addEvent(fmt.Sprintf("Agents from %s went looking for investments you do not hold.", from))
			return fmt.Sprintf("%s has no investments to undermine.", d.Name), 0, false, false
		}
		d.addEvent(fmt.Sprintf("Agents from %s undermined your investments — %d gold in principal lost.", from, lost))
		return fmt.Sprintf("You undermined %s's investments: %d gold lost.", d.Name, lost), 0, true, false

	case OpNuclear:
		regions := w.nuclearEffect(d)
		score = w.rng.Intn(NukeScoreAward)
		d.addEvent(fmt.Sprintf("%s hit you with a nuclear strike: %d regions reduced to waste.", from, regions))
		return fmt.Sprintf("Nuclear strike! %d regions of %s are now waste.", regions, d.Name), score, true, false

	case OpChemical:
		regions, people := w.chemicalEffect(d)
		score = w.rng.Intn(ChemScoreAward)
		d.addEvent(fmt.Sprintf("%s hit you with a chemical strike: %d regions reduced to waste and %d dead. Famine follows.", from, regions, people))
		return fmt.Sprintf("Chemical strike! %d regions of %s are now waste, and %d of its people are dead.", regions, d.Name, people), score, true, false

	case OpSabre:
		report, hit, backfired = w.sabreEffect(d, from)
		return report, 0, hit, backfired
	}
	return fmt.Sprintf("Nothing came of the operation against %s.", d.Name), 0, false, false
}
