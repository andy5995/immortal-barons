package game

import "fmt"

// ibbs_league.go — the league: its ruleset, its roster of boards, who
// coordinates it, and the season reset.

// LeagueConfig is the set of game rules the League Coordinator sets for the
// whole league. The coordinator (board #1) broadcasts it; member boards adopt
// it so everyone plays by the same turns/protection/length — replacing the
// hand-coordinated config that BRE distributed at reset.
type LeagueConfig struct {
	GameStartDate        string
	JoinDate             string
	TurnsPerDay          int
	ProtectionTurns      int
	GameLength           int
	IdleDaysRemove       int
	InitialMarketLand    int
	LandPerDay           int
	MoneyCapBillions     int
	InterestRate         int
	StdInvestRate        int
	SteadyInvest         bool
	FoodUnlimited        bool
	MaxTaxRate           int
	PlanetaryTaxRate     int
	MaxRegions           int
	MaxIndividualAttacks int
	MaxGroupAttacks      int
	MaxTerrorOps         int
	MaxBombingOps        int
	LostForcesDays       int
	BombingOps           bool
	MissileOps           bool
	GooieKablooie        bool `json:"ClingyAnnihilator"`
	LocalAttacks         bool
	LocalAttackScoring   bool
	DupeChecking         bool
	MinBoardVersion      string
	IPTrading            bool
	Pirates              bool
	MaxPlayers           int
	BuyMilitary          BuyMode
	MaintCosts           Level
	TradeCosts           Level
	RegionCosts          Level
	AttackCosts          Level
	TerrorCosts          Level
	AttackDamage         Level
	AttackRewards        Level
	SabreHandling        SabreMode `json:"SlappenheimerHandling"`
}

// leagueRuleset extracts the league-wide rules from this board's config, for the
// coordinator to broadcast. It is a WIDER set than the fields the Configuration
// Editor stars: the star means "an inter-BBS option", which a rule like the tax
// cap or the pirates is not, though both still have to be the same on every
// board for the game to be fair.
func (c Config) leagueRuleset() *LeagueConfig {
	return &LeagueConfig{
		GameStartDate:        c.GameStartDate,
		JoinDate:             c.JoinDate,
		TurnsPerDay:          c.TurnsPerDay,
		ProtectionTurns:      c.ProtectionTurns,
		GameLength:           c.GameLength,
		IdleDaysRemove:       c.IdleDaysRemove,
		InitialMarketLand:    c.InitialMarketLand,
		LandPerDay:           c.LandPerDay,
		MoneyCapBillions:     c.MoneyCapBillions,
		InterestRate:         c.InterestRate,
		StdInvestRate:        c.StdInvestRate,
		SteadyInvest:         c.SteadyInvest,
		FoodUnlimited:        c.FoodUnlimited,
		MaxTaxRate:           c.MaxTaxRate,
		PlanetaryTaxRate:     c.PlanetaryTaxRate,
		MaxRegions:           c.MaxRegions,
		MaxIndividualAttacks: c.MaxIndividualAttacks,
		MaxGroupAttacks:      c.MaxGroupAttacks,
		MaxTerrorOps:         c.MaxTerrorOps,
		MaxBombingOps:        c.MaxBombingOps,
		LostForcesDays:       c.LostForcesDays,
		BombingOps:           c.BombingOps,
		MissileOps:           c.MissileOps,
		GooieKablooie:        c.GooieKablooie,
		LocalAttacks:         c.LocalAttacks,
		LocalAttackScoring:   c.LocalAttackScoring,
		DupeChecking:         c.DupeChecking,
		MinBoardVersion:      c.MinBoardVersion,
		IPTrading:            c.IPTrading,
		Pirates:              c.Pirates,
		MaxPlayers:           c.MaxPlayers,
		BuyMilitary:          c.BuyMilitary,
		MaintCosts:           c.MaintCosts,
		TradeCosts:           c.TradeCosts,
		RegionCosts:          c.RegionCosts,
		AttackCosts:          c.AttackCosts,
		TerrorCosts:          c.TerrorCosts,
		AttackDamage:         c.AttackDamage,
		AttackRewards:        c.AttackRewards,
		SabreHandling:        c.SabreHandling,
	}
}

