package menu

import (
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

func buyFoodMarket(s session.Session, w *ctx) Result {
	p := w.Player()
	maxBuy := game.UnitsAffordable(p.Gold, w.FoodBuyPrice())
	if !w.Config.FoodUnlimited && w.FoodMarketSupply < maxBuy {
		maxBuy = w.FoodMarketSupply // can't buy more than the market has today
	}
	// Default (Enter) to a meal's shortfall — what one obligation costs minus what
	// the realm has — the inverse of the Sell default, capped by what the player
	// can afford and the market has. See sellFoodMarket for why it does not care
	// whether this turn's obligations are already paid.
	suggested := min(max(0, w.FoodDue(p)-p.Food), maxBuy)
	n := promptSuggested(s, "How much food to buy?", suggested, maxBuy)
	if n <= 0 {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		return w.World.BuyFoodMarket(p, n) // re-checks gold atomically
	})
	if err != nil {
		fail(s, err)
	}
	// No confirmation/pause on success: the Food Market redraws with the updated
	// gold and food in its footer.
	return Stay
}

// sellFoodMarket holds one meal back from the Enter default, and does so
// whether or not the food stage has already taken this turn's — which is what
// the original does (trade_food, BRE.OVR 0x3792a+0x8c7: a plain
// `Food − (people_food_need + forces_food_need)` with no test for it) and what
// keeps a baron who empties his granary on purpose fed NEXT turn.
//
// Since feeding moved to the prompt this reads like a double count, because the
// reserved meal has been eaten already. It is not, and it was changed on that
// reading and reverted the same day (2026-09-17); docs/mechanics-reference.md
// records it under the food stage. The whole granary is still one keystroke
// away — the max is the second figure in the prompt.
func sellFoodMarket(s session.Session, w *ctx) Result {
	p := w.Player()
	suggested := max(0, p.Food-w.FoodDue(p))
	n := promptSuggested(s, "How much food to sell?", suggested, p.Food)
	if n <= 0 {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		return w.World.SellFood(p, n) // re-clamps to fresh food stock atomically
	})
	if err != nil {
		fail(s, err)
	}
	// No confirmation/pause on success: the Food Market redraws with the updated
	// gold and food in its footer.
	return Stay
}
