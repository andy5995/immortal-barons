package game

// CombatRules are the figures the sysop's Attack Rewards and Attack Damage
// levels select between, and what a winning same-planet attack pays (#359). A
// mod redefines what each level means; the sysop still chooses the level. The
// base values are the binary-verified constants of balance_combat.go and
// balance_costs.go. Level.AttackCapturePct and its siblings keep returning
// those constants, and the resolvers read these instead.
//
// What an interplanetary strike loses is not here: it is the attack type's own
// rate rescaled by the Attack Damage level (Level.InterplanetaryLossPct), with
// no table of its own to redefine.
type CombatRules struct {
	// Capture is the share of a beaten defender's regions a winning same-planet
	// attack takes, and InterplanetaryCapture the same for an interplanetary
	// strike (the two ladders differ at High). Retreat is the share of its own
	// force a side loses before it breaks off. All three are percents by level.
	Capture, InterplanetaryCapture, Retreat LevelPcts
	// CaptureFloor is the fewest regions a winning same-planet attack takes.
	CaptureFloor int
	// ScoreWin and ScoreCrush are the score a region captured pays, in an
	// ordinary win and in a total conquest.
	ScoreWin, ScoreCrush int
}

// LevelPcts is one percent per level of a None/Low/Medium/High setting.
type LevelPcts struct{ None, Low, Medium, High int }

// At is the percent for level l.
func (p LevelPcts) At(l Level) int {
	switch l {
	case None:
		return p.None
	case Low:
		return p.Low
	case High:
		return p.High
	default:
		return p.Medium
	}
}

func baseCombatRules() CombatRules {
	return CombatRules{
		Capture: LevelPcts{AttackCaptureNonePct, AttackCaptureLowPct,
			AttackCaptureMediumPct, AttackCaptureHighPct},
		InterplanetaryCapture: LevelPcts{AttackCaptureNonePct, AttackCaptureLowPct,
			AttackCaptureMediumPct, InterplanetaryCaptureHighPct},
		Retreat: LevelPcts{AttackRetreatNonePct, AttackRetreatLowPct,
			AttackRetreatMediumPct, AttackRetreatHighPct},
		CaptureFloor: RegularAttackCaptureFloor,
		ScoreWin:     CombatScoreWinPerRegion,
		ScoreCrush:   CombatScoreCrushPerRegion,
	}
}

// Limits for the combat figures. A percent is a share of something, so 0 to 100.
// A retreat share starts at 1: a side that will lose none of its force never
// fights a round. The floor is a region count, held at a million so that it
// stays a figure no realm outgrows on a 32-bit build, and the score per region
// at 100,000, which times a captured count is worked in int64 and saturates
// (combat.go).
const (
	ModCombatPctMax   = 100
	ModCombatFloorMax = 1_000_000
	ModCombatScoreMax = 100_000
)

// CombatField is one figure of CombatRules as a mod names it (see Field).
type CombatField = Field[CombatRules]

func pctField(key, desc string, min int64, table func(c *CombatRules) *LevelPcts, level func(p *LevelPcts) *int) CombatField {
	return CombatField{Key: key, Min: min, Max: ModCombatPctMax,
		Get:  func(c CombatRules) int64 { return int64(*level(table(&c))) },
		Set:  func(c *CombatRules, v int64) { *level(table(c)) = int(v) },
		Desc: desc}
}

// CombatFields are the figures a mod can set under [combat], in the order
// -print-base-mod writes them and validation reports them.
var CombatFields = func() []CombatField {
	capture := func(c *CombatRules) *LevelPcts { return &c.Capture }
	remote := func(c *CombatRules) *LevelPcts { return &c.InterplanetaryCapture }
	retreat := func(c *CombatRules) *LevelPcts { return &c.Retreat }
	none := func(p *LevelPcts) *int { return &p.None }
	low := func(p *LevelPcts) *int { return &p.Low }
	medium := func(p *LevelPcts) *int { return &p.Medium }
	high := func(p *LevelPcts) *int { return &p.High }
	return []CombatField{
		pctField("capture_none", "Percent of the defender's regions a winning same-planet attack takes when Attack Rewards is None.", 0, capture, none),
		pctField("capture_low", "The same at Attack Rewards Low.", 0, capture, low),
		pctField("capture_medium", "The same at Attack Rewards Medium.", 0, capture, medium),
		pctField("capture_high", "The same at Attack Rewards High.", 0, capture, high),
		pctField("ip_capture_none", "Percent of each defending realm's regions a winning interplanetary attack takes when Attack Rewards is None. It is then scaled by the attack's type.", 0, remote, none),
		pctField("ip_capture_low", "The same at Attack Rewards Low.", 0, remote, low),
		pctField("ip_capture_medium", "The same at Attack Rewards Medium.", 0, remote, medium),
		pctField("ip_capture_high", "The same at Attack Rewards High.", 0, remote, high),
		pctField("retreat_none", "Percent of its own force a side loses in a same-planet battle before it breaks off, when Attack Damage is None.", 1, retreat, none),
		pctField("retreat_low", "The same at Attack Damage Low.", 1, retreat, low),
		pctField("retreat_medium", "The same at Attack Damage Medium.", 1, retreat, medium),
		pctField("retreat_high", "The same at Attack Damage High.", 1, retreat, high),
		{Key: "capture_floor", Min: 0, Max: ModCombatFloorMax,
			Get:  func(c CombatRules) int64 { return int64(c.CaptureFloor) },
			Set:  func(c *CombatRules, v int64) { c.CaptureFloor = int(v) },
			Desc: "The fewest regions a winning same-planet attack takes, up to all the defender holds."},
		{Key: "score_win", Min: 0, Max: ModCombatScoreMax,
			Get:  func(c CombatRules) int64 { return int64(c.ScoreWin) },
			Set:  func(c *CombatRules, v int64) { c.ScoreWin = int(v) },
			Desc: "Score the attacker gains for each region an ordinary same-planet win captures."},
		{Key: "score_crush", Min: 0, Max: ModCombatScoreMax,
			Get:  func(c CombatRules) int64 { return int64(c.ScoreCrush) },
			Set:  func(c *CombatRules, v int64) { c.ScoreCrush = int(v) },
			Desc: "The same when the capture takes every region the defender had."},
	}
}()

func (r *Rules) validateCombat(bad func(format string, a ...any)) {
	validateFields(CombatFields, r.Combat, bad)
}
