package game

import "testing"

// An arriving missile is NOT the local missile of the same name (#255). The
// receiving board runs one resolver for all three, with its own gates and its
// own bands, so an arriving chemical strike kills people and touches nothing
// else — where the local one also ruins land and breaks morale and support.
func TestArrivingChemicalStrikeIsAPopulationWeaponAlone(t *testing.T) {
	from, to, attacker, target := specialOpWorlds(t)
	target.Land, target.People = 0, 0
	to.GrantRegions(target, Desert.Count(&target.Regions), 500)
	target.People = 1_000_000
	target.Morale, target.Support = 100, 100
	land, waste := target.Land, target.Regions.Waste

	if err := from.SendSpecialOp(attacker, "Bravo BBS", target.Name, OpChemical, 0); err != nil {
		t.Fatalf("SendSpecialOp: %v", err)
	}
	// Seeded so neither the misfire nor SDI stops it; assert it actually landed.
	answer := to.ApplyPacket(from.Outbox[0])
	if got := answer.Results[0].outcome(); got != OutcomeWon {
		t.Fatalf("the strike never landed (outcome %q), so this proves nothing", got)
	}

	if target.People >= 1_000_000 {
		t.Errorf("nobody died: People = %d", target.People)
	}
	if dead := 1_000_000 - target.People; dead < 150_000 || dead > 290_000 {
		t.Errorf("killed %d, want the 15-29%% band of a million", dead)
	}
	if target.Land != land || target.Regions.Waste != waste {
		t.Errorf("an arriving chemical strike must not touch land: %d -> %d land, %d -> %d waste",
			land, target.Land, waste, target.Regions.Waste)
	}
	if target.Morale != 100 || target.Support != 100 {
		t.Errorf("an arriving chemical strike must not touch morale or support: %d / %d",
			target.Morale, target.Support)
	}
}

// The arriving nuclear band is wider than the local one: 10-14% of the target's
// regions, against the local strike's 5-9%.
func TestArrivingNuclearStrikeUsesTheWiderBand(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 7)
	d := w.AddHuman("d", "Defendia")
	for i := 0; i < 200; i++ {
		d.Regions = RegionMix{}
		w.GrantRegions(d, Desert.Count(&d.Regions), 1000)
		ruined := w.arrivingNuclearEffect(d)
		if ruined < 100 || ruined > 140 {
			t.Fatalf("ruined %d of 1000 regions, want the 10-14%% band", ruined)
		}
	}
}

// Both rolls ahead of the damage stop the strike outright, and each says which
// of the two it was — a shield that worked and a weapon that failed are
// different news to the sender.
func TestArrivingMissileGatesReportSeparately(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 3)
	d := w.AddHuman("d", "Defendia")

	d.SDI = 100 // a full program: the interception roll is what will fire
	intercepted := 0
	for i := 0; i < 400; i++ {
		if got, why := w.stopArrivingMissile(d, OpNuclear, "Selby of Home"); why == specialIntercepted {
			if got.English() == "Defendia's SDI shot down your nuclear missile over "+w.Config.BoardID+"." {
				intercepted++
			}
		}
	}
	if intercepted == 0 {
		t.Error("a full SDI program never intercepted an arriving nuclear strike")
	}

	// With no program at all the interception roll still fires on a zero draw —
	// the original's comparison is inclusive, and that is faithful — so the
	// misfire is asserted by its own message rather than by being the only stop.
	d.SDI = 0
	misfired := 0
	for i := 0; i < 400; i++ {
		if got, why := w.stopArrivingMissile(d, OpNuclear, "Selby of Home"); why == specialMisfire {
			if _, ok := misfireEntryFor(got.English(), "nuclear missile", "Defendia", w.Config.BoardID); !ok {
				t.Fatalf("a misfire reported %q, which is not from the misfire pool", got)
			}
			misfired++
		}
	}
	if misfired == 0 {
		t.Error("no launch ever misfired across 400 tries of a 1-in-10 roll")
	}
}

// Each missile meets its own garrison: turrets against a nuclear strike, tanks
// against a chemical one (resolve_received_sabre_strike +0x4c7..+0x53c reads
// record +0x82 or +0x86 by op type). A realm deep in turrets and holding no
// tanks stops nuclear strikes and never a chemical one, and the reverse.
func TestEachMissileMeetsItsOwnGarrison(t *testing.T) {
	guarded := func(op SpecialOp, turrets, tanks int) int {
		n := 0
		for seed := int64(1); seed <= 200; seed++ {
			w := NewWorldSeed(DefaultConfig(), seed)
			d := w.AddHuman("d", "Defendia")
			d.SDI, d.Turrets, d.Tanks, d.Troopers = 0, turrets, tanks, 0
			if _, _, why, _ := w.applySpecialOp(op, d, "Selby of Home", 0); why == specialGuarded {
				n++
			}
		}
		return n
	}
	deep := 1_000_000_000
	if guarded(OpNuclear, deep, 0) == 0 || guarded(OpChemical, 0, deep) == 0 {
		t.Error("a deep garrison of the missile's own unit never stopped it")
	}
	if n := guarded(OpNuclear, 0, deep); n != 0 {
		t.Errorf("tanks stopped %d nuclear strikes; the garrison is turrets", n)
	}
	if n := guarded(OpChemical, deep, 0); n != 0 {
		t.Errorf("turrets stopped %d chemical strikes; the garrison is tanks", n)
	}
}

// The garrison roll is the original's (resolve_received_sabre_strike
// +0x540..+0x5b6): the unit count over regions plus one, against
// Random(50000), and then a second die above 2. Golden literals rather than
// the constants. The second die is drawn only when the first roll comes in
// under the garrison, as the original draws it.
func TestMissileGuardedIsTheOriginalsRoll(t *testing.T) {
	never := func(int) int { t.Fatal("the second die was drawn after a roll the garrison did not meet"); return 0 }
	die := func(v int) func(int) int {
		return func(n int) int {
			if n != 10 {
				t.Fatalf("second die is Random(%d), want Random(10)", n)
			}
			return v
		}
	}
	for _, c := range []struct {
		guard, regions, roll, second int
		draws, want                  bool
	}{
		// 999 regions + 1 = 1,000: 5,000,000 turrets is 5,000 a region.
		{5_000_000, 999, 4_999, 3, true, true},
		{5_000_000, 999, 4_999, 2, true, false},
		{5_000_000, 999, 4_999, 9, true, true},
		{5_000_000, 999, 5_000, 0, false, false}, // not under: no second die
		{4_999, 0, 4_998, 3, true, true},         // a realm with no land divides by 1
		{0, 100, 0, 0, false, false},             // no garrison never stops one
	} {
		intn := never
		if c.draws {
			intn = die(c.second)
		}
		if got := missileGuarded(c.guard, c.regions, c.roll, intn); got != c.want {
			t.Errorf("guard %d over %d regions, roll %d, die %d: got %v, want %v",
				c.guard, c.regions, c.roll, c.second, got, c.want)
		}
	}
}
