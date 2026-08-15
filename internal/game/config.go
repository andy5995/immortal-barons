package game

import "path/filepath"

// Level is a cost/damage/reward preset, as BRE's Configuration Editor uses
// ([H,M,L,N]). Medium is the baseline. Percent() gives the multiplier applied
// to the underlying value. None (0×) is valid only for the cost knobs; the
// damage/reward knobs use only High/Medium/Low.
type Level int

const (
	Medium Level = iota // baseline (zero value so old saves default here)
	Low
	High
	None
)

func (l Level) String() string {
	switch l {
	case Low:
		return "Low"
	case High:
		return "High"
	case None:
		return "None"
	default:
		return "Medium"
	}
}

// Percent is the multiplier this level applies, in percent: None 0, Low 50,
// Medium 100, High 200.
func (l Level) Percent() int {
	switch l {
	case None:
		return 0
	case Low:
		return 50
	case High:
		return 200
	default:
		return 100
	}
}

// AttackCapturePct is the share of a beaten defender's regions the winner
// takes at this Attack Rewards level, and AttackRetreatPct the share of its own
// force a side will lose before it breaks off at this Attack Damage level.
// Both are read out of the binary and neither is Percent() applied to a
// baseline — see the tables in balance.go for why that matters.
func (l Level) AttackCapturePct() int {
	switch l {
	case None:
		return AttackCaptureNonePct
	case Low:
		return AttackCaptureLowPct
	case High:
		return AttackCaptureHighPct
	default:
		return AttackCaptureMediumPct
	}
}

// InterplanetaryLossPct is the share of its own force each side spends before
// breaking off an INTERPLANETARY battle: the attack type's own published rate
// (8/15/20%), rescaled by this Attack Damage level.
//
// BINARY-VERIFIED. BRE.OVR 0x405a8 (unit ovr_03f4a0 +0x1184) reads the same
// config byte +0x181 the local resolver switches on at 0xF85E, but does
// something different with it: it starts from the type's own survival constant
// (0.92 / 0.85 / 0.80) and then leaves it alone at Medium, replaces it with 0.99
// at None, halves the loss at Low ((1-X)/2) and doubles it at High ((1-X)*2).
// So the sysop's Attack Damage knob reaches interplanetary strikes too — IB used
// to apply the type's flat rate whatever the setting.
func (l Level) InterplanetaryLossPct(kindLoss int) int {
	switch l {
	case None:
		return AttackRetreatNonePct
	case Low:
		return kindLoss / 2
	case High:
		if kindLoss*2 > 100 {
			return 100
		}
		return kindLoss * 2
	default:
		return kindLoss
	}
}

func (l Level) AttackRetreatPct() int {
	switch l {
	case None:
		return AttackRetreatNonePct
	case Low:
		return AttackRetreatLowPct
	case High:
		return AttackRetreatHighPct
	default:
		return AttackRetreatMediumPct
	}
}

// CostPercent is the multiplier the two ATTACK/TERRORISM cost levels apply, in
// percent. BRE gives those two their own spread — None 0, Low 20, Medium 100,
// High 300 — rather than the Percent() ladder above; see the balance.go block
// for the binary evidence. Use this for AttackCosts and TerrorCosts, Percent()
// for the rest.
func (l Level) CostPercent() int {
	switch l {
	case None:
		return CostLevelNonePct
	case Low:
		return CostLevelLowPct
	case High:
		return CostLevelHighPct
	default:
		return CostLevelMediumPct
	}
}

// BuyMode controls military purchasing (BRE "Buy Military": Yes/No/Limited).
type BuyMode int

const (
	BuyYes     BuyMode = iota // unlimited purchasing (zero value / BRE default)
	BuyNo                     // no purchasing; players must build via industry
	BuyLimited                // a limited amount is on the market each day
)

func (b BuyMode) String() string {
	switch b {
	case BuyNo:
		return "No"
	case BuyLimited:
		return "Limited"
	default:
		return "Yes"
	}
}

// SlappenheimerMode controls R5-Slappenheimer missile handling (IB's rename of
// BRE's S3-Sabre "Sabre Handling").
type SlappenheimerMode int

const (
	SlappenheimerUserSelect SlappenheimerMode = iota // User Select/Original (BRE default)
	SlappenheimerNone                                // None/Disabled
	SlappenheimerRandom                              // random return
	SlappenheimerConstant                            // constant return
)

