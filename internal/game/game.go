// Package game holds the BRE world: empires, economy, turn engine, and
// combat. The world is persistent and shared; one session is active at a
// time (Active), and dates flow in as ISO strings for testability.
package game

import (
	"math/rand"
	"strings"
	"sync"
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

	Tax      int
	SDI      int    // 0-75, percentage reduction of incoming strike damage
	HQ       int    // 0 = none/not started; 1-100 = percent complete
	Support  int    // 0-100, popular support; erodes with high tax, slashes Coastal income when low
	Morale   int    // 0-100, military morale; low morale weakens combat and causes desertion
	Language string // help/UI language ("" = English; "de", "ru")

	TurnsLeft int
	// RegionsBoughtThisTurn counts regions bought since the turn began,
	// enforcing Config.MaxRegions cumulatively across every Buy Regions visit
	// in the turn rather than per purchase. Reset to 0 at the start of each
	// turn (see runTurn in internal/menu/gameflow.go).
	RegionsBoughtThisTurn int
	Protection            int
	LastPlayed            string
	Events                []string
	Mail                  []string
	PirateRaids           []string // raids suffered since last play; shown in the income report
	ImmuneFrom            []string // empires whose covert ops against us auto-fail (we bribed their agents)

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
	// regions build. See Manufacture() in turn.go.
	ProdTroopers int
	ProdJets     int
	ProdTurrets  int
	ProdBombers  int
	ProdTanks    int
	ProdCarriers int
	Specialized  string // "" = none, else a unit type name; specialization concentrates output

	// Transient per-turn stats for the end-of-turn report; not persisted.
	LastSpoiled         int  `json:"-"`
	LastPopGrowth       int  `json:"-"`
	LastRiot            bool `json:"-"`
	LastMoraleDesertion int  `json:"-"`
	MadeTroopers        int  `json:"-"`
	MadeJets            int  `json:"-"`
	MadeTurrets         int  `json:"-"`
	MadeBombers         int  `json:"-"`
	MadeTanks           int  `json:"-"`
	MadeCarriers        int  `json:"-"`
	IndustryGold        int  `json:"-"`
	LastGoldPaid        int  `json:"-"`
	LastFoodConsumed    int  `json:"-"`
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

// EnsureMorale repairs Morale after loading a save that predates the Morale
// field (Morale zero), the same way EnsureSupport does.
func (e *Empire) EnsureMorale() {
	if e.Morale == 0 {
		e.Morale = 100
	}
}

// moraleFactor maps military morale (0-100) to a combat-effectiveness percent.
// Full morale fights at 100%; empty morale still fights at MoraleCombatFloor
// (units don't become useless, just weaker). Placeholder curve — tunable.
func moraleFactor(morale int) int {
	return MoraleCombatFloor + (100-MoraleCombatFloor)*morale/100
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
	return sum * (100 + e.TechFactor()) / 100
}

func (e *Empire) Defense() int {
	sum := e.Troopers + e.Turrets*2 + e.Tanks*4*(100+e.HQ)/100
	return sum * (100 + e.TechFactor()) / 100
}

// TechFactorCap is the maximum percent bonus/reduction Technology regions
// can grant (see TechFactor).
const TechFactorCap = 40

// TechFactor is the Technology bonus percent: the share of land that is
// Technology regions, capped. Bigger empires need proportionally more
// Technology to get the same factor (it's a share, not a raw count).
func (e *Empire) TechFactor() int {
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

// PlanetTotals is a snapshot of planet-wide aggregates over living empires.
type PlanetTotals struct {
	Population int // Σ Empire.People
	Regions    int // Σ Empire.Land
	NetWorth   int // Σ NetWorth(e)
}

// DailyBulletin is one day's frozen header: the totals at that day's
// maintenance and the change since the prior day.
type DailyBulletin struct {
	Totals PlanetTotals
	Change PlanetTotals // Totals minus the previous day's Totals
}

type World struct {
	Empires       []*Empire
	Prices        Prices
	Config        Config `json:"-"` // authoritative copy is config.json; Load sets this
	GameDay       int
	InvestRate    int // percent per day, floats each daily maintenance
	LastMaintDate string

	// NewsToday/NewsYesterday split the planetary news feed by day; the JSON
	// key stays "Bulletin" (the field's old name) so old saves load their
	// lines into today's news. BulletinToday/BulletinYesterday are the frozen
	// daily headers of planet-wide totals; see rollNews.
	NewsToday         []string `json:"Bulletin"`
	NewsYesterday     []string
	BulletinToday     DailyBulletin
	BulletinYesterday DailyBulletin

	Alliances     []string // legacy (pre-typed-treaties); migrated by EnsureTreaties
	Treaties      []Treaty
	LastMaster    string // crowned at league end (endGame); shown as "Last Planetary Master"
	CurrentMaster string // daily net-worth leader, tracked by postMasterNews
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
	LeagueNodes     []LeagueNode `json:"-"` // league roster, loaded from ibnodes.dat at startup

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

	Today string `json:"-"` // ISO date for this session

	mu  sync.Mutex // guards concurrent access when a server shares one World
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

// EnsureNews migrates a save that predates the Today/Yesterday news split.
// The old Bulletin field is loaded into NewsToday via the reused JSON key, so
// there is no data to move; this is a no-op kept for symmetry with the other
// Ensure* migrations and as a hook for future news-related repairs.
func (w *World) EnsureNews() {}

// planetTotals sums People, Land, and NetWorth over living empires.
func planetTotals(w *World) PlanetTotals {
	var t PlanetTotals
	for _, e := range w.Empires {
		if !e.Alive {
			continue
		}
		t.Population += e.People
		t.Regions += e.Land
		t.NetWorth += w.NetWorth(e)
	}
	return t
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
		Troopers: 150, Carriers: 1, Tax: 15, Support: 100, Morale: 100,
		TurnsLeft: cfg.TurnsPerDay, Protection: cfg.ProtectionTurns,
		ProdTroopers: 30, ProdJets: 20, ProdTurrets: 15, ProdBombers: 5, ProdTanks: 20, ProdCarriers: 10,
	}
}

// HumanCount is the number of human (caller-owned) empires in the world.
func (w *World) HumanCount() int {
	n := 0
	for _, e := range w.Empires {
		if e.Owner != "" {
			n++
		}
	}
	return n
}

// BoardFull reports whether the Max Players Per BBS cap is reached (0 =
// unlimited), so no new caller may enroll.
func (w *World) BoardFull() bool {
	return w.Config.MaxPlayers > 0 && w.HumanCount() >= w.Config.MaxPlayers
}

// AddHuman creates and registers a human empire keyed by handle.
func (w *World) AddHuman(handle, realm string) *Empire {
	e := newEmpire(realm, strings.ToLower(strings.TrimSpace(handle)), w.Config)
	w.Empires = append(w.Empires, e)
	return e
}

// RealmNameTaken reports whether an empire already uses this realm name
// (case-insensitive). Call it under the world lock together with AddHuman so a
// concurrent onboarding cannot slip a duplicate name in between the check and
// the insert.
func (w *World) RealmNameTaken(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, e := range w.Empires {
		if strings.ToLower(e.Name) == n {
			return true
		}
	}
	return false
}

// Lock/Unlock guard the shared World when a single process runs concurrent
// sessions (the web server). The door/local front-ends run one session and
// take it uncontended.
func (w *World) Lock()   { w.mu.Lock() }
func (w *World) Unlock() { w.mu.Unlock() }

// With runs fn while holding the world lock. Use it around a short
// mutate-or-snapshot window — never around player input.
func (w *World) With(fn func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fn()
}

// RemoveEmpire deletes e from the world (Abdicate). The empire is gone
// entirely; the caller gets a fresh realm on their next visit.
func (w *World) RemoveEmpire(e *Empire) {
	for i, x := range w.Empires {
		if x == e {
			w.Empires = append(w.Empires[:i], w.Empires[i+1:]...)
			break
		}
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
// NetWorth values each asset by BRE's net-worth table (per unit: Trooper 0.250,
// Jet 0.325, Turret 0.425, Tank 1.250, Bomber 3.000, Carrier 1.000, Agent
// 0.500, Region 12.50). Computed in thousandths so the 0.x25 values are exact
// (tenths rounded Trooper/Jet/Turret/Tank down).
func (w *World) NetWorth(e *Empire) int {
	thou := e.Land*12500 +
		e.Troopers*250 + e.Jets*325 + e.Turrets*425 + e.Bombers*3000 +
		e.Agents*500 + e.Tanks*1250 + e.Carriers*1000
	return thou/1000 - e.Debt/100
}
