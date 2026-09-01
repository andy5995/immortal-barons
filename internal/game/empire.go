package game

import "math"

// empire.go — a realm: everything one holds, and the figures read straight off
// it. The rules that CHANGE a realm live with their mechanic; what is here is
// the record and the repairs a loaded save needs.

type Empire struct {
	Name string
	// Prefs are this player's own menu settings; see prefs.go. Held per realm
	// rather than per board.
	Prefs Prefs
	// FormerName is the name this realm carried before its one permitted rename
	// (see RenameEmpire), and "" for a realm that has never been renamed — which
	// is also what makes the rename once-only. It stays readable for the realm's
	// whole life because packets already in the air are addressed to it; see
	// FindByNameOrFormer.
	FormerName string `json:",omitempty"`
	// PriorNames holds the names a SYSOP rename moved this realm off (#161).
	// They serve the same delivery job FormerName does — packets in the air are
	// addressed to them, and nobody else may claim them — but they are kept
	// apart because FormerName also spends the player's one rename, and a name
	// changed for a player must not cost them theirs.
	PriorNames []string `json:",omitempty"`
	Owner      string   // normalized BBS handle; "" for AI
	// Slot is the realm's permanent place in the planet's fixed roster, 1..25,
	// and the public identity every screen addresses it by: the Id column and
	// every picker letter is 'A' + Slot - 1, so slot 1 is A and slot 25 is Y.
	// It is assigned once at creation and never moves, so a realm keeps its
	// letter for its whole life however many neighbours die or join.
	//
	// BRE stores its empires in a 25-entry array and uses the letter as the
	// index straight into it — the diplomatic relation row is 25 words, "one
	// per empire letter A..Y" (docs/dev/bre-save-format.md), and its rosters
	// print gaps where a slot is empty rather than closing them up.
	//
	// 0 marks a realm saved before slots existed, or one a legacy over-full
	// world left no room for; EnsureSlots settles both on load.
	Slot int
	// DupeLockedBy names the league board that last reported this owner playing
	// there too, or "" when no duplicate is known. See dupe.go; only consulted
	// while Config.DupeChecking is on, so turning the switch off releases
	// everyone without a sweep.
	DupeLockedBy string `json:",omitempty"`
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
	SDI int // 0-100, the shield percentage against strikes from another planet
	// SDIFunding is the gold in the program to date, which is what the original
	// stores and what its upkeep and spending allowance are both figured from.
	// SDI above is derived from it and from the land it has to cover, so gameplay
	// code changes the funding and lets syncSDI follow. See specials.go.
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
	// MissileUsedToday is the once-a-day gate on each of the three Special
	// Operations missiles, keyed by op. BRE keeps them as three separate flag
	// BYTES on the empire record — Nuclear Assault at +0x27e, Chemical Bombing at
	// +0x27f and the S3-Sabre at +0x280 — set when the strike launches (BRE.OVR
	// 0x2a1c4, 0x2a2b1, 0x2a64a), tested when the menu is drawn so a spent
	// missile's item is not listed at all (0x29f9c, 0x29fd0, 0x2a004), and cleared
	// together at daily maintenance (0x08669). They are booleans, NOT a counted
	// allowance: MaxBombingOps governs the four bombing ops and nothing else.
	MissileUsedToday map[SpecialOp]bool `json:",omitempty"`
	// RefundTaken records that this realm has already drawn the Queen's tax
	// refund today (#93), and is cleared with the counters above.
	//
	// BRE spells the same gate as two tests in its recap: no turn part-way
	// through (its turn-stage counter is 0) and none taken today. IB has no
	// equivalent counter to test — TurnProgress is a set of named flags, not
	// BRE's 0-20 stage number — so it records the day's draw directly.
	RefundTaken bool
	// LotteryTaken records that this realm has already been offered the Queen's
	// lottery today, and is cleared with RefundTaken. Set when the offer is made,
	// not when it is accepted: BRE runs the two as one first-play-of-the-day
	// block, so declining a ticket does not bring a second offer that day.
	LotteryTaken bool
	// TurnProgress records which stages of the current turn have already completed,
	// so a turn REPLAYED after an idle-boot skips what was done — no double income
	// or double charge, and no re-showing a menu the player already exited (GH #10).
	// Serialized (NOT json:"-"): surviving a cross-boot process restart is the whole
	// point. Cleared at turn-commit (PlayTurn) and on daily rollover (DailyMaintenance).
	TurnProgress TurnProgress
	Protection   int
	// Score is BRE's cumulative score (shown on the scores board, distinct from
	// Net Worth): += ScorePerTurn once per turn played, plus combat/covert score.
	// Seeded 0, and nothing in the economy takes it away — riots and spoilage do
	// not touch it. Matches BRE live data (a standard realm scored a flat
	// +213/turn, 8 turns = 1704).
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
	CreatedDay int
	LastPlayed string
	// LastActive is the unix time of this baron's most recent menu action, and
	// feeds only the online indicator on the roster screens. Zeroed on a clean
	// session end; see presence.go.
	LastActive int64 `json:"lastActive,omitempty"`
	Events     []Event
	Mail       []Message
	PirateHits []PirateHit // raids suffered since last play; shown in the income report
	// RaidersThisTurn is the pirate-faction slots (PirateHit.Slot) that raided
	// since last play. Set alongside PirateHits at the same recap that drains
	// it, but left standing afterward so the Attack Pirates menu can flag the
	// raider for the rest of the turn; the next recap overwrites it, empty if
	// no new raid landed.
	RaidersThisTurn []int
	// Bribed lists the realms this empire holds a bribed agent inside. It is an
	// OFFENSIVE holding: it doubles this empire's own side of a covert roll
	// against that realm, and it is the list Expose Enemy Ops chooses from. The
	// JSON key is the one it carried when IB read the flag the other way round,
	// as a shield against the bribed realm's ops; the stored content is the same
	// list of realms either way, so old saves load unchanged.
	Bribed []string `json:"ImmuneFrom"`
	// ExposedFrom maps a rival realm's name to the GameDay through which Expose
	// Enemy Ops turns its operations against this empire away. The shield is per
	// realm, never blanket, and it is bought against one realm at a time.
	ExposedFrom map[string]int `json:"exposedFrom,omitempty"`

	// PendingRegions is land captured on ANOTHER planet and waiting for its owner
	// to say what to hold it as. A Regular Attack asks at the moment of victory,
	// but an interplanetary strike resolves days later on a board its owner is not
	// logged in to, so the land is parked here and the picker runs at the start of
	// their next turn (#107). It is real land the realm owns; it just has no type
	// yet, which is why it is counted nowhere until it is allocated.
	PendingRegions int `json:"pendingRegions,omitempty"`

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
	// LastInterest is the savings interest credited at the end of the previous
	// turn and InvestReturnsToday what today's matured investments paid. Both are
	// reported at the START of a turn, so unlike the transients above they have to
	// survive the save: the turn that earns them and the turn that shows them are
	// separate door runs, and the investment payout comes from daily maintenance,
	// which may be a different process again.
	LastInterest       int64 `json:"lastInterest,omitempty"`
	InvestReturnsToday int64 `json:"investReturnsToday,omitempty"`
	// PendingSupportPenalty and PendingMoralePenalty are stat points owed but not
	// yet deducted. BRE accumulates shortfall penalties during the maintenance and
	// food stages in two signed bytes on the empire record (+0x2ba support, +0x2b9
	// morale) and applies them at turn rollover, not on the spot, so the drop
	// surfaces on the next turn's display. Persisted so they survive a mid-turn
	// save.
	PendingSupportPenalty int `json:"pendingSupportPenalty,omitempty"`
	PendingMoralePenalty  int `json:"pendingMoralePenalty,omitempty"`
	// CivilWarSeverity is the percentage a pending civil war will destroy, filed
	// by a severe food shortfall (BRE empire record +0x2bb) and spent at rollover.
	CivilWarSeverity    int   `json:"civilWarSeverity,omitempty"`
	LastCivilWar        int   `json:"-"` // severity of the civil war that fired this turn, 0 if none
	LastRiot            bool  `json:"-"`
	LastMoraleDesertion int   `json:"-"`
	MadeTroopers        int   `json:"-"`
	MadeJets            int   `json:"-"`
	MadeTurrets         int   `json:"-"`
	MadeBombers         int   `json:"-"`
	MadeTanks           int   `json:"-"`
	MadeCarriers        int   `json:"-"`
	LastGoldPaid        int64 `json:"-"`
	LastFoodConsumed    int   `json:"-"`
}

