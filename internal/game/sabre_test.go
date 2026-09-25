package game

import (
	"fmt"
	"strings"
	"testing"
)

// sabre_test.go — the S3-Sabre's own tests, moved out of covert_test.go with the
// code they cover (2026-09-14). The missile is an interplanetary weapon, not a
// covert operation.

func TestSabreStrikeDamagesTarget(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	// sabreEffect is the target-side half and rolls no covert attempt, so
	// no agents are needed here. No Troopers on the target so it can never
	// backfire, and a stock of every strikeable resource. Only ~3 in 10 launches
	// land, so fire many.
	a.Agents, d.Agents, d.Troopers, d.SDI = 50, 0, 0, 0
	d.Jets, d.Turrets, d.Tanks, d.Food, d.People = 1000, 1000, 1000, 1000, 1_000_000
	before := d.Jets + d.Turrets + d.Tanks + d.Food + d.People
	for i := 0; i < 100; i++ {
		w.sabreEffect(d, a.Name, w.rng.Intn(SabreDialMax+1))
	}
	after := d.Jets + d.Turrets + d.Tanks + d.Food + d.People
	if after >= before {
		t.Errorf("expected S3-Sabre strikes to reduce the target's resources, before=%d after=%d", before, after)
	}
}

// The dial AIMS the missile — this is the whole of what was wrong before
// 2026-08-31, when IB treated it as a bluff that changed nothing. Dial 5 maps to
// airbases, which hit jets and nothing else, so a realm holding jets and tanks
// in equal number must lose jets and keep its tanks. Asserting the asymmetry
// rather than "something was destroyed" is what makes this fail if the mapping
// is ignored again.
func TestSabreDialAimsTheMissile(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents, d.Troopers, d.SDI = 50, 0, 0, 0
	d.Jets, d.Tanks, d.Food = 100_000, 100_000, 100_000
	// Few enough launches that neither field bottoms out: about nine in ten now
	// land (misfire + SDI, #255), where an invented 3-in-10 roll used to gate
	// them, and a saturated field cannot show the asymmetry this asserts.
	for i := 0; i < 25; i++ {
		w.sabreEffect(d, a.Name, 5)
	}
	if d.Jets >= 100_000 {
		t.Errorf("dial 5 is airbases and must cost the target jets; still %d", d.Jets)
	}
	// Tanks and food are reachable only through the ±1 jitter (dial 4, military
	// bases) and the 1-in-10 wild roll, so they may move — but far less than the
	// jets the dial actually aimed at.
	if 100_000-d.Tanks >= 100_000-d.Jets {
		t.Errorf("dial 5 cost more tanks (%d) than jets (%d); the mapping is not being read",
			100_000-d.Tanks, 100_000-d.Jets)
	}
}

// Every dial setting resolves to a defined effect. The mapper's table is the
// binary's, so an out-of-range dial must be clamped rather than indexing past it.
func TestSabreAimCoversEveryDial(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	seen := map[SabreEffect]bool{}
	for dial := SabreDialMin; dial <= SabreDialMax; dial++ {
		for i := 0; i < 200; i++ {
			seen[w.SabreAim(dial)] = true
		}
	}
	// Every row but the last, which no dial can reach (the input is taken mod 11).
	for eff := SabreHitIntelligence; eff < SabreDevelopRegions; eff++ {
		if !seen[eff] {
			t.Errorf("effect %d is unreachable from any dial", eff)
		}
	}
}

// Gold is NOT one of the S3-Sabre's targets, and neither are the HeadQuarters,
// bombers or carriers. IB used to roll a target across every good it held, which
// is what the dial mapping replaced: the original's effect switch writes back
// exactly the covert agents, population, food, jets, and the four military
// counts, and nothing else. The HeadQuarters (+0x26b) is named because IB hit it
// until 2026-09-25, reading the Intelligence Headquarters row's +0x26f as it.
func TestSabreLeavesGoldAlone(t *testing.T) {
	w, a, d := newAttackerAndTarget(t)
	a.Agents, d.Agents, d.Troopers, d.SDI = 50, 0, 0, 0
	d.Gold, d.Bombers, d.Carriers, d.HQ = 1_000_000, 1000, 1000, 100
	for i := 0; i < 300; i++ {
		w.sabreEffect(d, a.Name, w.rng.Intn(SabreDialMax+1))
	}
	if d.Gold != 1_000_000 || d.Bombers != 1000 || d.Carriers != 1000 || d.HQ != 100 {
		t.Errorf("the missile reached an asset no effect names: gold %d, bombers %d, carriers %d, HQ %d",
			d.Gold, d.Bombers, d.Carriers, d.HQ)
	}
}

