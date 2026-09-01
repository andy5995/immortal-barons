package game

import (
	"strings"
	"testing"
	"time"
)

// twoBoards sets up an attacker on boardA and a fat, defenceless victim on
// boardB, which is the shape every round-trip test below needs.
func twoBoards(t *testing.T) (wA, wB *World, attacker, victim *Empire) {
	t.Helper()
	return twoBoardsSeed(t, 1)
}

// twoBoardsSeed is twoBoards on a chosen seed, for a test that has to hold on
// more than one trajectory.
func twoBoardsSeed(t *testing.T, seed int64) (wA, wB *World, attacker, victim *Empire) {
	t.Helper()
	cfgA := DefaultConfig()
	cfgA.BoardID = "boardA"
	wA = NewWorldSeed(cfgA, seed)
	attacker = wA.AddHuman("alice", "Alethia")
	attacker.Protection = 0
	attacker.Troopers, attacker.Tanks = 1_000_000, 10_000
	attacker.Gold = 10_000_000 // an individual strike is charged per unit sent

	cfgB := DefaultConfig()
	cfgB.BoardID = "boardB"
	wB = NewWorldSeed(cfgB, seed)
	victim = wB.AddHuman("victim", "Victim")
	victim.Protection = 0
	victim.Regions = RegionMix{Desert: 40_000}
	victim.syncLand()
	victim.Troopers, victim.Turrets, victim.Tanks, victim.Jets = 1000, 1000, 1000, 1000
	return wA, wB, attacker, victim
}

// The whole round trip for an individual strike: it leaves, the far board
// resolves it, and the answer comes home with the survivors, a private report
// for the baron who sent it, and land parked for the region picker (#107).
func TestIndividualStrikeRoundTrip(t *testing.T) {
	wA, wB, attacker, victim := twoBoards(t)
	force := AttackForce{Troopers: 500_000, Tanks: 5000}
	if _, err := wA.CreateIndividualAttack(attacker, "boardB", "Victim", NormalAttack, force); err != nil {
		t.Fatalf("CreateIndividualAttack: %v", err)
	}
	if attacker.Troopers != 500_000 || attacker.Tanks != 5000 {
		t.Fatalf("committed force not deducted: %d troopers, %d tanks", attacker.Troopers, attacker.Tanks)
	}
	if len(wA.InFlight) != 1 {
		t.Fatalf("in flight = %d, want 1", len(wA.InFlight))
	}

	defenderUnits := victim.Troopers + victim.Turrets + victim.Tanks + victim.Jets
	result := wB.ApplyPacket(wA.Outbox[0])
	if len(result.Results) != 1 {
		t.Fatalf("target board returned %d results, want 1", len(result.Results))
	}
	res := result.Results[0]
	if res.Outcome != OutcomeWon {
		t.Fatalf("outcome = %q, want %q", res.Outcome, OutcomeWon)
	}
	if res.Enemy.Total() == 0 {
		t.Error("the defender took no casualties; a strike that lands must cost the defender units")
	}
	if now := victim.Troopers + victim.Turrets + victim.Tanks + victim.Jets; now >= defenderUnits {
		t.Errorf("defender still holds %d units, was %d", now, defenderUnits)
	}

	wA.Outbox = nil
	wA.ApplyPacket(result)
	// The strike overwhelmed the defence, so it was barely touched and nearly the
	// whole detachment comes home. What it paid is the battle's outcome, not the
	// type's threshold (#199).
	if attacker.Troopers < 500_000+499_000 || attacker.Tanks < 5000+4990 {
		t.Errorf("survivors home: %d troopers, %d tanks; an overwhelming strike should return nearly all of 500000/5000",
			attacker.Troopers, attacker.Tanks)
	}
	if len(wA.InFlight) != 0 {
		t.Errorf("%d strikes still in flight after the answer came home", len(wA.InFlight))
	}
	if attacker.PendingRegions != res.LandTaken {
		t.Errorf("parked regions = %d, want the %d captured", attacker.PendingRegions, res.LandTaken)
	}
	// The baron gets a private report naming the attack type, the target, what it
	// cost, and what it destroyed — none of which was told to them before (#107).
	last := attacker.Events[len(attacker.Events)-1].Text
	// "You lost" is absent when the strike came home intact, which this one did —
	// so the assertion is on the lines the report always carries.
	for _, want := range []string{"Normal Attack", "Victim", "boardB", "returned.", "You destroyed"} {
		if !strings.Contains(last, want) {
			t.Errorf("report is missing %q:\n%s", want, last)
		}
	}
}

