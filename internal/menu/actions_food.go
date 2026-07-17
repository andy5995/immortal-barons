package menu

import (
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

func buyFoodMarket(s session.Session, w *ctx) Result {
	p := w.Player()
	maxBuy := p.Gold / w.FoodBuyPrice()
	if !w.Config.FoodUnlimited && w.FoodMarketSupply < maxBuy {
		maxBuy = w.FoodMarketSupply // can't buy more than the market has today
	}
	n := promptSuggested(s, "How much food to buy?", 0, maxBuy)
	if n <= 0 {
		return Stay
	}
	err := w.withPlayer(func(p *game.Empire) error {
		return w.World.BuyFoodMarket(p, n) // re-checks gold atomically
	})
	if err != nil {
		fail(s, err)
	}
	// No confirmation/pause on success: the Food Market redraws with the updated
	// gold and food in its footer.
	return Stay
}

func sellFoodMarket(s session.Session, w *ctx) Result {
	p := w.Player()
	suggested := max(0, p.Food-w.FoodNeededNextTurn(p))
	n := promptSuggested(s, "How much food to sell?", suggested, p.Food)
	if n <= 0 {
		return Stay
	}
	err := w.withPlayer(func(p *game.Empire) error {
		return w.World.SellFood(p, n) // re-clamps to fresh food stock atomically
	})
	if err != nil {
		fail(s, err)
	}
	// No confirmation/pause on success: the Food Market redraws with the updated
	// gold and food in its footer.
	return Stay
}
