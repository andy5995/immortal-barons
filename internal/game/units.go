package game

// The canonical table of the things a realm holds by the unit and trades (#134).
//
// Before this, every screen, switch and report that needed the set wrote its own
// literal list — a dozen of them, in several orders and in two naming forms —
// and three of those lists were coupled BY POSITION to a list in another file,
// so reordering one half silently mislabeled the other. Each row is declared
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
	// ID names the good in a mod's values file ("trooper"). Like the two
	// names below it is an identity key, so it stays English and never changes.
	ID string
	// Singular is the market's name for it ("Trooper") and Plural the name
	// every other screen shows ("Troopers"). Both are stored in save files —
	// see the package comment above.
	Singular string
	Plural   string
	// Counted names a number of it in a report, {n} the count: "{n} Trooper",
	// "{n} Troopers". The original's reports agree the count — a capture has
	// "1 Trooper attacked!" (eots-ibbs-03.cap) — and a catalog gives each count
	// its own form (#297).
	Counted forms
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
	// Force points at an attack detachment's number of it. Nil for a good an
	// interplanetary attack cannot carry (turrets, carriers, agents, food).
	Force func(f *AttackForce) *int
	// Price is a good whose price follows a rule of its own rather than the
	// walk — the covert agent's, which climbs with the realm's age. Nil for the
	// six walk units (see Stored) and for a good the shop does not sell.
	Price func(w *World, e *Empire) int
	// Stored points at the good's slot in a Prices set — the per-empire walk
	// value stepPrices advances once a turn, and the world's own base behind it.
	// Nil for a good whose price takes no walk (agents, food, regions).
	Stored func(p *Prices) *int
	// WalkTag keys the draw that walks the price (the band and step are in
	// Rules), so it is part of the save's arithmetic and must not be renamed:
	// change it and every realm's prices jump (#210, draw.go).
	WalkTag string
	// ShipWeight is the good's weight in a trade deal's cost formula, in
	// hundredths (shipWeightScale), and CarrierPer how many of it one carrier
	// holds. Both are BINARY-VERIFIED; a CarrierPer of 0 means the good needs no
	// carrier space at all, which is true of food, bombers and agents — and of
	// carriers themselves, which carry each other. See TradeDealCarriers and
	// TradeOfferCost.
	ShipWeight int
	CarrierPer int
	// Military marks a fighting unit — the six Industrial regions build — as
	// against agents and food.
	Military bool
}

