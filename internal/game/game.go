// Package game holds the BRE world: empires, economy, turn engine, and
// combat. The world is persistent and shared; one session is active at a
// time (Active), and dates flow in as ISO strings for testability.
package game

import (
	"fmt"
	"math"
	"math/rand"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Version is the single source of truth for the game's version string,
// displayed in the status bar and reported by the front-ends. The
// in-development version; bumped after each release.
const Version = "0.0.5"

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

	// Money is int64 so a 32-bit door build holds the same range as a 64-bit one:
	// plain int is 32 bits there, which is what capped the treasury at 2 billion
	// and silently discarded anything above it. See MoneyCap.
	Gold    int64
	Bank    int64
	Debt    int64
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

	Tax int
	SDI int // 0-75, percentage reduction of incoming strike damage
	// SDIFunding is the gold in the program to date, which is what the original
	// stores and what its upkeep and spending allowance are both figured from.
	// SDI above is derived from it. See specials.go.
	SDIFunding int64
	HQ         int // 0 = none/not started; 1-100 = percent complete
	// TurnsPlayed is the empire's lifetime turn count. BRE keeps the same counter
	// (record +0x281) and prices the HeadQuarters off it — see World.HQPrice.
	TurnsPlayed int
	Support     int    // 0-100, popular support; erodes with high tax, slashes Coastal income when low
	Morale      int    // 0-100, military morale; low morale weakens combat and causes desertion
	Language    string // help/UI language ("" = English; "de", "ru")

	// LandAvailable is how much unclaimed land this realm may still buy — BRE's
	// "Daily Land Creation" allowance. BINARY-VERIFIED as a PER-EMPIRE field
	// (BRE.OVR 0x12D30 bounds a purchase against it; 0x12EF9 subtracts the number
	// bought), not a shared planet pool: each realm gets its own allowance topped
	// up every day, so nobody races anyone else for land.
	//
	// Config.LandPerDay and InitialMarketLand were editable, shown on the Game
	// Setup screen and broadcast over inter-BBS, but nothing consumed them, so
	// land was infinite. A beaten realm then rebuilt faster than any war could
	// take land from it — a 60-day bot game reached 267,000 regions with not one
	// realm ever conquered.
	LandAvailable int

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
	// The interplanetary allowances, counted and reset the same way as
	// AttacksToday, against Config.MaxGroupAttacks / MaxTerrorOps / MaxBombingOps.
	GroupAttacksToday int
	TerrorOpsToday    int
	BombingOpsToday   int
	// RefundTaken records that this realm has already drawn the Queen's tax
	// refund today (#93), and is cleared with the counters above.
	//
	// BRE spells the same gate as two tests in its recap: no turn part-way
	// through (its turn-stage counter is 0) and none taken today. IB has no
	// equivalent counter to test — TurnProgress is a set of named flags, not
	// BRE's 0-20 stage number — so it records the day's draw directly.
	RefundTaken bool
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
	// TechSlots holds the empire's research, one counter per slot. Only the six
	// slots named in balance.go do anything; the rest are dilution, exactly as in
	// the original. Research NEVER decays — selling Technology regions stops it
	// accumulating and freezes what is banked (see advanceTech).
	TechSlots [TechSlotCount]int
	// CreatedDay is the GameDay this realm was founded. It is what keeps the
	// never-played sweep from erasing a realm the same day it was created — its
	// owner is entitled to the day they joined, and on a multi-node board another
	// caller's login can roll the day over while they are still at the menu.
	CreatedDay       int
	LastPlayed       string
	Events           []Event
	Mail             []Message
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
	// ProdGold is capacity the player has explicitly set aside for gold rather
	// than units. Capacity left unallocated pays out as gold too, so this is a
	// way of reserving it deliberately instead of by subtraction. 0 by default,
	// which leaves an existing empire producing exactly what it did before.
	ProdGold int
	// ProdInitialized distinguishes an empire whose production has been set up
	// (at creation, or by the player) from a pre-feature save whose Prod* are
	// all zero because the fields didn't exist. Without it, a player who sets
	// every percentage to 0 (all output → gold) has it reset to the default on
	// the next reload. See EnsureProduction.
	ProdInitialized bool
	Specialized     string // "" = none, else a unit type name; specialization concentrates output

	// Transient per-turn stats for the end-of-turn report; not persisted.
	LastSpoiled   int `json:"-"`
	LastPopGrowth int `json:"-"`
	LastStarved   int `json:"-"` // people who left this turn from a food shortfall
	// PendingSupportPenalty is popular support owed but not yet deducted. BRE
	// accumulates shortfall penalties during the maintenance sequence and applies
	// them at turn rollover, not on the spot, so the drop surfaces on the next
	// turn's display. Persisted so it survives a mid-turn save.
	PendingSupportPenalty int   `json:"pendingSupportPenalty,omitempty"`
	LastRiot              bool  `json:"-"`
	LastMoraleDesertion   int   `json:"-"`
	MadeTroopers          int   `json:"-"`
	MadeJets              int   `json:"-"`
	MadeTurrets           int   `json:"-"`
	MadeBombers           int   `json:"-"`
	MadeTanks             int   `json:"-"`
	MadeCarriers          int   `json:"-"`
	LastGoldPaid          int64 `json:"-"`
	LastFoodConsumed      int   `json:"-"`
}

