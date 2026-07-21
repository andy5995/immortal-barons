// Package game holds the BRE world: empires, economy, turn engine, and
// combat. The world is persistent and shared; one session is active at a
// time (Active), and dates flow in as ISO strings for testability.
package game

import (
	"math/rand"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Version is the single source of truth for the game's version string,
// displayed in the status bar and reported by the front-ends. Stays at
// 0.0.1 until the first release.
const Version = "0.0.1"

// Revision is the short VCS revision (7 hex chars, git's default short hash) the
// binary was built from, with a "-dirty" suffix when the working tree had
// uncommitted changes, or "" when the build carries no VCS info (e.g. `go build`
// outside a repo, or an unversioned release tarball). Go embeds this automatically
// for a `go build` from a git checkout. Shared by -version and the About screen.
func Revision() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	var rev string
	var dirty bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return ""
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if dirty {
		rev += "-dirty"
	}
	return rev
}

// VersionString is the game's full version identity: the release version plus the
// short VCS revision when the build has one — "0.0.1 (ffdec31)", or "0.0.1
// (ffdec31-dirty)" for a dirty build, else just "0.0.1". The single source both
// the -version output and the in-game About screen use, so they never diverge.
func VersionString() string {
	if rev := Revision(); rev != "" {
		return Version + " (" + rev + ")"
	}
	return Version
}