// applyLeagueRuleset copies broadcast league rules into this board's config,
// leaving per-board fields untouched. What counts as which is decided by one
// rule: anything that changes how the local game plays has to be the same on
// every planet, or the season is not a fair one. Only identity, file paths and
// session policy stay local — see perBoardConfigFields, which pins the list.
func (c *Config) applyLeagueRuleset(lc *LeagueConfig) {
	c.GameStartDate = lc.GameStartDate
	c.JoinDate = lc.JoinDate
	c.TurnsPerDay = lc.TurnsPerDay
	c.ProtectionTurns = lc.ProtectionTurns
	c.GameLength = lc.GameLength
	c.IdleDaysRemove = lc.IdleDaysRemove
	c.InitialMarketLand = lc.InitialMarketLand
	c.LandPerDay = lc.LandPerDay
	c.MoneyCapBillions = lc.MoneyCapBillions
	c.InterestRate = lc.InterestRate
	c.StdInvestRate = lc.StdInvestRate
	c.SteadyInvest = lc.SteadyInvest
	c.FoodUnlimited = lc.FoodUnlimited
	c.MaxTaxRate = lc.MaxTaxRate
	c.PlanetaryTaxRate = lc.PlanetaryTaxRate
	c.MaxRegions = lc.MaxRegions
	c.MaxIndividualAttacks = lc.MaxIndividualAttacks
	c.MaxGroupAttacks = lc.MaxGroupAttacks
	c.MaxTerrorOps = lc.MaxTerrorOps
	c.MaxBombingOps = lc.MaxBombingOps
	c.LostForcesDays = lc.LostForcesDays
	c.BombingOps = lc.BombingOps
	c.MissileOps = lc.MissileOps
	c.GooieKablooie = lc.GooieKablooie
	c.LocalAttacks = lc.LocalAttacks
	c.LocalAttackScoring = lc.LocalAttackScoring
	c.DupeChecking = lc.DupeChecking
	c.MinBoardVersion = lc.MinBoardVersion
	c.IPTrading = lc.IPTrading
	c.Pirates = lc.Pirates
	c.MaxPlayers = lc.MaxPlayers
	c.BuyMilitary = lc.BuyMilitary
	c.MaintCosts = lc.MaintCosts
	c.TradeCosts = lc.TradeCosts
	c.RegionCosts = lc.RegionCosts
	c.AttackCosts = lc.AttackCosts
	c.TerrorCosts = lc.TerrorCosts
	c.AttackDamage = lc.AttackDamage
	c.AttackRewards = lc.AttackRewards
	c.SabreHandling = lc.SabreHandling
}

// CoordinatorBoardID is the name of node #1 in the roster — the League
// Coordinator's board. Empty if no roster is loaded.
func (w *World) CoordinatorBoardID() string { return w.NodeName(1) }

// IsLeagueCoordinator reports whether this board is node #1 (the LC).
func (w *World) IsLeagueCoordinator() bool {
	return w.Config.BoardID != "" && w.Config.BoardID == w.CoordinatorBoardID()
}

// ExportLeagueConfig queues a broadcast packet carrying this board's league
// rules. Only meaningful from the coordinator; member boards accept it only
// when it comes from node #1 (see ApplyPacket).
//
// Prepended, not appended — see ExportNodeList's doc comment for why: the
// same Seq-ordering reasoning applies to every packet type
// CarriesCoordinatorOrders recognizes, this one included.
func (w *World) ExportLeagueConfig() {
	w.Outbox = append([]Packet{{
		FromBoard:    w.Config.BoardID,
		Date:         w.LastMaintDate,
		LeagueConfig: w.Config.leagueRuleset(),
	}}, w.Outbox...)
}

// ExportNodeList queues a broadcast of the league roster. Only the Coordinator
// sends it, and only members adopt it, so the roster stays in one sysop's hands
// instead of every board editing its own copy as boards join or move (#64). A
// no-op when this board has no roster loaded.
//
// Prepended, not appended: StampOutbox assigns Seq in Outbox slice order, and
// a Coordinator's own player actions (a trade bid, a land claim) are already
// queued in Outbox from earlier in the day by the time a scheduled planetary
// run gets here and calls this. Appending would give the roster the HIGHEST
// Seq of the batch every time, not the lowest — and a receiving board's
// inbound staging applies a Coordinator group's verified-orders packets
// first only up through the last one in Seq order, so the roster needs the
// LOWEST Seq for that to mean anything: giving it the highest makes every
// other packet in the group ride along as part of the same applied-first
// prefix instead of only the roster itself.
func (w *World) ExportNodeList() {
	if !w.IsLeagueCoordinator() || len(w.LeagueNodes) == 0 {
		return
	}
	w.Outbox = append([]Packet{{
		FromBoard:   w.Config.BoardID,
		Date:        w.LastMaintDate,
		LeagueNodes: append([]LeagueNode(nil), w.LeagueNodes...),
	}}, w.Outbox...)
}