func (m SlappenheimerMode) String() string {
	switch m {
	case SlappenheimerNone:
		return "None/Disabled"
	case SlappenheimerRandom:
		return "Random"
	case SlappenheimerConstant:
		return "Constant"
	default:
		return "User Select/Original"
	}
}

// Config holds the game rules. The first block is per-board; the rest mirror
// BRE's Configuration Editor (Image #15) and, in a league, are set by the
// Coordinator and broadcast to every board (see LeagueConfig). Defaults are
// from BRE's reset-init code and Configuration Help screens.
type Config struct {
	// Per-board (not part of the league ruleset).
	AICount int
	DataDir string
	IBBS    bool // participate in inter-BBS play (gates the interplanetary menus)

	// This board's own identity and packet directories. Stored in bbs.cfg, not
	// config.json — a Coordinator's broadcast rewrites config.json, and these are
	// nobody's business but this sysop's. See store.BoardConfigFile.
	BoardID      string         `json:"-"` // name of this board in exported inter-BBS packets
	InboundDir   string         `json:"-"` // inter-BBS packets arrive here (RunPlanetary reads them); relative to DataDir
	OutboundDir  string         `json:"-"` // inter-BBS packets are written here for the transport to move; relative to DataDir
	OutboundDirs map[int]string `json:"-"` // per-neighbour override of OutboundDir, keyed by roster node number (#106)
	LeagueNumber int            `json:"-"` // the Coordinator's league number, 1-999; tells two leagues apart in one inbound directory

	IdleTimeoutSecs int // boot a session after this many seconds with no keypress (0 = never), freeing the world lock
	MaxIdleWarnings int // idle warnings a session may collect before a hard boot

	// League ruleset (BRE Configuration Editor fields).
	GameStartDate        string // ISO date the game begins; before it, maintenance doesn't advance ("" = already started)
	JoinDate             string // ISO date after which no new player may join ("" = no cutoff)
	TurnsPerDay          int    // turns each player gets per day
	ProtectionTurns      int    // New Realm Protection length
	GameLength           int    // days before the league ends and resets; 0 = endless
	IdleDaysRemove       int    // days a realm may go unplayed before it is removed; 0 = never
	InitialMarketLand    int    // land on the market at reset
	LandPerDay           int    // land added to the market each day
	MoneyCapBillions     int    // most gold a realm may hold, on hand and again in the bank, in whole billions (BRE's own limit is 2)
	InterestRate         int    // bank interest (BRE: % over 10 days; 200 = 20%/day)
	StdInvestRate        int    // standard investment rate (BRE: % over 10 days)
	SteadyInvest         bool   // steady (fixed) investment rate instead of floating
	FoodUnlimited        bool   // food market has no daily supply limit (BRE "Food Unlimited"; default false = limited)
	MaxTaxRate           int    // highest tax rate a player may set (IB divergence: BRE caps nothing)
	PlanetaryTaxRate     int    // crown tax on each turn's gold income, as a whole percent (5 = 5%)
	MaxRegions           int    // most regions a player may own
	MaxIndividualAttacks int    // most individual (conventional) attacks a player may launch per day; 0 = unlimited (BRE "Maximum Individual Attacks Per Day")
	MaxGroupAttacks      int    // most group (interplanetary) attacks a player may join or lead per day; 0 = unlimited
	MaxTerrorOps         int    // most terrorist ops a player may launch per day; 0 = unlimited
	MaxBombingOps        int    // most bombing ops a player may launch per day; 0 = unlimited
	LostForcesDays       int    // days before a strike with no result packet gives its forces back (#96); 0 = never
	// The three league-play policies BRE groups with the switches above. All
	// default ON except LocalAttackScoring, which BRE ships Disabled and its help
	// recommends leaving there. They mean nothing off a league, so every use
	// checks IBBS first.
	LocalAttacks       bool // barons on this board may attack each other in a league game
	LocalAttackScoring bool // ... and winning one moves score
	DupeChecking       bool // a baron playing on another board in the league is locked out here

	// Pirates is IB's own switch, not one of BRE's: the original always has its
	// nine factions. A raid never crosses a planet, but the setting is still part of
	// the league ruleset the Coordinator broadcasts — a league where some planets
	// are robbed and others are not is not a fair one. It is NOT an inter-BBS
	// option, so it carries no star and a stand-alone board is asked it too.
	Pirates bool // the pirate factions exist, raid, and can be raided

	BombingOps            bool              // the four bombing ops are offered (Bomb Enemy Targets, Special Operations)
	MissileOps            bool              // nuclear/chemical/biological strikes are offered (Attack, Bomb Enemy Targets, Special Operations)
	ClingyAnnihilator     bool              // the doomsday weapon is offered
	MaxPlayers            int               // most human empires per board (0 = unlimited)
	BuyMilitary           BuyMode           // Yes / No / Limited
	MaintCosts            Level             // maintenance costs (regions + forces)
	TradeCosts            Level             // trade-deal costs
	RegionCosts           Level             // region purchase price
	AttackCosts           Level             // gold an attack costs to launch
	TerrorCosts           Level             // gold a terrorist op costs to launch
	AttackDamage          Level             // damage attacks inflict (never None)
	AttackRewards         Level             // land/goods gained from a win (never None)
	SlappenheimerHandling SlappenheimerMode // R5-Slappenheimer missile handling
}

