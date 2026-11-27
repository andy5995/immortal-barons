package game

import (
	"slices"
	"strings"
	"testing"
)

func gi(g *Good) int { return slices.Index(MilitaryGoods, g) }

func TestCaptureGoodsTableMatchesItsArray(t *testing.T) {
	if len(MilitaryGoods) != numCaptureGoods {
		t.Fatalf("MilitaryGoods has %d rows, numCaptureGoods is %d", len(MilitaryGoods), numCaptureGoods)
	}
	if !slices.Equal(AttackKillGoods[:numCaptureGoods], MilitaryGoods) {
		t.Error("the capture slots must sit in the kill slots' first six, in the same order")
	}
}

// capSet gives a kind's Target (or Attacker) set a unit-capture percent.
func capSet(fx *EffectSet, g *Good, pct int) { fx.CaptureUnits[gi(g)] = Roll{Pct: pct} }

// A won battle's target set: the attacker takes a share of what the defender
// has left, after the kills, and only of the units that stood in the defense.
func TestWonTargetSetCapturesAfterTheKills(t *testing.T) {
	base, ba, bd, bf := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	w, a, d, f := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	for _, e := range []*Empire{bd, d} {
		e.Bombers, e.Carriers = 1000, 1000
	}
	moddedConventional(w, EffectConventionalWon, func(fx *AttackEffects) {
		fx.Target.Kill[KillIndex(Tank)] = Roll{Pct: 50}
		capSet(&fx.Target, Tank, 20)
		capSet(&fx.Target, Trooper, 10)
		capSet(&fx.Target, Bomber, 100) // did not stand in the defense
	})
	baseReport, _ := base.Attack(ba, bd, bf)
	report, _ := w.Attack(a, d, f)
	if !strings.Contains(baseReport, "Victory") {
		t.Fatal("setup: the attack should win")
	}
	tanks := bd.Tanks - pctOf(bd.Tanks, 50)
	gotTanks, gotTroopers := pctOf(tanks, 20), pctOf(bd.Troopers, 10)
	if gotTanks == 0 || gotTroopers == 0 {
		t.Fatal("setup: the capture should take something")
	}
	if d.Tanks != tanks-gotTanks || d.Troopers != bd.Troopers-gotTroopers || d.Bombers != 1000 {
		t.Errorf("defender holds %d tanks %d troopers %d bombers, want %d %d 1000", d.Tanks, d.Troopers, d.Bombers, tanks-gotTanks, bd.Troopers-gotTroopers)
	}
	if a.Tanks != ba.Tanks+gotTanks || a.Troopers != ba.Troopers+gotTroopers || a.Bombers != ba.Bombers {
		t.Errorf("attacker holds %d tanks %d troopers, want %d %d", a.Tanks, a.Troopers, ba.Tanks+gotTanks, ba.Troopers+gotTroopers)
	}
	for _, want := range []string{"You captured", "of the enemy's forces"} {
		if !strings.Contains(report, want) {
			t.Errorf("report %q lacks %q", report, want)
		}
	}
	if got := d.Events[len(d.Events)-1].Msg.English(); !strings.Contains(got, "captured") || !strings.Contains(got, "of your forces") {
		t.Errorf("defender's event %q does not say what was captured", got)
	}
}

// A unit a kill destroyed cannot also be captured.
func TestKilledUnitsAreNotCaptured(t *testing.T) {
	base, ba, bd, bf := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	w, a, d, f := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	moddedConventional(w, EffectConventionalWon, func(fx *AttackEffects) {
		fx.Target.Kill[KillIndex(Tank)] = Roll{Pct: 100}
		capSet(&fx.Target, Tank, 100)
	})
	base.Attack(ba, bd, bf)
	w.Attack(a, d, f)
	if d.Tanks != 0 || a.Tanks != ba.Tanks {
		t.Errorf("defender %d tanks, attacker %d (base %d): nothing was left to capture", d.Tanks, a.Tanks, ba.Tanks)
	}
}

