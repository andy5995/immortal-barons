// Package game holds the BRE world: empires, economy, turn engine, and
// combat. The world is persistent and shared; one session is active at a
// time (Active), and dates flow in as ISO strings for testability.
package game

import (
	"math/rand"
	"strings"
	"time"
)

// Version is the single source of truth for the game's version string,
// displayed in the status bar and reported by the front-ends. Stays at
// 0.0.1 until the first release.
const Version = "0.0.1"

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
	Bombers  int
	Agents   int

	Tax     int
	SDI     int // 0-75, percentage reduction of incoming strike damage
	HQ      int // 0 = none/not started; 1-100 = percent complete
	Support int // 0-100, popular support; erodes with high tax, slashes Coastal income when low

	TurnsLeft   int
	Protection  int
	LastPlayed  string
	Events      []string
	Mail        []string
	PirateRaids []string // raids suffered since last play; shown in the income report
	ImmuneFrom  []string // empires whose covert ops against us auto-fail (we bribed their agents)

	// CoordinatorVote is the owner handle this baron votes for as the BBS
	// Coordinator (the elected player who gets the Coordinator menu). Changeable
	// any time from the System menu.
	CoordinatorVote string

	AllianceOffers []string      // legacy (pre-typed-treaties); migrated by EnsureTreaties
	TreatyOffers   []TreatyOffer // pending treaty proposals received

	Investments []Investment

	// Macros maps a single uppercase letter (invoked in-game as Ctrl-<letter>)
	// to a saved keystroke sequence that is replayed when the player presses
	// that combo. BRE's "Write Macros" / Macro Editor feature.
	Macros map[string]string

	// Production percentages (should sum to ~100) for what Industrial
	// regions build. See manufacture() in turn.go.
	ProdTroopers int
	ProdJets     int
	ProdTurrets  int
	ProdBombers  int
	ProdTanks    int
	ProdCarriers int
	Specialized  string // "" = none, else a unit type name; specialization concentrates output

	// Transient per-turn stats for the end-of-turn report; not persisted.
	LastSpoiled      int  `json:"-"`
	LastPopGrowth    int  `json:"-"`
	LastRiot         bool `json:"-"`
	MadeTroopers     int  `json:"-"`
	MadeJets         int  `json:"-"`
	MadeTurrets      int  `json:"-"`
	MadeBombers      int  `json:"-"`
	MadeTanks        int  `json:"-"`
	MadeCarriers     int  `json:"-"`
	IndustryGold     int  `json:"-"`
	LastGoldPaid     int  `json:"-"`
	LastFoodConsumed int  `json:"-"`
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

// EnsureSupport repairs Support after loading a save that predates the
// Support field (Support zero). v1 choice: a legitimately-0-support empire
// is effectively collapsing anyway, so treating 0 as "unset" is safe.
func (e *Empire) EnsureSupport() {
	if e.Support == 0 {
		e.Support = 100
	}
}

// EnsureProduction repairs the production percentages after loading a save
// that predates industrial production (all Prod* fields zero).
func (e *Empire) EnsureProduction() {
	if e.ProdTroopers+e.ProdJets+e.ProdTurrets+e.ProdBombers+e.ProdTanks+e.ProdCarriers == 0 {
		e.ProdTroopers, e.ProdJets, e.ProdTurrets, e.ProdBombers, e.ProdTanks, e.ProdCarriers = 30, 20, 15, 5, 20, 10
	}
}

func (e *Empire) Offense() int {
	usableJets := min(e.Jets, e.Carriers*100)
	sum := e.Troopers + usableJets*2 + e.Tanks*4*(100+e.HQ)/100
	return sum * (100 + e.techFactor()) / 100
}

func (e *Empire) Defense() int {
	sum := e.Troopers + e.Turrets*2 + e.Tanks*4*(100+e.HQ)/100
	return sum * (100 + e.techFactor()) / 100
}

// TechFactorCap is the maximum percent bonus/reduction Technology regions
// can grant (see techFactor).
const TechFactorCap = 40

// techFactor is the Technology bonus percent: the share of land that is
// Technology regions, capped. Bigger empires need proportionally more
// Technology to get the same factor (it's a share, not a raw count).
func (e *Empire) techFactor() int {
	if e.Land <= 0 {
		return 0
	}
	f := e.Regions.Technology * 100 / e.Land
	if f > TechFactorCap {
		f = TechFactorCap
	}
	return f
}

