package game

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

// The three functions below are the strike routines as they were before the
// effect table, kept verbatim as the reference the table must reproduce.

func refNuclear(w *World, a, d *Empire) (string, error) {
	if err := payArmsDealer(a, w.NukeCost(d)); err != nil {
		return "", err
	}
	w.BreachTreaty(a, d)
	pct := NukeWastePct + w.rng.Intn(NukeWasteJitter) - w.rng.Intn(NukeWasteJitter)
	regions := ruinToWaste(d, pct)
	addScore(a, w.rng.Intn(NukeScoreRoll))
	d.addEvent(say("{from}'s nuclear strike turned {n} of your regions into waste.", "from", a.Name, "n", regions))
	w.postStrikeNews(a, d, "nuclear")
	return fmt.Sprintf("Your nuclear strike turned %d of %s's regions into waste.", regions, d.Name), nil
}

func refChemical(w *World, a, d *Empire) (string, error) {
	if err := payArmsDealer(a, w.ChemCost(d)); err != nil {
		return "", err
	}
	w.BreachTreaty(a, d)
	pct := ChemWastePct + w.rng.Intn(ChemWasteJitter) - w.rng.Intn(ChemWasteJitter)
	regions := ruinToWaste(d, pct)
	people := int(int64(d.People) * ChemPopKillPct / 100)
	d.People -= people
	d.Morale = roundDiv(d.Morale*ChemMoraleKeepNum, ChemMoraleKeepDen)
	d.Support = roundDiv(d.Support*StrikeSupportKeepNum, StrikeSupportKeepDen)
	addScore(a, w.rng.Intn(ChemScoreRoll))
	d.addEvent(say("{from}'s chemical strike laid waste to {n} of your regions and killed {people} of your people; famine follows.",
		"from", a.Name, "n", regions, "people", people))
	w.postStrikeNews(a, d, "chemical")
	return fmt.Sprintf("Your gas laid waste to %d of %s's regions and killed %d of their people; their morale and support are broken.", regions, d.Name, people), nil
}

func refBiological(w *World, a, d *Empire) (string, error) {
	if err := payArmsDealer(a, w.BioCost(d)); err != nil {
		return "", err
	}
	w.BreachTreaty(a, d)
	addScore(a, w.rng.Intn(BioScoreRoll))
	popPct := BioPopKillPct + w.rng.Intn(BioPopKillJitterUp) - w.rng.Intn(BioPopKillJitterDown)
	people := int(int64(d.People) * int64(popPct) / 100)
	d.People -= people
	troopPct := BioTroopKillPct + w.rng.Intn(BioTroopKillJitterUp) - w.rng.Intn(BioTroopKillJitterDn)
	troops := pctOf(d.Troopers, troopPct)
	d.Troopers -= troops
	d.Morale /= BioMoraleDivisor
	d.Support = roundDiv(d.Support*StrikeSupportKeepNum, StrikeSupportKeepDen)
	d.addEvent(say("{from}'s plague killed {troopers} of your troopers and {people} of your people; famine follows.",
		"from", a.Name, "troopers", troops, "people", people))
	w.postStrikeNews(a, d, "biological")
	return fmt.Sprintf("Your plague killed %d of %s's troopers and %d civilians, and took half their morale with it.", troops, d.Name, people), nil
}

// strikeWorld is a seeded world with an attacker and a target of a given size.
func strikeWorld(seed int64, land, people, troopers int) (*World, *Empire, *Empire) {
	w := NewWorldSeed(DefaultConfig(), seed)
	w.AddAIEmpires(1)
	a := w.AddHuman("att", "Attacker")
	a.Protection, a.Gold = 0, 1_000_000_000_000
	a.Founded = "" // a wall-clock stamp, which would differ between two runs
	d := w.Empires[0]
	d.Protection = 0
	d.Regions = RegionMix{Coastal: land / 2, Agricultural: land - land/2 - land/7, Industrial: land / 7}
	d.syncLand()
	d.People, d.Troopers = people, troopers
	d.Turrets, d.Jets, d.Tanks, d.Bombers, d.Carriers, d.Agents = troopers/3, troopers/5, troopers/7, 11, 13, 17
	d.Food, d.HQ = people/9, 60
	d.Morale, d.Support = 40+int(seed%61), 30+int(seed%71)
	d.SDIFunding = 5_000_000
	d.syncSDI()
	return w, a, d
}

