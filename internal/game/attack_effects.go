package game

import (
	"fmt"
	"slices"
)

// attack_effects.go — what a strike DOES, as one set of effects any kind of
// attack can use (#301). Each kind has a row in Rules (Rules.Attacks) holding
// two sets of figures: one for the realm attacked and one for the realm that
// attacked, its blowback. applyAttack reads the row. The base rows are the
// original's local missiles, built from the balance_costs.go constants, with
// nothing in the blowback set, so the base game plays exactly as it did when
// each missile had its own routine.

// EffectKind names a kind of attack. The names are what a mod's values file
// uses for the section, so they never change; a kind with sub-cases uses a
// dotted name ("conventional.won"). A later kind is one row in attackKinds and
// one in baseAttackEffects.
type EffectKind string

const (
	EffectNuclear    EffectKind = "nuclear"
	EffectChemical   EffectKind = "chemical"
	EffectBiological EffectKind = "biological"
	// A same-planet attack's own battle is not in the table; these add to it,
	// the first on a win and the second when the attacker is repelled.
	EffectConventionalWon  EffectKind = "conventional.won"
	EffectConventionalLost EffectKind = "conventional.lost"
)

// Roll is a percentage drawn as Pct + Random(Up) - Random(Down), held to 0..100.
// A Random(n) is drawn only when n is above 0, so a zero Roll draws nothing and
// does nothing. The draw order is Up, then Down: it is part of the contract,
// because a changed order moves every later draw of the game.
type Roll struct{ Pct, Up, Down int }

// IsZero reports whether r does nothing.
func (r Roll) IsZero() bool { return r == Roll{} }

// Fraction is the part of a figure that is kept: Num/Den. Num equal to Den
// leaves the figure alone.
type Fraction struct{ Num, Den int }

// numKillGoods is len(AttackKillGoods); a test holds the two together.
const numKillGoods = 7

// AttackKillGoods are the units an attack can kill, in the order their effects
// are applied: the six military units, then agents.
var AttackKillGoods = append(slices.Clone(MilitaryGoods), Agent)

// KillIndex is the slot of g in EffectSet.Kill, or -1 for a unit an attack
// cannot kill.
func KillIndex(g *Good) int { return slices.Index(AttackKillGoods, g) }

// EffectSet is what an attack does to one realm, applied in this order: waste,
// destroy, capture, people, Kill (in AttackKillGoods order), food, morale,
// support, HQ.
type EffectSet struct {
	// Waste, Destroy and Capture are percents of the realm's land: turned to
	// waste, removed outright, and taken by the other side as untyped land.
	Waste, Destroy, Capture Roll
	// People is the percent of the realm's people killed.
	People Roll
	// Kill is the percent of each unit killed, by index in AttackKillGoods.
	Kill [numKillGoods]Roll
	// Food is the percent of the realm's food destroyed.
	Food Roll
	// Morale and Support are the part of each that is kept.
	Morale, Support Fraction
	// HQ is the HeadQuarters completion points lost, read as points and not
	// percent.
	HQ Roll
}

// noEffect is the set that does nothing: every roll zero, all morale and
// support kept. It is what a side with no effects holds.
func noEffect() EffectSet {
	return EffectSet{Morale: Fraction{1, 1}, Support: Fraction{1, 1}}
}

// IsZero reports whether the set does nothing and draws nothing.
func (s EffectSet) IsZero() bool {
	s.Morale, s.Support = Fraction{}, Fraction{}
	return s == EffectSet{}
}

// keepsAll reports whether f leaves a figure alone.
func (f Fraction) keepsAll() bool { return f.Num == f.Den }

// AttackEffects is both sides of one kind of attack. The Target set runs
// first, then the Attacker set. An Attacker set's Capture gives the TARGET that
// share of the attacker's land.
type AttackEffects struct {
	Target, Attacker EffectSet
}

// attackKindRow is the fixed part of a kind: what is not a mod's to change.
type attackKindRow struct {
	Kind EffectKind
	// Weapon names the attack in a headline built from the loss list.
	Weapon string
	// ScoreRoll is the Score a launch pays the attacker, drawn Random(ScoreRoll),
	// and ScoreFirst says whether the draw comes before the damage. A
	// ScoreRoll of 0 draws nothing: the conventional kinds are not launches.
	ScoreRoll  int
	ScoreFirst bool
	// MoraleRounded: morale is rounded to the nearest, as the chemical strike
	// does, and not truncated, as the plague does. Support is always rounded.
	MoraleRounded bool
	// The headline names these effects of the Target set. A loss one of them
	// took is in the headline; any other loss follows as an extra sentence.
	NamesWaste, NamesPeople, NamesTroopers bool
	// CaptureAsCoastal: land the Target set captures is placed as Coastal for
	// the attacker, like the battle's own capture, and counted in it (see
	// Attack), so the caller parks one total.
	CaptureAsCoastal bool
	// Headline is the target's event and the attacker's report when the
	// headline's effects are in play. Nil for a kind whose attack writes its
	// own report.
	Headline func(from, who string, l AttackLoss) (Msg, string)
}

