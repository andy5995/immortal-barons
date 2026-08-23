package game

import "testing"

// Local Attacks and Local Attack Scoring are league settings; a stand-alone
// board fights and scores whatever they say, because BRE scopes both to
// interplanetary games.
func TestLocalAttackSwitchesOnlyBiteInALeague(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LocalAttacks, cfg.LocalAttackScoring = false, false
	w := NewWorldSeed(cfg, 1)
	if !w.LocalAttacksAllowed() {
		t.Error("a stand-alone board should always allow local attacks")
	}
	if !w.localAttacksScore() {
		t.Error("a stand-alone board should always score local attacks")
	}

	w.Config.IBBS = true
	if w.LocalAttacksAllowed() {
		t.Error("a league with Local Attacks off should refuse them")
	}
	if w.localAttacksScore() {
		t.Error("a league with Local Attack Scoring off should not score them")
	}
}

// With Local Attack Scoring off, a won battle moves neither side's score — the
// realm still takes the land and the casualties.
func TestLocalAttackScoringOffMovesNoScore(t *testing.T) {
	fight := func(scoring bool) (attacker, defender *Empire, captured int) {
		cfg := DefaultConfig()
		cfg.IBBS = true
		cfg.LocalAttackScoring = scoring
		w := NewWorldSeed(cfg, 7)
		a := w.AddHuman("alice", "Alethia")
		d := w.AddHuman("bob", "Borealis")
		a.Protection, d.Protection = 0, 0
		a.Troopers, a.Tanks = 200_000, 20_000
		d.Troopers, d.Turrets, d.Land = 100, 0, 500
		d.Regions = RegionMix{Agricultural: 500}
		a.Score, d.Score = 5_000, 5_000
		_, captured = w.Attack(a, d, FullForce(a), false)
		return a, d, captured
	}

	on, onDef, captured := fight(true)
	if captured == 0 {
		t.Fatalf("the attack did not win, so the test proves nothing")
	}
	if on.Score <= 5_000 {
		t.Fatalf("with scoring on the winner should gain, got %d", on.Score)
	}
	// The loser is not docked — BRE never writes a second realm's Score.
	if onDef.Score != 5_000 {
		t.Fatalf("the loser's score should be untouched, got %d", onDef.Score)
	}

	off, offDef, captured := fight(false)
	if captured == 0 {
		t.Fatal("the attack did not win with scoring off")
	}
	if off.Score != 5_000 {
		t.Errorf("winner Score = %d, want it unmoved at 5,000", off.Score)
	}
	if offDef.Score != 5_000 {
		t.Errorf("loser Score = %d, want it unmoved at 5,000", offDef.Score)
	}
}

// A league that has turned local fighting off binds the AI too, or the computer
// barons would be the only ones still at war.
func TestAIStandsDownWhenLocalAttacksAreOff(t *testing.T) {
	setUp := func(allowed bool) *Empire {
		cfg := DefaultConfig()
		cfg.IBBS = true
		cfg.LocalAttacks = allowed
		w := NewWorldSeed(cfg, 3)
		ai := w.addAIEmpire("Warlord")
		ai.AIProfile = AIProfileAggressor
		ai.Protection = 0
		ai.Troopers, ai.Tanks = 500_000, 50_000
		victim := w.AddHuman("bob", "Borealis")
		victim.Protection, victim.Troopers, victim.Land = 0, 10, 400
		victim.Regions = RegionMix{Agricultural: 400}
		w.aiWageWar(ai)
		return ai
	}

	// The control: this AI, this target, is one the aggression check would take.
	if ai := setUp(true); ai.AttacksToday == 0 {
		t.Fatal("the AI declined an easy target even with local attacks allowed, so the test proves nothing")
	}
	if ai := setUp(false); ai.AttacksToday != 0 {
		t.Errorf("the AI attacked %d times with local attacks disabled", ai.AttacksToday)
	}
}