// TurnProgress marks the stages of the current turn that have already completed,
// so replaying a turn interrupted by an idle-boot skips what was already done
// (GH #10). Correctness flags (IncomeCollected, MaintPaid, Fed) prevent
// re-applying a resource effect; the rest prevent re-showing a menu the player
// already exited. All are cleared at turn-commit (PlayTurn) and daily rollover.
type TurnProgress struct {
	IncomeCollected    bool  // turn-start Manufacture + CollectIncome + regions-cap reset done
	MaintPaid          bool  // paymentStage done (set with the forces/regions charge)
	SDIFunded          int64 // gold put into the SDI program this turn, against its allowance
	Fed                bool  // feedStage done
	CovertDone         bool
	CovertOpUsed       bool // an EFFECT covert op ran this turn (BRE's one-per-turn cap; info ops exempt)
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
		e.ProdTroopers, e.ProdJets, e.ProdTurrets = DefaultProdTroopersPct, DefaultProdJetsPct, DefaultProdTurretsPct
		e.ProdBombers, e.ProdTanks, e.ProdCarriers = DefaultProdBombersPct, DefaultProdTanksPct, DefaultProdCarriersPct
		e.ProdGold = DefaultProdGoldPct
	}
}

// Offense is the empire's full attack strength — every usable unit committed (see
// FullForce/AttackForce.offense). A regular attack may commit less (force select).
func (e *Empire) Offense() int {
	return FullForce(e).groundOffense(e)
}

func (e *Empire) Defense() int {
	sum := e.Troopers + e.Turrets*2 + tankStrength(e.Tanks, e.HQ)
	return techRaise(sum, e.TechMilitaryFactor())
}

// techFactor is the multiplier slot `slot` currently grants, given that effect's
// ceiling in hundredths (150 == x1.50), returned in TechFactorUnit fixed point.
//
// BRE: factor = 1 + (cap-1) * (1 - exp(-level / (totalRegions+1))). The
// denominator is why expanding dilutes technology, and why the same research
// reads as more effective in a smaller realm.
func (e *Empire) techFactor(slot, capPct int) int {
	lvl := e.TechSlots[slot]
	total := e.Regions.Total()
	if lvl <= 0 || capPct <= 100 || total < 0 {
		return TechFactorUnit
	}
	x := float64(lvl) / float64(total+1)
	if x > TechExpClamp {
		x = TechExpClamp
	}
	f := 1 + (float64(capPct)/100-1)*(1-math.Exp(-x))
	return int(f * TechFactorUnit)
}

