package game

import "errors"

var (
	ErrCantAfford = errors.New("You cannot afford that.")
	ErrNoBank     = errors.New("You do not have that much in the bank.")
	ErrNoDebt     = errors.New("You do not owe that much.")
	ErrNoAgents   = errors.New("You have no agents for that operation.")
	ErrHQExists   = errors.New("Your HeadQuarters is already under construction or built.")
	ErrRegionCap  = errors.New("You have reached your region purchase limit for this turn.")
)

// HQCost is the gold price to start HeadQuarters construction (see balance.go).

// StartHQ begins HeadQuarters construction (5% the first turn); it then
// advances during daily play. Errors if already started/built or unaffordable.
func (w *World) StartHQ(e *Empire) error {
	if e.HQ > 0 {
		return ErrHQExists
	}
	if e.Gold < HQCost {
		return ErrCantAfford
	}
	e.Gold -= HQCost
	e.HQ = 5
	return nil
}

// spend deducts gold for buying n units at unit cost, returning an error
// if the empire can't afford it. n <= 0 is a no-op.
func (e *Empire) spend(n, unit int) error {
	if n <= 0 {
		return nil
	}
	cost := n * unit
	if e.Gold < cost {
		return ErrCantAfford
	}
	e.Gold -= cost
	return nil
}

// LandPriceStep controls how fast land gets more expensive as an empire
// grows (v1 balance knob — tune freely). Each region you own raises the
// next region's price by Prices.Land/LandPriceStep.
const LandPriceStep = 50

// landUnitPrice is the base per-region price, scaled by the league's Region
// Costs knob (Medium = 100% = unchanged). LandPrice, MaxAffordableRegions, and
// BuyRegions all build the rising-price formula on top of it.
func (w *World) landUnitPrice() int {
	return w.Prices.Land * w.Config.RegionCosts.Percent() / 100
}

// regionBuyLimit is the most regions e may still buy this turn, from the
// league's Max Purchasable Regions knob minus what e has already bought
// since the turn began (Empire.RegionsBoughtThisTurn, which is reset to 0 at
// the start of each turn). The cap is cumulative across every purchase made
// during the turn, not just the single transaction in front of the player —
// re-entering the Spending menu does not refresh it. A knob of 0 means
// unlimited.
func (w *World) regionBuyLimit(e *Empire) int {
	if w.Config.MaxRegions <= 0 {
		return 1 << 30
	}
	remaining := w.Config.MaxRegions - e.RegionsBoughtThisTurn
	if remaining < 0 {
		return 0
	}
	return remaining
}

// LandPrice is the current gold cost of the next region for empire e.
func (w *World) LandPrice(e *Empire) int {
	base := w.landUnitPrice()
	return base + e.Land*base/LandPriceStep
}

// MaxAffordableRegions is the most regions e can buy at the current rising
// price, also bounded by the Max Purchasable Regions cap. Because each
// successive region costs more (see BuyRegions), a simple gold/price divide
// overcounts — this sums the real climbing cost.
func (w *World) MaxAffordableRegions(e *Empire) int {
	base := w.landUnitPrice()
	limit := w.regionBuyLimit(e)
	total := 0
	for n := 0; ; n++ {
		if n >= limit {
			return n
		}
		cost := base + (e.Land+n)*base/LandPriceStep
		if total+cost > e.Gold {
			return n
		}
		total += cost
	}
}

// BuyRegions buys n regions of the type pointed to by field (a pointer into
// e.Regions, e.g. &e.Regions.Coastal), using the same rising-price formula
// as before region types existed. All-or-nothing: either the whole
// purchase is affordable or nothing happens.
func (w *World) BuyRegions(e *Empire, field *int, n int) error {
	if n <= 0 {
		return nil
	}
	if n > w.regionBuyLimit(e) {
		return ErrRegionCap
	}
	base := w.landUnitPrice()
	total := 0
	for i := 0; i < n; i++ {
		total += base + (e.Land+i)*base/LandPriceStep
	}
	if e.Gold < total {
		return ErrCantAfford // must afford the whole purchase
	}
	e.Gold -= total
	*field += n
	e.RegionsBoughtThisTurn += n
	e.syncLand()
	return nil
}

// BuyLand is a thin wrapper over BuyRegions that buys Coastal regions, kept
// for callers that don't care about region type.
func (w *World) BuyLand(e *Empire, n int) error {
	return w.BuyRegions(e, &e.Regions.Coastal, n)
}

func (w *World) BuyFood(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Food); err != nil {
		return err
	}
	e.Food += n
	return nil
}

// Food market prices (FoodBuyPrice / FoodSellPrice) live in balance.go: the
// market sells food to you dearer than it buys.

// BuyFoodMarket buys n units of food at FoodBuyPrice. This is the canonical
// way to buy food; the Spending Menu's "Buy Food" item routes here too.
func (w *World) BuyFoodMarket(e *Empire, n int) error {
	if n <= 0 {
		return nil
	}
	cost := n * FoodBuyPrice
	if e.Gold < cost {
		return ErrCantAfford
	}
	e.Gold -= cost
	e.Food += n
	return nil
}

