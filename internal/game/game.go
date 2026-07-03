// Package game holds the BRE world: the player's empire, rival empires,
// the economy, the turn engine, and combat. The menu system reads and
// mutates a *World through these methods.
package game

import (
	"math/rand"
	"time"
)

// Empire is one barony in the world — the human player or a rival.
type Empire struct {
	Name  string
	Human bool
	Alive bool

	Gold int
	Bank int
	Debt int
	Food int
	Land int // regions

	People   int
	Troopers int
	Jets     int
	Turrets  int
	Tanks    int
	Carriers int

	Tax int // percent
}

// Army counts all combat units.
func (e *Empire) Army() int { return e.Troopers + e.Jets + e.Turrets + e.Tanks }

// Offense is attack strength. Per the reference, trooper=1, jet=2, tank=4;
// turrets are defense-only. Jets can only attack if carriers are available
// to move them (1 carrier moves 100 jets), so uncarried jets don't count.
func (e *Empire) Offense() int {
	usableJets := min(e.Jets, e.Carriers*100)
	return e.Troopers + usableJets*2 + e.Tanks*4
}

// Defense is defensive strength: trooper=1, turret=2, tank=4; jets=0.
func (e *Empire) Defense() int {
	return e.Troopers + e.Turrets*2 + e.Tanks*4
}

// Prices are the world's current unit costs, in gold.
type Prices struct {
	Land    int
	Food    int
	Trooper int
	Jet     int
	Turret  int
	Tank    int
	Carrier int
}

// Money caps from the reference: gold above InterestCap earns no interest,
// and no empire can hold more than MoneyCap coins at once.
const (
	InterestCap = 1_599_999_999
	MoneyCap    = 2_000_000_000
)

// World is the whole game state.
type World struct {
	Empires  []*Empire
	Turn     int
	MaxTurns int
	Prices   Prices

	// Coordinator gates the Sysop/Coordinator menu (league admin).
	Coordinator bool

	// Player preferences (toggled from the Preferences menu).
	EnterExitsBuy  bool
	DepositEndTurn bool
	AutoPayMaint   bool
	AutoFeed       bool

	rng *rand.Rand
}

// New builds a fresh world seeded from the clock.
func New() *World { return NewSeed(time.Now().UnixNano()) }

// NewSeed builds a world with a fixed RNG seed (deterministic — used by tests).
func NewSeed(seed int64) *World {
	w := &World{
		MaxTurns: 20,
		Prices:   Prices{Land: 100, Food: 2, Trooper: 50, Jet: 60, Turret: 60, Tank: 350, Carrier: 40},
		rng:      rand.New(rand.NewSource(seed)),
	}
	w.Empires = append(w.Empires, &Empire{
		Name: "New Barony", Human: true, Alive: true,
		Gold: 10000, Food: 20000, Land: 100, People: 2000,
		Troopers: 150, Carriers: 1, Tax: 15,
	})
	for _, n := range []string{"Crimson Horde", "Iron Dominion", "Ashfall Clan"} {
		w.Empires = append(w.Empires, &Empire{
			Name: n, Alive: true,
			Gold: 8000, Food: 18000, Land: 90, People: 1800,
			Troopers: 120, Jets: 5, Turrets: 40, Carriers: 1, Tax: 18,
		})
	}
	return w
}

// Player returns the human empire.
func (w *World) Player() *Empire {
	for _, e := range w.Empires {
		if e.Human {
			return e
		}
	}
	return nil
}

// Rivals returns the living non-human empires.
func (w *World) Rivals() []*Empire {
	var r []*Empire
	for _, e := range w.Empires {
		if !e.Human && e.Alive {
			r = append(r, e)
		}
	}
	return r
}

// NetWorth values everything an empire holds, minus debt. Unit weights are
// the reference net-worth values (×1000 to stay in whole coins): region
// 12500, tank 1250, carrier 1000, turret 425, jet 325, trooper 250.
func (w *World) NetWorth(e *Empire) int {
	return e.Gold + e.Bank - e.Debt +
		e.Land*12500 + e.Food*w.Prices.Food +
		e.Troopers*250 + e.Jets*325 + e.Turrets*425 + e.Tanks*1250 + e.Carriers*1000 +
		e.People*5
}

// GameOver reports whether the game has ended (turns exhausted or the
// player is the last empire standing / has fallen).
func (w *World) GameOver() bool {
	p := w.Player()
	return w.Turn >= w.MaxTurns || p == nil || !p.Alive || len(w.Rivals()) == 0
}
