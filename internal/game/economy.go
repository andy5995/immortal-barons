package game

import (
	"encoding/binary"
	"errors"
	"hash/fnv"
	"io"
)

var (
	ErrCantAfford   = errors.New("You cannot afford that.")
	ErrNoBank       = errors.New("You do not have that much in the bank.")
	ErrNoDebt       = errors.New("You do not owe that much.")
	ErrNoAgents     = errors.New("You have no agents for that operation.")
	ErrHQExists     = errors.New("Your HeadQuarters is already under construction or built.")
	ErrRegionCap    = errors.New("You have reached your region purchase limit for this turn.")
	ErrNoFoodSupply = errors.New("The food market is out of food for today.")
	ErrNoLand       = errors.New("There is no unclaimed land left on the planet today.")
)

// FoodBuyPrice is today's price to buy one unit of food, varying planet-wide
// within [FoodBuyPriceMin, 3×FoodBuyPriceMin] — BRE's [20,60] band (IB is
// BRE-native scale). Deterministic per game-day, so it holds for the whole day.
func (w *World) FoodBuyPrice() int {
	h := fnv.New32a()
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(w.GameDay))
	h.Write(buf[:])
	io.WriteString(h, "foodprice")
	span := uint32(2*FoodBuyPriceMin + 1) // range min .. 3×min
	return FoodBuyPriceMin + int(h.Sum32()%span)
}

// FoodSellPrice is today's price the market pays per unit sold: buy/3 (BRE's
// sell ≈ buy/3, band ~[7,20]).
func (w *World) FoodSellPrice() int { return w.FoodBuyPrice() / 3 }

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
	e.HQ = HQBuildStart
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

// regionCost is the gold cost of the next region when the empire already owns
// `owned` regions: BRE's rising price ≈ Prices.Land + owned×LandPerRegion
// (≈ 917 + owned×33, live-sampled), scaled by the league's Region Costs knob
// (Medium = 100% = unchanged). LandPrice, MaxAffordableRegions, and BuyRegions
// all build on it.
func (w *World) regionCost(owned int) int {
	return (w.Prices.Land + owned*LandPerRegion) * w.Config.RegionCosts.Percent() / 100
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
	return w.regionCost(e.Land)
}