// A repelled attack's attacker set: the defender takes a share of the force the
// attacker SENT, as it stands after the battle, and never what was held back.
func TestLostAttackerSetCapturesOnlyTheSurvivorsOfTheSentForce(t *testing.T) {
	base, ba, bd, bf := battleWorld(8, 1000, 1_000_000, 2_000_000, 4000)
	w, a, d, f := battleWorld(8, 1000, 1_000_000, 2_000_000, 4000)
	for _, e := range []*Empire{ba, a} {
		e.Troopers *= 2 // half stays home
	}
	moddedConventional(w, EffectConventionalLost, func(fx *AttackEffects) {
		capSet(&fx.Attacker, Trooper, 50)
		capSet(&fx.Attacker, Tank, 100)
	})
	baseReport, _ := base.Attack(ba, bd, bf)
	report, _ := w.Attack(a, d, f)
	if !strings.Contains(baseReport, "Defeat") {
		t.Fatal("setup: the attack should be repelled")
	}
	// 4000 troopers were committed and 4000 held back: what is left of the
	// sent half is what the base run has beyond the held-back 4000.
	took := pctOf(ba.Troopers-bf.Troopers, 50)
	if took == 0 {
		t.Fatal("setup: the capture should take something")
	}
	if a.Troopers != ba.Troopers-took || d.Troopers != bd.Troopers+took {
		t.Errorf("attacker %d troopers defender %d, want %d and %d", a.Troopers, d.Troopers, ba.Troopers-took, bd.Troopers+took)
	}
	if a.Tanks != 0 || d.Tanks != bd.Tanks+ba.Tanks {
		t.Errorf("attacker %d tanks defender %d, want 0 and %d", a.Tanks, d.Tanks, bd.Tanks+ba.Tanks)
	}
	if !strings.Contains(report, "captured") || !strings.Contains(report, "of your forces") {
		t.Errorf("report %q does not tell the attacker", report)
	}
	if got := d.Events[len(d.Events)-1].Msg.English(); !strings.Contains(got, "You captured") {
		t.Errorf("defender's event %q does not tell the defender", got)
	}
}

// Both tables in one battle: the target set runs first, and the attacker set
// draws on the force sent, not on what the first set just handed over.
func TestBothTablesCaptureTargetFirst(t *testing.T) {
	base, ba, bd, bf := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	w, a, d, f := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	moddedConventional(w, EffectConventionalWon, func(fx *AttackEffects) {
		capSet(&fx.Target, Tank, 100)
		capSet(&fx.Attacker, Tank, 100)
	})
	base.Attack(ba, bd, bf)
	w.Attack(a, d, f)
	// The attacker takes every defending tank, then gives back every tank of
	// the force it sent that survived; ba.Tanks is exactly that force's tanks.
	if a.Tanks != bd.Tanks || d.Tanks != ba.Tanks {
		t.Errorf("attacker %d tanks defender %d, want %d and %d", a.Tanks, d.Tanks, bd.Tanks, ba.Tanks)
	}
}

// An ally's detachment can be captured at the same percent, but only what
// survived of the 30% it sent, never its whole stock; capture_from_stock does
// not widen that.
func TestAlliesDetachmentIsTheBaseNotItsStock(t *testing.T) {
	build := func() (*World, *Empire, *Empire, *Empire, AttackForce) {
		w, a, d, f := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
		ally := w.AddHuman("ally", "Carthage")
		ally.Protection = 0
		ally.Troopers, ally.Tanks = 10_000, 5_000
		w.setRelation(d.Name, ally.Name, fullDefenseAlliance)
		return w, a, d, ally, f
	}
	base, ba, bd, bally, bf := build()
	w, a, d, ally, f := build()
	moddedConventional(w, EffectConventionalWon, func(fx *AttackEffects) {
		capSet(&fx.Target, Trooper, 100)
		fx.Target.CaptureFromStock = 1
	})
	baseReport, _ := base.Attack(ba, bd, bf)
	w.Attack(a, d, f)
	if !strings.Contains(baseReport, "Victory") {
		t.Fatal("setup: the attack should win")
	}
	left := allySent(10_000) - (10_000 - bally.Troopers) // the detachment less what the base run bled from it
	if ally.Troopers != bally.Troopers-left || ally.Tanks != bally.Tanks {
		t.Errorf("ally holds %d troopers %d tanks, want %d and %d", ally.Troopers, ally.Tanks, bally.Troopers-left, bally.Tanks)
	}
	if d.Troopers != 0 {
		t.Errorf("defender holds %d troopers, want 0 (from_stock takes the whole stock)", d.Troopers)
	}
	if a.Troopers != ba.Troopers+bd.Troopers+left {
		t.Errorf("attacker holds %d troopers, want %d", a.Troopers, ba.Troopers+bd.Troopers+left)
	}
	if got := ally.Events[len(ally.Events)-1].Msg.English(); !strings.Contains(got, "captured") || !strings.Contains(got, "of the troops you sent") {
		t.Errorf("ally's event %q", got)
	}
}

