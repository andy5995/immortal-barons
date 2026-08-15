package game

import (
	"strings"
	"testing"
)

func TestBombingRunDestroysGroundedJets(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := &Empire{Bombers: 10}
	d := &Empire{Jets: 100} // no turrets, no SDI
	kills, lost := w.bombingRun(a, d, a.Bombers)
	if lost != 0 {
		t.Errorf("no turrets means no bombers lost, got %d", lost)
	}
	if kills != 10*BomberJetKills {
		t.Errorf("10 bombers should down %d jets, got %d", 10*BomberJetKills, kills)
	}
	if d.Jets != 100-kills {
		t.Errorf("defender jets not reduced: %d", d.Jets)
	}
}

// Turrets down bombers; the defender's SDI does not touch a raid from a
// neighbour, however well funded it is.
func TestBombingRunTurretsDownBombersAndSDIIsIrrelevant(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := &Empire{Bombers: 10}
	d := &Empire{Jets: 1000, Turrets: 50, SDI: SDIMax} // 50/25 = 2 bombers lost
	kills, lost := w.bombingRun(a, d, a.Bombers)
	if lost != 2 {
		t.Errorf("50 turrets should down 2 bombers, got %d", lost)
	}
	// 8 survivors * 3 kills = 24, and the shield takes none of them.
	if kills != 24 {
		t.Errorf("expected 24 kills, got %d", kills)
	}
	if a.Bombers != 8 {
		t.Errorf("attacker should have 8 bombers left, got %d", a.Bombers)
	}
}

func TestBombingRunCapsAtDefenderJets(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := &Empire{Bombers: 100}
	d := &Empire{Jets: 5}
	kills, _ := w.bombingRun(a, d, a.Bombers)
	if kills != 5 || d.Jets != 0 {
		t.Errorf("kills should cap at 5 and zero out jets, got kills=%d jets=%d", kills, d.Jets)
	}
}

func TestAttackRecordsVictimEvent(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("att", "Attacker")
	a.Troopers = 100000 // overwhelming, deterministic win
	d := w.Empires[0]
	d.Protection = 0

	before := len(d.Events)
	w.Attack(a, d, FullForce(a), true)
	if len(d.Events) != before+1 {
		t.Fatalf("victim should get one event, got %d new", len(d.Events)-before)
	}
	if !strings.Contains(d.Events[len(d.Events)-1].Text, a.Name) {
		t.Errorf("event should name the attacker: %q", d.Events[len(d.Events)-1].Text)
	}
	if !strings.Contains(d.Events[len(d.Events)-1].Text, "attacked you") {
		t.Errorf("event should be victim-perspective: %q", d.Events[len(d.Events)-1].Text)
	}
}

// A battle ends the moment one side has lost the share it will accept, so the
// side that breaks off pays that share — the 20% Medium Attack Damage figure
// read out of the binary — and only overshoots it by the single blow that
// carried it past. Asserted against the golden 20 rather than the constant, so
// a retune has to bring new evidence.
func TestTheLoserPaysTheRetreatShare(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	for _, ratio := range []struct {
		name   string
		ap, dp int
	}{
		{"even", 10000, 10000},
		{"five to one", 50000, 10000},
		{"overwhelming", 5_000_000, 10000},
	} {
		won, aLost, dLost := w.battleAttrition(ratio.ap, ratio.dp, 20)
		if !won {
			t.Errorf("%s: the stronger side should win", ratio.name)
		}
		if dLost < 0.20 || dLost > 0.22 {
			t.Errorf("%s: loser lost %.3f, want the 0.20 retreat share plus at most one blow", ratio.name, dLost)
		}
		if aLost >= dLost {
			t.Errorf("%s: winner lost %.3f, loser %.3f — the winner must pay less", ratio.name, aLost, dLost)
		}
	}
}

// The winner's bill is set by the strength ratio, which is the whole reason the
// fight is simulated instead of rated: an even match costs the winner nearly as
// much as the loser, an overwhelming one costs it almost nothing. A fixed
// winner rate — what IB had — cannot express either end. Averaged over seeds
// because one battle is one trajectory.
func TestALopsidedWinIsCheaperThanAnEvenOne(t *testing.T) {
	mean := func(ap, dp int) float64 {
		total := 0.0
		for seed := int64(1); seed <= 20; seed++ {
			w := NewWorldSeed(DefaultConfig(), seed)
			_, aLost, _ := w.battleAttrition(ap, dp, 20)
			total += aLost
		}
		return total / 20
	}
	even, lopsided := mean(10000, 10000), mean(5_000_000, 10000)
	if even <= lopsided {
		t.Errorf("an even fight cost the winner %.4f and a lopsided one %.4f; the ratio must drive it", even, lopsided)
	}
	if lopsided > 0.05 {
		t.Errorf("a 500:1 attacker lost %.4f of its force; a rout should be close to free", lopsided)
	}
}