var attackKinds = []attackKindRow{
	{Kind: EffectNuclear, Weapon: msgid("nuclear strike"), ScoreRoll: NukeScoreRoll, NamesWaste: true,
		Headline: func(from, who string, l AttackLoss) (Msg, string) {
			return say("{from}'s nuclear strike turned {n} of your regions into waste.", "from", from, "n", l.Waste),
				fmt.Sprintf("Your nuclear strike turned %d of %s's regions into waste.", l.Waste, who)
		}},
	{Kind: EffectChemical, Weapon: msgid("chemical strike"), ScoreRoll: ChemScoreRoll, MoraleRounded: true,
		NamesWaste: true, NamesPeople: true,
		Headline: func(from, who string, l AttackLoss) (Msg, string) {
			return say("{from}'s chemical strike laid waste to {n} of your regions and killed {people} of your people; famine follows.",
					"from", from, "n", l.Waste, "people", l.People),
				fmt.Sprintf("Your gas laid waste to %d of %s's regions and killed %d of their people; their morale and support are broken.", l.Waste, who, l.People)
		}},
	{Kind: EffectBiological, Weapon: msgid("plague"), ScoreRoll: BioScoreRoll, ScoreFirst: true,
		NamesPeople: true, NamesTroopers: true,
		Headline: func(from, who string, l AttackLoss) (Msg, string) {
			t := l.Units[KillIndex(Trooper)]
			return say("{from}'s plague killed {troopers} of your troopers and {people} of your people; famine follows.",
					"from", from, "troopers", t, "people", l.People),
				fmt.Sprintf("Your plague killed %d of %s's troopers and %d civilians, and took half their morale with it.", t, who, l.People)
		}},
	{Kind: EffectConventionalWon, CaptureAsCoastal: true},
	{Kind: EffectConventionalLost},
}

// EffectKinds are the kinds of attack a mod can set, in the order they are
// listed and validated.
func EffectKinds() []EffectKind {
	var out []EffectKind
	for _, r := range attackKinds {
		out = append(out, r.Kind)
	}
	return out
}

func attackKindOf(k EffectKind) *attackKindRow {
	for i := range attackKinds {
		if attackKinds[i].Kind == k {
			return &attackKinds[i]
		}
	}
	panic("game: unknown attack kind " + string(k))
}

// baseAttackEffects are the original's figures for each kind (balance_costs.go).
// Nothing is set on the Attacker side: a missile costs its sender only gold.
func baseAttackEffects() map[EffectKind]AttackEffects {
	support := Fraction{StrikeSupportKeepNum, StrikeSupportKeepDen}
	nuke := noEffect()
	nuke.Waste = Roll{NukeWastePct, NukeWasteJitter, NukeWasteJitter}
	chem := noEffect()
	chem.Waste = Roll{ChemWastePct, ChemWasteJitter, ChemWasteJitter}
	chem.People = Roll{Pct: ChemPopKillPct}
	chem.Morale, chem.Support = Fraction{ChemMoraleKeepNum, ChemMoraleKeepDen}, support
	bio := noEffect()
	bio.People = Roll{BioPopKillPct, BioPopKillJitterUp, BioPopKillJitterDown}
	bio.Morale, bio.Support = Fraction{1, BioMoraleDivisor}, support
	bio.Kill[KillIndex(Trooper)] = Roll{BioTroopKillPct, BioTroopKillJitterUp, BioTroopKillJitterDn}
	return map[EffectKind]AttackEffects{
		EffectNuclear:    {Target: nuke, Attacker: noEffect()},
		EffectChemical:   {Target: chem, Attacker: noEffect()},
		EffectBiological: {Target: bio, Attacker: noEffect()},
		// The battle, its capture and its losses stay in Attack; a mod adds to them.
		EffectConventionalWon:  {Target: noEffect(), Attacker: noEffect()},
		EffectConventionalLost: {Target: noEffect(), Attacker: noEffect()},
	}
}

// attackEffects is kind's row under w's rules. Validate refuses rules that
// lack a kind, so every kind is there.
func (w *World) attackEffects(kind EffectKind) AttackEffects {
	return w.rules().Attacks[kind]
}

