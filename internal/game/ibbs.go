package game

import (
	"errors"
	"fmt"
)

// Inter-BBS (interplanetary) play. Transport is Option A from the design spec
// (docs/superpowers/specs/2026-07-04-interbbs-design.md): the game reads and
// writes packet files in inbound/outbound dirs in its own JSON format; how the
// files move between boards is the operator's concern. This file is the pure
// in-memory model and resolution logic, testable with several World instances;
// the store layer handles reading/writing the JSON files.

var (
	ErrNoAttack = errors.New("no such group attack")
	ErrDeparted = errors.New("that attack force has already left")
)

// Packet carries inter-BBS actions from one board to another (or, with an empty
// ToBoard, broadcast to the whole league).
type Packet struct {
	FromBoard    string
	ToBoard      string
	Date         string
	Scores       []RemoteScore  // score share (feeds RemoteBoards / IP scores)
	Attacks      []RemoteAttack // strikes landing on ToBoard
	Results      []AttackResult // outcomes returning to the origin
	LeagueConfig *LeagueConfig  // LC-authored league settings (nil if absent)
}

// LeagueConfig is the set of game rules the League Coordinator sets for the
// whole league. The coordinator (board #1) broadcasts it; member boards adopt
// it so everyone plays by the same turns/protection/length — replacing the
// hand-coordinated config that BRE distributed at reset.
type LeagueConfig struct {
	GameStartDate         string
	JoinDate              string
	TurnsPerDay           int
	ProtectionTurns       int
	GameLength            int
	InitialMarketLand     int
	LandPerDay            int
	InterestRate          int
	StdInvestRate         int
	SteadyInvest          bool
	MaxTaxRate            int
	MaxRegions            int
	MaxPlayers            int
	BuyMilitary           BuyMode
	MaintCosts            Level
	TradeCosts            Level
	RegionCosts           Level
	AttackDamage          Level
	AttackRewards         Level
	SlappenheimerHandling SlappenheimerMode
}

// leagueRuleset extracts the league-wide rules (the fields marked * in the
// Configuration Editor) from this board's config, for the coordinator to
// broadcast.
func (c Config) leagueRuleset() *LeagueConfig {
	return &LeagueConfig{
		GameStartDate:         c.GameStartDate,
		JoinDate:              c.JoinDate,
		TurnsPerDay:           c.TurnsPerDay,
		ProtectionTurns:       c.ProtectionTurns,
		GameLength:            c.GameLength,
		InitialMarketLand:     c.InitialMarketLand,
		LandPerDay:            c.LandPerDay,
		InterestRate:          c.InterestRate,
		StdInvestRate:         c.StdInvestRate,
		SteadyInvest:          c.SteadyInvest,
		MaxTaxRate:            c.MaxTaxRate,
		MaxRegions:            c.MaxRegions,
		MaxPlayers:            c.MaxPlayers,
		BuyMilitary:           c.BuyMilitary,
		MaintCosts:            c.MaintCosts,
		TradeCosts:            c.TradeCosts,
		RegionCosts:           c.RegionCosts,
		AttackDamage:          c.AttackDamage,
		AttackRewards:         c.AttackRewards,
		SlappenheimerHandling: c.SlappenheimerHandling,
	}
}

// applyLeagueRuleset copies broadcast league rules into this board's config,
// leaving per-board fields (BoardID, dirs, AICount, IBBS) untouched.
func (c *Config) applyLeagueRuleset(lc *LeagueConfig) {
	c.GameStartDate = lc.GameStartDate
	c.JoinDate = lc.JoinDate
	c.TurnsPerDay = lc.TurnsPerDay
	c.ProtectionTurns = lc.ProtectionTurns
	c.GameLength = lc.GameLength
	c.InitialMarketLand = lc.InitialMarketLand
	c.LandPerDay = lc.LandPerDay
	c.InterestRate = lc.InterestRate
	c.StdInvestRate = lc.StdInvestRate
	c.SteadyInvest = lc.SteadyInvest
	c.MaxTaxRate = lc.MaxTaxRate
	c.MaxRegions = lc.MaxRegions
	c.MaxPlayers = lc.MaxPlayers
	c.BuyMilitary = lc.BuyMilitary
	c.MaintCosts = lc.MaintCosts
	c.TradeCosts = lc.TradeCosts
	c.RegionCosts = lc.RegionCosts
	c.AttackDamage = lc.AttackDamage
	c.AttackRewards = lc.AttackRewards
	c.SlappenheimerHandling = lc.SlappenheimerHandling
}

