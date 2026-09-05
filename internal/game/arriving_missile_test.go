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
		if got := w.arrivingMissileStopped(d, "nuclear strike"); got != "" {
			if got == "Defendia's SDI intercepted your nuclear strike." {
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
		if got := w.arrivingMissileStopped(d, "nuclear strike"); got == "The nuclear strike misfired and never reached Defendia." {
			misfired++
		}
	}
	if misfired == 0 {
		t.Error("no launch ever misfired across 400 tries of a 1-in-10 roll")
	}
}
