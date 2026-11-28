package game

import (
	"reflect"
	"strings"
	"testing"
)

// refApplyIP is the arriving nuclear and chemical branches of applySpecialOp as
// they were before the attack table: the gates, the band and the cap in code.
func refApplyIP(w *World, op SpecialOp, d *Empire, from string) (report Msg, score int, outcome specialOutcome) {
	board := w.Config.BoardID
	if stopped, why := w.stopArrivingMissile(d, op, from); why != specialHit {
		return stopped, 0, why
	}
	if op == OpNuclear {
		pct := IPNukeWastePctBase + w.rng.Intn(IPNukeWastePctRoll)
		regions := ruinToWasteCapped(d, pct, IPMissileRegionCap)
		score = w.rng.Intn(NukeScoreRoll)
		d.addEvent(say("{from}'s nuclear strike turned {n} of your regions into waste.", "from", from, "n", regions))
		return say("Your nuclear strike turned {n} of {who}'s regions on {board} into waste.",
			"n", regions, "who", d.Name, "board", board), score, specialHit
	}
	pct := IPChemKillPctBase + w.rng.Intn(IPChemKillPctRoll)
	dead := min(pctOf(d.People, pct), IPMissilePeopleCap)
	d.People -= dead
	score = w.rng.Intn(ChemScoreRoll)
	d.addEvent(say("{from}'s chemical strike killed {n} of your people.", "from", from, "n", dead))
	return say("Your chemical strike killed {n} of {who}'s people on {board}.",
		"n", dead, "who", d.Name, "board", board), score, specialHit
}

// remoteTarget is a seeded world and a realm holding n of everything a missile
// can hit. Troopers stay small so the Sabre's backfire and the garrison roll
// are seen on some seeds and not others.
func remoteTarget(seed int64, n, land int) (*World, *Empire) {
	w := NewWorldSeed(DefaultConfig(), seed)
	d := w.AddHuman("d", "Defendia")
	d.Founded = ""
	d.Regions = RegionMix{Coastal: land / 2, Agricultural: land - land/2}
	d.syncLand()
	d.People, d.Troopers, d.Jets, d.Turrets, d.Tanks, d.Bombers, d.Carriers = n*50, n, n, n, n, n, n
	d.Agents, d.Food, d.SDI = n, n, 0
	d.SDIFunding = 0
	return w, d
}

var remoteCounts = []int{0, 1, 2, 3, 7, 99, 100, 101, 244, 300, 301, 999, 12345, 1_000_000, 40_000_000}

func TestArrivingNuclearAndChemicalMatchTheOriginalRoutine(t *testing.T) {
	for _, op := range []SpecialOp{OpNuclear, OpChemical} {
		for _, land := range []int{0, 1, 40, 1000, 400_000} {
			for _, n := range remoteCounts {
				for seed := int64(1); seed <= 40; seed++ {
					w1, d1 := remoteTarget(seed, n, land)
					w2, d2 := remoteTarget(seed, n, land)
					r1, s1, o1, _ := w1.applySpecialOp(op, d1, "Selby of Home", 0)
					r2, s2, o2 := refApplyIP(w2, op, d2, "Selby of Home")
					clearStamps(d1, d2)
					if r1.English() != r2.English() || s1 != s2 || o1 != o2 || !reflect.DeepEqual(d1, d2) {
						t.Fatalf("op %v land %d n %d seed %d: %q vs %q", op, land, n, seed, r1.English(), r2.English())
					}
					if w1.rng.Intn(1<<30) != w2.rng.Intn(1<<30) {
						t.Fatalf("op %v land %d n %d seed %d: rng streams diverged", op, land, n, seed)
					}
				}
			}
		}
	}
}

func TestSabreMatchesTheOriginalRoutineOnEveryDialRow(t *testing.T) {
	hits := map[string]int{}
	for dial := SabreDialMin; dial <= SabreDialMax; dial++ {
		for _, land := range []int{0, 7, 1000} {
			for _, n := range remoteCounts {
				for seed := int64(1); seed <= 40; seed++ {
					w1, d1 := remoteTarget(seed, n, land)
					w2, d2 := remoteTarget(seed, n, land)
					d1.Troopers, d2.Troopers = n/50, n/50 // let the backfire show on some seeds only
					r1, o1, g1 := w1.sabreEffect(d1, "Selby of Home", dial)
					r2, o2, g2 := w2.refSabreEffect(d2, "Selby of Home", dial)
					clearStamps(d1, d2)
					if r1.English() != r2.English() || o1 != o2 || g1 != g2 || !reflect.DeepEqual(d1, d2) {
						t.Fatalf("dial %d land %d n %d seed %d: %q vs %q", dial, land, n, seed, r1.English(), r2.English())
					}
					if w1.rng.Intn(1<<30) != w2.rng.Intn(1<<30) {
						t.Fatalf("dial %d land %d n %d seed %d: rng streams diverged", dial, land, n, seed)
					}
					if o1 == specialHit {
						for _, row := range []string{"Agents", "Jets", "Food", "Regions", "People", "Troopers"} {
							if strings.Contains(r1.English(), row) {
								hits[row]++
							}
						}
					}
				}
			}
		}
	}
	for _, row := range []string{"Agents", "Jets", "Food", "Regions", "People", "Troopers"} {
		if hits[row] < 50 {
			t.Errorf("only %d landed hits named %s; the comparison missed that row", hits[row], row)
		}
	}
}