// The eight Technology effects. Three of them share slot 0 with different
// ceilings, exactly as the original does.
func (e *Empire) TechGoldFactor() int  { return e.techFactor(TechSlotGold, TechCapGold) }
func (e *Empire) TechFoodFactor() int  { return e.techFactor(TechSlotGold, TechCapFood) }
func (e *Empire) TechUnitFactor() int  { return e.techFactor(TechSlotGold, TechCapUnits) }
func (e *Empire) TechSDIFactor() int   { return e.techFactor(TechSlotSDI, TechCapSDI) }
func (e *Empire) TechTaxFactor() int   { return e.techFactor(TechSlotTax, TechCapTax) }
func (e *Empire) TechMaintFactor() int { return e.techFactor(TechSlotMaint, TechCapMaint) }
func (e *Empire) TechDecayFactor() int { return e.techFactor(TechSlotDecay, TechCapDecay) }
func (e *Empire) TechMilitaryFactor() int {
	return e.techFactor(TechSlotMilitary, TechCapMilitary)
}

// techRaise scales v up by a Technology factor; techLower scales it down (used
// for costs and decay, which the original divides by the factor rather than
// multiplying).
// int64 intermediates: both multiply by a factor around 10,000, so any v past
// ~215,000 overflows int32 on a 32-bit build. Region upkeep reaches that at 235
// regions — one report on a 32-bit door showed a 710-region realm billed
// -210,763 gold. The 64-bit build was never affected, which is why it survived
// so long. Same for NetWorth (see World.NetWorth).
func techRaise[T number](v T, factor int) T { return T(int64(v) * int64(factor) / TechFactorUnit) }

// number is the numeric width these scaling helpers accept: plain counts (int)
// and money (int64). One generic body keeps the int64 intermediate — and so the
// 32-bit correctness the comment above describes — for both.
type number interface{ ~int | ~int64 }

// pctOf is v * p / 100 with an int64 intermediate, for a percentage taken of a
// quantity that can reach money scale. A plain v*p/100 overflows int32 on a
// 32-bit build once v passes ~21 million, which turns a rich empire's spending
// budget negative and makes it buy nothing at all.
func pctOf[T number](v T, p int) T { return T(int64(v) * int64(p) / 100) }

// goldCost is the price of n units at `unit` gold each, in money width. Always
// int64: a large order at a high price passes 2^31, which on a 32-bit door
// wrapped the bill negative and handed the goods over for free.
func goldCost(n, unit int) int64 { return int64(n) * int64(unit) }

// unitsAffordable is how many units at `price` gold each `gold` buys. The divide
// happens in money width; the count comes back in count width, clamped so a vast
// treasury against a cheap unit cannot wrap int on a 32-bit door.
func unitsAffordable(gold int64, price int) int {
	if price <= 0 {
		return 0
	}
	return int(min(gold/int64(price), math.MaxInt32))
}
func techLower[T number](v T, factor int) T { return T(int64(v) * TechFactorUnit / int64(factor)) }

// TechPercent renders a factor the way the in-game advisor does: a raised effect
// as round(100*f), a lowered one as round(100/f).
func TechPercent(factor int, lowered bool) int {
	if lowered {
		return (TechFactorUnit*100 + factor/2) / factor
	}
	return (factor*100 + TechFactorUnit/2) / TechFactorUnit
}

// advanceTech runs one turn of research. Called once per turn played.
//
// BRE-verified: points are quadratic in Technology regions and only
// inverse-linear in realm size, so a dense tech block in a small realm
// out-researches the same block in a large one. Each point lands in a slot
// chosen uniformly at random, and nine of the fifteen slots do nothing — the
// waste is the mechanic, not a bug. A Technology Agreement partner adds an
// unmultiplied contribution capped by whichever of the two holds less tech.
//
// With no Technology regions this returns immediately: research stops and the
// banked levels FREEZE. They are never decremented anywhere.
func (w *World) advanceTech(e *Empire) {
	tech := e.Regions.Technology
	total := e.Regions.Total()
	if tech <= 0 || total <= 0 {
		return
	}
	points := TechResearchMul * techResearchPoints(tech, total)
	for _, ally := range w.alliesOf(e, "Technology Agreement") {
		points += techResearchPoints(min(tech, ally.Regions.Technology), total)
	}
	for range points {
		e.TechSlots[w.rng.Intn(TechSlotCount)]++
	}
}

