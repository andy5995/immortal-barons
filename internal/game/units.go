package game

// The canonical table of the things a realm holds by the unit and trades (#134).
//
// Before this, every screen, switch and report that needed the set wrote its own
// literal list — a dozen of them, in several orders and in two naming forms —
// and three of those lists were coupled BY POSITION to a list in another file,
// so reordering one half silently mislabelled the other. Each row is declared
// once here and referred to by its variable, which is what makes the coupling a
// compile error rather than a wrong label.
//
// The names are identity keys as well as labels: Empire.Specialized stores a
// plural and is compared with ==, so it is in the save file, and a market
// listing stores a singular. Both forms therefore stay ENGLISH here, and
// translation happens at render time through tr(). Screens keep their own
// orders (BRE keys the Trading Market and Buy Military differently) by
// declaring a slice of these rows, never by restating the set.

// Good is one tradeable holding: its two names, the accessors for an Empire's
// count and production, and the balance figures that go with it.
type Good struct {
	// Singular is the market's name for it ("Trooper") and Plural the name
	// every other screen shows ("Troopers"). Both are stored in save files —
	// see the package comment above.
	Singular string
	Plural   string
	// Count points at e's holding of it. Every good has one.
	Count func(e *Empire) *int
	// Prod and Made are the production percentage the player sets and the tally
	// of what was built last turn. Both are nil for a good Industrial regions
	// cannot build (agents and food).
	Prod func(e *Empire) *int
	Made func(e *Empire) *int
	// Basket points at a trade deal's amount of it. Every tradeable good has
	// one; gold is the exception the trade code handles beside the loop, since
	// it alone is held in money width.
	Basket func(b *TradeBasket) *int
	// Price is what the shop charges for one, and is nil for a good the shop
	// does not sell at a per-unit price.
	Price func(w *World, e *Empire) int
	// Cost is the production cost in points, and NetWorth the good's weight in
	// net worth, in thousandths. Both are 0 where they do not apply; the
	// figures themselves live in balance.go.
	Cost     int
	NetWorth int64
	// Military marks a fighting unit — the six Industrial regions build — as
	// against agents and food.
	Military bool
}