// TurnProgress marks the stages of the current turn that have already completed,
// so replaying a turn interrupted by an idle-boot skips what was already done
// (GH #10). Correctness flags (IncomeCollected, MaintPaid, Fed) prevent
// re-applying a resource effect; the rest prevent re-showing a menu the player
// already exited. All are cleared at turn-commit (PlayTurn) and daily rollover.
type TurnProgress struct {
	IncomeCollected bool  // turn-start Manufacture + CollectIncome + regions-cap reset done
	MaintPaid       bool  // paymentStage done (set with the forces/regions charge)
	SDIFunded       int64 // gold put into the SDI program this turn, against its allowance
	Fed             bool  // feedStage done
	CovertDone      bool
	// CovertOpsUsed holds the EFFECT covert operations already run this turn.
	// BRE's "Limit one try per turn!" is keyed per OPERATION, so each item on the
	// menu carries its own slot; info ops (Send Spy, Spy on Relations) never take
	// one. A map rather than a bool is what makes TurnProgress uncomparable, so
	// the whole-struct checks around it use reflect.DeepEqual.
	CovertOpsUsed      map[CovertOp]bool `json:"covertOpsUsed,omitempty"`
	BankDone           bool
	SpendingDone       bool
	AttackDone         bool
	TradingDone        bool
	InterPlanetaryDone bool
	MessageDone        bool
}

