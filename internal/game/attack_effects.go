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
	// A missile that arrives from another board resolves here, against this
	// board's own rules; only its Target set applies, because the attacker is
	// not on this board.
	EffectIPNuclear  EffectKind = "ip.nuclear"
	EffectIPChemical EffectKind = "ip.chemical"
	// An interplanetary attack of any type, Group Attack included, that fought
	// a battle. The Target set runs on the defending board after the resolver's
	// own outcome; the Attacker set runs on the attacker's board when the
	// answer comes home, on each realm that contributed.
	EffectIPAttackWon  EffectKind = "ip.attack.won"
	EffectIPAttackLost EffectKind = "ip.attack.lost"

	// One kind for each row of the S3-Sabre's dial. The dial's aim, its
	// backfire and the gates it meets stay built in.
	EffectSabreIntelligence  EffectKind = "sabre.intelligence"
	EffectSabrePeople        EffectKind = "sabre.people"
	EffectSabreMilitaryBases EffectKind = "sabre.military_bases"
	EffectSabreAirbases      EffectKind = "sabre.airbases"
	EffectSabreRegions       EffectKind = "sabre.regions"
	EffectSabreFood          EffectKind = "sabre.food"
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
	NamesWaste, NamesPeople, NamesTroopers, NamesMorale bool
	// CaptureAsCoastal: land the Target set captures is left for the caller,
	// which places it as Coastal with the battle's own capture and counts it
	// there (see Attack), so the session parks one total.
	CaptureAsCoastal bool
	// RemoteTarget marks a kind that arrives from another board: only its
	// Target set applies, and that set cannot capture, since the attacker's
	// realm is not here to receive the land (see Rules.Validate).
	RemoteTarget bool
	// AttackerAway marks a kind whose target is on another board when its
	// Attacker set runs, so that set cannot capture (see Rules.Validate).
	AttackerAway bool
	// WasteCap and PeopleCap, when above 0, hold the regions made waste and the
	// people killed by one strike to that many, whatever a mod sets.
	WasteCap, PeopleCap int
	// KeptShare: the kill, people and food effects are worked as the share
	// that is KEPT, truncated (lost = n - n x (100 - pct) / 100), as the
	// S3-Sabre does, not as the share lost.
	KeptShare bool
	// Exact, by effect key, is the original's own arithmetic for a row that
	// is not the integer percent it stands for (Real48 on the Sabre's
	// agents, airbases and food). It gets n and the raw Random() draw and
	// returns what is kept. It is used only while that effect holds its base
	// value; a modded value uses the generic arithmetic.
	Exact map[string]func(n, draw int) int
	// Arrive is the target's event and the firing board's report for an arriving
	// missile, for the headline's effects.
	Arrive func(from, who, board string, l AttackLoss) (event, report Msg)
	// DestroyedAs words a region count in a loss list; the default is
	// lostDestroyed.
	DestroyedAs forms
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
		NamesWaste: true, NamesPeople: true, NamesMorale: true,
		Headline: func(from, who string, l AttackLoss) (Msg, string) {
			return say("{from}'s chemical strike laid waste to {n} of your regions and killed {people} of your people; famine follows.",
					"from", from, "n", l.Waste, "people", l.People),
				fmt.Sprintf("Your gas laid waste to %d of %s's regions and killed %d of their people; their morale and support are broken.", l.Waste, who, l.People)
		}},
	{Kind: EffectBiological, Weapon: msgid("plague"), ScoreRoll: BioScoreRoll, ScoreFirst: true,
		NamesPeople: true, NamesTroopers: true, NamesMorale: true,
		Headline: func(from, who string, l AttackLoss) (Msg, string) {
			t := l.Units[KillIndex(Trooper)]
			return say("{from}'s plague killed {troopers} of your troopers and {people} of your people; famine follows.",
					"from", from, "troopers", t, "people", l.People),
				fmt.Sprintf("Your plague killed %d of %s's troopers and %d civilians, and took half their morale with it.", t, who, l.People)
		}},
	{Kind: EffectConventionalWon, CaptureAsCoastal: true},
	{Kind: EffectConventionalLost},
	{Kind: EffectIPAttackWon, AttackerAway: true},
	{Kind: EffectIPAttackLost, AttackerAway: true},
	{Kind: EffectIPNuclear, Weapon: msgid("nuclear strike"), NamesWaste: true, RemoteTarget: true, WasteCap: IPMissileRegionCap,
		Arrive: func(from, who, board string, l AttackLoss) (Msg, Msg) {
			return say("{from}'s nuclear strike turned {n} of your regions into waste.", "from", from, "n", l.Waste),
				say("Your nuclear strike turned {n} of {who}'s regions on {board} into waste.", "n", l.Waste, "who", who, "board", board)
		}},
	{Kind: EffectIPChemical, Weapon: msgid("chemical strike"), NamesPeople: true, RemoteTarget: true, PeopleCap: IPMissilePeopleCap,
		Arrive: func(from, who, board string, l AttackLoss) (Msg, Msg) {
			return say("{from}'s chemical strike killed {n} of your people.", "from", from, "n", l.People),
				say("Your chemical strike killed {n} of {who}'s people on {board}.", "n", l.People, "who", who, "board", board)
		}},
	{Kind: EffectSabreIntelligence, RemoteTarget: true, KeptShare: true, DestroyedAs: sabreRegions,
		Exact: map[string]func(n, draw int) int{killKey(Agent): sabreIntelKept}},
	{Kind: EffectSabrePeople, RemoteTarget: true, KeptShare: true, DestroyedAs: sabreRegions},
	{Kind: EffectSabreMilitaryBases, RemoteTarget: true, KeptShare: true, DestroyedAs: sabreRegions},
	{Kind: EffectSabreAirbases, RemoteTarget: true, KeptShare: true, DestroyedAs: sabreRegions,
		Exact: map[string]func(n, draw int) int{killKey(Jet): sabreAirbaseKept}},
	{Kind: EffectSabreRegions, RemoteTarget: true, DestroyedAs: sabreRegions},
	{Kind: EffectSabreFood, RemoteTarget: true, KeptShare: true, DestroyedAs: sabreRegions,
		Exact: map[string]func(n, draw int) int{keyFood: sabreFoodKept}},
}

