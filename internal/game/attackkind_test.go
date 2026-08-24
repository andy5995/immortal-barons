package game

import "testing"

// The figures below are golden literals, not mirrors of the balance.go
// constants — a retune has to fail here and produce new evidence (CLAUDE.md's
// fidelity contract). Capture and loss are quoted from BRE's own in-game help,
// game/attack.hlp. The STRENGTH multipliers are quoted from the resolver
// instead (BRE.OVR 0x4055a-0x405a8), because the two sources disagree about the
// quick strike: the help advertises 110% and the code loads Real48 1.2.

// TestAttackKindRatesMatchBRE pins each variant's three published figures.
func TestAttackKindRatesMatchBRE(t *testing.T) {
	for _, c := range []struct {
		kind                    AttackKind
		name                    string
		strength, capture, loss int
	}{
		{QuickStrike, "Quick Strike", 120, 50, 8},
		{NormalAttack, "Normal Attack", 100, 100, 15},
		{ExtendedBattle, "Extended Battle", 85, 125, 20},
	} {
		if got := c.kind.String(); got != c.name {
			t.Errorf("%v name = %q, want %q", int(c.kind), got, c.name)
		}
		if got := c.kind.strengthPct(); got != c.strength {
			t.Errorf("%s strength = %d%%, want %d%%", c.name, got, c.strength)
		}
		if got := c.kind.capturePct(); got != c.capture {
			t.Errorf("%s capture = %d%%, want %d%%", c.name, got, c.capture)
		}
		if got := c.kind.lossPct(); got != c.loss {
			t.Errorf("%s losses = %d%%, want %d%%", c.name, got, c.loss)
		}
	}
}

// TestAttackKindWireCodes pins the values BRE stores in its outbound attack
// record (BRE.OVR's IBBS attack unit presets 1 and writes 0 for '2', 2 for '3').
// A packet has to stay readable to the original, so these are not free to
// renumber.
func TestAttackKindWireCodes(t *testing.T) {
	for _, c := range []struct {
		kind AttackKind
		code int
	}{{QuickStrike, 0}, {NormalAttack, 1}, {ExtendedBattle, 2}} {
		if int(c.kind) != c.code {
			t.Errorf("%s wire code = %d, want %d", c.kind, int(c.kind), c.code)
		}
	}
}

// TestQuickStrikeSendsAtGreaterStrength proves the strength multiplier is
// applied to the offense that actually leaves the board, not just recorded.
func TestQuickStrikeSendsAtGreaterStrength(t *testing.T) {
	f := AttackForce{Troopers: 1000} // offense 1000, so the percentages read directly
	for _, c := range []struct {
		kind AttackKind
		want int
	}{{QuickStrike, 1200}, {NormalAttack, 1000}, {ExtendedBattle, 850}} {
		w := NewWorldSeed(DefaultConfig(), 1)
		e := w.AddHuman("alice", "Alethia")
		e.Troopers = 5000
		if _, err := w.CreateIndividualAttack(e, "faraway", "Rome", c.kind, f); err != nil {
			t.Fatalf("%s: CreateIndividualAttack: %v", c.kind, err)
		}
		got := w.Outbox[0].Attacks[0].Offense
		if got != c.want {
			t.Errorf("%s offense = %d, want %d", c.kind, got, c.want)
		}
		if k := w.Outbox[0].Attacks[0].Kind; k != c.kind {
			t.Errorf("packet carried kind %v, want %v", k, c.kind)
		}
	}
}