// MaxAffordableRegions is the most regions e can buy at the current rising
// price, also bounded by the Max Purchasable Regions cap. Because each
// successive region costs more (see BuyRegions), a simple gold/price divide
// overcounts — this sums the real climbing cost.
func (w *World) MaxAffordableRegions(e *Empire) int {
	limit := w.regionBuyLimit(e)
	total := 0
	for n := 0; ; n++ {
		if n >= limit {
			return n
		}
		cost := w.regionCost(e.Land + n)
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
	if n > e.LandAvailable {
		return ErrNoLand // this realm's Daily Land Creation allowance is used up
	}
	total := 0
	for i := 0; i < n; i++ {
		total += w.regionCost(e.Land + i)
	}
	if e.Gold < total {
		return ErrCantAfford // must afford the whole purchase
	}
	e.Gold -= total
	*field += n
	e.RegionsBoughtThisTurn += n
	e.LandAvailable -= n
	e.syncLand()
	return nil
}

// GrantRegions adds n regions to a specific field of e's mix and resyncs Land.
// It is the no-cost analogue of BuyRegions, for land AWARDED rather than bought:
// a winning attacker allocating the regions captured in a Regular Attack (#58).
// field must point into e.Regions (use the menu's regionField). Unlike a
// purchase it ignores the per-turn buy cap and gold.
func (w *World) GrantRegions(e *Empire, field *int, n int) {
	if n <= 0 {
		return
	}
	*field += n
	e.syncLand()
}

// BuyLand is a thin wrapper over BuyRegions that buys Coastal regions, kept
// for callers that don't care about region type.
func (w *World) BuyLand(e *Empire, n int) error {
	return w.BuyRegions(e, &e.Regions.Coastal, n)
}

// Food market prices (FoodBuyPrice / FoodSellPrice) live in balance.go: the
// market sells food to you dearer than it buys.

// BuyFoodMarket buys n units of food at today's FoodBuyPrice. Unless the sysop's
// Food Unlimited toggle is on, purchases draw from the shared planet-wide daily
// pool (FoodMarketSupply) and are clamped to what remains. This is the canonical
// way to buy food; the Spending Menu's "Buy Food" item routes here too.
func (w *World) BuyFoodMarket(e *Empire, n int) error {
	if n <= 0 {
		return nil
	}
	if !w.Config.FoodUnlimited {
		if w.FoodMarketSupply <= 0 {
			return ErrNoFoodSupply
		}
		if n > w.FoodMarketSupply {
			n = w.FoodMarketSupply // buy what's left today
		}
	}
	cost := n * w.FoodBuyPrice()
	if e.Gold < cost {
		return ErrCantAfford
	}
	e.Gold -= cost
	e.Food += n
	if !w.Config.FoodUnlimited {
		w.FoodMarketSupply -= n
	}
	return nil
}

// SellFood sells n units of food at today's FoodSellPrice, clamped to what e
// owns. In limited mode the sold food goes back into the planet-wide pool.
func (w *World) SellFood(e *Empire, n int) error {
	if n <= 0 {
		return nil
	}
	if n > e.Food {
		n = e.Food
	}
	e.Food -= n
	e.Gold += n * w.FoodSellPrice()
	if !w.Config.FoodUnlimited {
		w.FoodMarketSupply += n
	}
	return nil
}

// FoodNeededNextTurn estimates the empire's next-turn food consumption
// (so the sell prompt can suggest keeping enough on hand). Same figure the
// turn engine consumes — FoodUpkeep — kept as one formula so they can't drift.
func (w *World) FoodNeededNextTurn(e *Empire) int {
	return e.FoodUpkeep()
}

// curPrice returns e's stored walk price for a unit, or the world base when the
// walk hasn't seeded it yet (stored==0: a fresh empire's first turn, or a save
// from before per-empire prices existed).
func curPrice(stored, base int) int {
	if stored <= 0 {
		return base
	}
	return stored
}

// Current per-turn buy prices for each unit: the empire's own stored walk value
// (World.stepPrices advances it once per turn), falling back to the world base
// until seeded. Both the Spending-menu display and the charge/sell paths call
// these, so shown == charged within a turn; each empire sees its own prices.
// Regions have no equivalent — their price rises with holdings (see regionCost)
// but takes no per-turn walk.
func (w *World) TrooperPrice(e *Empire) int { return curPrice(e.Prices.Trooper, w.Prices.Trooper) }
func (w *World) JetPrice(e *Empire) int     { return curPrice(e.Prices.Jet, w.Prices.Jet) }
func (w *World) TurretPrice(e *Empire) int  { return curPrice(e.Prices.Turret, w.Prices.Turret) }
func (w *World) TankPrice(e *Empire) int    { return curPrice(e.Prices.Tank, w.Prices.Tank) }
func (w *World) BomberPrice(e *Empire) int  { return curPrice(e.Prices.Bomber, w.Prices.Bomber) }
func (w *World) CarrierPrice(e *Empire) int { return curPrice(e.Prices.Carrier, w.Prices.Carrier) }
func (w *World) AgentPrice(e *Empire) int   { return curPrice(e.Prices.Agent, w.Prices.Agent) }

// stepPrice advances one stored per-empire price one walk step: it moves by up to
// stepPct% of the base (deterministic, keyed per empire+turn like riversFish) and
// is clamped to ±PriceWalkBandPct% of the base so the walk drifts but can't run
// away. A zero `stored` seeds from base first (fresh empire / pre-feature save).
func (w *World) stepPrice(e *Empire, stored, base, stepPct int, tag string) int {
	if base <= 0 {
		return stored
	}
	if stored <= 0 {
		stored = base
	}
	if stepMax := base * stepPct / 100; stepMax > 0 {
		h := fnv.New32a()
		var buf [8]byte
		binary.LittleEndian.PutUint32(buf[0:4], uint32(w.GameDay))
		binary.LittleEndian.PutUint32(buf[4:8], uint32(e.TurnsLeft))
		h.Write(buf[:])
		io.WriteString(h, tag)
		io.WriteString(h, e.Name)
		stored += int(h.Sum32()%uint32(2*stepMax+1)) - stepMax
	}
	band := base * PriceWalkBandPct / 100
	if stored > base+band {
		stored = base + band
	}
	if stored < base-band {
		stored = base - band
	}
	if stored < 1 {
		stored = 1
	}
	return stored
}

// stepPrices advances every per-empire unit price one walk step. Called once per
// turn from PlayTurn (after the turn's buys), so a price is stable during a turn
// (shown == charged) and drifts turn to turn, persisting across days via the save.
func (w *World) stepPrices(e *Empire) {
	e.Prices.Trooper = w.stepPrice(e, e.Prices.Trooper, w.Prices.Trooper, PriceWalkStepTrooper, "trooper")
	e.Prices.Jet = w.stepPrice(e, e.Prices.Jet, w.Prices.Jet, PriceWalkStepJet, "jet")
	e.Prices.Turret = w.stepPrice(e, e.Prices.Turret, w.Prices.Turret, PriceWalkStepTurret, "turret")
	e.Prices.Tank = w.stepPrice(e, e.Prices.Tank, w.Prices.Tank, PriceWalkStepTank, "tank")
	e.Prices.Bomber = w.stepPrice(e, e.Prices.Bomber, w.Prices.Bomber, PriceWalkStepBomber, "bomber")
	e.Prices.Carrier = w.stepPrice(e, e.Prices.Carrier, w.Prices.Carrier, PriceWalkStepCarrier, "carrier")
	e.Prices.Agent = w.stepPrice(e, e.Prices.Agent, w.Prices.Agent, PriceWalkStepAgent, "agent")
}

func (w *World) Recruit(e *Empire, n int) error {
	if err := e.spend(n, w.TrooperPrice(e)); err != nil {
		return err
	}
	e.Troopers += n
	return nil
}

func (w *World) BuildJets(e *Empire, n int) error {
	if err := e.spend(n, w.JetPrice(e)); err != nil {
		return err
	}
	e.Jets += n
	return nil
}

func (w *World) BuildTurrets(e *Empire, n int) error {
	if err := e.spend(n, w.TurretPrice(e)); err != nil {
		return err
	}
	e.Turrets += n
	return nil
}

func (w *World) BuildCarriers(e *Empire, n int) error {
	if err := e.spend(n, w.CarrierPrice(e)); err != nil {
		return err
	}
	e.Carriers += n
	return nil
}

func (w *World) BuildTanks(e *Empire, n int) error {
	if err := e.spend(n, w.TankPrice(e)); err != nil {
		return err
	}
	e.Tanks += n
	return nil
}

func (w *World) RecruitAgents(e *Empire, n int) error {
	if err := e.spend(n, w.AgentPrice(e)); err != nil {
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
	e.Gold += n * price / 3 // BRE: sell price is buy/3
	return nil
}

func (w *World) SellTroopers(e *Empire, n int) error {
	return sellUnit(&e.Troopers, n, w.TrooperPrice(e), e)
}

func (w *World) SellJets(e *Empire, n int) error {
	return sellUnit(&e.Jets, n, w.JetPrice(e), e)
}

// BuildBombers buys n bombers directly (they can also be produced by Industrial
// regions). Old saves lacking a Bomber price default to it via NewWorld.
func (w *World) BuildBombers(e *Empire, n int) error {
	if err := e.spend(n, w.BomberPrice(e)); err != nil {
		return err
	}
	e.Bombers += n
	return nil
}

func (w *World) SellBombers(e *Empire, n int) error {
	return sellUnit(&e.Bombers, n, w.BomberPrice(e), e)
}

func (w *World) SellTurrets(e *Empire, n int) error {
	return sellUnit(&e.Turrets, n, w.TurretPrice(e), e)
}

func (w *World) SellTanks(e *Empire, n int) error {
	return sellUnit(&e.Tanks, n, w.TankPrice(e), e)
}

func (w *World) SellCarriers(e *Empire, n int) error {
	return sellUnit(&e.Carriers, n, w.CarrierPrice(e), e)
}

func (w *World) SellAgents(e *Empire, n int) error {
	// BRE sells agents at a flat SellAgentPrice, not buy/3 like other units.
	if n <= 0 {
		return nil
	}
	if n > e.Agents {
		n = e.Agents
	}
	e.Agents -= n
	e.Gold += n * SellAgentPrice
	return nil
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