func TestBaseStrikesMatchTheOriginalRoutines(t *testing.T) {
	sizes := []struct{ land, people, troopers int }{
		{0, 0, 0}, {1, 5, 1}, {3, 1000, 7}, {17, 99_999, 333}, {200, 3_000_000, 50_000},
		{1000, 90_000_000, 2_000_000}, {1_000_000, 2_000_000_000, 2_000_000_000},
	}
	type strike struct {
		name     string
		got, ref func(w *World, a, d *Empire) (string, error)
	}
	strikes := []strike{
		{"nuclear", (*World).NuclearStrike, refNuclear},
		{"chemical", (*World).ChemicalStrike, refChemical},
		{"biological", (*World).BiologicalStrike, refBiological},
	}
	for _, s := range strikes {
		for _, sz := range sizes {
			for seed := int64(1); seed <= 120; seed++ {
				w1, a1, d1 := strikeWorld(seed, sz.land, sz.people, sz.troopers)
				w2, a2, d2 := strikeWorld(seed, sz.land, sz.people, sz.troopers)
				r1, e1 := s.got(w1, a1, d1)
				r2, e2 := s.ref(w2, a2, d2)
				if r1 != r2 || (e1 == nil) != (e2 == nil) {
					t.Fatalf("%s %v seed %d: report %q, want %q", s.name, sz, seed, r1, r2)
				}
				for _, e := range []*Empire{d1, d2} {
					for i := range e.Events {
						e.Events[i].When = time.Time{}
					}
				}
				if !reflect.DeepEqual(d1, d2) || !reflect.DeepEqual(a1, a2) {
					t.Fatalf("%s %v seed %d: empires differ\n got  %+v\n want %+v", s.name, sz, seed, d1, d2)
				}
				if x, y := w1.rng.Intn(1<<30), w2.rng.Intn(1<<30); x != y {
					t.Fatalf("%s %v seed %d: rng streams diverged", s.name, sz, seed)
				}
			}
		}
	}
}

func TestKillGoodsTableMatchesItsArray(t *testing.T) {
	if len(AttackKillGoods) != numKillGoods {
		t.Fatalf("AttackKillGoods has %d rows, numKillGoods is %d", len(AttackKillGoods), numKillGoods)
	}
}

func TestModdedNuclearHitsTurretsAndCapturesLand(t *testing.T) {
	w, a, d := strikeWorld(5, 1000, 1_000_000, 10_000)
	rules := BaseRules()
	fx := rules.Attacks[EffectNuclear]
	fx.Target.Kill[KillIndex(Turret)] = Roll{Pct: 30}
	fx.Target.Capture = Roll{Pct: 5}
	rules.Attacks[EffectNuclear] = fx
	w.Rules = rules
	d.Regions = RegionMix{Agricultural: 1000}
	d.syncLand()
	d.Turrets = 200
	report, err := w.NuclearStrike(a, d)
	if err != nil {
		t.Fatal(err)
	}
	// Waste comes first (70 of 1000 on this seed), then the capture takes 5% of
	// the 1000 spread over every type, waste included, so 3 of the 70 go with it.
	if a.PendingRegions != 50 {
		t.Errorf("captured %d regions, want 50", a.PendingRegions)
	}
	if d.Turrets != 140 {
		t.Errorf("turrets %d, want 140", d.Turrets)
	}
	if d.Land != 950 {
		t.Errorf("land %d, want 950", d.Land)
	}
	if d.Regions.Waste != 67 {
		t.Errorf("waste %d, want 67", d.Regions.Waste)
	}
	want := "Your nuclear strike turned 70 of " + d.Name + "'s regions into waste. It also took 50 Regions seized and 60 Turrets."
	if report != want {
		t.Errorf("report %q, want %q", report, want)
	}
}