// A strike aimed at a realm that no longer exists on the far board is not the
// same as one that was beaten, and the baron is owed the difference. BRE prints
// four verdicts on its returning report; IB carries them in Outcome.
func TestReturningStrikeVerdicts(t *testing.T) {
	for _, c := range []struct {
		name    string
		prepare func(w *World, victim *Empire)
		target  string
		want    AttackOutcome
		says    string
	}{
		{"missing", func(*World, *Empire) {}, "Nobody", OutcomeNotFound, "no such realm"},
		{"protected", func(_ *World, v *Empire) { v.Protection = 5 }, "Victim", OutcomeProtected, "New Realm Protection"},
		{"repelled", func(_ *World, v *Empire) { v.Turrets = 100_000_000 }, "Victim", OutcomeRepelled, "beaten off"},
	} {
		t.Run(c.name, func(t *testing.T) {
			wA, wB, attacker, victim := twoBoards(t)
			c.prepare(wB, victim)
			if _, err := wA.CreateIndividualAttack(attacker, "boardB", c.target, NormalAttack,
				AttackForce{Troopers: 1000}); err != nil {
				t.Fatalf("CreateIndividualAttack: %v", err)
			}
			result := wB.ApplyPacket(wA.Outbox[0])
			if got := result.Results[0].Outcome; got != c.want {
				t.Fatalf("outcome = %q, want %q", got, c.want)
			}
			wA.Outbox = nil
			wA.ApplyPacket(result)
			if attacker.PendingRegions != 0 {
				t.Errorf("a strike that took nothing parked %d regions", attacker.PendingRegions)
			}
			last := attacker.Events[len(attacker.Events)-1].Text
			if !strings.Contains(last, c.says) {
				t.Errorf("report should say %q:\n%s", c.says, last)
			}
		})
	}
}

// A group attack's spoils are split between the barons who paid for them, in
// proportion to what each committed, and every region is handed to somebody.
func TestGroupAttackSpoilsSplitByContribution(t *testing.T) {
	cs := []Contribution{
		{Owner: "alice", AttackForce: AttackForce{Troopers: 3000}},
		{Owner: "bob", AttackForce: AttackForce{Troopers: 1000}},
	}
	shares := splitSpoils(cs, 100)
	got := map[string]int{}
	total := 0
	for _, s := range shares {
		got[s.Owner] = s.Land
		total += s.Land
	}
	if total != 100 {
		t.Errorf("shares total %d, want 100 — captured land was dropped", total)
	}
	if got["alice"] != 75 || got["bob"] != 25 {
		t.Errorf("split = %v, want alice 75 / bob 25 (3:1 by committed offense)", got)
	}
	// An indivisible remainder goes to the largest contributor rather than
	// nowhere: three regions between two barons is still three regions.
	shares = splitSpoils(cs, 3)
	total = 0
	for _, s := range shares {
		total += s.Land
	}
	if total != 3 {
		t.Errorf("shares total %d, want 3", total)
	}
}

// A result for a strike nobody is waiting on is discarded whole. BRE does the
// same and says so ("Duplicate or Late Attack Return Recieved - Packet
// Deleted"); without it the lost-forces timer and a slow packet between them
// hand the owner two armies for one.
func TestLateReturnIsDiscarded(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.LostForcesDays = 1
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")
	e.Protection = 0
	e.Troopers = 500

	if _, err := w.CreateIndividualAttack(e, "faraway", "Rome", NormalAttack, AttackForce{Troopers: 300}); err != nil {
		t.Fatalf("CreateIndividualAttack: %v", err)
	}
	id := w.InFlight[0].ID
	w.GameDay += 5
	if got := w.ReturnLostForces(); got != 1 {
		t.Fatalf("recovered %d strikes, want 1", got)
	}
	if e.Troopers != 500 {
		t.Fatalf("troopers after recovery = %d, want 500", e.Troopers)
	}

	// The result finally turns up, days after the forces were written off.
	w.ApplyPacket(Packet{FromBoard: "faraway", Results: []AttackResult{{
		ID: id, TargetBoard: "faraway", TargetEmpire: "Rome", Kind: "Normal Attack",
		Won: true, LandTaken: 40, Outcome: OutcomeWon,
		Survivors: []Contribution{{Owner: "alice", AttackForce: AttackForce{Troopers: 255}}},
	}}})
	if e.Troopers != 500 {
		t.Errorf("troopers = %d, want 500 — a late return raised a second army", e.Troopers)
	}
	if e.PendingRegions != 0 {
		t.Errorf("a late return paid out %d regions", e.PendingRegions)
	}
}