// The rows. Referred to by variable everywhere, so a screen's order cannot
// drift out of step with the field it labels.
var (
	Trooper = &Good{
		ID: "trooper", Singular: "Trooper", Plural: "Troopers", Counted: msgidN("{n} Trooper", "{n} Troopers"), Military: true,
		Count:      func(e *Empire) *int { return &e.Troopers },
		Prod:       func(e *Empire) *int { return &e.ProdTroopers },
		Made:       func(e *Empire) *int { return &e.MadeTroopers },
		Basket:     func(b *TradeBasket) *int { return &b.Troopers },
		Force:      func(f *AttackForce) *int { return &f.Troopers },
		Stored:     func(p *Prices) *int { return &p.Trooper },
		WalkTag:    "trooper",
		ShipWeight: 100, CarrierPer: 1000,
	}
	Jet = &Good{
		ID: "jet", Singular: "Jet", Plural: "Jets", Counted: msgidN("{n} Jet", "{n} Jets"), Military: true,
		Count:      func(e *Empire) *int { return &e.Jets },
		Prod:       func(e *Empire) *int { return &e.ProdJets },
		Made:       func(e *Empire) *int { return &e.MadeJets },
		Basket:     func(b *TradeBasket) *int { return &b.Jets },
		Force:      func(f *AttackForce) *int { return &f.Jets },
		Stored:     func(p *Prices) *int { return &p.Jet },
		WalkTag:    "jet",
		ShipWeight: 100, CarrierPer: 100,
	}
	Turret = &Good{
		ID: "turret", Singular: "Turret", Plural: "Turrets", Counted: msgidN("{n} Turret", "{n} Turrets"), Military: true,
		Count:      func(e *Empire) *int { return &e.Turrets },
		Prod:       func(e *Empire) *int { return &e.ProdTurrets },
		Made:       func(e *Empire) *int { return &e.MadeTurrets },
		Basket:     func(b *TradeBasket) *int { return &b.Turrets },
		Stored:     func(p *Prices) *int { return &p.Turret },
		WalkTag:    "turret",
		ShipWeight: 100, CarrierPer: 1000,
	}
	Bomber = &Good{
		ID: "bomber", Singular: "Bomber", Plural: "Bombers", Counted: msgidN("{n} Bomber", "{n} Bombers"), Military: true,
		Count:      func(e *Empire) *int { return &e.Bombers },
		Prod:       func(e *Empire) *int { return &e.ProdBombers },
		Made:       func(e *Empire) *int { return &e.MadeBombers },
		Basket:     func(b *TradeBasket) *int { return &b.Bombers },
		Force:      func(f *AttackForce) *int { return &f.Bombers },
		Stored:     func(p *Prices) *int { return &p.Bomber },
		WalkTag:    "bomber",
		ShipWeight: 300, CarrierPer: 0,
	}
	Tank = &Good{
		ID: "tank", Singular: "Tank", Plural: "Tanks", Counted: msgidN("{n} Tank", "{n} Tanks"), Military: true,
		Count:      func(e *Empire) *int { return &e.Tanks },
		Prod:       func(e *Empire) *int { return &e.ProdTanks },
		Made:       func(e *Empire) *int { return &e.MadeTanks },
		Basket:     func(b *TradeBasket) *int { return &b.Tanks },
		Force:      func(f *AttackForce) *int { return &f.Tanks },
		Stored:     func(p *Prices) *int { return &p.Tank },
		WalkTag:    "tank",
		ShipWeight: 100, CarrierPer: 5000,
	}
	Carrier = &Good{
		ID: "carrier", Singular: "Carrier", Plural: "Carriers", Counted: msgidN("{n} Carrier", "{n} Carriers"), Military: true,
		Count:      func(e *Empire) *int { return &e.Carriers },
		Prod:       func(e *Empire) *int { return &e.ProdCarriers },
		Made:       func(e *Empire) *int { return &e.MadeCarriers },
		Basket:     func(b *TradeBasket) *int { return &b.Carriers },
		Stored:     func(p *Prices) *int { return &p.Carrier },
		WalkTag:    "carrier",
		ShipWeight: 100, CarrierPer: 0,
	}
	Agent = &Good{
		ID: "agent", Singular: "Agent", Plural: "Agents", Counted: msgidN("{n} Agent", "{n} Agents"),
		Count:      func(e *Empire) *int { return &e.Agents },
		Basket:     func(b *TradeBasket) *int { return &b.Agents },
		Price:      func(w *World, e *Empire) int { return w.AgentPrice(e) },
		ShipWeight: 50, CarrierPer: 0,
	}
	Food = &Good{
		ID: "food", Singular: "Food", Plural: "Food", Counted: msgidN("{n} Food", "{n} Food"),
		Count:      func(e *Empire) *int { return &e.Food },
		Basket:     func(b *TradeBasket) *int { return &b.Food },
		Price:      func(w *World, e *Empire) int { return w.FoodBuyPrice() },
		ShipWeight: 5, CarrierPer: 0,
	}
)

// MilitaryGoods are the six units Industrial regions build, in the order the
// Set Industries screen lists them. ProjectedProduction and Manufacture walk
// this, so the projection's index i is this slice's index i.
var MilitaryGoods = []*Good{Trooper, Jet, Turret, Bomber, Tank, Carrier}

// AllGoods is every row, for a caller that wants the whole set rather than a
// screen's order — net worth, the S3-Sabre's target list, a name
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