// TestAttackKindCasualtiesAndCapture proves the target board applies the
// arriving kind's own rates: a quick strike brings more men home and takes less
// land than an extended battle against the same defender.
func TestAttackKindCasualtiesAndCapture(t *testing.T) {
	type outcome struct{ survivors, land int }
	got := map[AttackKind]outcome{}
	for _, kind := range []AttackKind{QuickStrike, NormalAttack, ExtendedBattle} {
		w := NewWorldSeed(DefaultConfig(), 1)
		victim := w.AddHuman("bob", "Rome")
		victim.Protection = 0
		// Enough land that the integer division in the capture chain does not
		// swamp the differences between the variants.
		victim.Regions = RegionMix{Desert: 40_000}
		victim.syncLand()
		res := w.resolveRemoteAttack(RemoteAttack{
			ID: 1, FromBoard: "far", TargetEmpire: "Rome", Kind: kind,
			// Far above any starting defence, so every variant wins and the
			// difference on show is the kind's own rates.
			Offense:      50_000_000,
			Contributors: []Contribution{{Owner: "alice", AttackForce: AttackForce{Troopers: 1000}}},
		})
		if !res.Won {
			t.Fatalf("%s did not overwhelm the defender", kind)
		}
		if res.Kind != kind.String() {
			t.Errorf("result named %q, want %q", res.Kind, kind.String())
		}
		got[kind] = outcome{res.Survivors[0].Troopers, res.LandTaken}
	}
	// A force this far above the defence is almost never the side that gets hit,
	// so it walks away nearly whole WHATEVER its type: the published 8/15/20 is
	// where the loop stops, not what the winner pays (#199). The kinds differ
	// here only in what they carry off, asserted below.
	for kind, o := range got {
		if o.survivors < 995 {
			t.Errorf("%s survivors = %d, want at least 995 of 1000 for an overwhelming strike", kind, o.survivors)
		}
	}
	// Land follows the capture rates relative to a normal attack: half, and a
	// quarter again on top.
	n := got[NormalAttack].land
	if n <= 0 {
		t.Fatalf("normal attack took no land, nothing to compare against")
	}
	if want := n * 50 / 100; got[QuickStrike].land != want {
		t.Errorf("quick strike took %d regions, want %d (50%% of a normal %d)", got[QuickStrike].land, want, n)
	}
	if want := n * 125 / 100; got[ExtendedBattle].land != want {
		t.Errorf("extended battle took %d regions, want %d (125%% of a normal %d)", got[ExtendedBattle].land, want, n)
	}
}

// TestGroupAttackFightsAsNormal proves a group attack, which BRE gives no type
// choice, resolves on the normal attack's terms rather than inheriting
// QuickStrike from AttackKind's zero value.
func TestGroupAttackFightsAsNormal(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	victim := w.AddHuman("bob", "Rome")
	victim.Protection = 0
	// The share is only visible on the side that BREAKS OFF, so send a force that
	// loses: it grinds down to its own threshold and stops there.
	victim.Troopers, victim.Turrets, victim.Tanks = 500_000, 200_000, 50_000
	res := w.resolveRemoteAttack(RemoteAttack{
		ID: 1, FromBoard: "far", TargetEmpire: "Rome", Group: true,
		Offense:      1000,
		Contributors: []Contribution{{Owner: "alice", AttackForce: AttackForce{Troopers: 1000}}},
	})
	if res.Kind != "Group Attack" {
		t.Errorf("result named %q, want %q", res.Kind, "Group Attack")
	}
	if res.Won {
		t.Fatal("the token force was meant to lose, or the share below proves nothing")
	}
	// 15% is the normal attack's threshold, and a losing side stops just past it.
	// Quick Strike (8%) would leave 920+ and Extended Battle (20%) 800 or fewer,
	// so this band names the normal attack and nothing else.
	if s := res.Survivors[0].Troopers; s < 830 || s > 850 {
		t.Errorf("survivors = %d, want 830..850 (the normal attack's 15%% threshold)", s)
	}
}

