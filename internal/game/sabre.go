package game

import (
	"fmt"
	"math/big"
	"strings"
)

// sabre.go — the S3-Sabre missile: the dial the launch carries, the mapper from
// that dial to an effect, and what the effect does where the strike lands.
//
// It is NOT a covert operation, which is where this code sat until 2026-09-14.
// The Sabre is an interplanetary weapon on the InterPlanetary Special Operations
// menu beside the nuclear and chemical strikes, and it reaches a target through
// ibbs_special.go, not through an agent. It had grown up inside covert.go next
// to the shared Bomb Enemy Targets effects; this file is a pure move of that
// block, no behavior changed.
//
// The mapper's table and the two rolls that blur the dial live in
// balance_costs.go with the rest of the binary-verified numbers.

// sabreDialFor settles the dial the launch actually carries. Only User Select
// handling uses the number the player typed; Random rolls one per launch and
// Constant fires the same setting every time. Under the sysop's None mode the
// menu does not offer the missile at all.
func (w *World) sabreDialFor(dial int) int {
	clamp := func(n int) int { return min(max(n, SabreDialMin), SabreDialMax) }
	switch w.Config.SabreHandling {
	case SabreRandom:
		return w.rng.Intn(SabreDialMax - SabreDialMin + 1)
	case SabreConstant:
		return clamp(w.Config.SabreConstantDial)
	}
	return clamp(dial)
}

// SabreAim turns the dial the firer set into the effect that lands, applying the
// two rolls the original wraps around its mapper: one launch in SabreWildOdds
// ignores the dial and takes a wholly random row, and otherwise the dial is
// nudged by one either way before the table is read. See the table in
// balance_costs.go for the binary's addresses.
func (w *World) SabreAim(dial int) SabreEffect {
	if w.rng.Intn(SabreWildOdds) == 0 {
		return sabreDialTable[w.rng.Intn(SabreDialWrap)]
	}
	n := dial + w.rng.Intn(SabreDialJitterSides) - w.rng.Intn(SabreDialJitterSides)
	n %= SabreDialWrap
	if n < 0 {
		n += SabreDialWrap
	}
	return sabreDialTable[n]
}

// sabreDamage applies a landed S3-Sabre hit to e and returns a human-readable
// list of what was destroyed (empty if the roll removed nothing). The effect
// decides WHAT is hit and the row's own formula how much — both the original's,
// read from its effect switch (the Sabre*Keep* and SabreRegionLoss* constants
// carry the addresses).
//
// The field each branch touches, own-record: covert agents at +0x26f for the
// Intelligence Headquarters (it never touches the HeadQuarters at +0x26b),
// population at +0x62, food at +0x6e, jets alone at +0x7e for airbases, and all
// four of troopers, jets, turrets and tanks for military bases, each with its
// own roll. Regions go through the RegionMix, whose Total must always equal
// e.Land; they are destroyed, not turned to waste.
func (w *World) sabreDamage(e *Empire, eff SabreEffect) string {
	row, ok := sabreRows[eff]
	if !ok {
		return ""
	}
	s := &sabreStrike{w: w, e: e}
	row.hit(s)
	return strings.Join(s.parts, ", ")
}

// sabreRow is one of the six damage rows: what it goes for, as the target's
// event names it when a hit takes nothing, and what it does.
type sabreRow struct {
	aim string
	hit func(s *sabreStrike)
}