// CoordinatorBoardID is the name of node #1 in the roster — the League
// Coordinator's board. Empty if no roster is loaded.
func (w *World) CoordinatorBoardID() string {
	for _, n := range w.LeagueNodes {
		if n.Number == 1 {
			return n.Name
		}
	}
	return ""
}

// IsLeagueCoordinator reports whether this board is node #1 (the LC).
func (w *World) IsLeagueCoordinator() bool {
	return w.Config.BoardID != "" && w.Config.BoardID == w.CoordinatorBoardID()
}

// ExportLeagueConfig queues a broadcast packet carrying this board's league
// rules. Only meaningful from the coordinator; member boards accept it only
// when it comes from node #1 (see ApplyPacket).
func (w *World) ExportLeagueConfig() {
	w.Outbox = append(w.Outbox, Packet{
		FromBoard:    w.Config.BoardID,
		Date:         w.LastMaintDate,
		LeagueConfig: w.Config.leagueRuleset(),
	})
}

// Contribution records one baron's share of a group attack, so spoils split
// proportionally.
type Contribution struct {
	Owner   string
	Offense int
}

// GroupAttack is a strike being assembled on this board. Until DepartDay,
// other barons here may join it.
type GroupAttack struct {
	ID           int
	TargetBoard  string
	TargetEmpire string // "" = the whole planet (its strongest baron)
	DepartDay    int
	Contributors []Contribution
}

// Offense totals the committed offensive strength.
func (g GroupAttack) Offense() int {
	total := 0
	for _, c := range g.Contributors {
		total += c.Offense
	}
	return total
}

// RemoteAttack is a departed strike aimed at an empire on another board.
type RemoteAttack struct {
	ID           int
	FromBoard    string
	TargetEmpire string
	Offense      int
	Contributors []Contribution
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
}

// SpyReport is intel on a remote empire, stored in the planet-wide Spy
// Database and readable by every baron here. Populated by interplanetary spy
// ops (built in a later increment).
type SpyReport struct {
	Board   string
	Empire  string
	Date    string
	Land    int
	Offense int
	Defense int
	Gold    int
}

// AttackResult returns to the origin board after a remote strike resolves.
type AttackResult struct {
	ID           int
	TargetBoard  string
	TargetEmpire string
	LandTaken    int
	Won          bool
}

// CreateGroupAttack starts a new group strike led by e (contributing offense),
// aimed at targetEmpire on targetBoard, leaving on departDay.
func (w *World) CreateGroupAttack(e *Empire, targetBoard, targetEmpire string, departDay, offense int) *GroupAttack {
	w.NextAttackID++
	w.GroupAttacks = append(w.GroupAttacks, GroupAttack{
		ID:           w.NextAttackID,
		TargetBoard:  targetBoard,
		TargetEmpire: targetEmpire,
		DepartDay:    departDay,
		Contributors: []Contribution{{Owner: e.Owner, Offense: offense}},
	})
	return &w.GroupAttacks[len(w.GroupAttacks)-1]
}

// JoinGroupAttack adds e's offense to a pending group attack (before it leaves).
func (w *World) JoinGroupAttack(e *Empire, id, offense int) error {
	for i := range w.GroupAttacks {
		ga := &w.GroupAttacks[i]
		if ga.ID != id {
			continue
		}
		if w.GameDay >= ga.DepartDay {
			return ErrDeparted
		}
		ga.Contributors = append(ga.Contributors, Contribution{Owner: e.Owner, Offense: offense})
		return nil
	}
	return ErrNoAttack
}

// LaunchDueGroupAttacks turns every group attack whose DepartDay has arrived
// into an outbound RemoteAttack and removes it from the pending list. Run
// during the PLANETARY maintenance step.
func (w *World) LaunchDueGroupAttacks() {
	var remaining []GroupAttack
	for _, ga := range w.GroupAttacks {
		if w.GameDay < ga.DepartDay {
			remaining = append(remaining, ga)
			continue
		}
		w.enqueue(ga.TargetBoard, RemoteAttack{
			ID:           ga.ID,
			FromBoard:    w.Config.BoardID,
			TargetEmpire: ga.TargetEmpire,
			Offense:      ga.Offense(),
			Contributors: ga.Contributors,
		})
	}
	w.GroupAttacks = remaining
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
			scores = append(scores, RemoteScore{Empire: e.Name, NetWorth: w.NetWorth(e), Land: e.Land})
		}
	}
	if len(scores) == 0 {
		return
	}
	w.Outbox = append(w.Outbox, Packet{FromBoard: w.Config.BoardID, Date: w.LastMaintDate, Scores: scores})
}