// seedWhere finds a seed whose first draws are all `want`, one per bound given,
// each drawn from its own fresh stream: a row's raw draw is then known.
func seedWhere(t *testing.T, want int, bounds ...int) int64 {
	t.Helper()
	for seed := int64(1); seed < 5000; seed++ {
		ok := true
		for _, n := range bounds {
			if NewWorldSeed(DefaultConfig(), seed).rng.Intn(n) != want {
				ok = false
			}
		}
		if ok {
			return seed
		}
	}
	t.Fatalf("no seed draws %d for each of %v", want, bounds)
	return 0
}

// The original's Real48 arithmetic holds while the effect has its base value and
// stops the moment it does not. On a draw of 3, 300 agents lose 82 under Real48
// (the original keeps 218) and 81 under integer math.
func TestSabreRealArithmeticIsGatedOnTheBaseValue(t *testing.T) {
	seed := seedWhere(t, 3, SabreIntelKeepSpread, SabreIntelKeepSpread-1)
	row := attackKindOf(EffectSabreIntelligence)
	base := baseRules.Attacks[EffectSabreIntelligence].Target.Kill[KillIndex(Agent)]
	if got := NewWorldSeed(DefaultConfig(), seed).share(row, "kill_agent", 300, base, base); got != 82 {
		t.Errorf("base value lost %d agents, want 82 (Real48)", got)
	}
	// The same percent with another Down is a modded value: integer math.
	other := Roll{base.Pct, base.Up, base.Down - 1}
	if got := NewWorldSeed(DefaultConfig(), seed).share(row, "kill_agent", 300, other, base); got != 81 {
		t.Errorf("modded value lost %d agents, want 81 (integer)", got)
	}
	// Another key changed in the same set leaves this one on its base value.
	w := NewWorldSeed(DefaultConfig(), seed)
	d := w.AddHuman("d", "Defendia")
	d.Agents, d.Jets = 300, 200
	fx := baseRules.Attacks[EffectSabreIntelligence].Target
	fx.Kill[KillIndex(Jet)] = Roll{Pct: 10}
	l := w.applySet(fx, row, d, nil, false, nil)
	if l.Units[KillIndex(Agent)] != 82 || d.Agents != 218 || d.Jets != 180 {
		t.Errorf("agents lost %d (left %d), jets left %d; want 82, 218 and 180", l.Units[KillIndex(Agent)], d.Agents, d.Jets)
	}
}

func TestModdedSabreRowUsesGenericArithmetic(t *testing.T) {
	w, d := remoteTarget(1, 300, 100)
	rules := BaseRules()
	fx := rules.Attacks[EffectSabreIntelligence]
	fx.Target.Kill[KillIndex(Agent)] = Roll{Pct: 10}
	rules.Attacks[EffectSabreIntelligence] = fx
	w.Rules = rules
	d.Agents = 300
	got := list(w.sabreDamage(d, SabreHitIntelligence))
	if d.Agents != 270 || got != "30 Agents" {
		t.Errorf("agents %d report %q, want 270 and 30 Agents", d.Agents, got)
	}
}

func TestModdedArrivingMissileKeepsItsCapAndWordsExtras(t *testing.T) {
	w, d := remoteTarget(2, 100, 400_000)
	rules := BaseRules()
	fx := rules.Attacks[EffectIPNuclear]
	fx.Target.Waste = Roll{Pct: 50}
	fx.Target.Kill[KillIndex(Turret)] = Roll{Pct: 30}
	rules.Attacks[EffectIPNuclear] = fx
	w.Rules = rules
	d.SDI = 0
	var got Msg
	for seed := int64(1); ; seed++ {
		w.rng = NewWorldSeed(DefaultConfig(), seed).rng
		var why specialOutcome
		d.Regions, d.Turrets = RegionMix{Coastal: 400_000}, 100
		d.syncLand()
		got, _, why, _ = w.applySpecialOp(OpNuclear, d, "Selby of Home", 0)
		if why == specialHit {
			break
		}
	}
	want := "Your nuclear strike turned 32000 of Defendia's regions on " + w.Config.BoardID + " into waste. It also took 30 Turrets."
	if got.English() != want {
		t.Errorf("report %q, want %q", got.English(), want)
	}
}

func TestRemoteKindsRefuseAnAttackerSetAndCapture(t *testing.T) {
	r := BaseRules()
	fx := r.Attacks[EffectSabreFood]
	fx.Attacker.People = Roll{Pct: 1}
	fx.Target.Capture = Roll{Pct: 1}
	r.Attacks[EffectSabreFood] = fx
	err := r.Validate()
	if err == nil || !strings.Contains(err.Error(), "attacks.sabre.food.attacker: this missile comes from another board") ||
		!strings.Contains(err.Error(), "attacks.sabre.food.target.capture_regions: the attacker is on another board") {
		t.Errorf("Validate: %v", err)
	}
}
