package game

import (
	"reflect"
	"strings"
	"testing"
)

// ipBoards is the two boards of an interplanetary attack: boardA holds three
// barons who contribute, boardB three realms of its own, one of them shielded.
// shield and size scale the defense and the attacking forces.
func ipBoards(seed int64, defense, force int) (wA, wB *World, contributors []Contribution) {
	cfgA := DefaultConfig()
	cfgA.BoardID = "boardA"
	wA = NewWorldSeed(cfgA, seed)
	for i, h := range []string{"alice", "bob", "carol"} {
		e := wA.AddHuman(h, strings.ToUpper(h[:1])+h[1:])
		e.Founded = ""
		e.Protection = 0
		e.Regions = RegionMix{Agricultural: 50}
		e.syncLand()
		e.People, e.Troopers, e.Tanks, e.Jets, e.Turrets = 5000, 100, 100, 100, 100
		contributors = append(contributors, Contribution{
			Owner: e.Owner,
			AttackForce: AttackForce{Troopers: force * (i + 1), Tanks: force / 10 * (i + 1),
				Jets: force / 20, Bombers: force / 40},
			Tech: TechFactorUnit,
		})
	}
	cfgB := DefaultConfig()
	cfgB.BoardID = "boardB"
	wB = NewWorldSeed(cfgB, seed)
	for i, n := range []string{"Victim", "Second", "Shielded"} {
		e := wB.AddHuman(strings.ToLower(n), n)
		e.Founded = ""
		e.Protection = 0
		if n == "Shielded" {
			e.Protection = 5
		}
		e.Regions = RegionMix{Desert: 4000 / (i + 1), Coastal: 300}
		e.syncLand()
		e.People = 90_000 * (i + 1)
		e.Troopers, e.Turrets, e.Tanks, e.Jets = defense, defense/2, defense/3, defense/4
		e.HQ, e.Food = 80, 5000
	}
	return wA, wB, contributors
}

func newsTexts(w *World) []string {
	var out []string
	for _, l := range w.NewsToday {
		out = append(out, l.Text)
	}
	return out
}

func ipStrike(id int, target string, group bool, kind AttackKind, cs []Contribution) (RemoteAttack, InFlightStrike) {
	atk := RemoteAttack{ID: id, FromBoard: "boardA", TargetEmpire: target, Contributors: cs, Kind: kind, Group: group}
	sent := InFlightStrike{ID: id, Kind: "attack", TargetBoard: "boardB", TargetEmpire: target,
		Contributors: cs, Group: group, Whole: target == ""}
	if group {
		for _, c := range cs {
			atk.Offense += c.offense()
		}
	} else {
		atk.FromEmpire = "Alice"
		atk.Contributors, sent.Contributors = cs[:1], cs[:1]
		atk.Offense = pctOf(cs[0].offense(), kind.strengthPct())
	}
	return atk, sent
}

func TestBaseInterplanetaryAttackMatchesTheOriginalRoutines(t *testing.T) {
	outcomes := map[AttackOutcome]int{}
	kinds := []AttackKind{QuickStrike, NormalAttack, ExtendedBattle}
	for _, target := range []string{"Victim", "", "Nobody", "Shielded"} {
		for _, group := range []bool{false, true} {
			for _, kind := range kinds {
				if group && kind != NormalAttack {
					continue
				}
				for _, sz := range []struct{ defense, force int }{{50, 10}, {2_000, 1_500}, {40_000, 20_000}, {40_000, 4_000_000}} {
					for seed := int64(1); seed <= 12; seed++ {
						wA1, wB1, cs := ipBoards(seed, sz.defense, sz.force)
						wA2, wB2, _ := ipBoards(seed, sz.defense, sz.force)
						atk, sent := ipStrike(7, target, group, kind, cs)
						res1 := wB1.resolveRemoteAttack(atk)
						res2 := wB2.refResolveRemoteAttack(atk)
						outcomes[res1.outcome()]++
						if !reflect.DeepEqual(res1, res2) {
							t.Fatalf("%s group %v %v %v seed %d: result differs\n got  %+v\n want %+v", target, group, kind, sz, seed, res1, res2)
						}
						for _, w := range []*World{wB1, wB2} {
							for _, e := range w.Empires {
								clearStamps(e)
							}
						}
						if !reflect.DeepEqual(wB1.Empires, wB2.Empires) || !reflect.DeepEqual(wB1.Battles, wB2.Battles) ||
							!reflect.DeepEqual(newsTexts(wB1), newsTexts(wB2)) {
							t.Fatalf("%s group %v %v %v seed %d: the defending board differs", target, group, kind, sz, seed)
						}
						if wB1.rng.Intn(1<<30) != wB2.rng.Intn(1<<30) {
							t.Fatalf("%s group %v %v %v seed %d: rng streams diverged", target, group, kind, sz, seed)
						}
						// And the answer coming home.
						wA1.InFlight, wA2.InFlight = []InFlightStrike{sent}, []InFlightStrike{sent}
						wA1.applyAttackResult(res1, nil)
						wA2.refApplyAttackResult(res2, nil)
						for _, w := range []*World{wA1, wA2} {
							for _, e := range w.Empires {
								clearStamps(e)
							}
						}
						if !reflect.DeepEqual(wA1.Empires, wA2.Empires) || !reflect.DeepEqual(newsTexts(wA1), newsTexts(wA2)) {
							t.Fatalf("%s group %v %v %v seed %d: the home board differs", target, group, kind, sz, seed)
						}
						if wA1.rng.Intn(1<<30) != wA2.rng.Intn(1<<30) {
							t.Fatalf("%s group %v %v %v seed %d: home rng streams diverged", target, group, kind, sz, seed)
						}
					}
				}
			}
		}
	}
	for _, o := range []AttackOutcome{OutcomeWon, OutcomeRepelled, OutcomeNotFound, OutcomeProtected} {
		if outcomes[o] < 20 {
			t.Errorf("only %d resolutions ended %q; the comparison missed that outcome", outcomes[o], o)
		}
	}
}

