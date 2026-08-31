package game

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// The per-contributor factor #200 recorded as unread is the contributor's
// Technology military factor, written into their own force slot by the
// original's configure_attack_forces (technology_factor(1.4, slot 5), stored at
// the slot's +0x1c). Golden literals: 1.4 is the verified ceiling, and the
// offence table (trooper 1, jet 2, tank 4) is verified too.
func TestContributorTechnologyScalesTheOffence(t *testing.T) {
	for _, c := range []struct {
		name string
		c    Contribution
		sdi  int
		want int
	}{
		{"no technology", Contribution{AttackForce: AttackForce{Troopers: 1000}}, 0, 1000},
		{"a packet carrying no factor reads as x1", Contribution{AttackForce: AttackForce{Troopers: 1000}, Tech: 0}, 0, 1000},
		{"the military ceiling", Contribution{AttackForce: AttackForce{Troopers: 1000}, Tech: 14000}, 0, 1400},
		{"tanks weigh four", Contribution{AttackForce: AttackForce{Tanks: 100}, Tech: 14000}, 0, 560},
		// The shield blunts the jets first (x0.7 at SDI 100), THEN the factor
		// applies to what is left: 1000 x 2 x 0.7 = 1400, x1.4 = 1960.
		{"technology applies after the shield", Contribution{AttackForce: AttackForce{Jets: 1000}, Tech: 14000}, 100, 1960},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := c.c.offense()
			if c.sdi > 0 {
				got = c.c.offenseAgainstSDI(c.sdi)
			}
			if got != c.want {
				t.Errorf("offence = %d, want %d", got, c.want)
			}
		})
	}
}

// The factor is fixed on the SENDING board when the detachment is committed and
// rides the packet; a researched realm's strike leaves stronger than an
// identical force from one that never bought Technology.
func TestAStrikeCarriesEachContributorsTechnology(t *testing.T) {
	force := AttackForce{Troopers: 100_000, Tanks: 1000}
	launch := func(t *testing.T, research int) (int, Contribution) {
		t.Helper()
		wA, _, attacker, _ := twoBoardsSeed(t, 1)
		attacker.TechSlots[TechSlotMilitary] = research
		if _, err := wA.CreateIndividualAttack(attacker, "boardB", "Victim", NormalAttack, force); err != nil {
			t.Fatalf("CreateIndividualAttack: %v", err)
		}
		atk := wA.Outbox[0].Attacks[0]
		return atk.Offense, atk.Contributors[0]
	}
	plainOff, plain := launch(t, 0)
	techOff, teched := launch(t, 1_000_000) // far past any region count: the factor sits at its ceiling

	if plain.Tech != TechFactorUnit {
		t.Errorf("an unresearched realm's slot carries %d, want x1 (%d)", plain.Tech, TechFactorUnit)
	}
	if teched.Tech != 14000 {
		t.Errorf("a fully researched realm's slot carries %d, want the 1.4 ceiling (14000)", teched.Tech)
	}
	if techOff <= plainOff {
		t.Fatalf("the researched strike left with offence %d, the plain one %d; technology must make it stronger", techOff, plainOff)
	}
	// Exactly the ceiling's worth: the same force at x1.4.
	if want := plainOff * 14000 / TechFactorUnit; techOff != want {
		t.Errorf("researched offence = %d, want %d (the plain %d x 1.4)", techOff, want, plainOff)
	}
}

// Spoils are split by what each baron's forces are worth, technology included:
// the original's returning routine runs the same offence builder that reads the
// slot's factor, so a researched realm's thousand troopers earn more land than
// an unresearched realm's thousand.
func TestGroupSpoilsWeighByTechnology(t *testing.T) {
	cs := []Contribution{
		{Owner: "plain", AttackForce: AttackForce{Troopers: 1000}},
		{Owner: "teched", AttackForce: AttackForce{Troopers: 1000}, Tech: 14000},
	}
	shares := splitSpoils(cs, 24)
	got := map[string]int{}
	for _, s := range shares {
		got[s.Owner] = s.Land
	}
	// 1000 against 1400 of 2400: 10 and 14.
	if got["plain"] != 10 || got["teched"] != 14 {
		t.Errorf("split = %v, want plain 10 / teched 14", got)
	}
}

