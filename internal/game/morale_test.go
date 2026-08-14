package game

import "testing"

// Golden literals from BRE.OVR 0xF37B: effectiveness is morale x 0.6 + 50, so a
// fully motivated army fights ABOVE par at 110% and a broken one at 50%.
func TestMoraleFactor(t *testing.T) {
	for _, c := range []struct{ morale, want int }{{0, 50}, {50, 80}, {100, 110}} {
		if got := moraleFactor(c.morale); got != c.want {
			t.Errorf("moraleFactor(%d) = %d, want %d", c.morale, got, c.want)
		}
	}
}

// Golden literals from BRE.OVR 0x2F6CA (the cost) and 0x2F91E (the award). The
// morale boost is priced off the ARMY: troopers and turrets at 0.10 a head, jets
// at 0.05 and tanks at 0.15, times the deficit, plus a flat 500. Bombers and
// carriers are not priced at all.
func TestMoraleBoostCostAndAward(t *testing.T) {
	mk := func() (*World, *Empire) {
		w := NewWorldSeed(DefaultConfig(), 1)
		e := w.AddHuman("me", "Mine")
		e.Morale = 40
		e.Troopers, e.Jets, e.Turrets, e.Tanks = 1000, 1000, 1000, 1000
		e.Bombers, e.Carriers = 1000, 1000 // free: neither is priced
		e.Gold = 10_000_000
		return w, e
	}
	w, e := mk()
	// deficit 15 x (1000x0.10 + 1000x0.05 + 1000x0.10 + 1000x0.15) + 500
	if got := w.MoraleBoostCost(e); got != 6500 {
		t.Fatalf("MoraleBoostCost = %d, want 6500", got)
	}
	if got := w.MoraleBoostMax(e); got != 9750 {
		t.Errorf("MoraleBoostMax = %d, want 9750 (cost x 3/2)", got)
	}
	if got := w.BoostMorale(e, 6500); got != 15 {
		t.Errorf("paying the request bought %d points, want the whole 15", got)
	}

	w, e = mk()
	if got := w.BoostMorale(e, 3250); got != 7 {
		t.Errorf("paying half bought %d points, want 7 (15 x 3251/6501, truncated)", got)
	}

	// Overpaying really does buy past the charged deficit — the same behaviour
	// the support boost has, and not an IB addition.
	w, e = mk()
	e.Morale = 40
	if got := w.BoostMorale(e, 9750); got != 22 {
		t.Errorf("paying the maximum bought %d points, want 22", got)
	}
}

// A realm with no army pays only the flat fee.
func TestMoraleBoostOfAnEmptyArmy(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Morale, e.Troopers, e.Gold = 90, 0, 10_000
	if got := w.MoraleBoostCost(e); got != MoraleBoostFlat {
		t.Errorf("MoraleBoostCost = %d, want the flat %d", got, MoraleBoostFlat)
	}
	e.Morale = 100
	if got := w.MoraleBoostCost(e); got != 0 {
		t.Errorf("a realm at full morale is asked for %d, want 0", got)
	}
}

// Underpaying the forces FILES a morale penalty rather than applying it: BRE
// accumulates the whole payment stage on the empire record and spends it at
// rollover, so the drop surfaces on the next turn's display.
func TestPayForcesShortfallFilesMoralePenalty(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Morale = 100
	e.Troopers = 1000
	e.Gold = 1_000_000
	if w.ForcesDue(e) == 0 {
		t.Skip("no forces upkeep to underpay")
	}
	w.PayForces(e, 0) // pay nothing
	if e.Morale != 100 {
		t.Errorf("the penalty is deferred, so morale should still read 100, got %d", e.Morale)
	}
	if e.PendingMoralePenalty <= 0 {
		t.Fatalf("PendingMoralePenalty = %d, want > 0", e.PendingMoralePenalty)
	}
	w.GrowFood(e)
	w.PlayTurn(e, "2026-07-03")
	if e.Morale >= 100 {
		t.Errorf("the filed penalty should land at rollover, got morale %d", e.Morale)
	}
}

// Morale does NOT recover on its own. Nothing in the original adds to it except
// the boost the baron pays for, and IB used to drift it back 4 points a turn,
// which quietly hid the whole mechanic. Property, so it holds on every seed.
func TestMoraleNeverRecoversUnpaid(t *testing.T) {
	for seed := int64(1); seed <= 8; seed++ {
		w := NewWorldSeed(DefaultConfig(), seed)
		e := w.AddHuman("me", "Mine")
		e.Morale = 60
		e.Troopers = 0 // nothing to desert, nothing to feed
		e.Gold = 10_000_000
		for i := 0; i < 10; i++ {
			w.GrowFood(e)
			w.PlayTurn(e, "2026-07-03")
		}
		if e.Morale > 60 {
			t.Errorf("seed %d: morale rose to %d without paying for it", seed, e.Morale)
		}
	}
}

// Desertion bands (BRE.OVR 0xC1F9): nobody deserts at 40 morale or above, and
// below it troopers, jets and tanks go while turrets, bombers and carriers stay.
// Asserted as a property across seeds — the rate itself is two random draws wide
// and often comes out zero in the milder bands.
func TestLowMoraleDesertsTheRightUnits(t *testing.T) {
	deserted := 0
	for seed := int64(1); seed <= 20; seed++ {
		w := NewWorldSeed(DefaultConfig(), seed)
		e := w.AddHuman("me", "Mine")
		e.Morale = 5 // the worst band, where the rate is never below 6
		e.Troopers, e.Jets, e.Turrets, e.Bombers, e.Tanks, e.Carriers = 1000, 1000, 1000, 1000, 1000, 1000

		w.moraleDesertion(e)

		if e.Turrets != 1000 || e.Bombers != 1000 || e.Carriers != 1000 {
			t.Fatalf("seed %d: turrets/bombers/carriers must never desert, got %d/%d/%d",
				seed, e.Turrets, e.Bombers, e.Carriers)
		}
		if e.Troopers > 1000 || e.Jets > 1000 || e.Tanks > 1000 {
			t.Fatalf("seed %d: desertion must not add units", seed)
		}
		if e.LastMoraleDesertion > 0 {
			deserted++
		}
	}
	if deserted == 0 {
		t.Errorf("a realm at 5 morale never lost a unit across 20 seeds")
	}

	for seed := int64(1); seed <= 20; seed++ {
		w := NewWorldSeed(DefaultConfig(), seed)
		e := w.AddHuman("me", "Mine")
		e.Morale = MoraleDesertBandTop
		e.Troopers = 1000
		w.moraleDesertion(e)
		if e.Troopers != 1000 || e.LastMoraleDesertion != 0 {
			t.Fatalf("seed %d: nobody deserts at %d morale, got %d troopers",
				seed, MoraleDesertBandTop, e.Troopers)
		}
	}
}