// The sysop's Attack Damage knob reaches interplanetary strikes: it rescales the
// attack type's own retreat share rather than being ignored. Golden literals off
// BRE's resolver (BRE.OVR 0x405a8) against a normal attack's 15%.
func TestAttackDamageRescalesInterplanetaryLosses(t *testing.T) {
	for _, c := range []struct {
		level Level
		want  int
	}{
		{Medium, 15}, // the type's own rate, untouched
		{None, 1},    // survival flattened to 0.99
		{Low, 7},     // half the loss
		{High, 30},   // double it
	} {
		if got := c.level.InterplanetaryLossPct(15); got != c.want {
			t.Errorf("%s: loss = %d%%, want %d%%", c.level, got, c.want)
		}
	}
	// And it reaches a real battle. The threshold is only visible on the side that
	// breaks off, so the strike below is built to lose: at High it grinds down to
	// 30% and stops, where Medium would have stopped at 15% (830..850 survivors).
	cfg := DefaultConfig()
	cfg.BoardID = "boardB"
	cfg.AttackDamage = High
	w := NewWorldSeed(cfg, 1)
	victim := w.AddHuman("victim", "Victim")
	victim.Protection = 0
	victim.Troopers, victim.Turrets, victim.Tanks = 500_000, 200_000, 50_000
	res := w.resolveRemoteAttack(RemoteAttack{
		ID: 1, FromBoard: "far", TargetEmpire: "Victim", Kind: NormalAttack, Offense: 1000,
		Contributors: []Contribution{{Owner: "alice", AttackForce: AttackForce{Troopers: 1000}}},
	})
	if res.Won {
		t.Fatal("the token force was meant to lose, or the threshold below proves nothing")
	}
	if s := res.Survivors[0].Troopers; s < 680 || s > 700 {
		t.Errorf("survivors at Attack Damage High = %d, want 680..700 (the 30%% threshold)", s)
	}
}

// The departure window is BRE's, in hours: a delay under twelve or over 120 is
// pulled back into the window rather than honoured. Golden literals.
func TestGroupAttackDepartureWindow(t *testing.T) {
	base := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		asked, honoured int
	}{{0, 12}, {12, 12}, {36, 36}, {120, 120}, {500, 120}} {
		got := DepartureAfter(base, c.asked)
		if want := base.Add(time.Duration(c.honoured) * time.Hour); !got.Equal(want) {
			t.Errorf("asking for %dh departed at %v, want %v", c.asked, got, want)
		}
	}
}

// A group attack sits until its hour, then leaves — which a day number could not
// express, and is the whole of #124.
func TestGroupAttackLeavesOnTheHour(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BoardID = "boardA"
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")
	e.Troopers = 10_000
	ga, err := w.CreateGroupAttack(e, "boardB", "Victim", GroupAttackHoursMin, AttackForce{Troopers: 1000})
	if err != nil {
		t.Fatalf("CreateGroupAttack: %v", err)
	}
	filed := ga.DepartAt.Add(-GroupAttackHoursMin * time.Hour)

	w.LaunchDueGroupAttacksAt(filed.Add(11 * time.Hour))
	if len(w.GroupAttacks) != 1 || len(w.Outbox) != 0 {
		t.Fatalf("the force left after 11 hours; the floor is %d", GroupAttackHoursMin)
	}
	w.LaunchDueGroupAttacksAt(filed.Add(13 * time.Hour))
	if len(w.GroupAttacks) != 0 {
		t.Fatalf("the force was still forming 13 hours after a 12-hour delay")
	}
	if len(w.Outbox) != 1 || len(w.Outbox[0].Attacks) != 1 {
		t.Fatalf("no attack went out: %+v", w.Outbox)
	}
	if !w.InFlight[0].Group {
		t.Error("a launched group attack should be marked as one, so its report reads as the planet's")
	}
}

