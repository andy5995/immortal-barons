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

// HQPrice is the gold it costs e to start a HeadQuarters right now. Unlike the
// unit prices, which walk around a fixed base, this climbs with the empire's
// lifetime turn count — the constants and the shape are BRE's (see balance.go).
// The jitter is keyed per empire and turn, like stepPrice, so the price a player
// is shown is the price they are charged.
func (w *World) HQPrice(e *Empire) int {
	price := HQPriceBase + HQPricePerTurn*e.TurnsPlayed + w.priceJitter(e, "hq", HQPriceJitter)
	if cap := HQPriceCap - w.priceJitter(e, "hqcap", HQPriceCapJitter); price > cap {
		price = cap
	}
	return price
}

// priceJitter is a deterministic draw in [0, n) for empire e this turn.
func (w *World) priceJitter(e *Empire, tag string, n int) int {
	if n <= 0 {
		return 0
	}
	h := fnv.New32a()
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:4], uint32(w.GameDay))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(e.TurnsPlayed))
	h.Write(buf[:])
	io.WriteString(h, tag)
	io.WriteString(h, e.Name)
	return int(h.Sum32() % uint32(n))
}

// StartHQ begins HeadQuarters construction (HQBuildStart% the first turn); it
// then advances during daily play. Errors if already started/built or
// unaffordable.
func (w *World) StartHQ(e *Empire) error {
	if e.HQ > 0 {
		return ErrHQExists
	}
	price := int64(w.HQPrice(e))
	if e.Gold < price {
		return ErrCantAfford
	}
	e.Gold -= price
	e.HQ = HQBuildStart
	return nil
}

// spend deducts gold for buying n units at unit cost, returning an error
// if the empire can't afford it. n <= 0 is a no-op.
func (e *Empire) spend(n, unit int) error {
	if n <= 0 {
		return nil
	}
	cost := goldCost(n, unit)
	if e.Gold < cost {
		return ErrCantAfford
	}
	e.Gold -= cost
	return nil
}

// regionCost is the gold cost of the next region when the empire already owns
// `owned` regions: Prices.Land + owned × the per-region climb. LandPrice,
// MaxAffordableRegions and BuyRegions all build on it.
//
// The climb is where the sysop's Region Cost Change setting lands, and it does
// NOT scale the price. BINARY-VERIFIED at BRE.OVR 0x3019C, which selects a value
// from config byte +0x185 (None 0, Low 15, Medium 35, High 55), multiplies it by
// a FLAG — one when the realm owns RegionCostSurchargeAt regions or more, zero
// below — and adds LandPerRegion:
//
//	climb = LandPerRegion + (owned >= 300 ? levelValue : 0)
//
// So the knob is a big-realm surcharge, inert until a realm passes 300 regions
// and then steep: at Medium the climb goes 33 -> 68, at High 33 -> 88. It also
// explains why live sampling put the climb at a flat 33 — every realm sampled
// was under the threshold, so the knob had not engaged.
//
// IB used to multiply the WHOLE price by a percentage of the level instead,
// which is the wrong shape and hits small realms the original never touches.
func (w *World) regionCost(owned int) int {
	return w.Prices.Land + owned*w.regionClimb(owned)
}