// enqueue appends atk to the outbound packet for toBoard, creating it if needed.
func (w *World) enqueue(toBoard string, atk RemoteAttack) {
	for i := range w.Outbox {
		if w.Outbox[i].ToBoard == toBoard {
			w.Outbox[i].Attacks = append(w.Outbox[i].Attacks, atk)
			return
		}
	}
	w.Outbox = append(w.Outbox, Packet{
		FromBoard: w.Config.BoardID,
		ToBoard:   toBoard,
		Date:      w.LastMaintDate,
		Attacks:   []RemoteAttack{atk},
	})
}

// ApplyPacket applies an inbound packet to this board and returns a result
// packet (attack outcomes) addressed back to the origin.
func (w *World) ApplyPacket(p Packet) Packet {
	// League settings are accepted only from the coordinator board (node #1),
	// so a member board can't push rules onto the league. The LC ignores its
	// own echo.
	if p.LeagueConfig != nil && p.FromBoard != "" && p.FromBoard == w.CoordinatorBoardID() && !w.IsLeagueCoordinator() {
		w.Config.applyLeagueRuleset(p.LeagueConfig)
		w.postNews("The League Coordinator updated the league settings.")
	}
	if len(p.Scores) > 0 {
		w.ImportBoard(RemoteBoard{BoardID: p.FromBoard, Date: p.Date, Scores: p.Scores})
	}
	// Outcomes of our own strikes, returning from the target board.
	for _, res := range p.Results {
		if res.Won {
			w.postNews(fmt.Sprintf("Our interplanetary strike on %s (%s) took %d regions!", res.TargetEmpire, res.TargetBoard, res.LandTaken))
		} else {
			w.postNews(fmt.Sprintf("Our interplanetary strike on %s (%s) was repelled.", res.TargetEmpire, res.TargetBoard))
		}
	}
	result := Packet{FromBoard: w.Config.BoardID, ToBoard: p.FromBoard, Date: w.LastMaintDate}
	for _, atk := range p.Attacks {
		result.Results = append(result.Results, w.resolveRemoteAttack(atk))
	}
	return result
}

// resolveRemoteAttack resolves a remote strike against its target empire (or,
// for a whole-planet attack, this board's strongest baron).
func (w *World) resolveRemoteAttack(atk RemoteAttack) AttackResult {
	target := w.remoteTarget(atk.TargetEmpire)
	res := AttackResult{ID: atk.ID, TargetBoard: w.Config.BoardID, TargetEmpire: atk.TargetEmpire}
	if target == nil {
		return res
	}
	res.TargetEmpire = target.Name
	def := target.Defense()
	if atk.Offense <= def {
		target.Events = append(target.Events, fmt.Sprintf("You repelled an interplanetary strike from %s.", atk.FromBoard))
		return res
	}
	// Overwhelmed: take a bite of land proportional to the margin (capped).
	frac := (atk.Offense - def) * 100 / max(atk.Offense, 1)
	land := target.Land * frac / 100 / 4
	if land > target.Land {
		land = target.Land
	}
	if land > 0 {
		target.Regions.remove(land)
		target.syncLand()
	}
	target.Events = append(target.Events, fmt.Sprintf("An interplanetary strike from %s took %d regions!", atk.FromBoard, land))
	res.LandTaken = land
	res.Won = true
	return res
}

// remoteTarget finds the empire a remote attack should hit: the named baron, or
// (for a whole-planet strike) this board's strongest living human empire.
func (w *World) remoteTarget(name string) *Empire {
	if name != "" {
		for _, e := range w.Empires {
			if e.Alive && e.Name == name {
				return e
			}
		}
		return nil
	}
	var best *Empire
	for _, e := range w.Empires {
		if e.Alive && e.Owner != "" && (best == nil || w.NetWorth(e) > w.NetWorth(best)) {
			best = e
		}
	}
	return best
}