// capture_from_stock widens a defender's base to units that did not stand in
// the defense; the default leaves them alone.
func TestCaptureFromStockWidensTheDefendersBase(t *testing.T) {
	for _, fromStock := range []int{0, 1} {
		w, a, d, f := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
		d.Bombers, d.Carriers = 1000, 500
		moddedConventional(w, EffectConventionalWon, func(fx *AttackEffects) {
			capSet(&fx.Target, Carrier, 50)
			capSet(&fx.Target, Jet, 50)
			fx.Target.CaptureFromStock = fromStock
		})
		jets, carriers := d.Jets, a.Carriers
		w.Attack(a, d, f)
		wantCarriers := 500 * fromStock
		if d.Carriers != 500-wantCarriers/2 || a.Carriers != carriers+wantCarriers/2 {
			t.Errorf("from_stock %d: defender %d carriers attacker %d", fromStock, d.Carriers, a.Carriers)
		}
		if d.Jets >= jets {
			t.Errorf("from_stock %d: jets stood in the defense and should be captured either way", fromStock)
		}
	}
}

// capture_random takes its share of all the surviving units at once, and the
// draw is made only when the key is set.
func TestCaptureRandomTakesItsShareAndOnlyDrawsWhenSet(t *testing.T) {
	total := func(e *Empire) int {
		n := 0
		for _, g := range MilitaryGoods {
			n += *g.Count(e)
		}
		return n
	}
	base, ba, bd, bf := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	w, a, d, f := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	moddedConventional(w, EffectConventionalWon, func(fx *AttackEffects) {
		fx.Target.CaptureRandom = Roll{Pct: 10}
		fx.Target.CaptureFromStock = 1
	})
	base.Attack(ba, bd, bf)
	w.Attack(a, d, f)
	want := pctOf(total(bd), 10)
	if got := total(a) - total(ba); got != want || total(d) != total(bd)-want || want == 0 {
		t.Errorf("attacker gained %d units, defender holds %d, want %d taken from %d", got, total(d), want, total(bd))
	}
	for _, g := range MilitaryGoods {
		if *g.Count(d) > *g.Count(bd) || *g.Count(a) < *g.Count(ba) {
			t.Errorf("%s moved the wrong way", g.Plural)
		}
	}

	// A named capture draws nothing either: the stream stays the base game's.
	w2, a2, d2, f2 := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	b2, ba2, bd2, bf2 := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	moddedConventional(w2, EffectConventionalWon, func(fx *AttackEffects) { capSet(&fx.Target, Tank, 10) })
	b2.Attack(ba2, bd2, bf2)
	w2.Attack(a2, d2, f2)
	if x, y := w2.rng.Intn(1<<30), b2.rng.Intn(1<<30); x != y {
		t.Error("a capture_<unit> with no jitter drew from the stream")
	}
}

func TestTakeRandomIsExactAndStaysInsideEveryHolding(t *testing.T) {
	for seed := int64(1); seed <= 200; seed++ {
		w := NewWorldSeed(DefaultConfig(), seed)
		e := w.AddHuman("x", "X")
		e.Troopers, e.Jets, e.Turrets, e.Bombers, e.Tanks, e.Carriers = 1000, 7, 0, 3, 50_000, 1
		var base unitCounts
		for i, g := range MilitaryGoods {
			base[i] = *g.Count(e)
		}
		before := base
		fx := EffectSet{CaptureRandom: Roll{Pct: 37}}
		took := w.takeUnits(fx, []captureSource{empireSource(e, base)})
		sum := 0
		for i, g := range MilitaryGoods {
			sum += took[0][i]
			if took[0][i] < 0 || took[0][i] > before[i] || *g.Count(e) != before[i]-took[0][i] {
				t.Fatalf("seed %d: %s took %d of %d, holds %d", seed, g.Plural, took[0][i], before[i], *g.Count(e))
			}
		}
		if want := pctOf(51_011, 37); sum != want {
			t.Fatalf("seed %d: took %d units, want %d", seed, sum, want)
		}
	}
}