// syncLand resyncs the authoritative Land total from Regions. Every place
// that changes an empire's regions must call this afterward. The SDI follows,
// because the shield's strength is its funding spread over the land it covers:
// take land and your shield thins, lose land and it thickens.
func (e *Empire) syncLand() {
	e.Land = e.Regions.Total()
	e.syncSDI()
}

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
	return MoraleCombatFloor + MoraleCombatSlope*morale/100
}

// remoteMoraleFactor is the same idea for an ARRIVING interplanetary strike, and
// the slope is not the same. BINARY-VERIFIED: the local builder multiplies morale
// by Real48 0.6 and adds 50 (BRE.OVR +0x0b72), the invasion builder divides it by
// Real48 175 and adds 0.5 (+0x1040). So a defender at full morale holds at 110%
// against a neighbour and 107% against an invasion.
func remoteMoraleFactor(morale int) int {
	return MoraleCombatFloor + RemoteMoraleSlopeNum*morale/RemoteMoraleSlopeDen
}

// remoteDefense is what a realm brings to an ARRIVING interplanetary strike:
// the same units as Defense(), valued by the invasion resolver's own tank
// constant, and then scaled by morale — which Defense() does not do because its
// callers apply moraleFactor themselves.
func (e *Empire) remoteDefense() int {
	sum := e.Troopers + e.Turrets*2 + remoteTankStrength(e.Tanks, e.HQ)
	wide := int64(techRaise(sum, e.TechMilitaryFactor())) * int64(remoteMoraleFactor(e.Morale)) / 100
	return int(min(wide, math.MaxInt32))
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