func moddedIP(w *World, kind EffectKind, edit func(fx *AttackEffects)) {
	rules := BaseRules()
	fx := rules.Attacks[kind]
	edit(&fx)
	rules.Attacks[kind] = fx
	w.Rules = rules
}

// A won attack's target set takes tanks and regions from the defender, and the
// captured regions are counted with the battle's own so they travel home and
// are split among the contributors.
func TestModdedIPTargetSetCapturesAndKills(t *testing.T) {
	_, wBase, cs := ipBoards(3, 2_000, 4_000_000)
	wA, wB, _ := ipBoards(3, 2_000, 4_000_000)
	edit := func(fx *AttackEffects) {
		fx.Target.Kill[KillIndex(Tank)] = Roll{Pct: 50}
		fx.Target.Capture = Roll{Pct: 10}
	}
	moddedIP(wB, EffectIPAttackWon, edit)
	atk, sent := ipStrike(7, "Victim", true, NormalAttack, cs)
	base := wBase.resolveRemoteAttack(atk)
	res := wB.resolveRemoteAttack(atk)
	if !base.Won || !res.Won {
		t.Fatal("setup: the attack should win")
	}
	bv, v := wBase.FindByNameOrFormer("Victim"), wB.FindByNameOrFormer("Victim")
	if res.LandTaken != 817 || base.LandTaken != 430 {
		t.Errorf("land taken %d (base %d), want 817 (base 430)", res.LandTaken, base.LandTaken)
	}
	if v.Land != bv.Land-387 || v.Tanks != bv.Tanks-pctOf(bv.Tanks, 50) {
		t.Errorf("target land %d tanks %d, want %d and %d", v.Land, v.Tanks, bv.Land-387, bv.Tanks-pctOf(bv.Tanks, 50))
	}
	if res.Extra == nil || res.Extra.Units[KillIndex(Tank)] != 282 {
		t.Errorf("figures sent home %+v, want the 282 tanks the set took", res.Extra)
	}
	if got := v.Events[len(v.Events)-1].Msg.English(); !strings.HasSuffix(got, "It also took 282 Tanks.") {
		t.Errorf("defender's event %q", got)
	}
	wA.InFlight = []InFlightStrike{sent}
	wA.applyAttackResult(res, nil)
	total := 0
	for _, e := range wA.Empires {
		total += e.PendingRegions
	}
	if total != 817 {
		t.Errorf("land parked at home %d, want all 817", total)
	}
	// Each contributor is told its own share of the tanks, as with its kills.
	for _, c := range sent.Contributors {
		e := wA.FindByOwner(c.Owner)
		want := sayN("It also took {n} Tank.", "It also took {n} Tanks.", "n", "n",
			roundShare(282, killShare(sent.Contributors, c))).English()
		if got := e.Events[len(e.Events)-1].Msg.English(); !strings.HasSuffix(got, want) {
			t.Errorf("%s's report ends %q, want %q", c.Owner, got[max(0, len(got)-40):], want)
		}
	}
}

// The attacker set applies to EACH contributor of this board, in full, and a
// lost attack uses the lost kind.
func TestModdedIPAttackerSetHitsEveryContributor(t *testing.T) {
	for _, c := range []struct {
		name         string
		force        int
		kind         EffectKind
		people, left int
	}{
		{"won", 4_000_000, EffectIPAttackWon, 5000, 4900},
		{"lost", 10, EffectIPAttackLost, 5000, 4900},
	} {
		wA, wB, cs := ipBoards(3, 40_000, c.force)
		moddedIP(wA, c.kind, func(fx *AttackEffects) { fx.Attacker.People = Roll{Pct: 2} })
		atk, sent := ipStrike(7, "Victim", true, NormalAttack, cs)
		res := wB.resolveRemoteAttack(atk)
		if res.Won != (c.kind == EffectIPAttackWon) {
			t.Fatalf("%s: setup: the attack's outcome is %v", c.name, res.Won)
		}
		wA.InFlight = []InFlightStrike{sent}
		wA.applyAttackResult(res, nil)
		for _, h := range []string{"alice", "bob", "carol"} {
			e := wA.FindByOwner(OwnerKey(h))
			if e.People != c.left {
				t.Errorf("%s: %s has %d people, want %d", c.name, h, e.People, c.left)
			}
			if got := e.Events[len(e.Events)-1].Msg.English(); !strings.HasSuffix(got, "It also cost you 100 People.") {
				t.Errorf("%s: %s's report ends %q", c.name, h, got[max(0, len(got)-60):])
			}
		}
	}
}

