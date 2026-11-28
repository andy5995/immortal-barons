package game

import (
	"math"
	"slices"
)

// attack_capture.go — the capture_<unit> effects (#373): a battle's mod can let
// one side take a share of the other's military. The percent is of the units
// still alive after the kills, so a unit an effect destroyed is never also
// captured, and it is drawn per unit type once, whoever the units belong to.

// numCaptureGoods is len(MilitaryGoods); a test holds the two together. The
// units a mod can capture are the six an Industrial region builds, and they sit
// in AttackKillGoods' first six slots, in the same order.
const numCaptureGoods = 6

// unitCounts is a count of each captureable unit, in MilitaryGoods order.
type unitCounts [numCaptureGoods]int

func (u unitCounts) isZero() bool { return u == unitCounts{} }

// captureKey is the key of the capture effect on g.
func captureKey(g *Good) string { return "capture_" + g.ID }

// Keys of the two capture settings that are not a unit's.
const (
	keyCaptureRandom    = "capture_random"
	keyCaptureFromStock = "capture_from_stock"
)

// defenseGoods are the units that stand in a defense and bleed in its battle,
// local or interplanetary (loseForcesSplit): a defender's default capture base.
var defenseGoods = []*Good{Trooper, Jet, Turret, Tank}

// captureSource is one holding units can be taken from: a realm, an ally's
// detachment, or a returning detachment. holds is where each count lives, nil
// for a unit the holding cannot have, and base is the most of each that may be
// taken (the survivors of the force that took part), never past the count.
type captureSource struct {
	// ally is the realm whose detachment this is, told what was taken.
	ally  *Empire
	holds [numCaptureGoods]*int
	base  unitCounts
}

// empireSource is e's holdings as a source, with the base per unit type given.
func empireSource(e *Empire, base unitCounts) captureSource {
	s := captureSource{base: base}
	for i, g := range MilitaryGoods {
		s.holds[i] = g.Count(e)
	}
	return s
}

// forceSource is a detachment as a source; the base is what it holds now.
func forceSource(f *AttackForce) captureSource {
	var s captureSource
	for i, g := range MilitaryGoods {
		if g.Force != nil {
			s.holds[i] = g.Force(f)
			// An arriving force is the sender's word, so its count is held to
			// what a realm can hold.
			s.base[i] = min(max(*s.holds[i], 0), MaxUnits)
		}
	}
	return s
}

// defenderBase is what of a defending realm's units a capture may draw on: the
// units that stood in the defense, or with fromStock its whole stock.
func defenderBase(e *Empire, fromStock bool) unitCounts {
	var b unitCounts
	for i, g := range MilitaryGoods {
		if fromStock || slices.Contains(defenseGoods, g) {
			b[i] = max(*g.Count(e), 0)
		}
	}
	return b
}

// takeUnits applies the capture keys of fx to the sources and returns what was
// taken from each. A percent is drawn only for a key that is set, so a set with
// none draws nothing. The named units are drawn in unit order, then the random
// share, and each drawn percent applies to every source.
func (w *World) takeUnits(fx EffectSet, srcs []captureSource) []unitCounts {
	took := make([]unitCounts, len(srcs))
	avail := func(s *captureSource, i int) int {
		if s.holds[i] == nil {
			return 0
		}
		return min(s.base[i], max(*s.holds[i], 0))
	}
	take := func(s, i, n int) {
		src := &srcs[s]
		n = min(n, avail(src, i))
		if n <= 0 {
			return
		}
		*src.holds[i] -= n
		src.base[i] -= n
		took[s][i] += n
	}
	for i, r := range fx.CaptureUnits {
		if r.IsZero() {
			continue
		}
		pct := w.roll(r)
		for s := range srcs {
			take(s, i, pctOf(avail(&srcs[s], i), pct))
		}
	}
	if !fx.CaptureRandom.IsZero() {
		pct := w.roll(fx.CaptureRandom)
		var total int64
		for s := range srcs {
			for i := range numCaptureGoods {
				total += int64(avail(&srcs[s], i))
			}
		}
		w.takeRandom(srcs, avail, take, total, pctOf(total, pct))
	}
	return took
}