// A world saved before the hours change still knows when its pending attacks
// leave: DepartAt is zero there, so the old day number decides.
func TestPreHoursGroupAttackStillDeparts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BoardID = "boardA"
	w := NewWorldSeed(cfg, 1)
	w.GameDay = 4
	w.GroupAttacks = []GroupAttack{{ID: 1, TargetBoard: "boardB", DepartDay: 5}}
	w.LaunchDueGroupAttacksAt(time.Now())
	if len(w.GroupAttacks) != 1 {
		t.Fatalf("a legacy attack left before its day")
	}
	w.GameDay = 5
	w.LaunchDueGroupAttacksAt(time.Now())
	if len(w.GroupAttacks) != 0 {
		t.Fatalf("a legacy attack never left on its day")
	}
}

// An interplanetary exchange is the whole planet's business, so BOTH planets
// hear about it: the defending board used to post nothing at all while its
// scores moved, and the attacker's own line named nobody (#108).
func TestBothPlanetsReportAnInterplanetaryStrike(t *testing.T) {
	def := NewWorldSeed(DefaultConfig(), 1)
	def.Config.BoardID = "Bravo"
	victim := def.AddHuman("vic", "Redlands")
	victim.Protection = 0
	victim.Troopers, victim.Turrets = 100, 100
	victim.Regions.Urban = 200
	victim.syncLand()

	// A force that plainly beats that defence, sent by a named realm.
	atk := RemoteAttack{
		ID: 7, FromBoard: "Alpha", FromEmpire: "Ironhold", TargetEmpire: "Redlands",
		Offense:      10_000_000,
		Contributors: []Contribution{{Owner: "iron", AttackForce: AttackForce{Troopers: 500_000}}},
		Kind:         NormalAttack,
	}
	res := def.resolveRemoteAttack(atk)
	if !res.Won {
		t.Fatalf("the strike should have won; outcome %q", res.Outcome)
	}
	news := strings.Join(def.NewsToday, "\n")
	if !strings.Contains(news, "Ironhold of Alpha") {
		t.Errorf("the defending planet's news should name the raider, got:\n%s", news)
	}
	if !strings.Contains(news, "Redlands") {
		t.Errorf("the defending planet's news should name who was hit, got:\n%s", news)
	}

	// ...and the attacker's own planet names the realm that went abroad.
	org := NewWorldSeed(DefaultConfig(), 1)
	org.Config.BoardID = "Alpha"
	sender := org.AddHuman("iron", "Ironhold")
	sender.Protection = 0
	org.InFlight = append(org.InFlight, InFlightStrike{
		ID: 7, Kind: "attack", TargetBoard: "Bravo", TargetEmpire: "Redlands",
		Contributors: []Contribution{{Owner: "iron"}},
	})
	org.applyAttackResult(res)
	if home := strings.Join(org.NewsToday, "\n"); !strings.Contains(home, "Ironhold") {
		t.Errorf("our own planet's news should name the realm that struck, got:\n%s", home)
	}
}

// A group attack stays anonymous on both sides: it is the planet's doing, and
// naming one of several contributors would be a lie.
func TestAGroupAttackNamesNoBaron(t *testing.T) {
	def := NewWorldSeed(DefaultConfig(), 1)
	def.Config.BoardID = "Bravo"
	victim := def.AddHuman("vic", "Redlands")
	victim.Protection = 0
	victim.Troopers = 100
	victim.Regions.Urban = 200
	victim.syncLand()

	def.resolveRemoteAttack(RemoteAttack{
		ID: 8, FromBoard: "Alpha", TargetEmpire: "Redlands", Group: true,
		Offense:      10_000_000,
		Contributors: []Contribution{{Owner: "a"}, {Owner: "b"}},
	})
	news := strings.Join(def.NewsToday, "\n")
	if !strings.Contains(news, "Alpha") {
		t.Errorf("the planet should still be named, got:\n%s", news)
	}
	if strings.Contains(news, " of Alpha") {
		t.Errorf("a group attack must not name a baron, got:\n%s", news)
	}
}

