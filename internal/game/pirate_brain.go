package game

import (
	"fmt"
	"slices"
)

// PirateBrain decides pirate raids in place of the built-in roll (#202). A mod
// supplies one through Rules.Pirates; internal/mod runs it as a script, so this
// package never sees the scripting language.
//
// Raid is asked once per realm per turn, after the checks the built-in roll
// makes (pirates on, the realm alive and out of protection). Its orders are
// carried out by the same code the built-in raid uses. When Raid fails, or
// returns an order the game refuses, the game calls Failed with the reason and
// raids that realm the built-in way for the turn. A brain that called Rand
// before failing has already moved the RNG, so that fallback draws from a
// shifted stream.
type PirateBrain interface {
	Raid(v PirateView) ([]PirateOrder, error)
	Failed(err error)
}

// PirateOrder is one raid a PirateBrain orders: faction Slot takes Spoil from
// the realm's inventory, or from its Trading Market listing when FromMarket.
type PirateOrder struct {
	Slot       int
	Spoil      PirateSpoil
	FromMarket bool
}

// PirateView is what a PirateBrain sees: copies, so nothing it does reaches
// the world except through its orders. Rand is the world's RNG, so a game with
// a brain replays the same for a fixed seed (as tests use); n must be at least 1.
type PirateView struct {
	Day      int
	Realm    PirateRealmView
	Factions []PirateFactionView
	Rand     func(n int) int
}

// PirateRealmView is the realm a raid may hit.
type PirateRealmView struct {
	Name    string
	Regions int
	Gold    int64
	// Units is what the realm holds, Market what it has listed on the Trading
	// Market.
	Units, Market PirateHoldings
}

// PirateHoldings counts each kind of unit a raid can take, by its spoil. Gold
// is not a holding: it is a field of its own.
type PirateHoldings map[PirateSpoil]int

// PirateFactionView is one faction, at index Slot of World.Pirates.
type PirateFactionView struct {
	Slot    int
	Name    string
	Regions int
	Gold    int64
	Loot    PirateHoldings
}

func (w *World) pirateView(e *Empire) PirateView {
	units, market := PirateHoldings{}, PirateHoldings{}
	for _, s := range PirateSpoils() {
		if g := s.Good(); g != nil {
			units[s] = *g.Count(e)
			market[s] = w.MarketForSale(e.Name, g.Singular)
		}
	}
	v := PirateView{
		Day: w.GameDay,
		Realm: PirateRealmView{
			Name: e.Name, Regions: e.Land, Gold: e.Gold,
			Units: units, Market: market,
		},
		Factions: make([]PirateFactionView, len(w.Pirates)),
		Rand:     w.rng.Intn,
	}
	for i := range w.Pirates {
		p := &w.Pirates[i]
		loot := PirateHoldings{}
		for _, s := range PirateSpoils() {
			if hoard, _ := p.loot(s); hoard != nil {
				loot[s] = *hoard
			}
		}
		v.Factions[i] = PirateFactionView{
			Slot: i, Name: p.Name, Regions: p.Land, Gold: p.Gold, Loot: loot,
		}
	}
	return v
}

// askPirateBrain asks b about e and checks every order before any is carried
// out, so a refused answer changes nothing.
func (w *World) askPirateBrain(b PirateBrain, e *Empire) ([]PirateOrder, error) {
	orders, err := b.Raid(w.pirateView(e))
	if err != nil {
		return nil, err
	}
	if len(orders) > ModPirateOrdersMax {
		return nil, fmt.Errorf("%d raids ordered; at most %d are allowed", len(orders), ModPirateOrdersMax)
	}
	for i, o := range orders {
		switch {
		case o.Slot < 0 || o.Slot >= len(w.Pirates):
			return nil, fmt.Errorf("raid %d: faction %d is outside 0 to %d", i, o.Slot, len(w.Pirates)-1)
		case !slices.Contains(PirateSpoils(), o.Spoil):
			return nil, fmt.Errorf("raid %d: %d is not something a raid takes", i, o.Spoil)
		case o.Spoil == SpoilGold && o.FromMarket:
			return nil, fmt.Errorf("raid %d: gold is not traded, so it cannot be taken from the market", i)
		}
	}
	return orders, nil
}