// The Intelligence Headquarters row costs the target 1-30% of its covert agents:
// it keeps trunc(agents x (0.70 + Random(30)/100)). Golden band from 1000
// agents: 700..990 kept. Many rolls, so both ends of the band are seen.
func TestSabreIntelligenceHitKillsAgents(t *testing.T) {
	w := testWorld()
	d := w.AddHuman("victim", "Victim")
	lo, hi := 1000, 0
	for i := 0; i < 400; i++ {
		d.Agents, d.HQ = 1000, 100
		got := w.sabreDamage(d, SabreHitIntelligence)
		if d.Agents < 700 || d.Agents > 990 {
			t.Fatalf("1000 agents became %d, want 700..990", d.Agents)
		}
		if want := fmt.Sprintf("%d Agents", 1000-d.Agents); got != want {
			t.Fatalf("report %q, want %q", got, want)
		}
		if d.HQ != 100 {
			t.Fatalf("the Intelligence Headquarters hit the HeadQuarters: %d", d.HQ)
		}
		lo, hi = min(lo, d.Agents), max(hi, d.Agents)
	}
	if lo != 700 || hi != 990 {
		t.Errorf("kept range %d..%d over 400 rolls, want exactly 700..990", lo, hi)
	}
	d.Agents = 0
	if got := w.sabreDamage(d, SabreHitIntelligence); got != "" {
		t.Errorf("a realm with no agents reported %q", got)
	}
}

// A backfiring S3-Sabre develops land for the realm it was AIMED at, and the
// share is the original's: 10-19% of that realm's regions, handed over untyped
// (#266). Golden literals rather than the constants, because these two are
// binary-verified and a retune has to fail this test rather than follow it.
func TestSabreBackfireDevelopsTheTargetsLand(t *testing.T) {
	w := testWorld()
	d := w.AddHuman("victim", "Victim")
	d.Regions = RegionMix{Agricultural: 1000}
	d.syncLand()
	if d.Land != 1000 {
		t.Fatalf("fixture: Land = %d, want 1000", d.Land)
	}

	// Many rolls, because the share is random inside a fixed band.
	low, high := 0, 0
	for i := 0; i < 400; i++ {
		d.PendingRegions = 0
		got := w.sabreDevelop(d)
		if got < 100 || got > 190 {
			t.Fatalf("developed %d regions off 1000, want 100-190 (10-19%%)", got)
		}
		if d.PendingRegions != got {
			t.Fatalf("PendingRegions = %d, want %d", d.PendingRegions, got)
		}
		if got == 100 {
			low++
		}
		if got == 190 {
			high++
		}
	}
	// Both ends of the band have to be reachable, or the roll is not the
	// original's even when every sample sits inside the range.
	if low == 0 || high == 0 {
		t.Errorf("band ends unreached in 400 rolls: 10%% hit %d times, 19%% hit %d", low, high)
	}

	// The land arrives with no type: it is counted nowhere until the owner picks.
	if d.Land != 1000 || d.Regions.Total() != 1000 {
		t.Errorf("Land = %d and mix totals %d, want both still 1000 — developed land is untyped until allocated",
			d.Land, d.Regions.Total())
	}
}

// The firer loses nothing to a backfire. The original's return path writes no
// field of their record; IB damaged them until 2026-09-14, which was invented
// before that path was read (#266).
func TestSabreBackfireCostsTheFirerNothing(t *testing.T) {
	w := testWorld()
	e := w.AddHuman("firer", "Firer")
	e.Regions = RegionMix{Agricultural: 500}
	e.syncLand()
	e.Troopers, e.Jets, e.Tanks, e.Turrets, e.People, e.Food, e.HQ = 900, 800, 700, 600, 500_000, 400, 50
	before := *e

	sent := InFlightStrike{Kind: "special", Op: OpSabre, Owner: "firer",
		TargetBoard: "Far", TargetEmpire: "Victim"}
	// Report is what the TARGET's board composed and sent home, which is what the
	// firer is shown; a result without one is the fallback case, below.
	told := "Your S3-Sabre broke up over Victim and opened 3 Regions for them to settle."
	w.applySpecialOpResult(sent, AttackResult{TargetBoard: "Far", TargetEmpire: "Victim",
		Backfired: true, Report: told})

	if e.Troopers != before.Troopers || e.Jets != before.Jets || e.Tanks != before.Tanks ||
		e.Turrets != before.Turrets || e.People != before.People || e.Food != before.Food ||
		e.HQ != before.HQ || e.Land != before.Land {
		t.Errorf("the firer lost something to its own backfire:\n before %+v\n after  %+v", before, *e)
	}
	if len(e.Events) == 0 {
		t.Fatal("the firer was told nothing about the backfire")
	}
	last := e.Events[len(e.Events)-1].Text
	if !strings.Contains(last, told) {
		t.Errorf("backfire event = %q, want it to carry the target board's report %q", last, told)
	}

	// With no report to relay, the firer still learns what happened rather than
	// getting a bare heading.
	w.applySpecialOpResult(sent, AttackResult{TargetBoard: "Far", TargetEmpire: "Victim", Backfired: true})
	if last := e.Events[len(e.Events)-1].Text; !strings.Contains(last, "backfired") {
		t.Errorf("reportless backfire event = %q, want it to say the strike backfired", last)
	}
}
