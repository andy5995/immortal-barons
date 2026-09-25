package game

import (
	"testing"
	"time"
)

// A winning interplanetary strike takes a fifth of the defender's HeadQuarters,
// integer-divided (resolve_received_invasion +0x18f2, BRE.OVR 0x40D92). Golden
// literals: 100 loses 20, 57 loses 11.
func TestWinningInvasionTakesAFifthOfTheHQ(t *testing.T) {
	for _, tc := range []struct{ hq, want int }{{100, 80}, {57, 46}, {4, 4}, {0, 0}} {
		wA, wB, attacker, victim := twoBoardsSeed(t, 1)
		victim.HQ = tc.hq
		if _, err := wA.CreateIndividualAttack(attacker, "boardB", "Victim", NormalAttack, AttackForce{Troopers: 500_000, Tanks: 5000}); err != nil {
			t.Fatal(err)
		}
		res := wB.ApplyPacket(wA.Outbox[0]).Results[0]
		if res.Outcome != OutcomeWon {
			t.Fatalf("outcome %q; the test never reached the branch it covers", res.Outcome)
		}
		if victim.HQ != tc.want {
			t.Errorf("HQ %d after a lost invasion = %d, want %d", tc.hq, victim.HQ, tc.want)
		}
	}
}

// A repelled strike leaves the HeadQuarters alone: the loss sits after the
// won/lost test, in the block that takes regions.
func TestRepelledInvasionLeavesTheHQ(t *testing.T) {
	wA, wB, attacker, victim := twoBoardsSeed(t, 1)
	victim.Troopers, victim.Turrets, victim.Tanks = 5_000_000, 5_000_000, 5_000_000
	victim.HQ = 100
	if _, err := wA.CreateIndividualAttack(attacker, "boardB", "Victim", NormalAttack, AttackForce{Troopers: 1000}); err != nil {
		t.Fatal(err)
	}
	res := wB.ApplyPacket(wA.Outbox[0]).Results[0]
	if res.Outcome != OutcomeRepelled {
		t.Fatalf("outcome %q; the test never reached the branch it covers", res.Outcome)
	}
	if victim.HQ != 100 {
		t.Errorf("a repelled strike cost the HeadQuarters %d points", 100-victim.HQ)
	}
}

// On a planet-wide strike the per-realm block runs for every realm that stood,
// so each loses a fifth of its own HeadQuarters.
func TestPlanetWideInvasionTakesEveryDefendersHQ(t *testing.T) {
	wA, wB, attacker, victim := twoBoardsSeed(t, 1)
	second := wB.AddHuman("second", "Second")
	second.Protection = 0
	second.Regions = RegionMix{Desert: 40_000}
	second.syncLand()
	second.Troopers, second.Turrets, second.Tanks, second.Jets = 1000, 1000, 1000, 1000
	victim.HQ, second.HQ = 100, 50
	if _, err := wA.CreateGroupAttack(attacker, "boardB", "", GroupAttackHoursMin, AttackForce{Troopers: 900_000}); err != nil {
		t.Fatal(err)
	}
	wA.LaunchDueGroupAttacksAt(time.Now().Add(1000 * time.Hour))
	if len(wA.Outbox) == 0 || len(wA.Outbox[0].Attacks) == 0 {
		t.Fatal("the group attack never left")
	}
	res := wB.ApplyPacket(wA.Outbox[0]).Results[0]
	if res.Outcome != OutcomeWon {
		t.Fatalf("outcome %q; the test never reached the branch it covers", res.Outcome)
	}
	if victim.HQ != 80 || second.HQ != 40 {
		t.Errorf("HQs after a planet-wide win = %d and %d, want 80 and 40", victim.HQ, second.HQ)
	}
}