// sabreRows is the one table of the dial's damage rows. SabreDevelopRegions is
// not in it: a backfire applies that row directly (sabreDevelop).
var sabreRows = map[SabreEffect]sabreRow{
	SabreHitIntelligence: {"Intelligence Headquarters", func(s *sabreStrike) {
		s.keep("Agents", &s.e.Agents, SabreIntelKeepSpread, sabreIntelKept)
	}},
	SabreHitPeople: {"residential zones", func(s *sabreStrike) {
		s.keep("People", &s.e.People, SabrePeopleKeepSpread, func(n, roll int) int {
			return pctOf(n, SabrePeopleKeepBasePct+roll)
		})
	}},
	SabreHitMilitaryBases: {"military bases", func(s *sabreStrike) {
		for _, g := range []*Good{Trooper, Jet, Turret, Tank} {
			s.keep(g.Plural, g.Count(s.e), SabreBasesKeepSpread, func(n, roll int) int {
				return pctOf(n, SabreBasesKeepBasePct+roll)
			})
		}
	}},
	SabreHitAirbases: {"airbases", func(s *sabreStrike) {
		s.keep(Jet.Plural, Jet.Count(s.e), SabreAirbaseKeepSpread, sabreAirbaseKept)
	}},
	SabreHitRegions: {"regions", func(s *sabreStrike) {
		e := s.e
		lost := pctOf(e.Land, SabreRegionLossBasePct+s.w.rng.Intn(SabreRegionLossSpread))
		if lost > 0 {
			lost = e.Regions.remove(lost).Total()
			e.syncLand()
			noun := "Regions"
			if lost == 1 {
				noun = "Region"
			}
			s.parts = append(s.parts, fmt.Sprintf("%d %s", lost, noun))
		}
	}},
	SabreHitFood: {"food supply", func(s *sabreStrike) {
		s.keep("Food", &s.e.Food, SabreFoodKeepSpread, sabreFoodKept)
	}},
}

// sabreStrike is one landed hit in progress: the realm it struck and the list
// of what it destroyed.
type sabreStrike struct {
	w     *World
	e     *Empire
	parts []string
}

// keep leaves *n at kept(*n, Random(spread)) and records what went.
func (s *sabreStrike) keep(name string, n *int, spread int, kept func(n, roll int) int) {
	left := kept(*n, s.w.rng.Intn(spread))
	if lost := *n - left; lost > 0 {
		*n = left
		s.parts = append(s.parts, fmt.Sprintf("%d %s", lost, name))
	}
}

// The three rows below are the ones whose Real48 share is not the integer
// percent it stands for: each comes out one lower than integer math wherever
// count x share should land exactly on a whole number and the rounded share
// sits just under it. The people and military base rows never do (checked
// against every count to 2^31 - 1), so they stay pctOf.

// sabreIntelKept is Trunc(agents x (Random(30) / 100 + 0.7)) in the original's
// Real48 (resolve_received_sabre_strike +0x7e3..+0x839: RFloat(roll) / 100.0,
// RAdd 0.7, times RFloat(agents), RTrunc). 300 agents on a roll of 3 keep 218,
// not 219.
func sabreIntelKept(n, roll int) int {
	share := r48Add(r48Div(r48Int(int64(roll)), r48Int(100)), r48(big.NewRat(SabreIntelKeepBasePct, 100)))
	return int(r48Trunc(r48Mul(r48Int(int64(n)), share)))
}

// sabreAirbaseKept is Trunc(jets x ((Random(40) + 50) / 100)) in the original's
// Real48 (+0xacd..+0xb22: RFloat(roll) + 50.0, / 100.0, times RFloat(jets),
// RTrunc). The add is exact; the divide is not. 100 jets on a roll of 11 keep
// 60, not 61.
func sabreAirbaseKept(n, roll int) int {
	share := r48Div(r48Int(int64(SabreAirbaseKeepBasePct+roll)), r48Int(100))
	return int(r48Trunc(r48Mul(r48Int(int64(n)), share)))
}

// sabreFoodKept is Trunc(Random(30) x (food / 100)) in the original's Real48
// (+0xbb2..+0xbfa: RFloat(food) / 100.0, times RFloat(roll), RTrunc). 244 food
// on a roll of 25 keeps 60, not 61.
func sabreFoodKept(n, roll int) int {
	x := r48Div(r48Int(int64(n)), r48Int(100))
	return int(r48Trunc(r48Mul(r48Int(int64(roll)), x)))
}

// sabreDevelop is the mapper's last row: the backfire hands the TARGET
// 10-19% of its own region count as land it may keep, untyped, and returns how
// many. BINARY-VERIFIED — the original computes
// trunc((Random(10) + 10) / 100 x total regions) and adds it to the record's
// untyped-region slot (+0xBA), which its own total_regions deliberately skips
// and its region picker drains. PendingRegions is IB's slot of the same kind,
// fed by a won interplanetary strike and drained by the same picker, so the
// land arrives without a type and the owner chooses at the start of their next
// turn (#107, #266).
func (w *World) sabreDevelop(d *Empire) int {
	got := pctOf(d.Land, SabreDevelopBasePct+w.rng.Intn(SabreDevelopSpread))
	if got <= 0 {
		return 0
	}
	d.PendingRegions += got
	return got
}

