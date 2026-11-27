package game

import (
	"strings"
	"testing"
)

// The base combat rules are BRE-verified: golden literals, so a retune of a
// constant fails here and asks for new evidence.
func TestBaseCombatRulesAreTheBinaryFigures(t *testing.T) {
	c := BaseRules().Combat
	want := CombatRules{
		Capture:               LevelPcts{0, 5, 10, 25},
		InterplanetaryCapture: LevelPcts{0, 5, 10, 15},
		Retreat:               LevelPcts{1, 10, 20, 30},
		CaptureFloor:          15,
		ScoreWin:              82,
		ScoreCrush:            192,
	}
	if c != want {
		t.Errorf("base combat rules = %+v, want %+v", c, want)
	}
	for _, l := range []Level{None, Low, Medium, High} {
		if c.Capture.At(l) != l.AttackCapturePct() || c.InterplanetaryCapture.At(l) != l.InterplanetaryCapturePct() ||
			c.Retreat.At(l) != l.AttackRetreatPct() {
			t.Errorf("level %v: the rules table and the Level methods disagree", l)
		}
	}
}

func combatFight(t *testing.T, land int, edit func(*CombatRules)) (*World, *Empire, *Empire) {
	t.Helper()
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Rules = BaseRules()
	edit(&w.Rules.Combat)
	if err := w.Rules.Validate(); err != nil {
		t.Fatal(err)
	}
	a := &Empire{Name: "A", Troopers: 1_000_000, Morale: 100, Alive: true}
	d := &Empire{Name: "D", Troopers: 1, Morale: 100, Land: land,
		Regions: RegionMix{Mountain: land}, People: 1_000_000, Alive: true}
	return w, a, d
}

func TestModdedCaptureShareFloorAndScoreReachTheLocalAttack(t *testing.T) {
	// Medium attack rewards: 20% of 500 is 100, base 10% is 50.
	w, a, d := combatFight(t, 500, func(c *CombatRules) { c.Capture.Medium = 20; c.ScoreWin = 100 })
	w.Attack(a, d, FullForce(a))
	if got := 500 - d.Land; got != 100 {
		t.Errorf("captured %d, want 100 (20%% of 500)", got)
	}
	if a.Score != 100*100 {
		t.Errorf("score %d, want 10000 (100 regions x 100)", a.Score)
	}

	// A floor of 30 on a realm of 100 regions beats its 10% share.
	w, a, d = combatFight(t, 100, func(c *CombatRules) { c.CaptureFloor = 30 })
	w.Attack(a, d, FullForce(a))
	if got := 100 - d.Land; got != 30 {
		t.Errorf("captured %d, want the modded floor 30", got)
	}

	// A floor past the defender's land takes all of it: a total conquest.
	w, a, d = combatFight(t, 100, func(c *CombatRules) { c.CaptureFloor = 500; c.ScoreCrush = 7 })
	w.Attack(a, d, FullForce(a))
	if d.Land != 0 || a.Score != 100*7 {
		t.Errorf("defender land %d, score %d, want 0 and 700 (100 regions x 7)", d.Land, a.Score)
	}
}

// The level chosen by the sysop picks the figure: High reads capture_high.
func TestModdedCaptureFollowsTheSysopsLevel(t *testing.T) {
	w, a, d := combatFight(t, 500, func(c *CombatRules) { c.Capture.High = 40 })
	w.Config.AttackRewards = High
	w.Attack(a, d, FullForce(a))
	if got := 500 - d.Land; got != 200 {
		t.Errorf("captured %d, want 200 (40%% of 500)", got)
	}
}

func TestModdedRetreatShareSetsWhatALosingAttackerKeeps(t *testing.T) {
	lost := func(retreat int) float64 {
		w := NewWorldSeed(DefaultConfig(), 1)
		w.Rules = BaseRules()
		w.Rules.Combat.Retreat.Medium = retreat
		a := &Empire{Name: "A", Troopers: 1000, Morale: 100, Alive: true}
		d := &Empire{Name: "D", Turrets: 1_000_000, Morale: 100, Land: 500,
			Regions: RegionMix{Mountain: 500}, People: 1_000_000, Alive: true}
		w.Attack(a, d, FullForce(a))
		return float64(1000-a.Troopers) / 1000
	}
	base, modded := lost(20), lost(60)
	if base < 0.20 || base > 0.30 {
		t.Fatalf("base attacker lost %.3f, want just past 0.20", base)
	}
	if modded < 0.60 || modded > 0.70 {
		t.Errorf("modded attacker lost %.3f, want just past 0.60", modded)
	}
}

func TestModdedInterplanetaryCaptureReachesTheDefendingBoard(t *testing.T) {
	land := func(edit func(*CombatRules)) int {
		w := NewWorldSeed(DefaultConfig(), 1)
		w.Rules = BaseRules()
		edit(&w.Rules.Combat)
		victim := w.AddHuman("bob", "Rome")
		victim.Protection = 0
		victim.Regions = RegionMix{Desert: 40_000}
		victim.syncLand()
		return w.resolveRemoteAttack(RemoteAttack{
			ID: 1, FromBoard: "far", TargetEmpire: "Rome", Kind: NormalAttack, Group: true,
			Offense:      50_000_000,
			Contributors: []Contribution{{Owner: "alice", AttackForce: AttackForce{Troopers: 1000}}},
		}).LandTaken
	}
	if got := land(func(*CombatRules) {}); got != 4000 {
		t.Fatalf("base took %d, want 4000 (10%% of 40,000)", got)
	}
	if got := land(func(c *CombatRules) { c.InterplanetaryCapture.Medium = 20 }); got != 8000 {
		t.Errorf("modded took %d, want 8000 (20%%)", got)
	}
	// The local table does not reach an interplanetary strike.
	if got := land(func(c *CombatRules) { c.Capture.Medium = 20 }); got != 4000 {
		t.Errorf("a local capture figure moved an interplanetary strike to %d", got)
	}
}

// 40,000 regions at 100,000 score each is 4e9: past an int on a 32-bit door.
// The gain saturates rather than wrapping into a negative score.
func TestAHugeScoreAwardSaturates(t *testing.T) {
	w, a, d := combatFight(t, 400_000, func(c *CombatRules) { c.ScoreWin = ModCombatScoreMax })
	w.Attack(a, d, FullForce(a))
	if got, want := a.Score, fitInt(40_000*100_000); got != want || got <= 0 {
		t.Errorf("score %d, want %d", got, want)
	}
}

func TestCombatLimitsAreEnforced(t *testing.T) {
	for _, c := range []struct {
		edit func(*CombatRules)
		want string
	}{
		{func(c *CombatRules) { c.Capture.High = 101 }, "capture_high 101 is outside 0 to 100"},
		{func(c *CombatRules) { c.InterplanetaryCapture.None = -1 }, "ip_capture_none -1 is outside 0 to 100"},
		{func(c *CombatRules) { c.Retreat.Medium = 0 }, "retreat_medium 0 is outside 1 to 100"},
		{func(c *CombatRules) { c.CaptureFloor = ModCombatFloorMax + 1 }, "capture_floor 1000001 is outside 0 to 1000000"},
		{func(c *CombatRules) { c.ScoreWin = ModCombatScoreMax + 1 }, "score_win 100001 is outside 0 to 100000"},
		{func(c *CombatRules) { c.ScoreCrush = -5 }, "score_crush -5 is outside 0 to 100000"},
	} {
		r := BaseRules()
		c.edit(&r.Combat)
		if err := r.Validate(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("err = %v, want it to say %q", err, c.want)
		}
	}
}