// SellFood sells n units of food at FoodSellPrice, clamped to what e owns.
func (w *World) SellFood(e *Empire, n int) error {
	if n <= 0 {
		return nil
	}
	if n > e.Food {
		n = e.Food
	}
	e.Food -= n
	e.Gold += n * FoodSellPrice
	return nil
}

// FoodNeededNextTurn estimates the empire's next-turn food consumption
// (so the sell prompt can suggest keeping enough on hand).
func (w *World) FoodNeededNextTurn(e *Empire) int {
	return e.People + e.Troopers + e.Jets*2 + e.Tanks*2
}

func (w *World) Recruit(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Trooper); err != nil {
		return err
	}
	e.Troopers += n
	return nil
}

func (w *World) BuildJets(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Jet); err != nil {
		return err
	}
	e.Jets += n
	return nil
}

func (w *World) BuildTurrets(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Turret); err != nil {
		return err
	}
	e.Turrets += n
	return nil
}

func (w *World) BuildCarriers(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Carrier); err != nil {
		return err
	}
	e.Carriers += n
	return nil
}

func (w *World) BuildTanks(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Tank); err != nil {
		return err
	}
	e.Tanks += n
	return nil
}

func (w *World) RecruitAgents(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Agent); err != nil {
		return err
	}
	e.Agents += n
	return nil
}

// sellUnit sells n of a unit back to the market for half its buy price,
// clamped to what's owned (*stock).
func sellUnit(stock *int, n, price int, e *Empire) error {
	if n <= 0 {
		return nil
	}
	if n > *stock {
		n = *stock
	}
	*stock -= n
	e.Gold += n * price / 2
	return nil
}

func (w *World) SellTroopers(e *Empire, n int) error {
	return sellUnit(&e.Troopers, n, w.Prices.Trooper, e)
}

func (w *World) SellJets(e *Empire, n int) error {
	return sellUnit(&e.Jets, n, w.Prices.Jet, e)
}

// BuildBombers buys n bombers directly (they can also be produced by Industrial
// regions). Old saves lacking a Bomber price default to it via NewWorld.
func (w *World) BuildBombers(e *Empire, n int) error {
	if err := e.spend(n, w.Prices.Bomber); err != nil {
		return err
	}
	e.Bombers += n
	return nil
}

func (w *World) SellBombers(e *Empire, n int) error {
	return sellUnit(&e.Bombers, n, w.Prices.Bomber, e)
}

func (w *World) SellTurrets(e *Empire, n int) error {
	return sellUnit(&e.Turrets, n, w.Prices.Turret, e)
}

func (w *World) SellTanks(e *Empire, n int) error {
	return sellUnit(&e.Tanks, n, w.Prices.Tank, e)
}

func (w *World) SellCarriers(e *Empire, n int) error {
	return sellUnit(&e.Carriers, n, w.Prices.Carrier, e)
}

func (w *World) SellAgents(e *Empire, n int) error {
	return sellUnit(&e.Agents, n, w.Prices.Agent, e)
}

// SellRegions returns regions of the type pointed to by field for half
// their current market price per region. n is clamped to *field.
func (w *World) SellRegions(e *Empire, field *int, n int) error {
	if n <= 0 {
		return nil
	}
	if n > *field {
		n = *field
	}
	for i := 0; i < n; i++ {
		*field--
		e.syncLand()
		e.Gold += w.LandPrice(e) / 2
	}
	return nil
}

// SellLand is a thin wrapper over SellRegions that sells Coastal regions,
// kept for callers that don't care about region type.
func (w *World) SellLand(e *Empire, n int) error {
	return w.SellRegions(e, &e.Regions.Coastal, n)
}

// DropRegions abandons n regions of the type pointed to by field. Unlike a
// sale, no gold is returned — BRE lets you drop regions, not sell them. n is
// clamped to *field.
func (w *World) DropRegions(e *Empire, field *int, n int) error {
	if n <= 0 {
		return nil
	}
	if n > *field {
		n = *field
	}
	*field -= n
	e.syncLand()
	return nil
}

func (w *World) Deposit(e *Empire, n int) error {
	if n <= 0 {
		return nil
	}
	if e.Gold < n {
		return ErrCantAfford
	}
	// Don't consume gold that won't fit under the money cap — only bank as
	// much as there is room for.
	if room := MoneyCap - e.Bank; n > room {
		n = room
	}
	if n <= 0 {
		return nil
	}
	e.Gold -= n
	e.Bank += n
	return nil
}

func (w *World) Withdraw(e *Empire, n int) error {
	if n <= 0 {
		return nil
	}
	if e.Bank < n {
		return ErrNoBank
	}
	e.Bank -= n
	e.Gold += n
	return nil
}

// Loan borrows gold, added to the debt that accrues interest each turn.
func (w *World) Loan(e *Empire, n int) error {
	if n <= 0 {
		return nil
	}
	e.Gold += n
	e.Debt += n
	return nil
}

func (w *World) Repay(e *Empire, n int) error {
	if n <= 0 {
		return nil
	}
	if n > e.Debt {
		return ErrNoDebt
	}
	if e.Gold < n {
		return ErrCantAfford
	}
	e.Gold -= n
	e.Debt -= n
	return nil
}
