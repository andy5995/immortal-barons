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
	Tanks    int

	Tax int // percent
}

func (e *Empire) Army() int  { return e.Troopers + e.Jets + e.Tanks }
func (e *Empire) Power() int { return e.Troopers*2 + e.Jets*5 + e.Tanks*10 }

// Prices are the world's current unit costs, in gold.
type Prices struct {
	Land    int
	Food    int
	Trooper int
	Jet     int
	Tank    int
}

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
		Prices:   Prices{Land: 100, Food: 2, Trooper: 30, Jet: 200, Tank: 500},
		rng:      rand.New(rand.NewSource(seed)),
	}
	w.Empires = append(w.Empires, &Empire{
		Name: "New Barony", Human: true, Alive: true,
		Gold: 10000, Food: 20000, Land: 100, People: 2000, Troopers: 150, Tax: 15,
	})
	for _, n := range []string{"Crimson Horde", "Iron Dominion", "Ashfall Clan"} {
		w.Empires = append(w.Empires, &Empire{
			Name: n, Alive: true,
			Gold: 8000, Food: 18000, Land: 90, People: 1800, Troopers: 120, Jets: 5, Tax: 18,
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

// NetWorth values everything an empire holds, minus debt.
func (w *World) NetWorth(e *Empire) int {
	p := w.Prices
	return e.Gold + e.Bank - e.Debt +
		e.Land*p.Land + e.Food*p.Food +
		e.Troopers*p.Trooper + e.Jets*p.Jet + e.Tanks*p.Tank +
		e.People*5
}

// GameOver reports whether the game has ended (turns exhausted or the
// player is the last empire standing / has fallen).
func (w *World) GameOver() bool {
	p := w.Player()
	return w.Turn >= w.MaxTurns || p == nil || !p.Alive || len(w.Rivals()) == 0
}
