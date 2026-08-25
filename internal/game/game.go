package game

import (
	"math/rand"
	"sync"
	"time"
)

// game.go — the World: the one shared object a board's whole game lives in,
// how a fresh one is built, and the lock every mutation goes through.

type Prices struct {
	Land, Trooper, Jet, Turret, Tank, Carrier, Bomber int
}

// Investment is a term deposit: an amount locked until MaturesDay, paying
// out Return (principal + interest at the rate in effect when invested).
type Investment struct {
	Amount     int64 // principal locked
	Return     int64 // total paid out at maturity (principal + interest)
	MaturesDay int   // GameDay at/after which it pays out
}

// Investment tuning (v1, tunable — see docs/mechanics-reference.md).
const (
	// Investment term bounds — live-BRE-verified (screenshot, 2026-07-17): the
	// bank prints "There is now a 2 day minimum on investments." and the prompt
	// reads "How many days would you like to invest for? (2; 10)". So min 2, max
	// 10, and BRE's own suggested value is the minimum (2), not 0.
	MinInvestDays = 2
	MaxInvestDays = 10

	// The daily investment rate is held in TENTHS of a percent, which is the
	// resolution BRE works in: its Standard Investment Rate knob states the
	// return over ten days (35 → 3.5%/day), the Investments screen prints two
	// decimals ("5.00%"), View Bank Rates prints one ("5.0%"), and the bank
	// nudges the floating rate by half a point. A whole-percent rate can express
	// none of that, and its band ran to 25%/day — two and a half times BRE's
	// ceiling, which a ten-day term compounds into a ninefold return.
	//
	// The floating rate is bounded by the same range BRE allows the knob,
	// (35; 100): a live game whose knob sat at the default 35 was observed at
	// 5.0%/day, so the rate drifts well above its setting but not without limit.
	DefaultInvestRate = 35  // 3.5%/day, matching Config.StdInvestRate's default
	MinInvestRate     = 35  // 3.5%/day
	MaxInvestRate     = 100 // 10.0%/day
)

// RemoteScore is one empire's score as reported by another board's
// exported inter-BBS packet.
type RemoteScore struct {
	Empire   string
	NetWorth int
	Land     int
	Score    int // BRE cumulative score (Empire.Score); 0 in pre-Score packets
	// Protected marks a realm still under New Realm Protection, so the boards
	// that read this packet can leave it off their target lists — the same
	// courtesy a local attack list extends. It can go stale in transit (a strike
	// takes days, protection counts down), so the target board still refuses an
	// arriving strike on its own authority; this only stops a baron spending
	// forces on a target that was already known to be untouchable. Absent from a
	// packet written before this field existed, which reads as unprotected.
	Protected bool `json:",omitempty"`
	// OwnerHash identifies the baron behind this realm to the duplicate-user
	// check without putting anyone's BBS handle in a file that crosses the
	// league — see dupe.go. Sent only while the sending board has Dupe Checking
	// on, and absent from packets written before the field existed.
	OwnerHash string `json:",omitempty"`
}