// Config-editor upper bounds, from BRE's Configuration Help screens, which show
// each field as "(default; max)". Confirmed by Andy against the game.
const (
	MaxTurnsPerDay        = 20    // turns-per-day ceiling (default 8)
	MaxProtectionTurns    = 200   // Turns of Protection ceiling (default 15)
	MaxIdleDaysRemove     = 365   // Idle-removal ceiling (default 10; 0 = never remove)
	MaxLandPerDay         = 5000  // Daily Land Creation ceiling (default 1000)
	MaxInitialMarketLand  = 50000 // Initial Market Land ceiling (default 0)
	MaxPurchasableRegions = 10000 // Max Purchasable Regions ceiling (default 500)
	// MaxPlayersPerBoard is the ceiling for Max Players Per BBS. A realm is
	// addressed by a LETTER on every screen that lists one — See Scores, the
	// attack picker, the recipient picker, Relations — and **Z is reserved for
	// "All"** in the pickers ("(A-Y,Z=All,?=List) Send to:"). So realms may take
	// A..Y only, which is 25 and is why the default has always been 25. The
	// editor used to allow 100000, letting a sysop configure a board whose realms
	// could not all be named — the picker silently truncated its roster at 25.
	MaxPlayersPerBoard  = 25
	MaxPlanetaryTaxRate = 20 // Planetary (crown) Tax Rate ceiling, whole percent (default 5)
	MaxLostForcesDays   = 30 // "Days before lost forces returned" ceiling (default 3; 0 = never return)
	MaxPlayerTaxRate    = 50 // ceiling for MaxTaxRate (IB's own cap; BRE has none, its prompt is [0-100])
	// The two bank rates are stated over ten days, so the value is tenths of a
	// percent per day. Both ranges are BRE's own, read off its config-help
	// screens: "(50; 200)" and "(35; 100)". The floors matter — a league set to
	// zero has a bank that pays nothing, which the original never offers.
	MinBankInterest  = 50  // Bank Interest Rate floor (5.0%/day)
	MaxBankInterest  = 200 // ... and ceiling (20%/day); default 50
	MinStdInvestRate = 35  // Standard Investment Rate floor (3.5%/day)
	MaxStdInvestRate = 100 // ... and ceiling (10%/day); default 35
)

// MaxLeagueNumber is the League Number ceiling. Not from a Configuration Help
// screen like the bounds above — BRE asks for it during InterBBS setup, and the
// range is stated in its BBS.CFG documentation ("any number between 1 and 999").
// 0 means the number was never set.
const MaxLeagueNumber = 999

// InterBBSEnabled reports whether inter-BBS / interplanetary features (group
// attacks, IP scores, travel times, terrorist ops, planetary Gooie/SDI) should
// be offered. In BRE these live on a separate "InterPlanetary Operations" menu
// that is "only for InterBBS Games", so the sole gate is the IBBS flag. A timed
// single-BBS league (GameLength > 0) is not interplanetary: there are no other
// planets to attack or travel to.
func (c Config) InterBBSEnabled() bool {
	return c.IBBS
}