// AttackLoss is what one application of an effect set took.
type AttackLoss struct {
	Waste, Destroyed, Captured, People, Food, HQ int
	Units                                        [numKillGoods]int
}

// roll draws r.
func (w *World) roll(r Roll) int {
	p := r.Pct
	if r.Up > 0 {
		p += w.rng.Intn(r.Up)
	}
	if r.Down > 0 {
		p -= w.rng.Intn(r.Down)
	}
	return min(max(p, 0), 100)
}

// keep is v scaled by f, rounded to the nearest or truncated.
func keep(v int, f Fraction, rounded bool) int {
	if f.keepsAll() {
		return v
	}
	if rounded {
		return roundDiv(v*f.Num, f.Den)
	}
	return v * f.Num / f.Den
}

// applySet runs one effect set against victim; other is the realm a capture
// goes to. It returns what the set took.
func (w *World) applySet(fx EffectSet, row *attackKindRow, victim, other *Empire, coastal bool) AttackLoss {
	var l AttackLoss
	if fx.IsZero() && fx.Morale.keepsAll() && fx.Support.keepsAll() {
		return l
	}
	if !fx.Waste.IsZero() {
		l.Waste = ruinToWaste(victim, w.roll(fx.Waste))
	}
	if !fx.Destroy.IsZero() {
		l.Destroyed = victim.Regions.remove(pctOf(victim.Land, w.roll(fx.Destroy))).Total()
		victim.syncLand()
	}
	if !fx.Capture.IsZero() {
		l.Captured = victim.Regions.remove(pctOf(victim.Land, w.roll(fx.Capture))).Total()
		victim.syncLand()
		// An AI realm never types untyped land (only a player's turn does), so
		// it gets the land as Coastal, where the AI keeps it. So does a
		// battle's winner, whose caller parks the whole capture at once.
		if other.Owner == "" || coastal {
			other.Regions.Coastal += l.Captured
			other.syncLand()
		} else {
			other.PendingRegions += l.Captured
		}
	}
	if !fx.People.IsZero() {
		// int64 through the multiply: a big urban realm's head count times a
		// percentage passes 2^31 on the 32-bit door builds.
		l.People = int(int64(victim.People) * int64(w.roll(fx.People)) / 100)
		victim.People -= l.People
	}
	for i, g := range AttackKillGoods {
		if !fx.Kill[i].IsZero() {
			n := g.Count(victim)
			l.Units[i] = pctOf(*n, w.roll(fx.Kill[i]))
			*n -= l.Units[i]
		}
	}
	if !fx.Food.IsZero() {
		l.Food = pctOf(victim.Food, w.roll(fx.Food))
		victim.Food -= l.Food
	}
	victim.Morale = keep(victim.Morale, fx.Morale, row.MoraleRounded)
	victim.Support = keep(victim.Support, fx.Support, true)
	if !fx.HQ.IsZero() {
		l.HQ = min(w.roll(fx.HQ), victim.HQ)
		victim.HQ -= l.HQ
	}
	return l
}

// applyAttack runs kind's Target set against d and then its Attacker set
// against a, and returns what each took.
func (w *World) applyAttack(kind EffectKind, a, d *Empire) (target, attacker AttackLoss) {
	row := attackKindOf(kind)
	fx := w.attackEffects(kind)
	target = w.applySet(fx.Target, row, d, a, row.CaptureAsCoastal)
	attacker = w.applySet(fx.Attacker, row, a, d, false)
	return target, attacker
}

var (
	lostWaste     = msgidN("{n} Region laid waste", "{n} Regions laid waste")
	lostDestroyed = msgidN("{n} Region destroyed", "{n} Regions destroyed")
	lostCaptured  = msgidN("{n} Region seized", "{n} Regions seized")
	lostPeople    = msgidN("{n} Person", "{n} People")
	lostHQ        = msgidN("{n} HeadQuarters Point", "{n} HeadQuarters Points")
)

// parts lists the losses as counted phrases, in the order of EffectSet. With
// extra set, what the kind's headline already names is left out.
func (l AttackLoss) parts(row *attackKindRow, extra bool) []Msg {
	var p []Msg
	add := func(f forms, n int) {
		if n > 0 {
			p = append(p, sayIn(f, "n", "n", n))
		}
	}
	if !(extra && row.NamesWaste) {
		add(lostWaste, l.Waste)
	}
	add(lostDestroyed, l.Destroyed)
	add(lostCaptured, l.Captured)
	if !(extra && row.NamesPeople) {
		add(lostPeople, l.People)
	}
	for i, g := range AttackKillGoods {
		if !(extra && row.NamesTroopers && g == Trooper) {
			add(g.Counted, l.Units[i])
		}
	}
	add(Food.Counted, l.Food)
	add(lostHQ, l.HQ)
	return p
}

