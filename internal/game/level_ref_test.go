package game

// The level ladders as the resolvers read them before #359 moved the figures
// into Rules.Combat. Kept for the reference-copy tests and the golden checks.

// AttackCapturePct is the share of a beaten defender's regions the winner
// takes at this Attack Rewards level, and AttackRetreatPct the share of its own
// force a side will lose before it breaks off at this Attack Damage level.
// Both are read out of the binary rather than one ladder applied to a baseline
// — see the tables in balance_combat.go for why that matters.
func (l Level) AttackCapturePct() int {
	switch l {
	case None:
		return AttackCaptureNonePct
	case Low:
		return AttackCaptureLowPct
	case High:
		return AttackCaptureHighPct
	default:
		return AttackCaptureMediumPct
	}
}

// InterplanetaryCapturePct is the share of a beaten defender's regions a winning
// INTERPLANETARY strike carries off, by Attack Rewards. It is NOT the local
// table: the two resolvers keep separate ladders and disagree at the top.
//
// BINARY-VERIFIED. resolve_received_invasion (+0x12a4) switches on the same
// config byte +0x183 the local resolver reads at 0xFFF9, and loads a Real48 per
// level. Read against BRE's own byte encoding (Medium 0, None 1, Low 2, High 3)
// those are 10, 0, 5 and 15 — where the local ladder's High is 25.
//
// IB applied no Attack Rewards setting at all to an interplanetary strike until
// 2026-08-24, so the sysop's knob moved local attacks and nothing else.
func (l Level) InterplanetaryCapturePct() int {
	switch l {
	case None:
		return AttackCaptureNonePct
	case Low:
		return AttackCaptureLowPct
	case High:
		return InterplanetaryCaptureHighPct
	default:
		return AttackCaptureMediumPct
	}
}

func (l Level) AttackRetreatPct() int {
	switch l {
	case None:
		return AttackRetreatNonePct
	case Low:
		return AttackRetreatLowPct
	case High:
		return AttackRetreatHighPct
	default:
		return AttackRetreatMediumPct
	}
}
