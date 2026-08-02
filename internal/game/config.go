package game

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
	AICount         int
	DataDir         string
	BoardID         string // name of this board in exported inter-BBS packets
	IBBS            bool   // participate in inter-BBS play (gates the interplanetary menus)
	InboundDir      string // inter-BBS packets arrive here (RunPlanetary reads them)
	OutboundDir     string // inter-BBS packets are written here for the transport to move
	IdleTimeoutSecs int    // boot a session after this many seconds with no keypress (0 = never), freeing the world lock
	MaxIdleWarnings int    // idle warnings a session may collect before a hard boot

	// League ruleset (BRE Configuration Editor fields).
	GameStartDate         string            // ISO date the game begins; before it, maintenance doesn't advance ("" = already started)
	JoinDate              string            // ISO date after which no new player may join ("" = no cutoff)
	TurnsPerDay           int               // turns each player gets per day
	ProtectionTurns       int               // New Realm Protection length
	GameLength            int               // days before the league ends and resets; 0 = endless
	IdleDaysRemove        int               // days a realm may go unplayed before it is removed; 0 = never
	InitialMarketLand     int               // land on the market at reset
	LandPerDay            int               // land added to the market each day
	InterestRate          int               // bank interest (BRE: % over 10 days; 200 = 20%/day)
	StdInvestRate         int               // standard investment rate (BRE: % over 10 days)
	SteadyInvest          bool              // steady (fixed) investment rate instead of floating
	FoodUnlimited         bool              // food market has no daily supply limit (BRE "Food Unlimited"; default false = limited)
	MaxTaxRate            int               // highest tax rate a player may set (IB divergence: BRE caps nothing)
	PlanetaryTaxRate      int               // crown tax on each turn's gold income, as a whole percent (5 = 5%)
	MaxRegions            int               // most regions a player may own
	MaxIndividualAttacks  int               // most individual (conventional) attacks a player may launch per day; 0 = unlimited (BRE "Maximum Individual Attacks Per Day")
	MaxGroupAttacks       int               // most group (interplanetary) attacks a player may join or lead per day; 0 = unlimited
	MaxTerrorOps          int               // most terrorist ops a player may launch per day; 0 = unlimited
	MaxBombingOps         int               // most bombing ops a player may launch per day; 0 = unlimited
	LostForcesDays        int               // days before a strike with no result packet gives its forces back (#96); 0 = never
	BombingOps            bool              // Bomb Enemy Targets is offered
	MissileOps            bool              // nuclear/chemical/biological strikes are offered
	DoomerKaboomer        bool              // the doomsday weapon may be built
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
	MaxPlanetaryTaxRate   = 20    // Planetary (crown) Tax Rate ceiling, whole percent (default 5)
	MaxLostForcesDays     = 30    // "Days before lost forces returned" ceiling (default 3; 0 = never return)
	MaxPlayerTaxRate      = 50    // ceiling for MaxTaxRate (IB's own cap; BRE has none, its prompt is [0-100])
	MaxBankInterest       = 200   // Bank Interest Rate ceiling (default 50; 200 = 20%/day)
	MaxStdInvestRate      = 100   // Standard Investment Rate ceiling (default 35; 100 = 10%/day)
)

// InterBBSEnabled reports whether inter-BBS / interplanetary features (group
// attacks, IP scores, travel times, terrorist ops, planetary Gooie/SDI) should
// be offered. In BRE these live on a separate "InterPlanetary Operations" menu
// that is "only for InterBBS Games", so the sole gate is the IBBS flag. A timed
// single-BBS league (GameLength > 0) is not interplanetary: there are no other
// planets to attack or travel to.
func (c Config) InterBBSEnabled() bool {
	return c.IBBS
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
		InboundDir:      "./data/inbound",
		OutboundDir:     "./data/outbound",
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
		BombingOps:            true,
		MissileOps:            true,
		DoomerKaboomer:        true,
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