// attackCost words what an attack cost its own side, for the target's event
// and for the attacker's report; both are zero when it cost nothing.
func attackCost(row *attackKindRow, from string, at AttackLoss) (event, report Msg) {
	own := at.parts(row, false)
	if len(own) == 0 {
		return Msg{}, Msg{}
	}
	return say("It also cost {from} {lost}.", "from", from, "lost", own), say("It also cost you {lost}.", "lost", own)
}

// attackNews words a landed strike: the event the target reads and the report
// the attacker gets. The kind's own headline leads while every effect it names
// is in play, and what else the target lost follows in one sentence. A mod
// that sets any effect the headline names to zero gets a headline built from
// the loss list instead of "turned 0 regions into waste". What the attacker
// lost is added to both.
func (w *World) attackNews(row *attackKindRow, from, who string, t, at AttackLoss) (Msg, string) {
	fx := w.attackEffects(row.Kind).Target
	// The headline reads right only when every effect it names is in play;
	// with one zeroed it would say "laid waste to 0 of your regions".
	named := !(row.NamesWaste && fx.Waste.IsZero()) && !(row.NamesPeople && fx.People.IsZero()) &&
		!(row.NamesTroopers && fx.Kill[KillIndex(Trooper)].IsZero())
	var ev Msg
	var report string
	if !named {
		weapon := say(row.Weapon)
		if lost := t.parts(row, false); len(lost) > 0 {
			ev = say("{from}'s {weapon} hit your realm, taking {lost}.", "from", from, "weapon", weapon, "lost", lost)
			report = say("Your {weapon} hit {who}'s realm, taking {lost}.", "weapon", weapon, "who", who, "lost", lost).English()
		} else {
			ev = say("{from}'s {weapon} hit your realm and took nothing from it.", "from", from, "weapon", weapon)
			report = say("Your {weapon} hit {who}'s realm and took nothing from it.", "weapon", weapon, "who", who).English()
		}
	} else {
		ev, report = row.Headline(from, who, t)
		if more := t.parts(row, true); len(more) > 0 {
			also := say("It also took {lost}.", "lost", more)
			ev = sentences(ev, also)
			report += " " + also.English()
		}
	}
	cost, costReport := attackCost(row, from, at)
	ev = sentences(ev, cost)
	if !costReport.IsZero() {
		report += " " + costReport.English()
	}
	return ev, report
}

// localStrike is one missile fired at d: the arms dealer's price, the pact it
// breaks, the damage and the Score, in the order the original does them.
func (w *World) localStrike(kind EffectKind, a, d *Empire, cost int64) (string, error) {
	row := attackKindOf(kind)
	if err := payArmsDealer(a, cost); err != nil {
		return "", err
	}
	// A missile aimed at a pact partner tears the pact up and costs support and
	// morale, exactly as a regular attack does — all four share one target picker
	// in the original (see BreachTreaty). The menu asks first.
	w.BreachTreaty(a, d)
	// The Score is a draw, not a share of the damage: a strike that took
	// nothing still scores. The plague's lands on the sale, before any damage
	// is rolled; the other two follow the damage.
	if row.ScoreFirst && row.ScoreRoll > 0 {
		addScore(a, w.rng.Intn(row.ScoreRoll))
	}
	t, at := w.applyAttack(kind, a, d)
	if !row.ScoreFirst && row.ScoreRoll > 0 {
		addScore(a, w.rng.Intn(row.ScoreRoll))
	}
	ev, report := w.attackNews(row, a.Name, d.Name, t, at)
	d.addEvent(ev)
	w.postStrikeNews(a, d, string(kind))
	return report, nil
}

// conventionalEffects applies a conventional attack's won or lost set and words
// what it took, as a line for the defender's event and one for the attacker's
// report. Both are zero when nothing was taken, which is always so in the base
// rules, where the sets are empty. captured is the land a won set took for the
// attacker, placed as Coastal and left for the battle to count with its own
// capture, so the sentence does not repeat it.
func (w *World) conventionalEffects(kind EffectKind, a, d *Empire) (event, report Msg, captured int) {
	row := attackKindOf(kind)
	t, at := w.applyAttack(kind, a, d)
	if row.CaptureAsCoastal {
		captured, t.Captured = t.Captured, 0
	}
	var took Msg
	if lost := t.parts(row, false); len(lost) > 0 {
		took = say("It also took {lost}.", "lost", lost)
	}
	costEv, costRep := attackCost(row, a.Name, at)
	return sentences(took, costEv), sentences(took, costRep), captured
}