// Inbound and Outbound are the packet directories as paths on disk. A relative
// setting is relative to the DATA directory, not to the working directory: a
// door is launched from wherever the BBS happens to put it (a node temp dir on
// Mystic), so anything relative to the CWD lands somewhere different on every
// call. An absolute setting is used as given, for a board whose transport drops
// packets outside the data directory.
func (c Config) Inbound() string { return c.resolveDir(c.InboundDir) }

// Outbound is Inbound's counterpart; see it for how a path is resolved.
func (c Config) Outbound() string { return c.resolveDir(c.OutboundDir) }

// OutboundLink is this board's own link to one neighbour, and reports whether
// one is configured at all. Only a board that HOSTs others needs any: everything
// a leaf board sends goes to its uplink, which is the plain Outbound directory.
func (c Config) OutboundLink(node int) (string, bool) {
	if dir, ok := c.OutboundDirs[node]; ok && dir != "" {
		return c.resolveDir(dir), true
	}
	return "", false
}

func (c Config) resolveDir(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(c.DataDir, p)
}

// GameStarted reports whether the game has begun as of ISO date `today`. An
// empty GameStartDate (the default) means it started immediately. ISO dates
// sort chronologically as strings, so a plain comparison works.
func (c Config) GameStarted(today string) bool {
	return c.GameStartDate == "" || today >= c.GameStartDate
}

// JoinOpen reports whether a new player may still join as of ISO date `today`.
// An empty JoinDate means joining is always open.
func (c Config) JoinOpen(today string) bool {
	return c.JoinDate == "" || today <= c.JoinDate
}

func DefaultConfig() Config {
	return Config{
		AICount:         0,
		DataDir:         "./data",
		BoardID:         "local",
		InboundDir:      "inbound",
		OutboundDir:     "outbound",
		IdleTimeoutSecs: 300,
		MaxIdleWarnings: 3,

		// Defaults from BRE's reset-init code and Configuration Help screens,
		// except TurnsPerDay, raised from BRE's 8 to 10 for the modern door.
		TurnsPerDay:           10,
		ProtectionTurns:       15,
		GameLength:            0,
		IdleDaysRemove:        10,
		InitialMarketLand:     0,
		LandPerDay:            1000,
		MoneyCapBillions:      MoneyCapMinBillions, // BRE's own ceiling; a sysop may raise it to MoneyCapMaxBillions
		InterestRate:          50,
		StdInvestRate:         35,
		SteadyInvest:          false,
		MaxTaxRate:            50,
		PlanetaryTaxRate:      5,
		MaxRegions:            500,
		MaxIndividualAttacks:  3, // Andy's choice for the modern door; BRE has the setting but its stock default is unverified
		MaxGroupAttacks:       4,
		MaxTerrorOps:          25,
		MaxBombingOps:         4,
		LostForcesDays:        3,
		Pirates:               true, // the original has no switch; on is what it does
		BombingOps:            true,
		MissileOps:            true,
		ClingyAnnihilator:     true,
		LocalAttacks:          true,  // BRE's captured default, and its help "highly recommends" it
		LocalAttackScoring:    false, // BRE ships this one Disabled, so score can't be farmed at home
		DupeChecking:          true,  // BRE's captured default
		MaxPlayers:            25,
		BuyMilitary:           BuyYes,
		MaintCosts:            Medium,
		TradeCosts:            Medium,
		RegionCosts:           Medium,
		AttackCosts:           Medium,
		TerrorCosts:           Medium,
		AttackDamage:          Medium,
		AttackRewards:         Medium,
		SlappenheimerHandling: SlappenheimerUserSelect,
	}
}

// MoneyCap is the most gold a realm may hold, on hand or in the bank — the
// sysop's MoneyCapBillions knob, in gold.
func (w *World) MoneyCap() int64 { return int64(w.MoneyCapBillions()) * GoldPerBillion }

// MoneyCapBillions is the cap in the unit the sysop sets it in, clamped to the
// legal range. Clamped on read rather than on load so a config written before
// the knob existed (or edited by hand to nonsense) still yields a legal cap: a
// zero or missing field comes back as BRE's own 2 billion.
func (w *World) MoneyCapBillions() int {
	b := w.Config.MoneyCapBillions
	if b < MoneyCapMinBillions {
		b = MoneyCapMinBillions
	}
	if b > MoneyCapMaxBillions {
		b = MoneyCapMaxBillions
	}
	return b
}