func TestIPAttackerSetCannotCapture(t *testing.T) {
	r := BaseRules()
	fx := r.Attacks[EffectIPAttackLost]
	fx.Attacker.Capture = Roll{Pct: 1}
	fx.Target.Capture = Roll{Pct: 1} // allowed: the land is carried home
	r.Attacks[EffectIPAttackLost] = fx
	err := r.Validate()
	if err == nil || !strings.Contains(err.Error(), "attacks.ip.attack.lost.attacker.capture: the target is on another board") ||
		strings.Contains(err.Error(), "target.capture") {
		t.Errorf("Validate: %v", err)
	}
}

// A planet-wide strike fights every realm that stood: the target set hits each
// of them, the attacker is told the total, and the captured land comes home.
func TestModdedIPTargetSetOnAPlanetWideStrike(t *testing.T) {
	_, wBase, cs := ipBoards(3, 2_000, 4_000_000)
	wA, wB, _ := ipBoards(3, 2_000, 4_000_000)
	moddedIP(wB, EffectIPAttackWon, func(fx *AttackEffects) {
		fx.Target.Kill[KillIndex(Tank)] = Roll{Pct: 50}
		fx.Target.Capture = Roll{Pct: 10}
	})
	atk, sent := ipStrike(7, "", true, NormalAttack, cs)
	base := wBase.resolveRemoteAttack(atk)
	res := wB.resolveRemoteAttack(atk)
	if !base.Won || !res.Won {
		t.Fatal("setup: the strike should win")
	}
	killed, captured := 0, 0
	for _, n := range []string{"Victim", "Second"} {
		b, e := wBase.FindByNameOrFormer(n), wB.FindByNameOrFormer(n)
		k := b.Tanks - pctOf(b.Tanks, 50)
		if e.Tanks != k {
			t.Errorf("%s has %d tanks, want %d", n, e.Tanks, k)
		}
		killed += b.Tanks - k
		captured += b.Land - e.Land
	}
	if sh := wB.FindByNameOrFormer("Shielded"); sh.Tanks != wBase.FindByNameOrFormer("Shielded").Tanks {
		t.Error("a shielded realm was hit")
	}
	if res.LandTaken != base.LandTaken+captured || captured == 0 {
		t.Errorf("land taken %d, want the base %d and the %d captured", res.LandTaken, base.LandTaken, captured)
	}
	if res.Extra == nil || res.Extra.Units[KillIndex(Tank)] != killed {
		t.Errorf("figures sent home %+v, want %d tanks", res.Extra, killed)
	}
	wA.InFlight = []InFlightStrike{sent}
	wA.applyAttackResult(res, nil)
	total := 0
	for _, e := range wA.Empires {
		total += e.PendingRegions
	}
	if total != res.LandTaken {
		t.Errorf("land parked at home %d, want %d", total, res.LandTaken)
	}
}

// A repelled battle's target set can still capture: the attack lost, but the
// land it took is counted, carried home and told to both sides.
func TestModdedIPLostSetCapturesOnARepelledBattle(t *testing.T) {
	wA, wB, cs := ipBoards(3, 40_000, 10)
	moddedIP(wB, EffectIPAttackLost, func(fx *AttackEffects) {
		fx.Target.Capture = Roll{Pct: 1}
	})
	atk, sent := ipStrike(7, "Victim", true, NormalAttack, cs)
	res := wB.resolveRemoteAttack(atk)
	v := wB.FindByNameOrFormer("Victim")
	if res.Won || res.Outcome != OutcomeRepelled || res.LandTaken != 43 || v.Land != 4257 {
		t.Fatalf("won %v outcome %q land taken %d target land %d, want a repelled battle that took 43 of 4300",
			res.Won, res.Outcome, res.LandTaken, v.Land)
	}
	if got := v.Events[len(v.Events)-1].Msg.English(); !strings.HasSuffix(got, "You also lost 43 regions!") {
		t.Errorf("defender's event %q", got)
	}
	wA.InFlight = []InFlightStrike{sent}
	wA.applyAttackResult(res, nil)
	total := 0
	for _, e := range wA.Empires {
		total += e.PendingRegions
	}
	if total != 43 {
		t.Errorf("land parked at home %d, want 43", total)
	}
	// The battle was lost, so this line is the only place a contributor hears
	// of the regions it was given.
	for _, c := range sent.Contributors {
		e := wA.FindByOwner(c.Owner)
		if e.PendingRegions == 0 {
			continue
		}
		want := sayN("It also took {n} Region seized.", "It also took {n} Regions seized.", "n", "n", e.PendingRegions).English()
		if got := e.Events[len(e.Events)-1].Msg.English(); !strings.HasSuffix(got, want) {
			t.Errorf("%s's report ends %q, want %q", c.Owner, got[max(0, len(got)-40):], want)
		}
	}
}