// LeagueNode is one board in the inter-BBS league, as listed in the
// coordinator's node list (BRE's BRNODES.DAT). Node 1 is the League
// Coordinator's board.
type LeagueNode struct {
	Number  int
	Name    string // planet / BBS name
	Address string // FidoNet net/node address
	City    string
	State   string
	Country string
	// PublicKey is this board's ed25519 packet-signing key, hex-encoded, and is
	// what makes FromBoard checkable (#118). The Coordinator records it and the
	// signed roster carries it, so a member never has to trust a key it was
	// handed by the board the key belongs to. Empty on a roster written before
	// this existed.
	PublicKey string
	// Hosts are the node numbers this board forwards packets for — BRE's HOST
	// routing, written on the roster's first line as "2 HOST 3 4 8". The
	// Coordinator maintains it for the whole league, so a member board
	// configures one link to its uplink instead of one to every other board.
	Hosts []int
}

// LeaguePlanets lists the league's planets in the order the screens show them:
// the coordinator's roster first, which is what carries the numbers and
// locations a sysop configured, and then any board that has only ever been
// heard from over a packet, numbered on after the roster. THIS board is in the
// list, as it is in BRE's own "List of Planets"; KnownBoards is the view that
// leaves it out.
func (w *World) LeaguePlanets() []LeagueNode {
	var planets []LeagueNode
	seen := map[string]bool{}
	next := 0
	for _, n := range w.LeagueNodes {
		if n.Name == "" || seen[n.Name] {
			continue
		}
		seen[n.Name] = true
		planets = append(planets, n)
		if n.Number > next {
			next = n.Number
		}
	}
	for _, b := range w.RemoteBoards {
		if b.BoardID == "" || seen[b.BoardID] {
			continue
		}
		seen[b.BoardID] = true
		next++
		planets = append(planets, LeagueNode{Number: next, Name: b.BoardID})
	}
	return planets
}

// KnownBoards names every planet BUT this one — who a packet can be addressed
// to, and who Travel Times measures.
func (w *World) KnownBoards() []string {
	var boards []string
	for _, p := range w.LeaguePlanets() {
		if p.Name != w.Config.BoardID {
			boards = append(boards, p.Name)
		}
	}
	return boards
}

// VoteCoordinator records voter's vote for the empire owned by forOwner to be
// the BBS Coordinator.
func (w *World) VoteCoordinator(voter *Empire, forOwner string) {
	voter.CoordinatorVote = forOwner
}

// BBSCoordinator returns the empire elected this board's Coordinator: the
// living human empire with the most votes (ties break by net worth). It
// returns nil until at least one vote is cast.
func (w *World) BBSCoordinator() *Empire {
	votes := map[string]int{}
	for _, e := range w.Empires {
		if e.Alive && e.CoordinatorVote != "" {
			votes[e.CoordinatorVote]++
		}
	}
	var best *Empire
	bestVotes := 0
	for _, e := range w.Empires {
		if !e.Alive || e.Owner == "" {
			continue
		}
		v := votes[e.Owner]
		if v == 0 {
			continue
		}
		if v > bestVotes || (v == bestVotes && best != nil && w.NetWorth(e) > w.NetWorth(best)) {
			best, bestVotes = e, v
		}
	}
	return best
}

// ExportScores queues a broadcast packet of this board's living human empires'
// scores for the league — it feeds the other boards' interplanetary score
// screens and gives group attacks targets to choose from.
func (w *World) ExportScores() {
	var scores []RemoteScore
	for _, e := range w.Empires {
		if e.Alive && e.Owner != "" {
			s := RemoteScore{
				Empire: e.Name, NetWorth: w.NetWorth(e), Land: e.Land, Score: e.Score,
				Protected: e.Protection > 0,
			}
			if w.dupeCheckingOn() {
				s.OwnerHash = dupeHash(e.Owner)
			}
			scores = append(scores, s)
		}
	}
	if len(scores) == 0 {
		return
	}
	w.Outbox = append(w.Outbox, Packet{
		FromBoard: w.Config.BoardID, Date: w.LastMaintDate, Scores: scores,
		Market:  w.ExportMarket(), // so allied planets can bid on it (#47)
		Battles: w.ownBattles(),   // the wars, for every board's world report (#233)
	})
}