// The report a defender reads names what it lost by unit type, on a line each,
// zeros included — the original's shape — rather than one total.
func TestInvasionReportNamesEveryUnitLost(t *testing.T) {
	wA, wB, attacker, victim := twoBoardsSeed(t, 1)
	attacker.Jets, attacker.Carriers, attacker.Gold = 1000, 100, 1_000_000_000 // a strike is charged per unit, jets dearest
	victim.Troopers, victim.Turrets, victim.Tanks, victim.Jets = 20_000, 20_000, 20_000, 20_000
	if _, err := wA.CreateIndividualAttack(attacker, "boardB", "Victim", NormalAttack, AttackForce{Troopers: 40_000, Jets: 300}); err != nil {
		t.Fatal(err)
	}
	result := wB.ApplyPacket(wA.Outbox[0])
	res := result.Results[0]
	if res.Outcome != OutcomeRepelled {
		t.Fatalf("outcome %q; the test never reached the branch it covers", res.Outcome)
	}
	ev := victim.Events[len(victim.Events)-1].Text
	for _, want := range []string{
		"held the field",
		// Counts wear BRE's shortening on an interplanetary report, so 40,000
		// troopers read "40k" (numfmt.Short). The three-digit and zero rows
		// prove the rest of the line is left alone.
		"40k troopers attacked.", "300 jets attacked.", "0 tanks attacked.", "0 bombers attacked.",
		"You lost " + numfmt.Short(res.Enemy.Troopers) + " troopers.",
		"You lost " + numfmt.Short(res.Enemy.Jets) + " jets.",
		"You lost " + numfmt.Short(res.Enemy.Tanks) + " tanks.",
		"You lost " + numfmt.Short(res.Enemy.Turrets) + " turrets.",
	} {
		if !strings.Contains(ev, want) {
			t.Errorf("the defender's report lacks %q:\n%s", want, ev)
		}
	}
	if strings.Contains(ev, "units") {
		t.Errorf("the report still totals the losses as units:\n%s", ev)
	}
}

// On a planet-wide strike every realm that stood is told ITS OWN losses, and
// they add up to what the packet reports — each used to be handed the planet's
// total as if it were its own.
func TestEachDefenderOnAPlanetWideStrikeReadsItsOwnLosses(t *testing.T) {
	wA, wB, attacker, victim := twoBoardsSeed(t, 1)
	second := wB.AddHuman("second", "Second")
	second.Protection = 0
	second.Regions = RegionMix{Desert: 40_000}
	second.syncLand()
	for _, v := range []*Empire{victim, second} {
		v.Troopers, v.Turrets, v.Tanks, v.Jets = 30_000, 30_000, 30_000, 30_000
	}
	if _, err := wA.CreateGroupAttack(attacker, "boardB", "", GroupAttackHoursMin, AttackForce{Troopers: 50_000}); err != nil {
		t.Fatal(err)
	}
	wA.LaunchDueGroupAttacksAt(time.Now().Add(1000 * time.Hour))
	if len(wA.Outbox) == 0 || len(wA.Outbox[0].Attacks) == 0 {
		t.Fatal("the group attack never left")
	}
	res := wB.ApplyPacket(wA.Outbox[0]).Results[0]
	if res.Outcome != OutcomeRepelled && res.Outcome != OutcomeWon {
		t.Fatalf("outcome %q; no battle was fought", res.Outcome)
	}
	lostTroopers := regexp.MustCompile(`You lost (\d+) troopers\.`)
	sum := 0
	for _, v := range []*Empire{victim, second} {
		ev := v.Events[len(v.Events)-1].Text
		m := lostTroopers.FindStringSubmatch(ev)
		if m == nil {
			t.Fatalf("%s was not told its trooper losses:\n%s", v.Name, ev)
		}
		n, _ := strconv.Atoi(m[1])
		sum += n
	}
	if sum != res.Enemy.Troopers {
		t.Errorf("the realms were told %d troopers lost between them; the packet reports %d", sum, res.Enemy.Troopers)
	}
}

// The returning report lists what came home, what was lost and what was
// destroyed one unit type to a line, as the original's does.
func TestReturningReportListsUnitsOnTheirOwnLines(t *testing.T) {
	wA, wB, attacker, _ := twoBoardsSeed(t, 1)
	force := AttackForce{Troopers: 500_000, Tanks: 5000}
	if _, err := wA.CreateIndividualAttack(attacker, "boardB", "Victim", NormalAttack, force); err != nil {
		t.Fatal(err)
	}
	result := wB.ApplyPacket(wA.Outbox[0])
	wA.ApplyPacket(Packet{FromBoard: "boardB", ToBoard: "boardA", Results: result.Results})
	ev := attacker.Events[len(attacker.Events)-1].Text
	if !strings.Contains(ev, "Normal Attack") {
		t.Fatalf("never reached the returning report:\n%s", ev)
	}
	for _, want := range []string{
		"You lost ", " troopers.", " jets.", " tanks.", " bombers.",
		"You destroyed ", " turrets.",
		" troopers returned.", " jets returned.", " tanks returned.", " bombers returned.",
	} {
		if !strings.Contains(ev, want) {
			t.Errorf("the returning report lacks %q:\n%s", want, ev)
		}
	}
	// One line per unit, not a comma list.
	if strings.Contains(ev, "troopers, ") {
		t.Errorf("units are still listed on one line:\n%s", ev)
	}
}