// techResearchPoints is round((n^2 / total)^0.75), the original's own shape.
func techResearchPoints(n, total int) int {
	if n <= 0 || total <= 0 {
		return 0
	}
	return int(math.Round(math.Pow(float64(n)*float64(n)/float64(total), 0.75)))
}

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
	// Protected marks a realm still under New Realm Protection, so the boards
	// that read this packet can leave it off their target lists — the same
	// courtesy a local attack list extends. It can go stale in transit (a strike
	// takes days, protection counts down), so the target board still refuses an
	// arriving strike on its own authority; this only stops a baron spending
	// forces on a target that was already known to be untouchable. Absent from a
	// packet written before this field existed, which reads as unprotected.
	Protected bool `json:",omitempty"`
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

	Alliances     []string // legacy (pre-typed-treaties); migrated by EnsureTreaties
	Treaties      []Treaty
	LastMaster    string // crowned at league end (endGame); shown as "Last Planetary Master"
	CurrentMaster string // daily net-worth leader, tracked by postMasterNews
	RemoteBoards  []RemoteBoard
	Pirates       []PirateFaction

	// TravelTimes is the average packet round trip to each other board, in days,
	// and LastTravelPing the game day the probes for it last went out — see
	// ibbs_travel.go.
	TravelTimes    map[string]float64
	LastTravelPing string

	// Market holds every empire's listings on the general Trading Market (#17);
	// listed goods are escrowed out of the owner's inventory. MarketProceeds
	// accrues each seller's unpaid sale gold, deposited at daily maintenance.
	Market         []MarketListing
	MarketProceeds map[string]int64

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

	// League packet authentication (#53). CoordKey is the ed25519 private key,
	// held only by the Coordinator's board; CoordPub is the matching public key,
	// held by every board. Both load from files rather than saving with the
	// world, so a world file that leaks does not leak the league's key.
	CoordKey []byte `json:"-"`
	CoordPub []byte `json:"-"`
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
	Routes          []RouteRule  `json:"-"` // this board's routing overrides, loaded from ibroute.cfg at startup
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
// ResetForNewSeason wipes this board's world and starts it over, keeping only
// what identifies the board in its league — the roster, the keys, the season
// count and the packet history that stops an old order being replayed. Used by
// the Coordinator's league-wide reset (#65); a stand-alone board resets through
// -reset instead.
func (w *World) ResetForNewSeason(startDate string) {
	nodes, key, pub := w.LeagueNodes, w.CoordKey, w.CoordPub
	season, outSeq, high, seen := w.Season, w.OutSeq, w.HighSeq, w.SeenPackets
	outbox, transit := w.Outbox, w.Transit
	w.initFreshGame()
	w.LeagueNodes, w.CoordKey, w.CoordPub = nodes, key, pub
	w.Season, w.OutSeq, w.HighSeq, w.SeenPackets = season, outSeq, high, seen
	w.Outbox, w.Transit = outbox, transit // mail for the other boards must still go out
	w.StartedDate = startDate
	w.LastMaintDate = startDate
	w.seedAIEmpires() // a no-op on a league board, which never has any
}

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
	// Preferences, in the order the Preferences menu lists them. These are IB's
	// defaults, not BRE's: BRE opens with the three Visit menus and the two buy/
	// deposit toggles on and the two automations off, so an untouched realm walks
	// through every optional menu and answers the same maintenance and food
	// prompts by hand each turn. IB starts with the walk-through menus off and
	// the automations on, leaving the prompts to players who turn them back on.
	w.VisitCovert = false
	w.VisitTrading = false
	w.VisitMessage = false
	w.EnterExitsBuy = false
	w.DepositEndTurn = true
	w.AutoPayMaint = true
	w.AutoFeed = true
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
	e := newEmpire(name, "", w.Config, w.GameDay)
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
//
// A league board gets none, ever, and adds none later: an inter-BBS game is
// played between the boards' human realms, and a computer baron would take a
// share of the planet's standing in a league it cannot be held accountable in.
// The guard is here rather than only in the editor so no path — reset, seeding,
// or a later injection — can slip one in.
func (w *World) AddAIEmpires(n int) int {
	if w.Config.IBBS {
		return 0
	}
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

func newEmpire(name, owner string, cfg Config, day int) *Empire {
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
		CreatedDay: day,
		// A new realm opens with one day's land allowance already granted, plus
		// whatever the sysop seeded the market with, so it can expand on day one
		// before its first Daily Land Creation arrives.
		LandAvailable: cfg.InitialMarketLand + cfg.LandPerDay,
		// The full pool goes to units, split so carrier output matches jet lift
		// (see DefaultProdTroopersPct); BRE defaults to 90% units, 10% gold.
		ProdTroopers: DefaultProdTroopersPct, ProdJets: DefaultProdJetsPct, ProdTurrets: DefaultProdTurretsPct,
		ProdBombers: DefaultProdBombersPct, ProdTanks: DefaultProdTanksPct, ProdCarriers: DefaultProdCarriersPct,
		ProdGold:        DefaultProdGoldPct,
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
	e := newEmpire(realm, strings.ToLower(strings.TrimSpace(handle)), w.Config, w.GameDay)
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

// removeUnplayedEmpires erases a human realm whose owner created it and then
// never took a single turn. It is removed outright rather than left as a husk,
// so the same caller can build a fresh realm normally the next time they show
// up — nothing is held against them.
//
// Governed by Config.IdleDaysRemove: 0 disables realm removal entirely.
//
// Only human realms are eligible: AI barons are driven by aiPlay and always have
// LastPlayed set, and an empty Owner is what marks a baron as AI.
//
// The timing works because onboarding runs AFTER maintenance in the login flow
// (internal/play): a realm created today survives to its first turn, and it is
// the NEXT day's maintenance that collects it if nothing was ever played. For
// the same reason this must only run on a day that actually advances, never on
// the already-ran-today path, or a second login would erase a realm the player
// had not had a chance to play yet.
func (w *World) removeUnplayedEmpires() {
	// One switch governs whether this board reaps realms at all: a sysop who sets
	// Config.IdleDaysRemove to 0 ("never remove") keeps abandoned AND never-played
	// realms alike.
	if w.Config.IdleDaysRemove <= 0 {
		return
	}
	kept := w.Empires[:0]
	for _, e := range w.Empires {
		if e.Owner != "" && e.LastPlayed == "" && w.GameDay > e.CreatedDay {
			continue
		}
		kept = append(kept, e)
	}
	w.Empires = kept
}

// removeIdleEmpires removes a human realm nobody has played for
// Config.IdleDaysRemove days, so an abandoned realm does not sit in the
// rankings forever or grow the world file without bound (#83). 0 means never
// remove, matching how GameLength already treats 0 as endless.
//
// The realm is removed outright, like a never-played one, so the same caller may
// build fresh whenever they return. Its land is NOT returned to anyone's Daily
// Land Creation allowance: allowances are per-empire, so there is no shared pool
// for it to go back to.
//
// AI barons are exempt — they are played by aiPlay every day by definition, and
// an empty Owner is what marks a baron as AI.
// `today` is the REAL date maintenance is running for, not the game clock: a
// human's LastPlayed is stamped with the real date they played, so "unplayed for
// N days" is measured in real days away from the board. The two can differ,
// since the game clock advances at most one day per real day.
func (w *World) removeIdleEmpires(today string) {
	days := w.Config.IdleDaysRemove
	if days <= 0 {
		return
	}
	cutoff, err := time.Parse("2006-01-02", today)
	if err != nil {
		return // unparseable clock: remove nobody rather than guess
	}
	limit := cutoff.AddDate(0, 0, -days).Format("2006-01-02")
	kept := w.Empires[:0]
	for _, e := range w.Empires {
		if e.Owner != "" && e.LastPlayed != "" && e.LastPlayed < limit {
			w.postNews(fmt.Sprintf("%s has been abandoned by its ruler and fades from the map.", e.Name))
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
	thou := int64(e.Land)*NetWorthLand +
		int64(e.Troopers)*NetWorthTrooper + int64(e.Jets)*NetWorthJet + int64(e.Turrets)*NetWorthTurret + int64(e.Bombers)*NetWorthBomber +
		int64(e.Agents)*NetWorthAgent + int64(e.Tanks)*NetWorthTank + int64(e.Carriers)*NetWorthCarrier
	return int(thou / 1000)
}