// RemoteBoard is a snapshot of another board's scores, imported via an
// inter-BBS packet.
type RemoteBoard struct {
	BoardID string
	Date    string
	Scores  []RemoteScore
	// Market is that planet's Trading Market as of this snapshot, for barons here
	// to bid on. It is stale by a packet round trip by definition, which is why a
	// bid is an offer the far board may refuse rather than a purchase.
	Market []RemoteListing `json:",omitempty"`
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
	InvestRate int // tenths of a percent per day, floats each daily maintenance
	// FoodMarketSupply is the shared planet-wide pool of food available to buy
	// today; buying depletes it, selling replenishes it, and it resets to
	// FoodMarketDailySupply each day's maintenance (issue #19).
	FoodMarketSupply int
	// RefundPool is the planet-wide purse the Queen Royale's tax refund is paid
	// out of (#93). Crown tax paid flows in; each realm's first login of a game
	// day draws a share back out. Seeded at QueenRefundPoolSeed; a world saved
	// before the refund existed loads with 0 and fills from the next tax payment.
	RefundPool    int64
	LastMaintDate string
	// StartedDate is the real date the game actually began — the first day
	// maintenance ran with the game underway. Config.GameStartDate is the date a
	// sysop SCHEDULED it for and is usually empty (start immediately), so it
	// cannot answer "when did this game start?" on its own. The opening menu
	// shows this, the way BRE prints its start date above the menu.
	StartedDate string
	// LastMaintRun is the REAL date maintenance last actually ran, as distinct
	// from LastMaintDate, which is the game's own clock. The two diverge whenever
	// a game sits idle: maintenance advances the game clock by at most one day per
	// real day, so a realm left alone for a week comes back to one day's worth of
	// change, not seven. Without a separate record of when it last ran, every
	// login on the same real day would advance another game day while the clock
	// was still behind.
	LastMaintRun string

	// NewsToday/NewsYesterday split the planetary news feed by day; the JSON
	// key stays "Bulletin" (the field's old name) so old saves load their
	// lines into today's news. BulletinToday/BulletinYesterday are the frozen
	// daily headers of planet-wide totals; see rollNews.
	NewsToday         []string `json:"Bulletin"`
	NewsYesterday     []string
	BulletinToday     DailyBulletin
	BulletinYesterday DailyBulletin

	// BattlesTotal/ConquestsTotal count conventional battles and outright
	// conquests for the lifetime of the process. Not persisted: they exist so the
	// -spectate balance probe can report whether AI aggression actually fires,
	// without parsing the news prose.
	BattlesTotal   int `json:"-"`
	ConquestsTotal int `json:"-"`

	// CovertQueue holds the local covert operations that have been paid for and
	// are waiting on daily maintenance to resolve — BRE's type-7 records. Added
	// after v0.0.5, so a world saved before it loads with an empty queue.
	CovertQueue []QueuedCovertOp `json:",omitempty"`

	Alliances     []string // legacy (pre-typed-treaties); migrated by EnsureTreaties
	Treaties      []Treaty
	LastMaster    string // crowned at league end (endGame); shown as "Last Planetary Master"
	CurrentMaster string // daily net-worth leader, tracked by postMasterNews
	RemoteBoards  []RemoteBoard
	// SysopNotices are transport faults for the person running the game, not
	// for its players: a packet that could not be delivered, orders that failed
	// their check. They are NOT persisted (json:"-") and are drained by the
	// planetary step into its run report, because a bulletin is the wrong place
	// for them — no player can act on one, and the news cap is 20 lines, so a
	// fault that repeats every exchange silently deletes the day's real events.
	SysopNotices []string `json:"-"`
	// heldNoted dedupes the protocol-hold notice to one per board per run. Not
	// persisted: a new run should say so again.
	heldNoted map[string]bool
	Pirates   []PirateFaction

	// LastPacketFrom records the game day a packet from each board was PROCESSED
	// here, and BoardVersion the game version that board last said it was
	// running. Neither is gameplay: they are what the sysop reports need
	// (LastPacketReport, BBSInfoReport) to answer "who has gone quiet" and "who
	// is too old to read our packets", and nothing else reads them.
	LastPacketFrom map[string]string `json:",omitempty"`
	BoardVersion   map[string]string `json:",omitempty"`

	// TravelTimes is the average packet round trip to each other board, in days,
	// and LastTravelPing the game day the probes for it last went out — see
	// ibbs_travel.go.
	TravelTimes    map[string]float64
	LastTravelPing string

	// Market holds every empire's listings on the general Trading Market (#17);
	// listed goods are escrowed out of the seller's inventory. MarketProceeds
	// accrues each seller's unpaid sale gold, deposited at daily maintenance.
	// Both are keyed by realm NAME — see MarketListing.Realm for why the owner
	// handle could not do the job. A save from before that key change carried
	// both under different names; nothing migrates them, since no released
	// version's market had been traded on.
	Market         []MarketListing
	MarketProceeds map[string]int64 `json:"MarketProceedsByRealm"`

	// Inter-BBS (interplanetary) play — see ibbs.go. GroupAttacks assemble
	// locally until they depart; Outbox holds packets queued for other boards;
	// SpyDatabase holds spy reports shared across the planet.
	GroupAttacks []GroupAttack
	NextAttackID int
	// InFlight holds strikes that have left this board and are waiting for a
	// result packet. A result clears the matching entry; one that waits too long
	// has its forces given back instead (Config.LostForcesDays, #96).
	InFlight    []InFlightStrike
	Outbox      []Packet
	SpyDatabase []SpyReport
	// Annihilator is this planet's own doomsday weapon, one at a time, from the day a
	// baron starts it until it flies. Incoming is one aimed AT this planet that
	// another board has told us about — visible while it is being built and while
	// it is in the air, which is what gives the jets something to shoot at (#16,
	// #63).
	Annihilator *Annihilator
	Incoming    *Annihilator
	// BulletinDigest fingerprints every bulletin this board holds, keyed
	// "<scope>/<name>", so an edited file can be told from an untouched one
	// without keeping a second copy of it. BulletinsKnown marks the scopes
	// already recorded once, so the files a board had before this existed are
	// the baseline rather than a day's worth of news. See bulletin.go.
	BulletinDigest map[string]string `json:",omitempty"`
	BulletinsKnown map[string]bool   `json:",omitempty"`
	// PendingBulletins is a league set that has just arrived and still has to
	// reach the disk. Not persisted: internal/store drains it in the same run
	// that applied the packet.
	PendingBulletins *BulletinSet `json:"-"`

	// SpyGuys counts, per foreign planet, the game days a watcher of theirs has
	// left here. It is the WATCHED board that holds this — the planet paying for
	// the man keeps no record of him at all, which is BRE's arrangement.
	SpyGuys map[string]int `json:",omitempty"`

	// League packet authentication (#53). CoordKey is the ed25519 private key,
	// held only by the Coordinator's board; CoordPub is the matching public key,
	// held by every board. Both load from files rather than saving with the
	// world, so a world file that leaks does not leak the league's key.
	CoordKey []byte `json:"-"`
	CoordPub []byte `json:"-"`
	// BoardKey is this board's own ed25519 private key, which signs every
	// outbound packet so other boards can prove where it came from (#118). Read
	// from board.key at startup like the two above, never serialized.
	BoardKey []byte `json:"-"`
	// OutSeq numbers this board's outbound packets. HighSeq and SeenPackets are
	// what an inbound packet is checked against, so nothing is applied twice.
	OutSeq      uint64
	HighSeq     map[string]uint64
	SeenPackets map[string]bool
	// Season counts league-wide resets, so a board can tell the Coordinator's new
	// order from one it has already carried out (#65).
	Season          int
	LeagueDiplomacy string       // coordinator's league-wide declaration, made with a season reset
	LeagueNodes     []LeagueNode `json:"-"` // league roster, loaded from ibnodes.dat at startup
	// Transit holds packets that arrived here addressed to another board and are
	// waiting to be handed on. Kept apart from the Outbox because they must go
	// out exactly as they came in — see ForwardPacket. Saved with the world, as
	// the Outbox is: the inbound file is gone by now, so a run that dies before
	// writing would otherwise lose someone else's packet for good.
	Transit []Packet
	// PlanetDiplomacy is this board's own chart of where it stands with each
	// other planet — an annotation the BBS Coordinator keeps for their players,
	// binding nothing and never sent anywhere. See ibbs_diplomacy.go.
	PlanetDiplomacy map[string]PlanetRelation

	// Player preferences, kept here only so an older save can be
	// migrated: EnsurePrefs copies them onto each realm, and nothing reads them
	// afterwards. New code wants Empire.Prefs.
	EnterExitsBuy  bool
	DepositEndTurn bool
	AutoPayMaint   bool
	AutoFeed       bool
	VisitCovert    bool
	VisitTrading   bool
	VisitMessage   bool

	Today string `json:"-"` // ISO date for this session

	// unroutableSink absorbs a payload addressed to a board the roster cannot
	// place, and unroutableNoted keeps the sysop notice to one per board per run
	// rather than one per payload. Both are unexported, so neither is saved.
	// See outboxFor.
	unroutableSink  Packet
	unroutableNoted map[string]bool

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

// ResetForNewSeason wipes this board's world and starts it over, keeping only
// what identifies the board in its league — the roster, the keys, the season
// count and the packet history that stops an old order being replayed. Used by
// the Coordinator's league-wide reset (#65); a stand-alone board resets through
// -reset instead.
func (w *World) ResetForNewSeason(startDate string) {
	nodes, key, pub := w.LeagueNodes, w.CoordKey, w.CoordPub
	season, outSeq, high, seen := w.Season, w.OutSeq, w.HighSeq, w.SeenPackets
	outbox, transit := w.Outbox, w.Transit
	// The bulletins on disk survive a season, so their fingerprints have to as
	// well: forgetting them would file every one of them as newly posted.
	digest, known := w.BulletinDigest, w.BulletinsKnown
	w.initFreshGame()
	w.BulletinDigest, w.BulletinsKnown = digest, known
	w.LeagueNodes, w.CoordKey, w.CoordPub = nodes, key, pub
	w.Season, w.OutSeq, w.HighSeq, w.SeenPackets = season, outSeq, high, seen
	w.Outbox, w.Transit = outbox, transit // mail for the other boards must still go out
	w.StartedDate = startDate
	w.LastMaintDate = startDate
	w.seedAIEmpires() // a no-op on a league board, which never has any
}

// initFreshGame installs a brand-new game's state onto w, keeping only its
// infrastructure (mutex, rng, store) and Config. It is the SINGLE definition of
// what a fresh game contains, called both at world creation (NewWorldSeed) and
// on -reset (Reset). Keeping it in one place means a default can never
// be seeded at creation but forgotten on reset — the drift that stranded the
// old prices (and would silently carry pirates, news, and the master across a
// reset). Add any new creation-time world default here, not in NewWorldSeed.
func (w *World) initFreshGame() {
	w.Empires = nil
	w.Prices = defaultPrices()
	w.GameDay = 0
	w.InvestRate = DefaultInvestRate
	w.FoodMarketSupply = FoodMarketDailySupply
	w.RefundPool = QueenRefundPoolSeed
	w.LastMaintDate = ""
	w.LastMaintRun = ""
	w.StartedDate = ""
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
	w.Transit = nil
	w.SpyDatabase = nil
	w.LeagueDiplomacy = ""
	w.PlanetDiplomacy = nil
	// A fresh world has no bulletins, so both scopes start KNOWN and empty: the
	// silent baseline in RecordBulletins exists for a save made before bulletins
	// did, not for a board whose first bulletin is genuine news.
	w.BulletinDigest = nil
	w.BulletinsKnown = map[string]bool{"league": true, "local": true}
	// The world's own copies are migration input for older saves (see
	// prefs.go). A fresh world has no realm to migrate, so they simply hold the
	// same defaults every new realm is founded on.
	d := DefaultPrefs()
	w.VisitCovert, w.VisitTrading, w.VisitMessage = d.VisitCovert, d.VisitTrading, d.VisitMessage
	w.EnterExitsBuy, w.DepositEndTurn = d.EnterExitsBuy, d.DepositEndTurn
	w.AutoPayMaint, w.AutoFeed = d.AutoPayMaint, d.AutoFeed
	w.seedAIEmpires()
	w.Pirates = nil
	w.EnsurePirates() // a no-op when the sysop has turned pirates off
}

// EnsureInvestRate repairs InvestRate after loading a save that predates
// investments (InvestRate zero), and converts one written while the rate was a
// whole percent. The two units cannot be confused: the old band was 1..25 and
// the new one starts at 35, so anything below the floor is a percent figure
// that wants scaling by ten.
func (w *World) EnsureInvestRate() {
	if w.InvestRate == 0 {
		w.InvestRate = DefaultInvestRate
		return
	}
	if w.InvestRate < MinInvestRate {
		w.InvestRate *= 10
	}
	w.InvestRate = min(max(w.InvestRate, MinInvestRate), MaxInvestRate)
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

// PlanetTotals computes today's totals live, over the empires that exist
// right now. rollNews only freezes a BulletinToday.Totals snapshot at daily
// maintenance, so a board still on its first game day (or any realm created
// since the last maintenance) would otherwise report an empty planet (#109).
func (w *World) PlanetTotals() PlanetTotals { return planetTotals(w) }

// defaultPrices is the world's starting price table, from balance.go. Used at
// world creation and re-applied on -reset, so a reset always installs the
// current prices instead of carrying the old world's stale ones. Each unit starts
// at the centre of its walk band, which is where BRE's mean-reverting walk keeps
// pulling it back to anyway.
func defaultPrices() Prices {
	return Prices{
		Land:    PriceLand,
		Trooper: midPrice(PriceLoTrooper, PriceHiTrooper),
		Jet:     midPrice(PriceLoJet, PriceHiJet),
		Turret:  midPrice(PriceLoTurret, PriceHiTurret),
		Tank:    midPrice(PriceLoTank, PriceHiTank),
		Carrier: midPrice(PriceLoCarrier, PriceHiCarrier),
		Bomber:  midPrice(PriceLoBomber, PriceHiBomber),
	}
}

// Lock/Unlock guard the shared World when a single process runs concurrent
// sessions. The door and -local front-ends run one session and
// take it uncontended.
func (w *World) Lock() { w.mu.Lock() }

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
//
// Debt is NOT deducted: BRE's own function adds the weighted assets and stops
// there (BRE.EXE 0x8F53, read 2026-08-01). BRE also folds in a second per-unit
// count for forces that are away from home, so a realm with a strike in flight
// does not look poorer for it. IB's inter-BBS detachments are simply subtracted
// until they come back, so net worth dips for the round trip (#96).
func (w *World) NetWorth(e *Empire) int {
	// int64 intermediate: e.Land*12500 (and the unit terms) overflow int32 on a
	// 32-bit build for a large realm. Weights are BRE-exact and unchanged; only
	// the arithmetic is widened. Storage/return stay int.
	thou := int64(e.Land) * NetWorthLand
	for _, g := range AllGoods {
		thou += int64(*g.Count(e)) * g.NetWorth
	}
	return int(thou / 1000)
}