// A side driven to nothing is a total loss, not a fractional one.
func TestAnAnnihilatedSideLosesEverything(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	if _, _, dLost := w.battleAttrition(1_000_000, 0, 20); dLost != 1 {
		t.Errorf("a defender with no forces lost %.3f, want 1", dLost)
	}
}

// A won Normal attack captures max(floor, ~10% of the loser's regions) — a
// bigger share of a small realm, ~10% of a large one (BRE live).
func TestRegularAttackCaptureFloorAndPercent(t *testing.T) {
	newFight := func(defenderLand int) (*World, *Empire, *Empire) {
		w := NewWorldSeed(DefaultConfig(), 1)
		a := &Empire{Name: "A", Troopers: 1_000_000, Morale: 100, Alive: true}
		d := &Empire{Name: "D", Troopers: 1, Morale: 100, Land: defenderLand,
			Regions: RegionMix{Mountain: defenderLand}, People: 1_000_000, Alive: true}
		return w, a, d
	}

	// Golden literals, not the balance constants: the 10% and the 15-region
	// floor are live-verified against BRE (see balance.go), so retuning either
	// constant must FAIL here and force new evidence — a want computed from the
	// constant would follow the retune silently.

	// Large defender: ~10% (500 → capture 50).
	w, a, d := newFight(500)
	w.Attack(a, d, FullForce(a), true)
	if got := 500 - d.Land; got != 50 {
		t.Errorf("large defender: captured %d, want 50 (10%%, BRE-verified)", got)
	}

	// Small defender: the floor dominates (100 regions → 10% = 10 < floor 15).
	w, a, d = newFight(100)
	w.Attack(a, d, FullForce(a), true)
	if got := 100 - d.Land; got != 15 {
		t.Errorf("small defender: captured %d, want the floor 15 (BRE-verified)", got)
	}
}

// autoCapture=false defers the winner's land gain (#58): the defender loses its
// regions and Attack returns the captured count, but the attacker's own regions
// are untouched — the menu picker allocates the count freely. autoCapture=true
// keeps the old behavior (the winner gains the loser's proportional mix).
func TestAttackDeferredCapture(t *testing.T) {
	newFight := func() (*World, *Empire, *Empire) {
		w := NewWorldSeed(DefaultConfig(), 1)
		a := &Empire{Name: "A", Troopers: 1_000_000, Morale: 100, Alive: true}
		d := &Empire{Name: "D", Turrets: 1, Morale: 100, Land: 500,
			Regions: RegionMix{Mountain: 500}, People: 1_000_000, Alive: true}
		return w, a, d
	}

	w, a, d := newFight()
	_, captured := w.Attack(a, d, FullForce(a), false)
	if captured <= 0 {
		t.Fatalf("won attack should return a positive captured count, got %d", captured)
	}
	if d.Land != 500-captured {
		t.Errorf("defender should lose %d regions: 500 -> %d", captured, d.Land)
	}
	if a.Land != 0 || a.Regions.Total() != 0 {
		t.Errorf("deferred path must not add land to the attacker yet, got Land=%d mix=%d", a.Land, a.Regions.Total())
	}

	w, a, d = newFight()
	_, cap2 := w.Attack(a, d, FullForce(a), true)
	if a.Regions.Total() != cap2 || a.Land != cap2 {
		t.Errorf("autoCapture should add the %d captured regions to the attacker, got mix=%d Land=%d", cap2, a.Regions.Total(), a.Land)
	}
}

func TestAttackScoreAttackerWins(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := &Empire{Name: "A", Troopers: 100000, Morale: 100, Alive: true}
	d := &Empire{Name: "D", Turrets: 1000, Morale: 100, Land: 100,
		Regions: RegionMix{Mountain: 100}, Gold: 1000, People: 1000, Alive: true, Score: 1_000_000}

	w.Attack(a, d, FullForce(a), true)

	// The award is the combined casualties over CombatScoreDivisor. Casualties
	// come out of the battle rather than a fixed rate, so read what the fight
	// actually cost instead of predicting it.
	battle := (100000 - a.Troopers) + (1000 - d.Turrets)
	gain := battle / CombatScoreDivisor
	if a.Score != gain {
		t.Errorf("attacker Score = %d, want %d", a.Score, gain)
	}
	wantD := 1_000_000 - gain*CombatLoserPenaltyPct/100
	if d.Score != wantD {
		t.Errorf("defender Score = %d, want %d (loses less than the winner gains)", d.Score, wantD)
	}
	if gain-(1_000_000-d.Score) <= 0 {
		t.Errorf("loser penalty should be smaller than the winner's gain")
	}
}