type Prices struct {
	Land, Food, Trooper, Jet, Turret, Tank, Carrier, Agent, Bomber int
}

const (
	InterestCap = 1_599_999_999
	MoneyCap    = 2_000_000_000
)

// Investment is a term deposit: an amount locked until MaturesDay, paying
// out Return (principal + interest at the rate in effect when invested).
type Investment struct {
	Amount     int // principal locked
	Return     int // total paid out at maturity (principal + interest)
	MaturesDay int // GameDay at/after which it pays out
}

// Investment tuning (v1, tunable — see docs/mechanics-reference.md).
const (
	MinInvestDays     = 1
	DefaultInvestRate = 5 // % per day
	MinInvestRate     = 1
	MaxInvestRate     = 25
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
	InvestRate    int // percent per day, floats each daily maintenance
	LastMaintDate string
	Bulletin      []string
	Alliances     []string // legacy (pre-typed-treaties); migrated by EnsureTreaties
	Treaties      []Treaty
	LastMaster    string
	RemoteBoards  []RemoteBoard
	Pirates       []PirateFaction

	// Inter-BBS (interplanetary) play — see ibbs.go. GroupAttacks assemble
	// locally until they depart; Outbox holds packets queued for other boards;
	// SpyDatabase holds spy reports shared across the planet.
	GroupAttacks    []GroupAttack
	NextAttackID    int
	Outbox          []Packet
	SpyDatabase     []SpyReport
	LeagueDiplomacy string       // coordinator's league-wide diplomacy declaration
	LeagueNodes     []LeagueNode `json:"-"` // league roster, loaded from BRNODES.DAT at startup

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
		Prices:       Prices{Land: 100, Food: 2, Trooper: 50, Jet: 60, Turret: 60, Tank: 350, Carrier: 40, Agent: 100, Bomber: 200},
		Config:       cfg,
		rng:          rand.New(rand.NewSource(seed)),
		VisitCovert:  true,
		VisitTrading: true,
		VisitMessage: true,
		InvestRate:   DefaultInvestRate,
	}
	w.seedAIEmpires()
	w.seedPirates()
	return w
}

// EnsureInvestRate repairs InvestRate after loading a save that predates
// investments (InvestRate zero).
func (w *World) EnsureInvestRate() {
	if w.InvestRate == 0 {
		w.InvestRate = DefaultInvestRate
	}
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
		Troopers: 150, Carriers: 1, Tax: 15, Support: 100,
		TurnsLeft: cfg.TurnsPerDay, Protection: cfg.ProtectionTurns,
		ProdTroopers: 30, ProdJets: 20, ProdTurrets: 15, ProdBombers: 5, ProdTanks: 20, ProdCarriers: 10,
	}
}

// AddHuman creates and registers a human empire keyed by handle.
func (w *World) AddHuman(handle, realm string) *Empire {
	e := newEmpire(realm, strings.ToLower(strings.TrimSpace(handle)), w.Config)
	w.Empires = append(w.Empires, e)
	return e
}

func (w *World) Player() *Empire { return w.Active }

// RemoveEmpire deletes e from the world (Abdicate). The empire is gone
// entirely; the caller gets a fresh realm on their next visit. Active is
// cleared if it pointed at e.
func (w *World) RemoveEmpire(e *Empire) {
	for i, x := range w.Empires {
		if x == e {
			w.Empires = append(w.Empires[:i], w.Empires[i+1:]...)
			break
		}
	}
	if w.Active == e {
		w.Active = nil
	}
}

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

// NetWorth is the empire's strategic score at BRE scale: land dominates at
// the reference net-worth weight (~12.5 per region), military counts at the
// table weights, and debt subtracts. Weights are ×10 to keep the fractional
// per-unit values (trooper 0.25, jet 0.325, …) in integer math, then /10.
// Gold, bank, food, and population are NOT counted at face value — net worth
// is a ranking score, not a cash tally (a cash-rich baron still scores low).
func (w *World) NetWorth(e *Empire) int {
	tenths := e.Land*125 +
		e.Troopers*2 + e.Jets*3 + e.Turrets*4 + e.Bombers*30 +
		e.Agents*5 + e.Tanks*12 + e.Carriers*10
	return tenths/10 - e.Debt/100
}