// The rows. Referred to by variable everywhere, so a screen's order cannot
// drift out of step with the field it labels.
var (
	Trooper = &Good{
		Singular: "Trooper", Plural: "Troopers", Military: true,
		Count:  func(e *Empire) *int { return &e.Troopers },
		Prod:   func(e *Empire) *int { return &e.ProdTroopers },
		Made:   func(e *Empire) *int { return &e.MadeTroopers },
		Basket: func(b *TradeBasket) *int { return &b.Troopers },
		Price:  func(w *World, e *Empire) int { return w.TrooperPrice(e) },
		Cost:   CostTrooper, NetWorth: NetWorthTrooper,
	}
	Jet = &Good{
		Singular: "Jet", Plural: "Jets", Military: true,
		Count:  func(e *Empire) *int { return &e.Jets },
		Prod:   func(e *Empire) *int { return &e.ProdJets },
		Made:   func(e *Empire) *int { return &e.MadeJets },
		Basket: func(b *TradeBasket) *int { return &b.Jets },
		Price:  func(w *World, e *Empire) int { return w.JetPrice(e) },
		Cost:   CostJet, NetWorth: NetWorthJet,
	}
	Turret = &Good{
		Singular: "Turret", Plural: "Turrets", Military: true,
		Count:  func(e *Empire) *int { return &e.Turrets },
		Prod:   func(e *Empire) *int { return &e.ProdTurrets },
		Made:   func(e *Empire) *int { return &e.MadeTurrets },
		Basket: func(b *TradeBasket) *int { return &b.Turrets },
		Price:  func(w *World, e *Empire) int { return w.TurretPrice(e) },
		Cost:   CostTurret, NetWorth: NetWorthTurret,
	}
	Bomber = &Good{
		Singular: "Bomber", Plural: "Bombers", Military: true,
		Count:  func(e *Empire) *int { return &e.Bombers },
		Prod:   func(e *Empire) *int { return &e.ProdBombers },
		Made:   func(e *Empire) *int { return &e.MadeBombers },
		Basket: func(b *TradeBasket) *int { return &b.Bombers },
		Price:  func(w *World, e *Empire) int { return w.BomberPrice(e) },
		Cost:   CostBomber, NetWorth: NetWorthBomber,
	}
	Tank = &Good{
		Singular: "Tank", Plural: "Tanks", Military: true,
		Count:  func(e *Empire) *int { return &e.Tanks },
		Prod:   func(e *Empire) *int { return &e.ProdTanks },
		Made:   func(e *Empire) *int { return &e.MadeTanks },
		Basket: func(b *TradeBasket) *int { return &b.Tanks },
		Price:  func(w *World, e *Empire) int { return w.TankPrice(e) },
		Cost:   CostTank, NetWorth: NetWorthTank,
	}
	Carrier = &Good{
		Singular: "Carrier", Plural: "Carriers", Military: true,
		Count:  func(e *Empire) *int { return &e.Carriers },
		Prod:   func(e *Empire) *int { return &e.ProdCarriers },
		Made:   func(e *Empire) *int { return &e.MadeCarriers },
		Basket: func(b *TradeBasket) *int { return &b.Carriers },
		Price:  func(w *World, e *Empire) int { return w.CarrierPrice(e) },
		Cost:   CostCarrier, NetWorth: NetWorthCarrier,
	}
	Agent = &Good{
		Singular: "Agent", Plural: "Agents",
		Count:    func(e *Empire) *int { return &e.Agents },
		Basket:   func(b *TradeBasket) *int { return &b.Agents },
		Price:    func(w *World, e *Empire) int { return w.AgentPrice(e) },
		NetWorth: NetWorthAgent,
	}
	Food = &Good{
		Singular: "Food", Plural: "Food",
		Count:  func(e *Empire) *int { return &e.Food },
		Basket: func(b *TradeBasket) *int { return &b.Food },
		Price:  func(w *World, e *Empire) int { return w.FoodBuyPrice() },
	}
)

// MilitaryGoods are the six units Industrial regions build, in the order the
// Set Industries screen lists them. ProjectedProduction and Manufacture walk
// this, so the projection's index i is this slice's index i.
var MilitaryGoods = []*Good{Trooper, Jet, Turret, Bomber, Tank, Carrier}

// AllGoods is every row, for a caller that wants the whole set rather than a
// screen's order — net worth, the R5-Slappenheimer's target list, a name
// lookup.
var AllGoods = []*Good{Trooper, Jet, Turret, Bomber, Tank, Carrier, Agent, Food}

// MarketGoods are the goods tradeable on the general Trading Market: the six
// units plus food and agents (BRE-verified live, 2026-07-15). Regions and
// HeadQuarters are not tradeable. The order is BRE's Trading Market screen,
// which is its own — hence a slice of rows rather than a re-use of AllGoods.
var MarketGoods = []*Good{Trooper, Jet, Turret, Bomber, Food, Agent, Tank, Carrier}

// GoodByName resolves either naming form — the market's singular or the
// display plural — to its row, or nil. A name read out of a save file or typed
// at a prompt comes through here rather than through a switch of its own.
func GoodByName(name string) *Good {
	for _, g := range AllGoods {
		if g.Singular == name || g.Plural == name {
			return g
		}
	}
	return nil
}

// MarketGoodNames is the Trading Market's goods as the singular names a
// listing stores, in screen order.
func MarketGoodNames() []string {
	names := make([]string, len(MarketGoods))
	for i, g := range MarketGoods {
		names[i] = g.Singular
	}
	return names
}

// marketField returns a pointer to e's inventory count for a market good, or
// nil if the good is not tradeable.
func marketField(e *Empire, good string) *int {
	for _, g := range MarketGoods {
		if g.Singular == good {
			return g.Count(e)
		}
	}
	return nil
}

// GoodCount is e's holding of a good by name, or -1 when the name is not one.
// The front-ends read counts through this rather than through a switch of their
// own in the menu package.
func GoodCount(e *Empire, good string) int {
	if g := GoodByName(good); g != nil {
		return *g.Count(e)
	}
	return -1
}