func TestAttackScoreDefenderWinsWorthMore(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	// Big enough forces that the score arithmetic does not vanish into integer
	// truncation, but still a defence the attacker cannot break.
	a := &Empire{Name: "A", Troopers: 200_000, Morale: 100, Alive: true, Score: 1_000_000}
	d := &Empire{Name: "D", Turrets: 800_000, Morale: 100, Land: 100, People: 1000, Alive: true}

	w.Attack(a, d, FullForce(a), true)

	battle := (200_000 - a.Troopers) + (800_000 - d.Turrets)
	gain := battle / CombatScoreDivisor * DefenseWinBonusPct / 100
	if d.Score != gain {
		t.Errorf("defending winner Score = %d, want %d", d.Score, gain)
	}
	wantA := 1_000_000 - gain*CombatLoserPenaltyPct/100
	if a.Score != wantA {
		t.Errorf("repelled attacker Score = %d, want %d", a.Score, wantA)
	}
	// A defensive win awards more than the same-size attack would (150% vs 100%).
	if gain <= battle/CombatScoreDivisor {
		t.Errorf("defensive win (%d) should out-award an equivalent attack win (%d)", gain, battle/CombatScoreDivisor)
	}
}

func TestAttackPostsPlanetaryNews(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("att", "Attacker")
	a.Troopers = 100000 // overwhelming, deterministic win
	d := w.Empires[0]
	d.Protection = 0

	before := len(w.NewsToday)
	w.Attack(a, d, FullForce(a), true)
	if len(w.NewsToday) != before+1 {
		t.Fatalf("attack should post one planetary news line, got %d new", len(w.NewsToday)-before)
	}
	if last := w.NewsToday[len(w.NewsToday)-1]; !strings.Contains(last, "Attacker") {
		t.Errorf("news should name the attacker: %q", last)
	}
}

// A regular attack only commits (and only risks) the chosen force; held-back
// units stay home, and the offense scales with what's sent (#: force selection).
func TestAttackUsesOnlyCommittedForce(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	d := w.AddHuman("d", "Delta")
	a.Troopers, a.Morale = 200, 100
	d.Morale = 100

	// Offense scales with the committed force (no HQ/Tech → 1 per trooper).
	if got := (AttackForce{Troopers: 50}).groundOffense(a); got != 50 {
		t.Errorf("committed offense = %d, want 50", got)
	}
	if got := FullForce(a).groundOffense(a); got != 200 {
		t.Errorf("full offense = %d, want 200", got)
	}

	// Commit only 50. No side loses more than the retreat share of what it
	// committed — 20% at the default Attack Damage — so at most 10 of the 50 go,
	// and the 150 held back are safe.
	w.Attack(a, d, AttackForce{Troopers: 50}, true)
	if a.Troopers < 200-50*20/100 {
		t.Errorf("held-back troopers were hit: 200 -> %d", a.Troopers)
	}
	if a.Troopers < 150 {
		t.Errorf("the 150 held-back troopers should be safe, got %d", a.Troopers)
	}
}

// A regular attack takes LAND, not money: BRE plunders no gold on a Regular
// Attack (breins.txt/attack.hlp/overlay all say regions only). Gold is untouched.
func TestRegularAttackTakesNoGold(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := &Empire{Name: "A", Troopers: 100000, Morale: 100, Alive: true, Gold: 500}
	d := &Empire{Name: "D", Turrets: 10, Morale: 100, Land: 100,
		Regions: RegionMix{Mountain: 100}, Gold: 1000, People: 100000, Alive: true}
	w.Attack(a, d, FullForce(a), true)
	if a.Gold != 500 {
		t.Errorf("attacker gold changed: got %d, want 500 (a win takes land, not gold)", a.Gold)
	}
	if d.Gold != 1000 {
		t.Errorf("defender gold changed: got %d, want 1000 (no plunder on a regular attack)", d.Gold)
	}
}