// usableNodes drops roster entries this board cannot use: a node number outside
// 1 to MaxNodeNumber. The roster file's parser refuses the same numbers, so a
// roster adopted from a packet and one read off disk describe the same league
// (#180). Dropping the entry rather than the whole roster keeps one bad line
// from cutting a board off from every other board in the league.
func usableNodes(nodes []LeagueNode) []LeagueNode {
	out := make([]LeagueNode, 0, len(nodes))
	for _, n := range nodes {
		if n.Number < 1 || n.Number > MaxNodeNumber {
			continue
		}
		out = append(out, n)
	}
	return out
}

// SameRoster reports whether two league rosters hold the same boards in the
// same order. Lives here beside LeagueNode so both the packet reader and the
// store's node-list writer ask one question one way.
func SameRoster(a, b []LeagueNode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameNode(a[i], b[i]) {
			return false
		}
	}
	return true
}

func sameNode(a, b LeagueNode) bool {
	if a.Number != b.Number || a.Name != b.Name || a.Address != b.Address ||
		a.City != b.City || a.State != b.State || a.Country != b.Country ||
		len(a.Hosts) != len(b.Hosts) {
		return false
	}
	for i := range a.Hosts {
		if a.Hosts[i] != b.Hosts[i] {
			return false
		}
	}
	return true
}

// fromCoordinator reports whether p claims to come from the Coordinator's
// board (node #1, #105), and that this board is not the Coordinator hearing
// its own echo. It is only half the check — the signature is the half that
// cannot be faked.
func (w *World) fromCoordinator(p Packet) bool {
	if w.IsLeagueCoordinator() {
		return false
	}
	if p.FromNode != 0 {
		return p.FromNode == 1
	}
	return p.FromBoard != "" && p.FromBoard == w.CoordinatorBoardID()
}

// applyLeagueReset carries out the Coordinator's order to start a new season.
// The order names the season it starts, so a board that already ran it — or one
// replaying an old packet — does nothing (#65).
func (w *World) applyLeagueReset(r *LeagueReset) {
	if r.Season <= w.Season {
		return
	}
	w.Season = r.Season
	w.ResetForNewSeason(r.OnDate)
	if r.Announced != "" {
		w.LeagueDiplomacy = r.Announced
	}
	w.postNews(fmt.Sprintf("The League Coordinator has begun season %d. Every realm starts again.", r.Season))
}

// DeclareLeagueReset is the Coordinator ordering a new season. It resets this
// board too and queues the order for every other, signed so no other board can
// issue one (#65).
func (w *World) DeclareLeagueReset(onDate, announcement string) error {
	if !w.IsLeagueCoordinator() {
		return ErrNotCoordinator
	}
	if len(w.CoordKey) == 0 {
		return ErrNoCoordKey
	}
	w.Season++
	r := &LeagueReset{Season: w.Season, OnDate: onDate, Announced: announcement}
	p := Packet{FromBoard: w.Config.BoardID, Date: w.LastMaintDate, Seq: w.NextSeq(), Reset: r}
	if err := w.SignAsCoordinator(&p); err != nil {
		return err
	}
	w.Outbox = append(w.Outbox, p)
	w.ResetForNewSeason(onDate)
	if announcement != "" {
		w.LeagueDiplomacy = announcement
	}
	w.postNews(fmt.Sprintf("Season %d begins. Every realm starts again.", w.Season))
	return nil
}

// ownBattles is the log this board fought, stamped with its own name so a
// reader can tell whose war it was. Only this board's own entries go out: one
// that arrived from elsewhere is already on its way to everyone from the board
// that fought it, and forwarding it would multiply every battle by the size of
// the league.
func (w *World) ownBattles() []BattleLogEntry {
	var out []BattleLogEntry
	for _, b := range w.Battles {
		if b.Planet != "" {
			continue
		}
		b.Planet = w.Config.BoardID
		out = append(out, b)
	}
	return out
}
