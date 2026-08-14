package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

func sendSpy(s session.Session, w *ctx) Result {
	return localAttack(s, w, "Send Spy", nil, false, func(a, d *game.Empire) (string, error) { return w.SendSpy(a, d) })
}

func supportDissensions(s session.Session, w *ctx) Result {
	return localAttack(s, w, "Support Dissensions", nil, false, func(a, d *game.Empire) (string, error) { return w.SupportDissensions(a, d) })
}

func demoralizeForces(s session.Session, w *ctx) Result {
	return localAttack(s, w, "Demoralize Forces", nil, false, func(a, d *game.Empire) (string, error) { return w.DemoralizeForces(a, d) })
}

func setUp(s session.Session, w *ctx) Result {
	return localAttack(s, w, "Set Up", nil, false, func(a, d *game.Empire) (string, error) { return w.SetUp(a, d) })
}

func exposeEnemyOps(s session.Session, w *ctx) Result {
	return localAttack(s, w, "Expose Enemy Ops", nil, false, func(a, d *game.Empire) (string, error) { return w.ExposeEnemyOps(a, d) })
}

func stirRevolts(s session.Session, w *ctx) Result {
	return localAttack(s, w, "Stir Revolts", nil, false, func(a, d *game.Empire) (string, error) { return w.StirRevolts(a, d) })
}

// bombingAttack wraps localAttack with BRE's Bomb Enemy Targets 500-Bomber
// payload requirement (BRE.OVR: "All missiles and bombs require 500 Bombers
// to deliver their payloads").
func bombingAttack(s session.Session, w *ctx, label string, price costOf, strike func(a, d *game.Empire) (string, error)) Result {
	if w.Player().Bombers < game.BombingBombersRequired {
		fail(s, fmt.Errorf("you need at least %d Bombers to deliver a payload", game.BombingBombersRequired))
		return Stay
	}
	return localAttack(s, w, label, price, false, strike)
}

func bombFoodMarket(s session.Session, w *ctx) Result {
	return bombingAttack(s, w, "Bomb Food Market", nil, func(a, d *game.Empire) (string, error) { return w.BombFood(a, d) })
}

func bombTradingMarket(s session.Session, w *ctx) Result {
	return bombingAttack(s, w, "Bomb Trading Market", nil, func(a, d *game.Empire) (string, error) { return w.BombTradingMarket(a, d) })
}

func bombTradeRoutes(s session.Session, w *ctx) Result {
	return bombingAttack(s, w, "Bomb Trade Routes", nil, func(a, d *game.Empire) (string, error) { return w.BombTradeRoutes(a, d) })
}

func undermineInvestments(s session.Session, w *ctx) Result {
	return bombingAttack(s, w, "Undermine Investments", nil, func(a, d *game.Empire) (string, error) { return w.UndermineInvestments(a, d) })
}

func nuclearAssault(s session.Session, w *ctx) Result {
	return bombingAttack(s, w, "Nuclear Assault", nukePrice, func(a, d *game.Empire) (string, error) { return w.NuclearStrike(a, d) })
}

func chemicalBombing(s session.Session, w *ctx) Result {
	return bombingAttack(s, w, "Chemical Bombing", chemPrice, func(a, d *game.Empire) (string, error) { return w.ChemicalStrike(a, d) })
}

func slappenheimerStrike(s session.Session, w *ctx) Result {
	var mode game.SlappenheimerMode
	w.With(func() { mode = w.Config.SlappenheimerHandling })
	if mode == game.SlappenheimerNone {
		ok(s, "The R5-Slappenheimer is disabled.")
		return Stay
	}
	// Under User Select handling the player dials the missile in (0-10). The
	// dial is BRE's bluff — it changes nothing about the outcome — but we still
	// prompt for it to keep the original's feel.
	if mode == game.SlappenheimerUserSelect {
		promptInt(s, "Set the R5-Slappenheimer dial (0-10)")
	}
	return bombingAttack(s, w, "R5-Slappenheimer", nil, func(a, d *game.Empire) (string, error) { return w.SlappenheimerStrike(a, d) })
}

func spyRelations(s session.Session, w *ctx) Result {
	return localAttack(s, w, "Spy on Relations", nil, false, func(a, d *game.Empire) (string, error) { return w.SpyOnRelations(a, d) })
}

func briberyOp(s session.Session, w *ctx) Result {
	return localAttack(s, w, "Bribery", nil, false, func(a, d *game.Empire) (string, error) { return w.Bribery(a, d) })
}