// An interplanetary win: the units the attackers capture from a defender travel
// home in the result and are split among the contributors with nothing lost or
// made.
func TestIPWonTargetSetCarriesCapturedUnitsHome(t *testing.T) {
	wA, wB, cs := ipBoards(3, 2_000, 4_000_000)
	_, wBase, _ := ipBoards(3, 2_000, 4_000_000)
	edit := func(fx *AttackEffects) { capSet(&fx.Target, Tank, 20) }
	moddedIP(wB, EffectIPAttackWon, edit)
	moddedIP(wA, EffectIPAttackWon, edit) // the league plays one ruleset, which is what bounds the packet
	atk, sent := ipStrike(7, "Victim", true, NormalAttack, cs)
	base := wBase.resolveRemoteAttack(atk)
	res := wB.resolveRemoteAttack(atk)
	if !base.Won || !res.Won {
		t.Fatal("setup: the attack should win")
	}
	bv, v := wBase.FindByNameOrFormer("Victim"), wB.FindByNameOrFormer("Victim")
	want := pctOf(bv.Tanks, 20)
	if want == 0 || v.Tanks != bv.Tanks-want {
		t.Fatalf("defender holds %d tanks, want %d", v.Tanks, bv.Tanks-want)
	}
	if res.Extra == nil || res.Extra.Taken[gi(Tank)] != want {
		t.Fatalf("result carries %+v, want %d tanks taken", res.Extra, want)
	}
	if got := v.Events[len(v.Events)-1].Msg.English(); !strings.Contains(got, "The attackers captured") {
		t.Errorf("defender's event %q", got)
	}
	before := 0
	for _, c := range sent.Contributors {
		before += wA.FindByOwner(c.Owner).Tanks
	}
	wA.InFlight = []InFlightStrike{sent}
	wA.applyAttackResult(res, nil)
	after := 0
	for _, c := range sent.Contributors {
		after += wA.FindByOwner(c.Owner).Tanks
	}
	returned := 0
	for _, s := range res.Survivors {
		returned += s.Tanks
	}
	if after-before != returned+want {
		t.Errorf("contributors gained %d tanks, want %d returning plus %d captured", after-before, returned, want)
	}
	if e := wA.FindByOwner(sent.Contributors[0].Owner); !strings.Contains(e.Events[len(e.Events)-1].Msg.English(), "captured") {
		t.Errorf("a contributor's report does not mention the capture")
	}
}

// An interplanetary defender who captures the attacker's units: the result
// sends home the survivors minus them, and the defenders keep them.
func TestIPAttackerSetCapturesOnTheDefendingBoard(t *testing.T) {
	for _, kind := range []EffectKind{EffectIPAttackWon, EffectIPAttackLost} {
		force := 4_000_000
		if kind == EffectIPAttackLost {
			force = 10
		}
		wA, wB, cs := ipBoards(3, 40_000, force)
		_, wBase, _ := ipBoards(3, 40_000, force)
		moddedIP(wB, kind, func(fx *AttackEffects) { capSet(&fx.Attacker, Trooper, 25) })
		atk, sent := ipStrike(7, "", true, NormalAttack, cs)
		base := wBase.resolveRemoteAttack(atk)
		res := wB.resolveRemoteAttack(atk)
		if res.Won != (kind == EffectIPAttackWon) {
			t.Fatalf("%s: setup: outcome %v", kind, res.Won)
		}
		taken := 0
		for i, c := range res.Survivors {
			w := pctOf(base.Survivors[i].Troopers, 25)
			if c.Troopers != base.Survivors[i].Troopers-w {
				t.Errorf("%s: contributor %d returns %d troopers, want %d", kind, i, c.Troopers, base.Survivors[i].Troopers-w)
			}
			taken += w
		}
		if taken == 0 {
			t.Fatalf("%s: setup: nothing was captured", kind)
		}
		seized := 0
		for _, s := range res.Seized {
			seized += s.Troopers
		}
		if seized != taken {
			t.Errorf("%s: result says %d seized, want %d", kind, seized, taken)
		}
		gained, ev := 0, 0
		for _, e := range wB.Empires {
			if e.Protection > 0 {
				continue
			}
			b := wBase.FindByNameOrFormer(e.Name)
			gained += e.Troopers - b.Troopers
			if len(e.Events) > 0 && strings.Contains(e.Events[len(e.Events)-1].Msg.English(), "You captured") {
				ev++
			}
		}
		// The base battle's own losses are the same on both worlds (no draw was
		// added), so the gain is exactly what was captured.
		if gained != taken || ev == 0 {
			t.Errorf("%s: defenders gained %d troopers (%d told), want %d", kind, gained, ev, taken)
		}
		wA.InFlight = []InFlightStrike{sent}
		wA.applyAttackResult(res, nil)
		for _, c := range sent.Contributors {
			e := wA.FindByOwner(c.Owner)
			if !strings.Contains(e.Events[len(e.Events)-1].Msg.English(), "The defenders captured") {
				t.Errorf("%s: %s's report does not mention what was captured", kind, c.Owner)
			}
		}
	}
}

