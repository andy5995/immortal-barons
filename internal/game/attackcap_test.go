package game

import (
	"encoding/json"
	"strings"
	"testing"
)

// The per-day allowance is the original's "InterBBS: Max Individual Attacks"
// (game/reset.hlp), so it binds the Individual Attack Force and nothing else.
func TestIndividualAttackCapBindsInterplanetaryStrikes(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxIndividualAttacks = 3
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("att", "Attacker")
	a.Troopers = 500_000

	for i := 1; i <= 3; i++ {
		if !w.CanAttack(a) {
			t.Fatalf("interplanetary attack %d should be allowed", i)
		}
		if _, err := w.CreateIndividualAttack(a, "faraway", "Rome", NormalAttack,
			AttackForce{Troopers: 1000}); err != nil {
			t.Fatalf("attack %d: %v", i, err)
		}
		if a.AttacksToday != i {
			t.Fatalf("after attack %d, AttacksToday=%d", i, a.AttacksToday)
		}
	}
	if w.CanAttack(a) {
		t.Error("a 4th interplanetary attack should be blocked by the daily cap")
	}
	if _, err := w.CreateIndividualAttack(a, "faraway", "Rome", NormalAttack,
		AttackForce{Troopers: 1000}); err != ErrAttacksExhausted {
		t.Errorf("err = %v, want ErrAttacksExhausted", err)
	}

	w.Config.MaxIndividualAttacks = 0
	if !w.CanAttack(a) {
		t.Error("cap 0 should mean unlimited attacks")
	}
}

// A local attack neither spends the interplanetary allowance nor is refused for
// having spent it: run_attack_menu (BRE.OVR 0x3621) breaks its loop once a
// handler has fired, and no local launch routine holds a counter.
//
// The gate this replaced lived in the MENU, not here, so this cannot catch its
// return — internal/menu/attack_cap_test.go covers that end. What it holds is
// the game side: a realm far past the cap still fights, and fighting costs it
// nothing from that allowance.
func TestLocalAttacksDoNotSpendTheInterplanetaryAllowance(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxIndividualAttacks = 1
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("att", "Attacker")
	d := w.AddHuman("def", "Defender")
	a.Troopers = 500_000
	d.Regions = RegionMix{Mountain: 5000}
	d.syncLand()
	d.Turrets = 2_000_000
	a.AttacksToday = 99 // far past the cap

	// Casualties, not captured land: the marker has to hold whether the attacker
	// wins or loses, or the test is really asserting the balance. Checked after
	// EVERY attack, so a gate that stopped the second and third would show.
	for i := 1; i <= 3; i++ {
		before := d.Turrets
		w.Attack(a, d, FullForce(a), true)
		if d.Turrets >= before {
			t.Fatalf("attack %d drew no blood: defender kept all %d turrets", i, before)
		}
	}
	if a.AttacksToday != 99 {
		t.Errorf("AttacksToday=%d: a local attack spent the interplanetary allowance", a.AttacksToday)
	}
}

func TestAttackCapResetsAtDailyMaintenance(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.AttacksToday = 3
	w.LastMaintDate = "2026-07-16"
	w.DailyMaintenance("2026-07-17")
	if e.AttacksToday != 0 {
		t.Errorf("AttacksToday=%d after a day rolled over, want 0", e.AttacksToday)
	}
}

// BRE's reset writes these four per-day allowances (BRE.OVR 0x44db9, settings
// record +0x62..+0x68); the local one is IB's own and off.
func TestPerDayAllowanceDefaults(t *testing.T) {
	c := DefaultConfig()
	for name, got := range map[string]int{
		"MaxIndividualAttacks": c.MaxIndividualAttacks,
		"MaxGroupAttacks":      c.MaxGroupAttacks,
		"MaxTerrorOps":         c.MaxTerrorOps,
		"MaxBombingOps":        c.MaxBombingOps,
		"MaxLocalAttacks":      c.MaxLocalAttacks,
	} {
		want := map[string]int{"MaxIndividualAttacks": 1, "MaxGroupAttacks": 1,
			"MaxTerrorOps": 10, "MaxBombingOps": 5, "MaxLocalAttacks": 0}[name]
		if got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
}

// A league that leaves Max Local Attacks/Day at 0 sends the ruleset it sent
// before the setting existed, so boards that do not know it still agree on the
// fingerprint; setting it changes the fingerprint, so a board that cannot enforce
// it is noticed.
func TestLocalAttackAllowanceLeavesTheDefaultFingerprintAlone(t *testing.T) {
	c := DefaultConfig()
	b, err := json.Marshal(c.leagueRuleset())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "MaxLocalAttacks") {
		t.Errorf("the default ruleset carries MaxLocalAttacks: %s", b)
	}
	before := c.RulesetFingerprint()
	c.MaxLocalAttacks = 3
	if c.RulesetFingerprint() == before {
		t.Error("setting Max Local Attacks/Day left the ruleset fingerprint unchanged")
	}
}

// The local count resets with the others at daily maintenance.
func TestLocalAttackCountResetsAtDailyMaintenance(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")
	e.LocalAttacksToday = 4
	w.LastMaintDate = "2026-09-23"
	w.DailyMaintenance("2026-09-24")
	if e.LocalAttacksToday != 0 {
		t.Errorf("LocalAttacksToday = %d after maintenance, want 0", e.LocalAttacksToday)
	}
}