func TestModdedBlowbackHitsTheAttacker(t *testing.T) {
	w, a, d := strikeWorld(9, 400, 1_000_000, 10_000)
	rules := BaseRules()
	fx := rules.Attacks[EffectNuclear]
	fx.Target.Waste = Roll{}
	fx.Attacker.Kill[KillIndex(Trooper)] = Roll{Pct: 10}
	fx.Attacker.Capture = Roll{Pct: 10}
	fx.Attacker.Morale, fx.Attacker.Support = Fraction{1, 2}, Fraction{1, 1}
	rules.Attacks[EffectNuclear] = fx
	w.Rules = rules
	a.Regions = RegionMix{Agricultural: 100}
	a.syncLand()
	a.Troopers, a.Morale = 1000, 80
	coastal := d.Regions.Coastal
	report, err := w.NuclearStrike(a, d)
	if err != nil {
		t.Fatal(err)
	}
	if a.Troopers != 900 || a.Morale != 40 || a.Land != 90 || d.Regions.Coastal != coastal+10 {
		t.Errorf("attacker troopers %d morale %d land %d, target coastal %d", a.Troopers, a.Morale, a.Land, d.Regions.Coastal)
	}
	want := "Your nuclear strike hit " + d.Name + "'s realm and took nothing from it. It also cost you 10 Regions seized, 100 Troopers, and 40 points of morale."
	if report != want {
		t.Errorf("report %q, want %q", report, want)
	}
}

func TestHeadlineFallsBackWhenItsEffectsAreZeroed(t *testing.T) {
	w, a, d := strikeWorld(3, 100, 1_000_000, 100)
	rules := BaseRules()
	fx := rules.Attacks[EffectNuclear]
	fx.Target.Waste = Roll{}
	fx.Target.People = Roll{Pct: 10}
	rules.Attacks[EffectNuclear] = fx
	w.Rules = rules
	report, err := w.NuclearStrike(a, d)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Your nuclear strike hit " + d.Name + "'s realm, taking 100000 People."; report != want {
		t.Errorf("report %q, want %q", report, want)
	}
}

func TestCaptureReachesAnAIRealmAsCoastalAndAPlayerAsPending(t *testing.T) {
	w, a, d := strikeWorld(4, 1000, 1_000_000, 100)
	if a.Owner == "" || d.Owner != "" {
		t.Fatal("setup: attacker should be a player and target an AI realm")
	}
	rules := BaseRules()
	fx := rules.Attacks[EffectNuclear]
	fx.Target.Waste = Roll{}
	fx.Target.Capture = Roll{Pct: 10}   // the player gains land
	fx.Attacker.Capture = Roll{Pct: 10} // the AI realm gains land
	rules.Attacks[EffectNuclear] = fx
	w.Rules = rules
	d.Regions = RegionMix{Agricultural: 1000}
	d.syncLand()
	a.Regions = RegionMix{Agricultural: 200}
	a.syncLand()
	if _, err := w.NuclearStrike(a, d); err != nil {
		t.Fatal(err)
	}
	// Target set: 100 of the AI realm's 1000 go to the player's pending pool.
	// Attacker set: 10% of the player's 200 go to the AI as Coastal.
	if a.PendingRegions != 100 || a.Land != 180 {
		t.Errorf("player pending %d land %d, want 100 and 180", a.PendingRegions, a.Land)
	}
	if d.PendingRegions != 0 || d.Regions.Coastal != 20 || d.Land != 920 {
		t.Errorf("AI pending %d coastal %d land %d, want 0, 20 and 920", d.PendingRegions, d.Regions.Coastal, d.Land)
	}
}

func TestHeadlineNeedsEveryEffectItNames(t *testing.T) {
	w, a, d := strikeWorld(3, 100, 1_000_000, 100)
	rules := BaseRules()
	fx := rules.Attacks[EffectChemical]
	fx.Target.Waste = Roll{}
	rules.Attacks[EffectChemical] = fx
	w.Rules = rules
	report, err := w.ChemicalStrike(a, d)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Your chemical strike hit " + d.Name + "'s realm, taking 200000 People, 11 points of morale, and 11 points of support."; report != want {
		t.Errorf("report %q, want %q", report, want)
	}
}