// sabreBackfires reports whether the missile turns back on whoever
// fired it: the more Troopers the target garrisons, the likelier it does.
func (w *World) sabreBackfires(d *Empire) bool {
	return w.rng.Intn(100) < d.Troopers/SabreBackfireScale
}

// sabreEffect is the target-side half of the missile, for a strike that
// arrived from another planet (#49). It runs the gates every arriving missile
// meets, then IB's backfire roll (#266), then the dial.
//
// A backfire is applied HERE, to the realm that was aimed at: it costs the
// board that fired nothing and develops land for the target (#266, and
// sabreDevelop for what the original computes). The firer learns of it when the
// answer gets home, which is a delay the packet imposes rather than a rule of
// its own.
//
// This is the one covert event in this file that names the source on SUCCESS,
// and it is deliberate: BRE treats a Sabre that lands from another planet as a
// missile impact rather than an agent op and reports it with the firing realm
// and its planet, the same as an incoming nuclear or chemical strike. Agent ops
// stay anonymous unless the agent is caught (see covertFoiled).
//
// gained is the land a backfire opened for the target, zero otherwise.
func (w *World) sabreEffect(d *Empire, from string, dial int) (report string, outcome specialOutcome, gained int) {
	// The shared arriving-missile gates: the misfire, SDI and the garrison
	// (#255). All three missiles meet them, because the receiving board resolves
	// all three in one routine and the rolls sit ahead of its damage switch.
	//
	// IB used to fizzle 7 launches in 10 here instead. That roll was invented for
	// a gap that the resolver turned out to fill — read on 2026-09-03, after the
	// roll was written — and it is three times harsher than the original's gate,
	// so it goes rather than stacking with it. The rolls INSIDE the original's
	// sabre branch (`+0x6b6`) are the dial jitter, which SabreAim already models:
	// one launch in ten ignores the dial entirely and the rest are nudged by one
	// either way. Nothing there asks a second time whether the missile works.
	//
	// The third gate is the original's garrison roll, which for the Sabre reads
	// the target's troopers (record +0x76). It sits ahead of the damage switch,
	// so a Sabre it stops neither damages nor backfires.
	if stopped, why := w.stopArrivingMissile(d, OpSabre, from); stopped != "" {
		return stopped, why, 0
	}
	board := w.Config.BoardID
	if w.sabreBackfires(d) {
		// A backfire is the original's route to the mapper's last row: it does not
		// hurt the firer, it DEVELOPS land for the realm they aimed at (#266). The
		// firer is told when the answer gets home; the target sees it now.
		if got := w.sabreDevelop(d); got > 0 {
			d.addEvent(fmt.Sprintf("%s's S3-Sabre backfired, expanding your territory by %s.", from, regionCount(got)))
			return fmt.Sprintf("Your S3-Sabre backfired on %s of %s, expanding their territory by %s.", d.Name, board, regionCount(got)), specialBackfire, got
		}
		return fmt.Sprintf("Your S3-Sabre backfired on %s of %s, but they had too little land for it to give them any.", d.Name, board), specialBackfire, 0
	}
	eff := w.SabreAim(dial)
	lost := w.sabreDamage(d, eff)
	if lost == "" {
		// The original tells the target which row hit even when it took
		// nothing: the event writer at +0x07d6 runs before the damage switch.
		d.addEvent(fmt.Sprintf("%s's S3-Sabre hit your %s and did little harm.", from, sabreEffectAim(eff)))
		return fmt.Sprintf("Your S3-Sabre reached %s of %s and barely scratched the paint.", d.Name, board), specialNothing, 0
	}
	d.addEvent(fmt.Sprintf("%s's S3-Sabre hit your %s, destroying %s.", from, sabreEffectAim(eff), lost))
	return fmt.Sprintf("Your S3-Sabre hit %s of %s, destroying %s.", d.Name, board, lost), specialHit, 0
}

// regionCount is "1 region" or "N regions".
func regionCount(n int) string {
	if n == 1 {
		return "1 region"
	}
	return fmt.Sprintf("%d regions", n)
}

// sabreEffectAim names what a dial row goes for, for the target's event.
func sabreEffectAim(eff SabreEffect) string {
	if row, ok := sabreRows[eff]; ok {
		return row.aim
	}
	return "realm"
}