// A total conquest — the capture that reduces the defender to its last region —
// absorbs the loser's surviving military into the conqueror and leaves the loser
// with none. Strength alone doesn't wipe an empire out; being ground down to one
// region does. BRE's BRCRUSH reward.
func TestTotalConquestAbsorbsMilitary(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := &Empire{Name: "A", Troopers: 1_000_000, Morale: 100, Alive: true}
	d := &Empire{Name: "D", Troopers: 500, Jets: 40, Turrets: 30, Tanks: 20, Carriers: 5, Bombers: 10,
		Morale: 100, Land: 1, Regions: RegionMix{Mountain: 1}, People: 100, Alive: true}

	w.Attack(a, d, FullForce(a), true)

	if d.Alive {
		t.Fatalf("an overwhelming attack should conquer the defender")
	}
	if n := d.Troopers + d.Jets + d.Turrets + d.Tanks + d.Carriers + d.Bombers; n != 0 {
		t.Errorf("conquered empire should keep no military, got %d units total", n)
	}
	// Defender's post-battle survivors (after its loser-rate loss) transfer to the
	// attacker: carriers and bombers aren't hit by the battle loss, so all 5/10 carry over.
	if a.Carriers != 5 || a.Bombers != 10 {
		t.Errorf("attacker should absorb the defender's 5 carriers and 10 bombers, got C%d B%d", a.Carriers, a.Bombers)
	}
	// The winner keeps its own survivors and absorbs the loser's. An attacker
	// this far ahead is hardly ever the side taking the hit, so it should come
	// home with far more than the 800,000 a 20% retreat share would leave, and it
	// cannot end up with more than everyone started with.
	if a.Troopers <= 950_000 {
		t.Errorf("a 2000:1 attacker lost %d of 1,000,000 troopers; a lopsided win should be nearly free",
			1_000_000-a.Troopers)
	}
	if a.Troopers > 1_000_500 {
		t.Errorf("attacker troopers = %d, more than both sides brought", a.Troopers)
	}
}

// A beaten defender's HeadQuarters is knocked back 5-7 points and can be
// flattened by repeated defeats (BRE.OVR 0xFFA2). Golden band, not a mirror of
// the constants: the point is that a HeadQuarters is not permanent.
func TestLostDefenceDamagesHQ(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 3)
	a := w.AddHuman("me", "Mine")
	d := w.AddHuman("you", "Yours")
	for _, e := range []*Empire{a, d} {
		e.Protection, e.Morale = 0, 100
	}
	a.Troopers, a.Tanks = 5_000_000, 500_000 // overwhelming, so the attacker wins
	d.Troopers, d.Turrets, d.Tanks, d.Land = 10, 0, 0, 100
	d.HQ = 100

	before := d.HQ
	w.Attack(a, d, AttackForce{Troopers: 5_000_000, Tanks: 500_000}, true)
	lost := before - d.HQ
	if lost < 5 || lost > 7 {
		t.Fatalf("a lost defence cost the HeadQuarters %d points, want 5..7", lost)
	}

	// Repeated defeats flatten it, and it never goes negative.
	for i := 0; i < 40; i++ {
		d.Troopers = 10
		w.Attack(a, d, AttackForce{Troopers: 5_000_000, Tanks: 500_000}, true)
	}
	if d.HQ != 0 {
		t.Errorf("HeadQuarters = %d after 40 defeats, want 0", d.HQ)
	}
}

// The damage is gated on the attacker WINNING (BRE.OVR 0xFF82 tests the
// resolver's win flag before subtracting), so a repelled attack must leave the
// defender's HeadQuarters untouched. Several seeds, because the resolver can
// upset any single fight.
func TestRepelledAttackLeavesHQAlone(t *testing.T) {
	repelled := 0
	for seed := int64(1); seed <= 8; seed++ {
		w := NewWorldSeed(DefaultConfig(), seed)
		a := w.AddHuman("me", "Mine")
		d := w.AddHuman("you", "Yours")
		for _, e := range []*Empire{a, d} {
			e.Protection, e.Morale = 0, 100
		}
		a.Troopers = 10
		d.Troopers, d.Turrets, d.Land = 5_000_000, 500_000, 100
		d.HQ = 100

		report, _ := w.Attack(a, d, AttackForce{Troopers: 10}, true)
		if !strings.Contains(report, "Defeat!") {
			continue // an upset; the win path is the other test's business
		}
		repelled++
		if d.HQ != 100 {
			t.Errorf("seed %d: a repelled attack cost the HeadQuarters %d points, want 0", seed, 100-d.HQ)
		}
	}
	if repelled == 0 {
		t.Fatal("no attack was repelled; the test never reached the branch it covers")
	}
}