func TestCaptureValidation(t *testing.T) {
	try := func(edit func(r *Rules)) string {
		r := BaseRules()
		edit(r)
		if err := r.Validate(); err != nil {
			return err.Error()
		}
		return ""
	}
	set := func(kind EffectKind, side string, edit func(s *EffectSet)) func(r *Rules) {
		return func(r *Rules) {
			fx := r.Attacks[kind]
			edit(fx.Side(side))
			r.Attacks[kind] = fx
		}
	}
	if got := try(set(EffectConventionalWon, "target", func(s *EffectSet) { capSet(s, Tank, 100) })); got != "" {
		t.Errorf("a battle may capture: %v", got)
	}
	for _, kind := range []EffectKind{EffectNuclear, EffectChemical, EffectBiological, EffectIPNuclear, EffectIPChemical,
		EffectSabreIntelligence, EffectSabrePeople, EffectSabreMilitaryBases, EffectSabreAirbases, EffectSabreRegions, EffectSabreFood} {
		for _, edit := range []func(s *EffectSet){
			func(s *EffectSet) { capSet(s, Tank, 1) },
			func(s *EffectSet) { s.CaptureRandom = Roll{Pct: 1} },
			func(s *EffectSet) { s.CaptureFromStock = 1 },
		} {
			if got := try(set(kind, "target", edit)); !strings.Contains(got, "only a battle can capture units") {
				t.Errorf("%s: a missile captured units: %q", kind, got)
			}
		}
	}
	for name, c := range map[string]struct {
		edit func(r *Rules)
		want string
	}{
		"101 percent": {set(EffectConventionalLost, "target", func(s *EffectSet) { capSet(s, Jet, 101) }), "capture_jet: percent 101 is outside 0 to 100"},
		"negative":    {set(EffectConventionalLost, "target", func(s *EffectSet) { capSet(s, Jet, -1) }), "capture_jet: percent -1"},
		"random":      {set(EffectIPAttackWon, "target", func(s *EffectSet) { s.CaptureRandom = Roll{Pct: 200} }), "capture_random: percent 200"},
		"jitter":      {set(EffectIPAttackWon, "target", func(s *EffectSet) { s.CaptureRandom = Roll{Pct: 1, Up: 101} }), "capture_random: up 101"},
		"switch":      {set(EffectConventionalWon, "target", func(s *EffectSet) { s.CaptureFromStock = 2 }), "capture_from_stock: 2 is outside 0 to 1"},
		"attacker stock": {set(EffectConventionalWon, "attacker", func(s *EffectSet) { s.CaptureFromStock = 1 }),
			"attacker.capture_from_stock: an attacker's capture is always of the force it sent"},
		"attacker turrets": {set(EffectConventionalWon, "attacker", func(s *EffectSet) { capSet(s, Turret, 5) }),
			"attacker.capture_turret: an attack sends no Turrets"},
		"attacker carriers": {set(EffectIPAttackLost, "attacker", func(s *EffectSet) { capSet(s, Carrier, 5) }),
			"attacker.capture_carrier: an attack sends no Carriers"},
	} {
		if got := try(c.edit); !strings.Contains(got, c.want) {
			t.Errorf("%s: Validate said %q, want it to contain %q", name, got, c.want)
		}
	}
}