// sabreRegions words a region count in an S3-Sabre report.
var sabreRegions = msgidN("{n} Region", "{n} Regions")

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
	ipNuke := noEffect()
	ipNuke.Waste = Roll{IPNukeWastePctBase, IPNukeWastePctRoll, 0}
	ipChem := noEffect()
	ipChem.People = Roll{IPChemKillPctBase, IPChemKillPctRoll, 0}
	sabreSet := func(set func(e *EffectSet)) EffectSet {
		e := noEffect()
		set(&e)
		return e
	}
	return map[EffectKind]AttackEffects{
		EffectNuclear:    {Target: nuke, Attacker: noEffect()},
		EffectChemical:   {Target: chem, Attacker: noEffect()},
		EffectBiological: {Target: bio, Attacker: noEffect()},
		// The battle, its capture and its losses stay in Attack; a mod adds to them.
		EffectConventionalWon:  {Target: noEffect(), Attacker: noEffect()},
		EffectConventionalLost: {Target: noEffect(), Attacker: noEffect()},
		EffectIPAttackWon:      {Target: noEffect(), Attacker: noEffect()},
		EffectIPAttackLost:     {Target: noEffect(), Attacker: noEffect()},
		EffectIPNuclear:        {Target: ipNuke, Attacker: noEffect()},
		EffectIPChemical:       {Target: ipChem, Attacker: noEffect()},
		EffectSabreIntelligence: {Target: sabreSet(func(e *EffectSet) {
			e.Kill[KillIndex(Agent)] = Roll{100 - SabreIntelKeepBasePct, 0, SabreIntelKeepSpread}
		}), Attacker: noEffect()},
		EffectSabrePeople: {Target: sabreSet(func(e *EffectSet) {
			e.People = Roll{100 - SabrePeopleKeepBasePct, 0, SabrePeopleKeepSpread}
		}), Attacker: noEffect()},
		EffectSabreMilitaryBases: {Target: sabreSet(func(e *EffectSet) {
			for _, g := range []*Good{Trooper, Jet, Turret, Tank} {
				e.Kill[KillIndex(g)] = Roll{100 - SabreBasesKeepBasePct, 0, SabreBasesKeepSpread}
			}
		}), Attacker: noEffect()},
		EffectSabreAirbases: {Target: sabreSet(func(e *EffectSet) {
			e.Kill[KillIndex(Jet)] = Roll{100 - SabreAirbaseKeepBasePct, 0, SabreAirbaseKeepSpread}
		}), Attacker: noEffect()},
		EffectSabreRegions: {Target: sabreSet(func(e *EffectSet) {
			e.Destroy = Roll{SabreRegionLossBasePct, SabreRegionLossSpread, 0}
		}), Attacker: noEffect()},
		EffectSabreFood: {Target: sabreSet(func(e *EffectSet) {
			e.Food = Roll{100, 0, SabreFoodKeepSpread}
		}), Attacker: noEffect()},
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
	// Morale and Support are the points of each lost.
	Morale, Support int
	Units           [numKillGoods]int
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

// share draws r and returns how much of n it takes. Where the kind works in
// the share KEPT, or has the original's own arithmetic for the effect at its
// base value, that is used instead of a truncated share lost.
func (w *World) share(row *attackKindRow, key string, n int, r, base Roll) int {
	pct := w.roll(r)
	if hook := row.Exact[key]; hook != nil && r == base {
		return n - hook(n, r.Pct-pct) // the draw: Up is 0, so pct = Pct - draw
	}
	if row.KeptShare {
		return n - pctOf(n, 100-pct)
	}
	return pctOf(n, pct)
}

// applySet runs one effect set against victim; other is the realm a capture
// goes to. It returns what the set took.
func (w *World) applySet(fx EffectSet, row *attackKindRow, victim, other *Empire, coastal bool) AttackLoss {
	var l AttackLoss
	base := baseRules.Attacks[row.Kind].Target
	if fx.IsZero() && fx.Morale.keepsAll() && fx.Support.keepsAll() {
		return l
	}
	if !fx.Waste.IsZero() {
		l.Waste = ruinToWasteCapped(victim, w.roll(fx.Waste), row.WasteCap)
	}
	if !fx.Destroy.IsZero() {
		l.Destroyed = victim.Regions.remove(pctOf(victim.Land, w.roll(fx.Destroy))).Total()
		if l.Destroyed > 0 {
			victim.syncLand()
		}
	}
	if !fx.Capture.IsZero() {
		l.Captured = victim.Regions.remove(pctOf(victim.Land, w.roll(fx.Capture))).Total()
		victim.syncLand()
		// With no receiving realm here (other is nil) the land is carried to
		// its new owner on another board. With coastal set, the caller places
		// it with the battle's own capture.
		if other != nil && !coastal {
			// An AI realm never types untyped land (only a player's turn does),
			// so it gets the land as Coastal, where the AI keeps it.
			if other.Owner == "" {
				other.Regions.Coastal += l.Captured
				other.syncLand()
			} else {
				other.PendingRegions += l.Captured
			}
		}
	}
	if !fx.People.IsZero() {
		// share works in int64: a big urban realm's head count times a
		// percentage passes 2^31 on the 32-bit door builds.
		l.People = w.share(row, keyPeople, victim.People, fx.People, base.People)
		if row.PeopleCap > 0 {
			l.People = min(l.People, row.PeopleCap)
		}
		victim.People -= l.People
	}
	for i, g := range AttackKillGoods {
		if !fx.Kill[i].IsZero() {
			n := g.Count(victim)
			l.Units[i] = w.share(row, killKey(g), *n, fx.Kill[i], base.Kill[i])
			*n -= l.Units[i]
		}
	}
	if !fx.Food.IsZero() {
		l.Food = w.share(row, keyFood, victim.Food, fx.Food, base.Food)
		victim.Food -= l.Food
	}
	morale, support := victim.Morale, victim.Support
	victim.Morale = keep(victim.Morale, fx.Morale, row.MoraleRounded)
	victim.Support = keep(victim.Support, fx.Support, true)
	l.Morale, l.Support = morale-victim.Morale, support-victim.Support
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
	lostMorale    = msgidN("{n} point of morale", "{n} points of morale")
	lostSupport   = msgidN("{n} point of support", "{n} points of support")
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
	destroyed := lostDestroyed
	if row.DestroyedAs != (forms{}) {
		destroyed = row.DestroyedAs
	}
	add(destroyed, l.Destroyed)
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
	// A headline that names morale covers support too: both kinds that name
	// it break the two together.
	if !(extra && row.NamesMorale) {
		add(lostMorale, l.Morale)
		add(lostSupport, l.Support)
	}
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

// alsoTook is the sentence listing losses beyond what a line already names,
// or zero when there are none.
func alsoTook(lost []Msg) Msg {
	if len(lost) == 0 {
		return Msg{}
	}
	return say("It also took {lost}.", "lost", lost)
}

// lossNews words a strike whose kind's headline is not in play: the loss list,
// or that it took nothing. board names the target's board in a report sent to
// another board, and is "" for a strike on this one. The headline reads right
// only when every effect it names is in play; with one zeroed it would say
// "laid waste to 0 of your regions".
func lossNews(row *attackKindRow, from, who, board string, l AttackLoss) (event, report Msg) {
	weapon := say(row.Weapon)
	lost := l.parts(row, false)
	switch {
	case len(lost) > 0 && board == "":
		report = say("Your {weapon} hit {who}'s realm, taking {lost}.", "weapon", weapon, "who", who, "lost", lost)
	case len(lost) > 0:
		report = say("Your {weapon} hit {who}'s realm on {board}, taking {lost}.", "weapon", weapon, "who", who, "board", board, "lost", lost)
	case board == "":
		report = say("Your {weapon} hit {who}'s realm and took nothing from it.", "weapon", weapon, "who", who)
	default:
		report = say("Your {weapon} hit {who}'s realm on {board} and took nothing from it.", "weapon", weapon, "who", who, "board", board)
	}
	if len(lost) > 0 {
		return say("{from}'s {weapon} hit your realm, taking {lost}.", "from", from, "weapon", weapon, "lost", lost), report
	}
	return say("{from}'s {weapon} hit your realm and took nothing from it.", "from", from, "weapon", weapon), report
}

// attackNews words a landed strike: the event the target reads and the report
// the attacker gets. The kind's own headline leads while every effect it names
// is in play, and what else the target lost follows in one sentence; otherwise
// lossNews words it. What the attacker lost is added to both.
func (w *World) attackNews(row *attackKindRow, from, who string, t, at AttackLoss) (Msg, string) {
	var ev Msg
	var report string
	if w.headlineInPlay(row) {
		ev, report = row.Headline(from, who, t)
		if also := alsoTook(t.parts(row, true)); !also.IsZero() {
			ev = sentences(ev, also)
			report += " " + also.English()
		}
	} else {
		var rep Msg
		ev, rep = lossNews(row, from, who, "", t)
		report = rep.English()
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
// attacker, not yet placed: the battle places and counts it with its own
// capture, so the sentence does not repeat it.
func (w *World) conventionalEffects(kind EffectKind, a, d *Empire) (event, report Msg, captured int) {
	row := attackKindOf(kind)
	fx := w.attackEffects(kind)
	t := w.applySet(fx.Target, row, d, a, row.CaptureAsCoastal)
	// A realm the attack has left with no land is conquered and receives
	// nothing, so the attacker set's capture is not taken.
	if d.Land <= 0 {
		fx.Attacker.Capture = Roll{}
	}
	at := w.applySet(fx.Attacker, row, a, d, false)
	if row.CaptureAsCoastal {
		captured, t.Captured = t.Captured, 0
	}
	took := alsoTook(t.parts(row, false))
	costEv, costRep := attackCost(row, a.Name, at)
	return sentences(took, costEv), sentences(took, costRep), captured
}

// arrivingEffects applies kind's Target set to d for a missile that arrived
// from another board, and words it: the event for d and the report for the
// firing board. Only the target is here, so the attacker's set is not applied.
// A kind whose headline effects are not all in play is worded by lossNews, as
// a local missile is.
func (w *World) arrivingEffects(kind EffectKind, d *Empire, from string) (event, report Msg) {
	row := attackKindOf(kind)
	l := w.applySet(w.attackEffects(kind).Target, row, d, nil, false)
	board := w.Config.BoardID
	if !w.headlineInPlay(row) {
		return lossNews(row, from, d.Name, board, l)
	}
	event, report = row.Arrive(from, d.Name, board, l)
	also := alsoTook(l.parts(row, true))
	return sentences(event, also), sentences(report, also)
}

// headlineInPlay reports whether every effect row's headline names is set.
func (w *World) headlineInPlay(row *attackKindRow) bool {
	fx := w.attackEffects(row.Kind).Target
	return !(row.NamesWaste && fx.Waste.IsZero()) && !(row.NamesPeople && fx.People.IsZero()) &&
		!(row.NamesTroopers && fx.Kill[KillIndex(Trooper)].IsZero()) &&
		!(row.NamesMorale && fx.Morale.keepsAll())
}

// add sums o into l, for the effects one strike has on several realms.
func (l *AttackLoss) add(o AttackLoss) {
	l.Waste += o.Waste
	l.Destroyed += o.Destroyed
	l.Captured += o.Captured
	l.People += o.People
	l.Food += o.Food
	l.HQ += o.HQ
	l.Morale += o.Morale
	l.Support += o.Support
	for i := range l.Units {
		l.Units[i] += o.Units[i]
	}
}

// shareOf is frac of every figure of l, rounded as a returning strike's kills
// are, for one contributor's part of what a group strike took.
func (l AttackLoss) shareOf(frac float64) AttackLoss {
	s := AttackLoss{
		Waste: roundShare(l.Waste, frac), Destroyed: roundShare(l.Destroyed, frac),
		Captured: roundShare(l.Captured, frac), People: roundShare(l.People, frac),
		Food: roundShare(l.Food, frac), HQ: roundShare(l.HQ, frac),
		Morale: roundShare(l.Morale, frac), Support: roundShare(l.Support, frac),
	}
	for i := range l.Units {
		s.Units[i] = roundShare(l.Units[i], frac)
	}
	return s
}

// remoteAttackEffects applies kind's Target set to a realm that stood against
// an interplanetary attack, and returns the sentence for its event and the
// regions the set captured. The captured regions are left out of the sentence:
// the caller counts them with the battle's own (LandTaken), which carries them
// home. sum collects what the set took from every realm that stood.
func (w *World) remoteAttackEffects(kind EffectKind, e *Empire, sum *AttackLoss) (also Msg, captured int) {
	row := attackKindOf(kind)
	t := w.applySet(w.attackEffects(kind).Target, row, e, nil, false)
	captured, t.Captured = t.Captured, 0
	sum.add(t)
	return alsoTook(t.parts(row, false)), captured
}

// remoteAttackKind is the kind of the effects an interplanetary attack's
// outcome sets off.
func remoteAttackKind(won bool) EffectKind {
	if won {
		return EffectIPAttackWon
	}
	return EffectIPAttackLost
}

// remoteAttackTook is the sentence for one contributor's report of what a
// mod's ip.attack target set took from the defenders: its share of extra, and
// on a repelled strike the regions it was given, which no battle line names.
func remoteAttackTook(won bool, extra *EffectLoss, frac float64, land int) Msg {
	var share AttackLoss
	if extra != nil {
		share = extra.loss().shareOf(frac)
	}
	share.Captured = 0
	if !won {
		share.Captured = land
	}
	return alsoTook(share.parts(attackKindOf(remoteAttackKind(won)), false))
}

// homeAttackEffects applies an interplanetary attack's Attacker set to one
// realm of this board that contributed to it, as if it had attacked alone, and
// returns the sentence for its report, zero when it cost nothing.
func (w *World) homeAttackEffects(won bool, e *Empire) Msg {
	kind := remoteAttackKind(won)
	row := attackKindOf(kind)
	l := w.applySet(w.attackEffects(kind).Attacker, row, e, nil, false)
	_, report := attackCost(row, "", l)
	return report
}
