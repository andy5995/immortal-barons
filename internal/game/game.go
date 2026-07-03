// Package game holds the BRE world: empires, economy, turn engine, and
// combat. The world is persistent and shared; one session is active at a
// time (Active), and dates flow in as ISO strings for testability.
package game

import (
	"math/rand"
	"strings"
	"time"
)

type Empire struct {
	Name  string
	Owner string // normalized BBS handle; "" for AI
	Alive bool

	Gold    int
	Bank    int
	Debt    int
	Food    int
	Land    int
	Regions RegionMix

	People   int
	Troopers int
	Jets     int
	Turrets  int
	Tanks    int
	Carriers int
	Agents   int

	Tax int
	SDI int // 0-75, percentage reduction of incoming strike damage
	HQ  int // 0 = none/not started; 1-100 = percent complete

	TurnsLeft  int
	Protection int
	LastPlayed string
	Events     []string
	Mail       []string

	AllianceOffers []string

	// Transient per-turn stats for the end-of-turn report; not persisted.
	LastSpoiled   int `json:"-"`
	LastPopGrowth int `json:"-"`
}

func (e *Empire) Army() int { return e.Troopers + e.Jets + e.Turrets + e.Tanks }

// syncLand resyncs the authoritative Land total from Regions. Every place
// that changes an empire's regions must call this afterward.
func (e *Empire) syncLand() { e.Land = e.Regions.Total() }

// EnsureRegions repairs the Land/Regions invariant after loading a save that
// predates region types (Regions zero) or is otherwise inconsistent. If the
// breakdown already sums to Land, it is a no-op.
func (e *Empire) EnsureRegions() {
	if e.Regions.Total() == e.Land {
		return
	}
	e.Regions = defaultRegionMix(e.Land)
	e.syncLand() // Land now equals the rebuilt total (defaultRegionMix sums to Land, so Land is unchanged)
}

func (e *Empire) Offense() int {
	usableJets := min(e.Jets, e.Carriers*100)
	return e.Troopers + usableJets*2 + e.Tanks*4*(100+e.HQ)/100
}

func (e *Empire) Defense() int {
	return e.Troopers + e.Turrets*2 + e.Tanks*4*(100+e.HQ)/100
}

type Prices struct {
	Land, Food, Trooper, Jet, Turret, Tank, Carrier, Agent int
}

const (
	InterestCap = 1_599_999_999
	MoneyCap    = 2_000_000_000
)

// RemoteScore is one empire's score as reported by another board's
// exported inter-BBS packet.
type RemoteScore struct {
	Empire   string
	NetWorth int
	Land     int
}

// RemoteBoard is a snapshot of another board's scores, imported via an
// inter-BBS packet.
type RemoteBoard struct {
	BoardID string
	Date    string
	Scores  []RemoteScore
}

type World struct {
	Empires       []*Empire
	Prices        Prices
	Config        Config
	GameDay       int
	LastMaintDate string
	Bulletin      []string
	Alliances     []string
	LastMaster    string
	RemoteBoards  []RemoteBoard

	Coordinator bool

	// Player preferences (kept on the world for now; per-empire is a later
	// refinement). Referenced by the Preferences menu.
	EnterExitsBuy  bool
	DepositEndTurn bool
	AutoPayMaint   bool
	AutoFeed       bool
	VisitCovert    bool
	VisitTrading   bool
	VisitMessage   bool

	Active *Empire `json:"-"` // the empire playing this session
	Today  string  `json:"-"` // ISO date for this session

	rng *rand.Rand
}

func NewWorld(cfg Config) *World { return NewWorldSeed(cfg, time.Now().UnixNano()) }

func NewWorldSeed(cfg Config, seed int64) *World {
	w := &World{
		Prices:       Prices{Land: 100, Food: 2, Trooper: 50, Jet: 60, Turret: 60, Tank: 350, Carrier: 40, Agent: 100},
		Config:       cfg,
		rng:          rand.New(rand.NewSource(seed)),
		VisitCovert:  true,
		VisitTrading: true,
		VisitMessage: true,
	}
	w.seedAIEmpires()
	return w
}

// seedAIEmpires appends Config.AICount AI empires to the world.
func (w *World) seedAIEmpires() {
	names := []string{"Crimson Horde", "Iron Dominion", "Ashfall Clan", "Storm Reavers", "Dust Kings"}
	for i := 0; i < w.Config.AICount && i < len(names); i++ {
		w.Empires = append(w.Empires, newEmpire(names[i], "", w.Config))
		w.Empires[len(w.Empires)-1].Jets = 5
		w.Empires[len(w.Empires)-1].Turrets = 40
	}
}

func newEmpire(name, owner string, cfg Config) *Empire {
	regions := defaultRegionMix(100)
	return &Empire{
		Name: name, Owner: owner, Alive: true,
		Gold: 10000, Food: 20000, Land: regions.Total(), People: 2000,
		Regions:  regions,
		Troopers: 150, Carriers: 1, Tax: 15,
		TurnsLeft: cfg.TurnsPerDay, Protection: cfg.ProtectionTurns,
	}
}

// AddHuman creates and registers a human empire keyed by handle.
func (w *World) AddHuman(handle, realm string) *Empire {
	e := newEmpire(realm, strings.ToLower(strings.TrimSpace(handle)), w.Config)
	w.Empires = append(w.Empires, e)
	return e
}

func (w *World) Player() *Empire { return w.Active }

func (w *World) FindByOwner(handle string) *Empire {
	h := strings.ToLower(strings.TrimSpace(handle))
	for _, e := range w.Empires {
		if e.Owner == h {
			return e
		}
	}
	return nil
}

func (w *World) AIEmpires() []*Empire {
	var r []*Empire
	for _, e := range w.Empires {
		if e.Owner == "" && e.Alive {
			r = append(r, e)
		}
	}
	return r
}

func (w *World) Targets(attacker *Empire) []*Empire {
	var r []*Empire
	for _, e := range w.Empires {
		if e != attacker && e.Alive && e.Protection == 0 && !w.AreAllied(attacker, e) {
			r = append(r, e)
		}
	}
	return r
}

// ImportBoard records b's scores, replacing any existing entry for the
// same BoardID so re-importing updates rather than duplicates.
func (w *World) ImportBoard(b RemoteBoard) {
	for i, existing := range w.RemoteBoards {
		if existing.BoardID == b.BoardID {
			w.RemoteBoards[i] = b
			return
		}
	}
	w.RemoteBoards = append(w.RemoteBoards, b)
}

func (w *World) NetWorth(e *Empire) int {
	return e.Gold + e.Bank - e.Debt +
		e.Land*12500 + e.Food*w.Prices.Food +
		e.Troopers*250 + e.Jets*325 + e.Turrets*425 + e.Tanks*1250 + e.Carriers*1000 +
		e.Agents*500 + e.People*5
}