// The target board's word is not taken for units: a figure the rules cannot
// have produced, a negative one, and a huge one credit nothing beyond what a
// realm can hold, and a seized figure larger than the force sent is held to it.
func TestHostileResultDoesNotMintUnits(t *testing.T) {
	build := func(allowed bool) (*World, InFlightStrike, AttackResult) {
		wA, _, cs := ipBoards(3, 2_000, 1000)
		if allowed {
			moddedIP(wA, EffectIPAttackWon, func(fx *AttackEffects) { capSet(&fx.Target, Tank, 1) })
		}
		atk, sent := ipStrike(7, "Victim", false, NormalAttack, cs)
		_ = atk
		res := AttackResult{ID: 7, TargetBoard: "boardB", TargetEmpire: "Victim", Won: true, Outcome: OutcomeWon,
			Survivors: []Contribution{{Owner: sent.Contributors[0].Owner}},
			Extra:     &EffectLoss{Taken: []int{-5, MaxUnits, MaxUnits, -1, MaxUnits, MaxUnits, 99, 99}}}
		wA.InFlight = []InFlightStrike{sent}
		return wA, sent, res
	}
	w, sent, res := build(false)
	e := w.FindByOwner(sent.Contributors[0].Owner)
	before := *e
	w.applyAttackResult(res, nil)
	if e.Troopers != before.Troopers || e.Tanks != before.Tanks || e.Carriers != before.Carriers || e.Turrets != before.Turrets {
		t.Errorf("units were credited though the rules capture nothing: %d %d %d %d", e.Troopers, e.Tanks, e.Carriers, e.Turrets)
	}
	w, sent, res = build(true)
	e = w.FindByOwner(sent.Contributors[0].Owner)
	before = *e
	w.applyAttackResult(res, nil)
	if e.Tanks != MaxUnits || e.Troopers != before.Troopers || e.Tanks < 0 || e.Jets != before.Jets {
		t.Errorf("tanks %d troopers %d: a huge figure must stop at what a realm holds, and only for the unit the rules capture", e.Tanks, e.Troopers)
	}
	got := seizedOf(AttackForce{Troopers: 10, Tanks: 5}, AttackForce{Troopers: MaxUnits, Tanks: -3, Jets: 9})
	if got[gi(Trooper)] != 10 || got[gi(Tank)] != 0 || got[gi(Jet)] != 0 {
		t.Errorf("seized report figures %v are not held to what was sent", got)
	}
}

// A defender and two allies near MaxUnits sum past a 32-bit int; the capture
// saturates instead of wrapping the attacker negative (run under GOARCH=386).
func TestCaptureSumsSaturateInsteadOfWrapping(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a, d := w.AddHuman("a", "Alpha"), w.AddHuman("d", "Delta")
	a1, a2 := w.AddHuman("x", "Xerxes"), w.AddHuman("y", "Yara")
	d.Tanks, a1.Tanks, a2.Tanks = MaxUnits, MaxUnits, MaxUnits
	var left unitCounts
	left[gi(Tank)] = MaxUnits
	cx := &captureCtx{allies: []allyDetachment{{a1, left}, {a2, left}}}
	fx := EffectSet{}
	capSet(&fx, Tank, 100)
	own, all := w.takeFrom(fx, d, a, cx)
	if own[gi(Tank)] != MaxUnits || all[gi(Tank)] != MaxUnits || a.Tanks != MaxUnits || a.Tanks < 0 {
		t.Errorf("own %d all %d attacker %d, want every one held at %d", own[gi(Tank)], all[gi(Tank)], a.Tanks, MaxUnits)
	}
}

