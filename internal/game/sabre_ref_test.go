package game

// sabre_ref_test.go — the S3-Sabre's damage rows and target-side resolver as
// they were before the attack table, kept verbatim as the reference the base
// rules must still reproduce. The ip.* missiles' reference is in
// attack_remote_test.go.

// refSabreDamage applies a landed S3-Sabre hit to e and returns a human-readable
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
func (w *World) refSabreDamage(e *Empire, eff SabreEffect) []Msg {
	row, ok := refSabreRows[eff]
	if !ok {
		return nil
	}
	s := &refSabreStrike{w: w, e: e}
	row.hit(s)
	return s.parts
}

// refSabreRow is one of the six damage rows: the target's event when a hit takes
// nothing ({from}) and when it does ({from}, {lost}), and what it does.
type refSabreRow struct {
	harmless string
	hurt     string
	hit      func(s *refSabreStrike)
}

// refSabreRows is the one table of the dial's damage rows. SabreDevelopRegions is
// not in it: a backfire applies that row directly (sabreDevelop).
var refSabreRows = map[SabreEffect]refSabreRow{
	SabreHitIntelligence: {msgid("{from}'s S3-Sabre hit your Intelligence Headquarters and did little harm."),
		msgid("{from}'s S3-Sabre hit your Intelligence Headquarters, destroying {lost}."),
		func(s *refSabreStrike) {
			s.keep(Agent.Counted, Agent.Count(s.e), SabreIntelKeepSpread, sabreIntelKept)
		}},
	SabreHitPeople: {msgid("{from}'s S3-Sabre hit your residential zones and did little harm."),
		msgid("{from}'s S3-Sabre hit your residential zones, destroying {lost}."),
		func(s *refSabreStrike) {
			s.keep(msgidN("{n} Person", "{n} People"), &s.e.People, SabrePeopleKeepSpread, func(n, roll int) int {
				return pctOf(n, SabrePeopleKeepBasePct+roll)
			})
		}},
	SabreHitMilitaryBases: {msgid("{from}'s S3-Sabre hit your military bases and did little harm."),
		msgid("{from}'s S3-Sabre hit your military bases, destroying {lost}."),
		func(s *refSabreStrike) {
			for _, g := range []*Good{Trooper, Jet, Turret, Tank} {
				s.keep(g.Counted, g.Count(s.e), SabreBasesKeepSpread, func(n, roll int) int {
					return pctOf(n, SabreBasesKeepBasePct+roll)
				})
			}
		}},
	SabreHitAirbases: {msgid("{from}'s S3-Sabre hit your airbases and did little harm."),
		msgid("{from}'s S3-Sabre hit your airbases, destroying {lost}."),
		func(s *refSabreStrike) {
			s.keep(Jet.Counted, Jet.Count(s.e), SabreAirbaseKeepSpread, sabreAirbaseKept)
		}},
	SabreHitRegions: {msgid("{from}'s S3-Sabre hit your regions and did little harm."),
		msgid("{from}'s S3-Sabre hit your regions, destroying {lost}."),
		func(s *refSabreStrike) {
			e := s.e
			lost := pctOf(e.Land, SabreRegionLossBasePct+s.w.rng.Intn(SabreRegionLossSpread))
			if lost > 0 {
				lost = e.Regions.remove(lost).Total()
				e.syncLand()
				s.parts = append(s.parts, sayN("{n} Region", "{n} Regions", "n", "n", lost))
			}
		}},
	SabreHitFood: {msgid("{from}'s S3-Sabre hit your food supply and did little harm."),
		msgid("{from}'s S3-Sabre hit your food supply, destroying {lost}."),
		func(s *refSabreStrike) {
			s.keep(Food.Counted, Food.Count(s.e), SabreFoodKeepSpread, sabreFoodKept)
		}},
}

// refSabreStrike is one landed hit in progress: the realm it struck and the list
// of what it destroyed.
type refSabreStrike struct {
	w     *World
	e     *Empire
	parts []Msg
}

// keep leaves *n at kept(*n, Random(spread)) and records what went; what names
// the count, {n}, in the singular and the plural.
func (s *refSabreStrike) keep(what forms, n *int, spread int, kept func(n, roll int) int) {
	left := kept(*n, s.w.rng.Intn(spread))
	if lost := *n - left; lost > 0 {
		*n = left
		s.parts = append(s.parts, sayIn(what, "n", "n", lost))
	}
}

// refSabreEffect is the target-side half of the missile, for a strike that
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
func (w *World) refSabreEffect(d *Empire, from string, dial int) (report Msg, outcome specialOutcome, gained int) {
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
	if stopped, why := w.stopArrivingMissile(d, OpSabre, from); why != specialHit {
		return stopped, why, 0
	}
	board := w.Config.BoardID
	if w.sabreBackfires(d) {
		// A backfire is the original's route to the mapper's last row: it does not
		// hurt the firer, it DEVELOPS land for the realm they aimed at (#266). The
		// firer is told when the answer gets home; the target sees it now.
		if got := w.sabreDevelop(d); got > 0 {
			d.addEvent(sayN("{from}'s S3-Sabre backfired, expanding your territory by {n} region.",
				"{from}'s S3-Sabre backfired, expanding your territory by {n} regions.", "n", "from", from, "n", got))
			return sayN("Your S3-Sabre backfired on {who} of {board}, expanding their territory by {n} region.",
				"Your S3-Sabre backfired on {who} of {board}, expanding their territory by {n} regions.", "n",
				"who", d.Name, "board", board, "n", got), specialBackfire, got
		}
		return say("Your S3-Sabre backfired on {who} of {board}, but they had too little land for it to give them any.",
			"who", d.Name, "board", board), specialBackfire, 0
	}
	eff := w.SabreAim(dial)
	row := refSabreRows[eff]
	lost := w.refSabreDamage(d, eff)
	if len(lost) == 0 {
		// The original tells the target which row hit even when it took
		// nothing: the event writer at +0x07d6 runs before the damage switch.
		d.addEvent(say(row.harmless, "from", from))
		return say("Your S3-Sabre reached {who} of {board} and barely scratched the paint.", "who", d.Name, "board", board), specialNothing, 0
	}
	d.addEvent(say(row.hurt, "from", from, "lost", lost))
	return say("Your S3-Sabre hit {who} of {board}, destroying {lost}.", "who", d.Name, "board", board, "lost", lost), specialHit, 0
}