// TestIndividualAttackDoublesReturns pins BRE's stated trade for going alone:
// "You get twice as many returns if you send the attack yourself, but you can't
// attack an entire planet" (game/breins.txt and docs/bre.doc). Golden literal —
// the ratio is BRE's, not a knob.
func TestIndividualAttackDoublesReturns(t *testing.T) {
	land := func(group bool) int {
		w := NewWorldSeed(DefaultConfig(), 1)
		victim := w.AddHuman("bob", "Rome")
		victim.Protection = 0
		victim.Regions = RegionMix{Desert: 40_000}
		victim.syncLand()
		return w.resolveRemoteAttack(RemoteAttack{
			ID: 1, FromBoard: "far", TargetEmpire: "Rome",
			Kind: NormalAttack, Group: group,
			Offense:      50_000_000,
			Contributors: []Contribution{{Owner: "alice", AttackForce: AttackForce{Troopers: 1000}}},
		}).LandTaken
	}
	g, i := land(true), land(false)
	if g <= 0 {
		t.Fatalf("group attack took no land, nothing to compare against")
	}
	if i != g*2 {
		t.Errorf("individual attack took %d regions, want %d (twice the group attack's %d)", i, g*2, g)
	}
}

// TestTokenStrikeCostsTheDefenderAlmostNothing is the guard for #199: a strike
// far weaker than the defence must cost the defender almost nothing, because it
// reaches its OWN retreat threshold after a round or two and breaks off. The
// figures are the ones a live league reported — 100 troopers and 1,000 tanks,
// 1,100 gold, against a realm holding ~97,000 units — where the defender lost
// 7,696 units, 8% of everything, exactly as it would have to a full-scale
// invasion.
//
// The bound is deliberately loose (4%, against the 8% the bug produced): which
// side is hit each round is a die roll, so a figure that only holds on this
// seed would prove nothing about the mechanic.
func TestTokenStrikeCostsTheDefenderAlmostNothing(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	victim := w.AddHuman("bob", "Rome")
	victim.Protection = 0
	victim.Troopers, victim.Turrets, victim.Tanks, victim.Jets = 37_029, 27_250, 11_456, 21_164
	before := victim.Troopers + victim.Turrets + victim.Tanks + victim.Jets

	force := AttackForce{Troopers: 100, Tanks: 1000}
	res := w.resolveRemoteAttack(RemoteAttack{
		ID: 1, FromBoard: "far", TargetEmpire: "Rome", Kind: QuickStrike,
		Offense:      force.offense(),
		Contributors: []Contribution{{Owner: "alice", AttackForce: force}},
	})
	if res.Won {
		t.Fatal("a 1,100-unit force overran a realm holding 97,000; the defence is not being fought")
	}
	if lost := res.Enemy.Total(); lost > before/25 {
		t.Errorf("the defender lost %d of %d units to a token strike; the flat rate this replaced took 8%% (%d)",
			lost, before, before*8/100)
	}
	// No bombers were sent, so no jet can die: the air battle is decided by the
	// attacker's bombers alone and by nothing on the ground.
	if victim.Jets != 21_164 {
		t.Errorf("jets fell to %d with no bomber in the strike; they take the air battle's share, not the ground one", victim.Jets)
	}
}

// The other half of the air battle: bombers, and only bombers, cost the defender
// jets — and they cost it nothing else. Ground and air are resolved as two
// separate fights in one loop, so the two fractions differ.
func TestBombersCostTheDefenderJetsAndNothingElse(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 7)
	victim := w.AddHuman("bob", "Rome")
	victim.Protection = 0
	victim.Troopers, victim.Turrets, victim.Tanks, victim.Jets = 40_000, 30_000, 12_000, 20_000

	force := AttackForce{Troopers: 50_000, Tanks: 20_000, Bombers: 5_000}
	res := w.resolveRemoteAttack(RemoteAttack{
		ID: 1, FromBoard: "far", TargetEmpire: "Rome", Kind: NormalAttack,
		Offense:      force.offense(),
		Contributors: []Contribution{{Owner: "alice", AttackForce: force}},
	})
	if res.Enemy.Jets == 0 {
		t.Error("a strike carrying 5,000 bombers cost the defender no jets")
	}
	// Bombers add no ground strength, so they cannot be what took the regions or
	// the ground units: those follow the troopers and tanks.
	if res.Enemy.Troopers == 0 || res.Enemy.Tanks == 0 {
		t.Error("the ground battle cost the defender nothing")
	}
}