// Contributors near MaxUnits on a group attack sum past a 32-bit int on the
// defending board, and an incoming count is held to what a realm can hold.
func TestRemoteSeizeSumsAndBoundsTheIncomingForce(t *testing.T) {
	_, wB, _ := ipBoards(3, 2_000, 1000)
	moddedIP(wB, EffectIPAttackWon, func(fx *AttackEffects) { capSet(&fx.Attacker, Trooper, 100) })
	res := AttackResult{Survivors: []Contribution{
		{Owner: "a", AttackForce: AttackForce{Troopers: MaxUnits}},
		{Owner: "b", AttackForce: AttackForce{Troopers: MaxUnits}},
		{Owner: "c", AttackForce: AttackForce{Troopers: -7}},
	}}
	v := wB.FindByNameOrFormer("Victim")
	before := v.Troopers
	wB.remoteSeize(true, &res, []*Empire{v})
	if v.Troopers != MaxUnits || v.Troopers < before {
		t.Errorf("defender holds %d troopers, want %d", v.Troopers, MaxUnits)
	}
	if len(res.Seized) != 2 || res.Survivors[2].Troopers != -7 {
		t.Errorf("seized %+v: only the two sound contributions may be taken from", res.Seized)
	}
}

// A unit that never stands in a defense is creditable only when the rules
// widen the base to the whole stock.
func TestForgedTakenBombersNeedFromStock(t *testing.T) {
	for _, fromStock := range []int{0, 1} {
		wA, _, cs := ipBoards(3, 2_000, 1000)
		moddedIP(wA, EffectIPAttackWon, func(fx *AttackEffects) {
			fx.Target.CaptureUnits[gi(Bomber)] = Roll{Pct: 1}
			fx.Target.CaptureFromStock = fromStock
		})
		_, sent := ipStrike(7, "Victim", false, NormalAttack, cs)
		taken := make([]int, numCaptureGoods)
		taken[gi(Bomber)] = 50
		res := AttackResult{ID: 7, TargetBoard: "boardB", TargetEmpire: "Victim", Won: true, Outcome: OutcomeWon,
			Survivors: []Contribution{{Owner: sent.Contributors[0].Owner}}, Extra: &EffectLoss{Taken: taken}}
		wA.InFlight = []InFlightStrike{sent}
		e := wA.FindByOwner(sent.Contributors[0].Owner)
		before := e.Bombers
		wA.applyAttackResult(res, nil)
		if want := before + 50*fromStock; e.Bombers != want {
			t.Errorf("from_stock %d: bombers %d, want %d", fromStock, e.Bombers, want)
		}
	}
}

// The defender is told of its own units only; the ally of its own.
func TestDefenderEventNamesOnlyItsOwnUnits(t *testing.T) {
	w, a, d, f := battleWorld(8, 1000, 1_000_000, 10_000, 5_000_000)
	ally := w.AddHuman("ally", "Carthage")
	ally.Protection = 0
	ally.Troopers = 10_000
	w.setRelation(d.Name, ally.Name, fullDefenseAlliance)
	d.Troopers = 0
	moddedConventional(w, EffectConventionalWon, func(fx *AttackEffects) { capSet(&fx.Target, Trooper, 100) })
	w.Attack(a, d, f)
	if got := d.Events[len(d.Events)-1].Msg.English(); strings.Contains(got, "captured") {
		t.Errorf("defender was told of units it never held: %q", got)
	}
	if got := ally.Events[len(ally.Events)-1].Msg.English(); !strings.Contains(got, "captured") {
		t.Errorf("ally's event %q", got)
	}
}

// A win that takes the defender's last region conquers it: the attacker set's
// unit capture is not taken, so the battle plays as the base game's does.
func TestConquestSkipsTheAttackerSetsUnitCapture(t *testing.T) {
	base, ba, bd, bf := battleWorld(8, 1, 1000, 1, 4000)
	w, a, d, f := battleWorld(8, 1, 1000, 1, 4000)
	moddedConventional(w, EffectConventionalWon, func(fx *AttackEffects) {
		capSet(&fx.Attacker, Trooper, 50)
		fx.Attacker.CaptureRandom = Roll{Pct: 50}
	})
	baseReport, _ := base.Attack(ba, bd, bf)
	report, _ := w.Attack(a, d, f)
	if !strings.Contains(baseReport, "crushed") {
		t.Fatalf("setup: the attack should conquer the defender: %q", baseReport)
	}
	if a.Troopers != ba.Troopers || a.Tanks != ba.Tanks || report != baseReport {
		t.Errorf("troopers %d tanks %d, base %d and %d; report differs: %v", a.Troopers, a.Tanks, ba.Troopers, ba.Tanks, report != baseReport)
	}
}
