package game

import (
	"fmt"
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
// Constant fires the same setting every time. The sysop's None mode is refused
// in the menu, before a target is picked.
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
// decides WHAT is hit — the original's own mapping, read from the fields each
// branch of its effect switch writes back. How much is binary-verified for the
// Intelligence Headquarters row (SabreIntelKeep*); the other rows still use
// IB's own 5-30%.
//
// The field each branch touches, own-record: covert agents at +0x26f for the
// Intelligence Headquarters (it never touches the HeadQuarters at +0x26b),
// population at +0x62, food at +0x6e, jets alone at +0x7e for airbases, and all
// four of troopers, jets, turrets and tanks for military bases. Regions go
// through the RegionMix, whose Total must always equal e.Land.
func (w *World) sabreDamage(e *Empire, eff SabreEffect) string {
	pct := func() int { return SabreBaseDamagePct + w.rng.Intn(SabreDamageSpread) }
	var parts []string
	take := func(name string, n *int) {
		lost := pctOf(*n, pct())
		if lost <= 0 {
			return
		}
		*n -= lost
		parts = append(parts, fmt.Sprintf("%d %s", lost, name))
	}
	switch eff {
	case SabreHitIntelligence:
		keep := e.Agents * (SabreIntelKeepBasePct + w.rng.Intn(SabreIntelKeepSpread)) / 100
		if lost := e.Agents - keep; lost > 0 {
			e.Agents = keep
			parts = append(parts, fmt.Sprintf("%d Agents", lost))
		}
	case SabreHitPeople:
		take("People", &e.People)
	case SabreHitMilitaryBases:
		take(Trooper.Plural, Trooper.Count(e))
		take(Jet.Plural, Jet.Count(e))
		take(Turret.Plural, Turret.Count(e))
		take(Tank.Plural, Tank.Count(e))
	case SabreHitAirbases:
		take(Jet.Plural, Jet.Count(e))
	case SabreHitFood:
		take("Food", &e.Food)
	case SabreHitRegions:
		lost := e.Land * pct() / 100
		if lost > 0 {
			e.Regions.remove(lost)
			e.syncLand()
			parts = append(parts, fmt.Sprintf("%d Regions", lost))
		}
	}
	return strings.Join(parts, ", ")
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
	got := d.Land * (SabreDevelopBasePct + w.rng.Intn(SabreDevelopSpread)) / 100
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
// arrived from another planet (#49). It runs the same shield, fizzle and
// backfire rolls the local strike runs, in the same order.
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
func (w *World) sabreEffect(d *Empire, from string, dial int) (report string, outcome specialOutcome) {
	// The shared arriving-missile gates: the misfire, then SDI (#255). All three
	// missiles meet them, because the receiving board resolves all three in one
	// routine and both rolls sit ahead of its damage switch.
	//
	// IB used to fizzle 7 launches in 10 here instead. That roll was invented for
	// a gap that the resolver turned out to fill — read on 2026-09-03, after the
	// roll was written — and it is three times harsher than the original's gate,
	// so it goes rather than stacking with it. The rolls INSIDE the original's
	// sabre branch (`+0x6b6`) are the dial jitter, which SabreAim already models:
	// one launch in ten ignores the dial entirely and the rest are nudged by one
	// either way. Nothing there asks a second time whether the missile works.
	if stopped, why := w.arrivingMissileStopped(d, "S3-Sabre"); stopped != "" {
		return stopped, why
	}
	if w.sabreBackfires(d) {
		// A backfire is the original's route to the mapper's last row: it does not
		// hurt the firer, it DEVELOPS land for the realm they aimed at (#266). The
		// firer is told when the answer gets home; the target sees it now.
		if got := w.sabreDevelop(d); got > 0 {
			d.addEvent(fmt.Sprintf("An S3-Sabre from %s broke up over your realm, and the fallout left %d Regions fit to settle.", from, got))
			return fmt.Sprintf("Your S3-Sabre broke up over %s and opened %d Regions for them to settle.", d.Name, got), specialBackfire
		}
		return fmt.Sprintf("Your S3-Sabre broke up over %s with nothing to open for them.", d.Name), specialBackfire
	}
	lost := w.sabreDamage(d, w.SabreAim(dial))
	if lost == "" {
		return fmt.Sprintf("Your S3-Sabre reached %s but did negligible damage.", d.Name), specialNothing
	}
	d.addEvent(fmt.Sprintf("An S3-Sabre from %s struck your empire — lost %s.", from, lost))
	return fmt.Sprintf("Your S3-Sabre hit %s: %s destroyed.", d.Name, lost), specialHit
}