type Empire struct {
	Name  string
	Owner string // normalized BBS handle; "" for AI
	// AIProfile is the personality of an AI empire (#36): it shapes which treaty
	// offers the AI accepts and its military posture. Empty on human empires and
	// on AI empires saved before profiles existed (aiProfile() derives a stable
	// fallback from the name in that case).
	AIProfile string
	// AISkill is how well an AI empire plays its economy — sharp barons expand at
	// full tilt, dull ones hold back and grow slower, so a game has a mix of strong
	// and weak rivals. Empty on humans and pre-skill saves (aiSkill() derives a
	// stable fallback from the name then).
	AISkill string
	Alive   bool
	// DiedDay is the GameDay on which this empire was eliminated (People or
	// Land hit 0, or the owner abdicated). 0 means it never died. The husk is
	// kept until a LATER day so the owner cannot immediately re-onboard: BRE
	// deletes a destroyed/abdicated realm and lets the player rebuild the next
	// day. Daily maintenance and the login path remove husks once GameDay > DiedDay.
	DiedDay int

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
	// AttacksToday counts individual (conventional) attacks launched since the day
	// began, enforcing Config.MaxIndividualAttacks across every turn in the day.
	// Persisted so it survives the per-action reload/save cycle of door play; reset
	// to 0 at daily maintenance (see DailyMaintenance).
	AttacksToday int
	// TurnProgress records which stages of the current turn have already completed,
	// so a turn REPLAYED after an idle-boot skips what was done — no double income
	// or double charge, and no re-showing a menu the player already exited (GH #10).
	// Serialized (NOT json:"-"): surviving a cross-boot process restart is the whole
	// point. Cleared at turn-commit (PlayTurn) and on daily rollover (DailyMaintenance).
	TurnProgress TurnProgress
	Protection   int
	// Score is BRE's cumulative score (shown on the scores board, distinct from
	// Net Worth): += a flat ScorePerTurn once per turn played, minus small
	// IB-only penalties for riots/food spoilage, plus combat/covert score.
	// Seeded 0. Matches BRE live data (a standard realm scored a flat +213/turn,
	// 8 turns = 1704).
	Score int
	// TechLevel is the empire's accumulated Technology bonus, in TENTHS of a
	// percent (600 = 60.0%). Unlike a raw region-share ratio it BUILDS UP over
	// turns the empire holds Technology regions and saturates at a share-scaled
	// cap — BRE-verified: the bonus is not instantaneous but ramps slowly, and
	// faster the denser the tech share (see advanceTech, TechFactor). Seeded 0,
	// so a realm's tech advantage grows with sustained investment, not a single
	// purchase.
	TechLevel        int
	LastPlayed       string
	Events           []string
	Mail             []string
	PirateRaids      []string // raids suffered since last play; shown in the income report
	ImmuneFrom       []string // empires whose covert ops against us auto-fail (we bribed their agents)
	ShieldedUntilDay int      // GameDay through which ALL incoming covert ops auto-fail (Expose Enemy Ops)

	// CoordinatorVote is the owner handle this baron votes for as the BBS
	// Coordinator (the elected player who gets the Coordinator menu). Changeable
	// any time from the System menu.
	CoordinatorVote string

	AllianceOffers []string      // legacy (pre-typed-treaties); migrated by EnsureTreaties
	TreatyOffers   []TreatyOffer // pending treaty proposals received
	TradeDeals     []TradeDeal   // pending barter offers received (respond in the Trading menu)

	Investments []Investment
	Loans       []Loan // active Cash Relief loans (#40), each due on its DueDay

	// Prices is this empire's own current unit buy prices — a persistent per-turn
	// random walk (#30; BRE gives every empire its own drifting prices). Only the
	// seven unit fields are used: regions price by holdings and food by the market,
	// so Land/Food here stay unused. A zero field means unseeded (a fresh empire's
	// first turn, or a pre-feature save) — callers fall back to the world base
	// (World.*Price) until the first walk step (World.stepPrices, once per turn in
	// PlayTurn) seeds it.
	Prices Prices

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
	// ProdInitialized distinguishes an empire whose production has been set up
	// (at creation, or by the player) from a pre-feature save whose Prod* are
	// all zero because the fields didn't exist. Without it, a player who sets
	// every percentage to 0 (all output → gold) has it reset to the default on
	// the next reload. See EnsureProduction.
	ProdInitialized bool
	Specialized     string // "" = none, else a unit type name; specialization concentrates output

	// Transient per-turn stats for the end-of-turn report; not persisted.
	LastSpoiled         int  `json:"-"`
	LastPopGrowth       int  `json:"-"`
	LastStarved         int  `json:"-"` // people who left this turn from a food shortfall
	MaintUnderpaid      bool `json:"-"` // forces/regions maintenance was underpaid this turn (blocks the well-run support boost)
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

// TurnProgress marks the stages of the current turn that have already completed,
// so replaying a turn interrupted by an idle-boot skips what was already done
// (GH #10). Correctness flags (IncomeCollected, MaintPaid, Fed) prevent
// re-applying a resource effect; the rest prevent re-showing a menu the player
// already exited. All are cleared at turn-commit (PlayTurn) and daily rollover.
type TurnProgress struct {
	IncomeCollected    bool // turn-start Manufacture + CollectIncome + regions-cap reset done
	MaintPaid          bool // paymentStage done (set with the forces/regions charge)
	Fed                bool // feedStage done
	CovertDone         bool
	SpendingDone       bool
	AttackDone         bool
	TradingDone        bool
	InterPlanetaryDone bool
	MessageDone        bool
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

// EnsureProduction repairs the production percentages after loading a save that
// predates industrial production (all Prod* fields zero). It runs on every load,
// so it must NOT touch an empire that has already been initialized — otherwise a
// player who deliberately sets every percentage to 0 gets reset to the default.
func (e *Empire) EnsureProduction() {
	if e.ProdInitialized {
		return
	}
	e.ProdInitialized = true
	if e.ProdTroopers+e.ProdJets+e.ProdTurrets+e.ProdBombers+e.ProdTanks+e.ProdCarriers == 0 {
		e.ProdTroopers, e.ProdJets, e.ProdTurrets = DefaultProdPct, DefaultProdPct, DefaultProdPct
		e.ProdBombers, e.ProdTanks, e.ProdCarriers = DefaultProdPct, DefaultProdPct, DefaultProdPct
	}
}

// Offense is the empire's full attack strength — every usable unit committed (see
// FullForce/AttackForce.offense). A regular attack may commit less (force select).
func (e *Empire) Offense() int {
	return FullForce(e).groundOffense(e)
}

func (e *Empire) Defense() int {
	sum := e.Troopers + e.Turrets*2 + e.Tanks*4*(100+e.HQ)/100
	return sum * (100 + e.TechFactor()) / 100
}

// techShare is the percent of the empire's land that is Technology regions
// (0..100). It sets how fast TechLevel accumulates and how high it saturates,
// so bigger empires need proportionally more Technology for the same bonus.
func (e *Empire) techShare() int {
	if e.Land <= 0 {
		return 0
	}
	return e.Regions.Technology * 100 / e.Land
}

// TechFactor is the empire's current Technology bonus percent, derived from the
// accumulated TechLevel (advanceTech ramps it up over turns). It is NOT the
// instantaneous tech share — a realm has to hold Technology regions for a while
// before the bonus builds up.
func (e *Empire) TechFactor() int {
	f := e.TechLevel / 10
	if f > TechFactorCap {
		f = TechFactorCap
	}
	return f
}

// advanceTech ramps the empire's TechLevel one turn's worth toward its
// share-scaled ceiling. Called once per turn played. The per-turn gain grows
// with the square of the tech share (a tech-dense realm advances much faster),
// and the ceiling scales with the share (so the bonus tops out near the share).
// Selling Technology regions lowers the ceiling and settles the level back down.
func (w *World) advanceTech(e *Empire) {
	share := e.techShare()
	ceil := share * TechCeilMul
	if hardCap := TechFactorCap * 10; ceil > hardCap {
		ceil = hardCap
	}
	// A Technology Agreement raises the ceiling toward a capped share of a
	// higher-tech partner's level, so you gain some of their advances even with
	// little Technology of your own (#11).
	agCeil := w.techAgreementCeiling(e)
	if agCeil > ceil {
		ceil = agCeil
	}
	if e.TechLevel >= ceil {
		e.TechLevel = ceil // sold-off tech / broken treaty / shrunk share: settle to the ceiling
		return
	}
	gain := share * share / TechGainDiv
	if agCeil > e.TechLevel { // a partner pulls a low-Technology realm upward
		catchUp := (agCeil - e.TechLevel) / TechAgreementGainDiv
		if catchUp < 1 {
			catchUp = 1
		}
		if catchUp > gain {
			gain = catchUp
		}
	}
	e.TechLevel += gain
	if e.TechLevel > ceil {
		e.TechLevel = ceil
	}
}

type Prices struct {
	Land, Trooper, Jet, Turret, Tank, Carrier, Agent, Bomber int
}

// Investment is a term deposit: an amount locked until MaturesDay, paying
// out Return (principal + interest at the rate in effect when invested).
type Investment struct {
	Amount     int // principal locked
	Return     int // total paid out at maturity (principal + interest)
	MaturesDay int // GameDay at/after which it pays out
}

// Investment tuning (v1, tunable — see docs/mechanics-reference.md).
const (
	// Investment term bounds — live-BRE-verified (screenshot, 2026-07-17): the
	// bank prints "There is now a 2 day minimum on investments." and the prompt
	// reads "How many days would you like to invest for? (2; 10)". So min 2, max
	// 10, and BRE's own suggested value is the minimum (2), not 0.
	MinInvestDays     = 2
	MaxInvestDays     = 10
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
	Score    int // BRE cumulative score (Empire.Score); 0 in pre-Score packets
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
	Empires    []*Empire
	Prices     Prices
	Config     Config `json:"-"` // authoritative copy is config.json; Load sets this
	GameDay    int
	InvestRate int // percent per day, floats each daily maintenance
	// FoodMarketSupply is the shared planet-wide pool of food available to buy
	// today; buying depletes it, selling replenishes it, and it resets to
	// FoodMarketDailySupply each day's maintenance (issue #19).
	FoodMarketSupply int
	LastMaintDate    string

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

	// Market holds every empire's listings on the general Trading Market (#17);
	// listed goods are escrowed out of the owner's inventory. MarketProceeds
	// accrues each seller's unpaid sale gold, deposited at daily maintenance.
	Market         []MarketListing
	MarketProceeds map[string]int

	// Inter-BBS (interplanetary) play — see ibbs.go. GroupAttacks assemble
	// locally until they depart; Outbox holds packets queued for other boards;
	// SpyDatabase holds spy reports shared across the planet.
	GroupAttacks    []GroupAttack
	NextAttackID    int
	Outbox          []Packet
	SpyDatabase     []SpyReport
	LeagueDiplomacy string       // coordinator's league-wide diplomacy declaration
	LeagueNodes     []LeagueNode `json:"-"` // league roster, loaded from ibnodes.dat at startup

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
	// nameRng is a SEPARATE stream for cosmetic AI-name generation, so choosing
	// names never perturbs the gameplay rng — seed-reproducible combat/events stay
	// identical no matter how many names are drawn.
	nameRng *rand.Rand
	store   Store // transaction backend for With: in-memory (web) or file-per-action (door)
	// reloadGen counts wholesale reloads of the empire set. The door's FileStore
	// bumps it on every reload (which replaces every *Empire); a per-session
	// cache of the active empire (menu.ctx) re-resolves its handle when the count
	// changes. The web's in-memory store never reloads, so it stays 0 and the
	// cache is resolved once — the same stable pointer the pre-handle code held,
	// so concurrent onboarding (AddHuman) never races a cached read.
	reloadGen uint64
}

func NewWorld(cfg Config) *World { return NewWorldSeed(cfg, time.Now().UnixNano()) }

func NewWorldSeed(cfg Config, seed int64) *World {
	// nameRng is salted off the same seed so names are deterministic per seed yet
	// draw from a stream independent of gameplay rng.
	w := &World{Config: cfg, rng: rand.New(rand.NewSource(seed)), nameRng: rand.New(rand.NewSource(seed ^ 0x4149_6e61_6d65))}
	w.store = &MemStore{w}
	w.initFreshGame()
	return w
}

// initFreshGame installs a brand-new game's state onto w, keeping only its
// infrastructure (mutex, rng, store) and Config. It is the SINGLE definition of
// what a fresh game contains, called both at world creation (NewWorldSeed) and
// on -reset (resetForNewGame). Keeping it in one place means a default can never
// be seeded at creation but forgotten on reset — the drift that stranded the
// old prices (and would silently carry pirates, news, and the master across a
// reset). Add any new creation-time world default here, not in NewWorldSeed.
func (w *World) initFreshGame() {
	w.Empires = nil
	w.Prices = defaultPrices()
	w.GameDay = 0
	w.InvestRate = DefaultInvestRate
	w.FoodMarketSupply = FoodMarketDailySupply
	w.LastMaintDate = ""
	w.NewsToday = nil
	w.NewsYesterday = nil
	w.BulletinToday = DailyBulletin{}
	w.BulletinYesterday = DailyBulletin{}
	w.Alliances = nil
	w.Treaties = nil
	w.LastMaster = ""
	w.CurrentMaster = ""
	w.RemoteBoards = nil
	w.Pirates = nil
	w.Market = nil
	w.MarketProceeds = nil
	w.GroupAttacks = nil
	w.NextAttackID = 0
	w.Outbox = nil
	w.SpyDatabase = nil
	w.LeagueDiplomacy = ""
	w.EnterExitsBuy = false
	w.DepositEndTurn = false
	w.AutoPayMaint = false
	w.AutoFeed = false
	w.VisitCovert = true
	w.VisitTrading = true
	w.VisitMessage = true
	w.seedAIEmpires()
	w.seedPirates()
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

// AI baron names are built from a modifier x noun matrix rather than a fixed
// list, so a game can field far more distinct barons and the lineup varies
// game to game. The two single-word pools combine as "<modifier> <noun>"
// (e.g. "Crimson Horde"); ~24 x ~24 gives hundreds of gritty combinations.
var (
	aiNameModifiers = []string{
		"Crimson", "Iron", "Ashen", "Obsidian", "Void", "Rust", "Cinder", "Grim",
		"Ember", "Dread", "Storm", "Dust", "Salt", "Blood", "Frost", "Bone",
		"Shadow", "Molten", "Scarlet", "Onyx", "Thorn", "Gale", "Coal", "Ashfall",
	}
	aiNameNouns = []string{
		"Horde", "Dominion", "Clan", "Reavers", "Kings", "Pact", "Legion",
		"Marauders", "Barons", "Host", "Vanguard", "Coven", "Coil", "Wardens",
		"Raiders", "Syndicate", "Covenant", "Brood", "Vultures", "Jackals",
		"Corsairs", "Warband", "Conclave", "Sovereigns",
	}
)

// newAIName returns a "<modifier> <noun>" name not present in used, or ok=false
// only when every combination is taken. It tries random combinations first (via
// w.nameRng, so it is deterministic per seed and varied game to game), then
// falls back to a deterministic scan so the "pool exhausted" answer is exact.
func (w *World) newAIName(used map[string]bool) (string, bool) {
	for tries := 0; tries < 200; tries++ {
		name := aiNameModifiers[w.nameRng.Intn(len(aiNameModifiers))] + " " + aiNameNouns[w.nameRng.Intn(len(aiNameNouns))]
		if !used[strings.ToLower(name)] {
			return name, true
		}
	}
	for _, m := range aiNameModifiers {
		for _, n := range aiNameNouns {
			name := m + " " + n
			if !used[strings.ToLower(name)] {
				return name, true
			}
		}
	}
	return "", false
}

// addAIEmpire appends one AI empire (Owner "") with the standard AI starting
// setup and returns it. Shared by seedAIEmpires and AddAIEmpires.
func (w *World) addAIEmpire(name string) *Empire {
	e := newEmpire(name, "", w.Config)
	e.Jets = 5
	e.Turrets = 40
	// Spread personalities evenly across the AI pool (#36) so a game gets a mix
	// of diplomats, balanced realms, and aggressors rather than all alike.
	e.AIProfile = aiProfiles[len(w.AIEmpires())%len(aiProfiles)]
	// Economic skill is rolled randomly (not cycled), so the sharp/dull split
	// varies game to game rather than being a fixed pattern.
	if w.rng.Intn(2) == 0 {
		e.AISkill = AISkillDull
	} else {
		e.AISkill = AISkillSharp
	}
	w.Empires = append(w.Empires, e)
	return e
}

// seedAIEmpires appends Config.AICount AI empires to the world.
func (w *World) seedAIEmpires() {
	w.AddAIEmpires(w.Config.AICount)
}

// AddAIEmpires injects up to n new AI barons into the live world, generating
// matrix names not already used by any existing empire (dead or alive,
// case-insensitive). It returns the count actually added, which is less than n
// only if every name combination is already taken.
func (w *World) AddAIEmpires(n int) int {
	used := make(map[string]bool, len(w.Empires))
	for _, e := range w.Empires {
		used[strings.ToLower(e.Name)] = true
	}
	added := 0
	for added < n {
		name, ok := w.newAIName(used)
		if !ok {
			break
		}
		w.addAIEmpire(name)
		used[strings.ToLower(name)] = true
		added++
	}
	return added
}

// defaultPrices is the world's starting price table, from balance.go. Used at
// world creation and re-applied on -reset, so a reset always installs the
// current prices instead of carrying the old world's stale ones.
func defaultPrices() Prices {
	return Prices{Land: PriceLand, Trooper: PriceTrooper, Jet: PriceJet, Turret: PriceTurret, Tank: PriceTank, Carrier: PriceCarrier, Agent: PriceAgent, Bomber: PriceBomber}
}

func newEmpire(name, owner string, cfg Config) *Empire {
	// BRE's starting realm (verified from a live BRE new-empire screen): 15
	// regions — 2 Agricultural, 5 Desert, 5 Mountain, 3 Coastal — with 100
	// troopers, 1000 food, full morale, and no other units.
	regions := RegionMix{Agricultural: StartAgricultural, Desert: StartDesert, Mountain: StartMountain, Coastal: StartCoastal}
	return &Empire{
		Name: name, Owner: owner, Alive: true,
		Gold: StartGold, Food: StartFood, Land: regions.Total(), People: StartPeople,
		Regions:  regions,
		Troopers: StartTroopers, Tax: StartTax, Support: 100, Morale: 100,
		TurnsLeft: cfg.TurnsPerDay, Protection: cfg.ProtectionTurns,
		// BRE default: all six at DefaultProdPct (15% → 90% units, 10% remainder → gold).
		ProdTroopers: DefaultProdPct, ProdJets: DefaultProdPct, ProdTurrets: DefaultProdPct,
		ProdBombers: DefaultProdPct, ProdTanks: DefaultProdPct, ProdCarriers: DefaultProdPct,
		ProdInitialized: true, // so a player's later all-zero setting isn't overwritten
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

// ReloadGen reports how many times the empire set has been wholesale-reloaded
// from disk. A per-session cache of the active empire compares it to detect a
// FileStore reload (which invalidates every *Empire pointer) and re-resolve.
func (w *World) ReloadGen() uint64 { return w.reloadGen }

// MarkReloaded records that the empire set was just replaced (a FileStore
// reload), so cached empire pointers get re-resolved by handle.
func (w *World) MarkReloaded() { w.reloadGen++ }

// With runs fn while holding the world lock. Use it around a short
// mutate-or-snapshot window — never around player input.
func (w *World) With(fn func()) {
	if w.store != nil {
		w.store.Transact(fn)
		return
	}
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

// removeDeadHusks deletes eliminated empires whose death is in the past
// (GameDay > DiedDay), keeping husks that died today so the owner cannot
// re-onboard on the same day. AI barons are removed too; they never rebuild.
func (w *World) removeDeadHusks() {
	kept := w.Empires[:0]
	for _, e := range w.Empires {
		if !e.Alive && w.GameDay > e.DiedDay {
			continue
		}
		kept = append(kept, e)
	}
	w.Empires = kept
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
	// int64 intermediate: e.Land*12500 (and the unit terms) overflow int32 on a
	// 32-bit build for a large realm. Weights are BRE-exact and unchanged; only
	// the arithmetic is widened. Storage/return stay int.
	thou := int64(e.Land)*NetWorthLand +
		int64(e.Troopers)*NetWorthTrooper + int64(e.Jets)*NetWorthJet + int64(e.Turrets)*NetWorthTurret + int64(e.Bombers)*NetWorthBomber +
		int64(e.Agents)*NetWorthAgent + int64(e.Tanks)*NetWorthTank + int64(e.Carriers)*NetWorthCarrier
	return int(thou/1000 - int64(e.Debt)/100)
}
