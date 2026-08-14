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
	cfgA := DefaultConfig()
	cfgA.BoardID = "boardA"
	wA = NewWorldSeed(cfgA, 1)
	attacker = wA.AddHuman("alice", "Alethia")
	attacker.Protection = 0
	attacker.Troopers, attacker.Tanks = 1_000_000, 10_000
	attacker.Gold = 10_000_000 // an individual strike is charged per unit sent

	cfgB := DefaultConfig()
	cfgB.BoardID = "boardB"
	wB = NewWorldSeed(cfgB, 1)
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
	// A normal attack retreats at 15% losses, so 85% of each type comes home.
	if attacker.Troopers != 500_000+425_000 || attacker.Tanks != 5000+4250 {
		t.Errorf("survivors home: %d troopers, %d tanks; want 925000 and 9250",
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
	for _, want := range []string{"Normal Attack", "Victim", "boardB", "You lost", "came home", "You destroyed"} {
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
	// And it reaches the survivors that actually come home.
	cfg := DefaultConfig()
	cfg.BoardID = "boardB"
	cfg.AttackDamage = High
	w := NewWorldSeed(cfg, 1)
	victim := w.AddHuman("victim", "Victim")
	victim.Protection = 0
	res := w.resolveRemoteAttack(RemoteAttack{
		ID: 1, FromBoard: "far", TargetEmpire: "Victim", Kind: NormalAttack, Offense: 50_000_000,
		Contributors: []Contribution{{Owner: "alice", AttackForce: AttackForce{Troopers: 1000}}},
	})
	if res.Survivors[0].Troopers != 700 {
		t.Errorf("survivors at Attack Damage High = %d, want 700 (30%% losses)", res.Survivors[0].Troopers)
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