// One resolved outcome has to word all five places a strike is reported — the
// defender's own recap, the defending planet's news, the result that goes back,
// the attacker's report, and the attacking planet's news — and it has to word
// them so the reader can tell who won. A defender who beats a strike off still
// loses units, and IB used to tell him "You repelled ... You lost 424 of your
// forces", which reads as a contradiction and was filed as a bug (#201).
func TestArrivingStrikeIsWordedForTheSideThatWon(t *testing.T) {
	for _, c := range []struct {
		name    string
		prepare func(v *Empire)
		force   AttackForce
		want    AttackOutcome
		// What the defending board must say, and what it must not: each phrase
		// belongs to the outcome that did NOT happen.
		event, news, notNews string
	}{
		{
			name:  "attacker wins",
			force: AttackForce{Troopers: 500_000, Tanks: 5000},
			want:  OutcomeWon,
			event: "lost the field", news: "of its regions", notNews: "held the field",
		},
		{
			name:    "defender wins",
			prepare: func(v *Empire) { v.Troopers, v.Turrets, v.Tanks, v.Jets = 20_000, 20_000, 20_000, 20_000 },
			force:   AttackForce{Troopers: 40_000},
			want:    OutcomeRepelled,
			event:   "held the field", news: "of its own forces", notNews: "overran",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			// The outcome is a property of the whole battle, so it is asserted on
			// several trajectories rather than on the one seed that happened to
			// produce it.
			for seed := int64(1); seed <= 5; seed++ {
				wA, wB, attacker, victim := twoBoardsSeed(t, seed)
				if c.prepare != nil {
					c.prepare(victim)
				}
				if _, err := wA.CreateIndividualAttack(attacker, "boardB", "Victim", NormalAttack, c.force); err != nil {
					t.Fatalf("seed %d: CreateIndividualAttack: %v", seed, err)
				}
				result := wB.ApplyPacket(wA.Outbox[0])
				res := result.Results[0]
				if res.Outcome != c.want {
					t.Fatalf("seed %d: outcome = %q, want %q; the test never reached the branch it covers",
						seed, res.Outcome, c.want)
				}
				if res.Enemy.Total() == 0 {
					t.Fatalf("seed %d: the defender took no casualties, so nothing miswords them", seed)
				}
				recap := victim.Events[len(victim.Events)-1].Text
				if !strings.Contains(recap, c.event) {
					t.Errorf("seed %d: the defender's recap of a %q should say %q:\n%s", seed, c.want, c.event, recap)
				}
				line := wB.NewsToday[len(wB.NewsToday)-1]
				if !strings.Contains(line, c.news) {
					t.Errorf("seed %d: the defending planet's news of a %q should say %q:\n%s", seed, c.want, c.news, line)
				}
				if strings.Contains(line, c.notNews) {
					t.Errorf("seed %d: the defending planet's news of a %q claims the other side's outcome (%q):\n%s",
						seed, c.want, c.notNews, line)
				}
				// And the attacking planet is told the same thing about it.
				wA.Outbox, wA.NewsToday = nil, nil
				wA.ApplyPacket(result)
				home := wA.NewsToday[len(wA.NewsToday)-1]
				if (c.want == OutcomeWon) != strings.Contains(home, "triumph") {
					t.Errorf("seed %d: the attacking planet's news of a %q reads:\n%s", seed, c.want, home)
				}
			}
		})
	}
}

// A strike that found no such realm, or one turned away by New Realm Protection,
// was not beaten in battle — and the attacking planet's news said it was,
// because it read Won rather than the resolved outcome (#201).
func TestAStrikeThatFoughtNobodyIsNotAnnouncedAsADefeat(t *testing.T) {
	for _, c := range []struct {
		name    string
		prepare func(v *Empire)
		target  string
		want    AttackOutcome
	}{
		{"missing", func(*Empire) {}, "Nobody", OutcomeNotFound},
		{"protected", func(v *Empire) { v.Protection = 5 }, "Victim", OutcomeProtected},
	} {
		t.Run(c.name, func(t *testing.T) {
			wA, wB, attacker, victim := twoBoards(t)
			c.prepare(victim)
			if _, err := wA.CreateIndividualAttack(attacker, "boardB", c.target, NormalAttack,
				AttackForce{Troopers: 1000}); err != nil {
				t.Fatalf("CreateIndividualAttack: %v", err)
			}
			result := wB.ApplyPacket(wA.Outbox[0])
			if got := result.Results[0].Outcome; got != c.want {
				t.Fatalf("outcome = %q, want %q; the test never reached the branch it covers", got, c.want)
			}
			wA.Outbox, wA.NewsToday = nil, nil
			wA.ApplyPacket(result)
			if len(attacker.Events) == 0 {
				t.Fatal("the baron was told nothing about their own strike")
			}
			for _, line := range wA.NewsToday {
				if strings.Contains(line, "failure") {
					t.Errorf("a %q strike was announced to the planet as a defeat:\n%s", c.want, line)
				}
			}
		})
	}
}