// regionClimb is the per-region price step at the given size — see regionCost.
func (w *World) regionClimb(owned int) int {
	if owned < RegionCostSurchargeAt {
		return LandPerRegion
	}
	return LandPerRegion + w.Config.RegionCosts.RegionCostSurcharge()
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
	var total int64
	for n := 0; ; n++ {
		if n >= limit {
			return n
		}
		cost := int64(w.regionCost(e.Land + n))
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
	var total int64
	for i := 0; i < n; i++ {
		total += int64(w.regionCost(e.Land + i))
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
// market sells food to you for more than it pays to buy it back.

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
	cost := goldCost(n, w.FoodBuyPrice())
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
	w.creditGold(e, goldCost(n, w.FoodSellPrice()), "a food sale")
	if !w.Config.FoodUnlimited {
		w.FoodMarketSupply += n
	}
	return nil
}

// FoodNeededNextTurn estimates the empire's next-turn food consumption
// (so the sell prompt can suggest keeping enough on hand). Same figure the
// turn engine consumes — FoodDue — kept as one formula so they can't drift.
func (w *World) FoodNeededNextTurn(e *Empire) int {
	return w.FoodDue(e)
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

// AgentPrice is what a covert agent costs e right now. Agents take no walk: like
// the HeadQuarters price this climbs with the empire's lifetime turn count, but
// with no cap, so a long-lived realm pays steadily more to recruit (balance.go).
func (w *World) AgentPrice(e *Empire) int {
	return AgentPriceBase + AgentPricePerTurn*e.TurnsPlayed + w.priceJitter(e, "agent", AgentPriceJitter)
}

// midPrice is a band's centre — the point BRE's walk reverts towards.
func midPrice(lo, hi int) int { return (lo + hi) / 2 }

// walkRoll is a deterministic draw in [0, n) for the k-th random number BRE's
// price walk needs for one unit this turn. Keyed per empire and turn (like
// priceJitter and riversFish) so the walk is reproducible and needs no shared RNG.
func (w *World) walkRoll(e *Empire, tag string, k, n int) int {
	if n <= 0 {
		return 0
	}
	h := fnv.New32a()
	var buf [12]byte
	binary.LittleEndian.PutUint32(buf[0:4], uint32(w.GameDay))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(e.TurnsLeft))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(k))
	h.Write(buf[:])
	io.WriteString(h, tag)
	io.WriteString(h, e.Name)
	return int(h.Sum32() % uint32(n))
}

// stepPrice advances one stored per-empire price by one turn of BRE's walk (the
// exact shape is in balance.go). It is mean-reverting: the move away from the
// band's midpoint is divided by 1..PriceWalkDampMax while the move back is not,
// so a price wanders but keeps being pulled towards the middle. A zero `stored`
// seeds from the band floor the way BRE's own floor clamp does (fresh empire, or
// a save from before per-empire prices existed).
func (w *World) stepPrice(e *Empire, stored, lo, hi, step int, tag string) int {
	if stored <= 0 {
		stored = lo + w.walkRoll(e, tag, 0, PriceFloorJitter)
	}
	mid := midPrice(lo, hi)
	up, down := 1, 1
	if stored < mid {
		down = w.walkRoll(e, tag, 1, PriceWalkDampMax) + 1
	}
	if stored > mid {
		up = w.walkRoll(e, tag, 2, PriceWalkDampMax) + 1
	}
	stored += w.walkRoll(e, tag, 3, step) / up
	stored -= w.walkRoll(e, tag, 4, step) / down
	if stored < lo {
		stored = lo + w.walkRoll(e, tag, 5, PriceFloorJitter)
	}
	if stored > hi {
		stored = hi - w.walkRoll(e, tag, 6, PriceCeilJitter)
	}
	return stored
}

// stepPrices advances every per-empire unit price one walk step. Called once per
// turn from PlayTurn (after the turn's buys), so a price is stable during a turn
// (shown == charged) and drifts turn to turn, persisting across days via the save.
func (w *World) stepPrices(e *Empire) {
	e.Prices.Trooper = w.stepPrice(e, e.Prices.Trooper, PriceLoTrooper, PriceHiTrooper, PriceStepTrooper, "trooper")
	e.Prices.Jet = w.stepPrice(e, e.Prices.Jet, PriceLoJet, PriceHiJet, PriceStepJet, "jet")
	e.Prices.Turret = w.stepPrice(e, e.Prices.Turret, PriceLoTurret, PriceHiTurret, PriceStepTurret, "turret")
	e.Prices.Tank = w.stepPrice(e, e.Prices.Tank, PriceLoTank, PriceHiTank, PriceStepTank, "tank")
	e.Prices.Bomber = w.stepPrice(e, e.Prices.Bomber, PriceLoBomber, PriceHiBomber, PriceStepBomber, "bomber")
	e.Prices.Carrier = w.stepPrice(e, e.Prices.Carrier, PriceLoCarrier, PriceHiCarrier, PriceStepCarrier, "carrier")
}

// buyUnit buys n of a unit at its current price, the mirror of sellUnit: spend
// (which validates n and affordability), then credit the stock.
func buyUnit(stock *int, n, price int, e *Empire) error {
	if err := e.spend(n, price); err != nil {
		return err
	}
	*stock += n
	return nil
}

func (w *World) Recruit(e *Empire, n int) error {
	return buyUnit(&e.Troopers, n, w.TrooperPrice(e), e)
}

func (w *World) BuildJets(e *Empire, n int) error {
	return buyUnit(&e.Jets, n, w.JetPrice(e), e)
}

func (w *World) BuildTurrets(e *Empire, n int) error {
	return buyUnit(&e.Turrets, n, w.TurretPrice(e), e)
}

func (w *World) BuildCarriers(e *Empire, n int) error {
	return buyUnit(&e.Carriers, n, w.CarrierPrice(e), e)
}

func (w *World) BuildTanks(e *Empire, n int) error {
	return buyUnit(&e.Tanks, n, w.TankPrice(e), e)
}

func (w *World) RecruitAgents(e *Empire, n int) error {
	return buyUnit(&e.Agents, n, w.AgentPrice(e), e)
}

// sellUnit sells n of a unit back to the market for a third of its buy price
// (BRE's rate), clamped to what's owned (*stock).
func (w *World) sellUnit(stock *int, n, price int, e *Empire) error {
	if n <= 0 {
		return nil
	}
	if n > *stock {
		n = *stock
	}
	*stock -= n
	w.creditGold(e, goldCost(n, price)/3, "a unit sale") // BRE: sell price is buy/3
	return nil
}

func (w *World) SellTroopers(e *Empire, n int) error {
	return w.sellUnit(&e.Troopers, n, w.TrooperPrice(e), e)
}

func (w *World) SellJets(e *Empire, n int) error {
	return w.sellUnit(&e.Jets, n, w.JetPrice(e), e)
}

// BuildBombers buys n bombers directly (they can also be produced by Industrial
// regions). Old saves lacking a Bomber price default to it via NewWorld.
func (w *World) BuildBombers(e *Empire, n int) error {
	return buyUnit(&e.Bombers, n, w.BomberPrice(e), e)
}

func (w *World) SellBombers(e *Empire, n int) error {
	return w.sellUnit(&e.Bombers, n, w.BomberPrice(e), e)
}

func (w *World) SellTurrets(e *Empire, n int) error {
	return w.sellUnit(&e.Turrets, n, w.TurretPrice(e), e)
}

func (w *World) SellTanks(e *Empire, n int) error {
	return w.sellUnit(&e.Tanks, n, w.TankPrice(e), e)
}

func (w *World) SellCarriers(e *Empire, n int) error {
	return w.sellUnit(&e.Carriers, n, w.CarrierPrice(e), e)
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
	w.creditGold(e, goldCost(n, SellAgentPrice), "an agent sale")
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
		w.creditGold(e, int64(w.LandPrice(e)/2), "a region sale")
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

func (w *World) Deposit(e *Empire, n int64) error {
	if n <= 0 {
		return nil
	}
	if e.Gold < n {
		return ErrCantAfford
	}
	// Don't consume gold that won't fit under the money cap — only bank as
	// much as there is room for.
	if room := w.MoneyCap() - e.Bank; n > room {
		n = room
	}
	if n <= 0 {
		return nil
	}
	e.Gold -= n
	e.Bank += n
	return nil
}

func (w *World) Withdraw(e *Empire, n int64) error {
	if n <= 0 {
		return nil
	}
	if e.Bank < n {
		return ErrNoBank
	}
	// Mirror Deposit: don't draw savings that won't fit under the money cap —
	// take only as much as there is room for in hand, and leave the rest banked.
	if room := w.MoneyCap() - e.Gold; n > room {
		n = room
	}
	if n <= 0 {
		return nil
	}
	e.Bank -= n
	e.Gold += n
	return nil
}

func (w *World) Repay(e *Empire, n int64) error {
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