// takeRandom takes m of the total units the sources hold, drawn at random
// across every source and unit type. It walks the holdings in order and draws
// how many of each the sample holds from the hypergeometric distribution's
// normal approximation, held to what is possible, so the sample is exactly m and
// costs one draw per holding rather than one per unit. The last holding takes
// what is left.
func (w *World) takeRandom(srcs []captureSource, avail func(*captureSource, int) int, take func(s, i, n int), total, m int64) {
	left, need := total, m
	type cell struct{ s, i, n int }
	var cells []cell
	for s := range srcs {
		for i := range numCaptureGoods {
			if n := avail(&srcs[s], i); n > 0 {
				cells = append(cells, cell{s, i, n})
			}
		}
	}
	for k, c := range cells {
		if need <= 0 {
			return
		}
		var x int64
		n := int64(c.n)
		if k == len(cells)-1 {
			x = need
		} else {
			lo, hi := max(0, need-(left-n)), min(n, need)
			x = hi
			if lo < hi {
				p := float64(n) / float64(left)
				mean := float64(need) * p
				v := float64(need) * p * (1 - p) * float64(left-need) / float64(max(left-1, 1))
				x = min(max(int64(math.Round(mean+w.rng.NormFloat64()*math.Sqrt(max(v, 0)))), lo), hi)
			}
		}
		take(c.s, c.i, int(min(x, math.MaxInt32)))
		left -= n
		need -= x
	}
}

// captureCtx says whose units a set takes from, when that is not simply a
// defending realm's.
type captureCtx struct {
	// sent is the force an attacking realm sent, as it stands after the battle:
	// the base of a capture from the attacker.
	sent *unitCounts
	// allies are the Full Defense Alliance detachments that stood with the
	// defender, each with what of it survived.
	allies []allyDetachment
	// own is set by applySet to what was taken from the victim itself, apart
	// from its allies' detachments.
	own unitCounts
}

// allyDetachment is one partner's committed troops in a defense.
type allyDetachment struct {
	ally *Empire
	left unitCounts
}

// allyDetachments are d's Full Defense Alliance partners' committed troops
// (their 30% of troopers and tanks) as they stand once a battle that cost the
// defender frac is over; call it before bleedAllies takes the loss.
func (w *World) allyDetachments(d *Empire, frac float64) []allyDetachment {
	var out []allyDetachment
	for _, ally := range w.alliesOf(d, fullDefenseAlliance) {
		var left unitCounts
		left[slices.Index(MilitaryGoods, Trooper)] = allySent(ally.Troopers) - allyBled(ally.Troopers, frac)
		left[slices.Index(MilitaryGoods, Tank)] = allySent(ally.Tanks) - allyBled(ally.Tanks, frac)
		out = append(out, allyDetachment{ally, left})
	}
	return out
}

// takeFrom runs fx's unit capture against victim and, when the victim is a
// defender, its allies' detachments. It credits other, when there is one, and
// tells each ally what was taken. It returns what was taken from the victim and
// from the allies together; the realm's own part is the first figure.
func (w *World) takeFrom(fx EffectSet, victim, other *Empire, cx *captureCtx) (own, all unitCounts) {
	// The sums saturate: a defender and two allies near MaxUnits wrap a 32-bit int.
	if !fx.capturesUnits() {
		return
	}
	var base unitCounts
	switch {
	case cx != nil && cx.sent != nil:
		base = *cx.sent
	default:
		base = defenderBase(victim, fx.CaptureFromStock != 0)
	}
	srcs := []captureSource{empireSource(victim, base)}
	if cx != nil {
		for _, a := range cx.allies {
			s := empireSource(a.ally, a.left)
			s.ally = a.ally
			srcs = append(srcs, s)
		}
	}
	took := w.takeUnits(fx, srcs)
	for s, t := range took {
		for i := range t {
			all[i] = addUnits(all[i], t[i])
			if s == 0 {
				own[i] = addUnits(own[i], t[i])
			}
		}
		if s > 0 && !t.isZero() {
			srcs[s].ally.addEvent(say("{who} captured {units} of the troops you sent to defend {name}.",
				"who", otherName(other), "units", capturedList(t), "name", victim.Name))
		}
	}
	if other != nil {
		for i, g := range MilitaryGoods {
			*g.Count(other) = addUnits(*g.Count(other), all[i])
		}
	}
	return own, all
}

// otherName names the realm that took units, for an ally's notice.
func otherName(e *Empire) string {
	if e == nil {
		return ""
	}
	return e.Name
}

// capturesUnits reports whether the set takes any units.
func (s EffectSet) capturesUnits() bool {
	return s.CaptureUnits != [numCaptureGoods]Roll{} || !s.CaptureRandom.IsZero()
}

// capturedList is the units in u as counted phrases, empty when none.
func capturedList(u unitCounts) []Msg {
	var units []unitCount
	for i, g := range MilitaryGoods {
		units = append(units, unitCount{u[i], g})
	}
	return unitList(units, short[int])
}
