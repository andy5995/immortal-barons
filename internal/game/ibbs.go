package game

import (
	"errors"
	"fmt"
)

// Inter-BBS (interplanetary) play. Transport is Option A: the game reads and
// writes packet files in inbound/outbound dirs in its own JSON format; how the
// files move between boards is the operator's concern. This file is the pure
// in-memory model and resolution logic, testable with several World instances;
// the store layer handles reading/writing the JSON files.

var (
	ErrNoAttack = errors.New("no such group attack")
	ErrDeparted = errors.New("that attack force has already left")
	// The per-day interplanetary allowances (Config.MaxGroupAttacks and
	// MaxTerrorOps), the counterparts of the individual-attack cap.
	ErrGroupAttacksExhausted = errors.New("You have already launched all of your group attacks for today.")
	ErrTerrorOpsExhausted    = errors.New("You have already launched all of your terrorist operations for today.")
)

// Packet carries inter-BBS actions from one board to another (or, with an empty
// ToBoard, broadcast to the whole league).
type Packet struct {
	FromBoard    string
	ToBoard      string
	Date         string
	Scores       []RemoteScore  // score share (feeds RemoteBoards / IP scores)
	Attacks      []RemoteAttack // strikes landing on ToBoard
	Terrors      []RemoteTerror // terror ops landing on ToBoard
	Results      []AttackResult // outcomes returning to the origin
	LeagueConfig *LeagueConfig  // LC-authored league settings (nil if absent)
	LeagueNodes  []LeagueNode   // LC-authored league roster (nil if absent, #64)
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
	PlanetaryTaxRate      int
	MaxRegions            int
	MaxIndividualAttacks  int
	MaxGroupAttacks       int
	MaxTerrorOps          int
	MaxBombingOps         int
	LostForcesDays        int
	BombingOps            bool
	MissileOps            bool
	DoomerKaboomer        bool
	MaxPlayers            int
	BuyMilitary           BuyMode
	MaintCosts            Level
	TradeCosts            Level
	RegionCosts           Level
	AttackCosts           Level
	TerrorCosts           Level
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
		PlanetaryTaxRate:      c.PlanetaryTaxRate,
		MaxRegions:            c.MaxRegions,
		MaxIndividualAttacks:  c.MaxIndividualAttacks,
		MaxGroupAttacks:       c.MaxGroupAttacks,
		MaxTerrorOps:          c.MaxTerrorOps,
		MaxBombingOps:         c.MaxBombingOps,
		LostForcesDays:        c.LostForcesDays,
		BombingOps:            c.BombingOps,
		MissileOps:            c.MissileOps,
		DoomerKaboomer:        c.DoomerKaboomer,
		MaxPlayers:            c.MaxPlayers,
		BuyMilitary:           c.BuyMilitary,
		MaintCosts:            c.MaintCosts,
		TradeCosts:            c.TradeCosts,
		RegionCosts:           c.RegionCosts,
		AttackCosts:           c.AttackCosts,
		TerrorCosts:           c.TerrorCosts,
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
	c.PlanetaryTaxRate = lc.PlanetaryTaxRate
	c.MaxRegions = lc.MaxRegions
	c.MaxIndividualAttacks = lc.MaxIndividualAttacks
	c.MaxGroupAttacks = lc.MaxGroupAttacks
	c.MaxTerrorOps = lc.MaxTerrorOps
	c.MaxBombingOps = lc.MaxBombingOps
	c.LostForcesDays = lc.LostForcesDays
	c.BombingOps = lc.BombingOps
	c.MissileOps = lc.MissileOps
	c.DoomerKaboomer = lc.DoomerKaboomer
	c.MaxPlayers = lc.MaxPlayers
	c.BuyMilitary = lc.BuyMilitary
	c.MaintCosts = lc.MaintCosts
	c.TradeCosts = lc.TradeCosts
	c.RegionCosts = lc.RegionCosts
	c.AttackCosts = lc.AttackCosts
	c.TerrorCosts = lc.TerrorCosts
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

// ExportNodeList queues a broadcast of the league roster. Only the Coordinator
// sends it, and only members adopt it, so the roster stays in one sysop's hands
// instead of every board editing its own copy as boards join or move (#64). A
// no-op when this board has no roster loaded.
func (w *World) ExportNodeList() {
	if !w.IsLeagueCoordinator() || len(w.LeagueNodes) == 0 {
		return
	}
	w.Outbox = append(w.Outbox, Packet{
		FromBoard:   w.Config.BoardID,
		Date:        w.LastMaintDate,
		LeagueNodes: append([]LeagueNode(nil), w.LeagueNodes...),
	})
}

// AttackForce is the detachment a baron commits to a group attack (BRE lets you
// send troopers, jets, tanks, and bombers — real units, deducted from your
// army, not gold).
type AttackForce struct {
	Troopers int
	Jets     int
	Tanks    int
	Bombers  int
}

// Empty reports whether no units were committed.
func (f AttackForce) Empty() bool { return f.Troopers+f.Jets+f.Tanks+f.Bombers == 0 }

// offense values the detachment by the combat table (trooper 1, jet 2, tank 4,
// bomber GroupAttackBomberOffense).
func (f AttackForce) offense() int {
	return f.Troopers + f.Jets*2 + f.Tanks*4 + f.Bombers*GroupAttackBomberOffense
}

// Contribution records one baron's committed detachment, so the strike's
// strength and the returning survivors split per baron.
type Contribution struct {
	Owner string
	AttackForce
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

// Offense is the strike's offensive strength: every contributor's detachment
// valued by the combat table.
func (g GroupAttack) Offense() int {
	total := 0
	for _, c := range g.Contributors {
		total += c.offense()
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

// AttackResult returns to the origin board after a remote strike resolves. For
// a terror op (Kind == "terror") LandTaken carries the troopers destroyed
// instead of regions.
type AttackResult struct {
	ID           int
	TargetBoard  string
	TargetEmpire string
	LandTaken    int
	Won          bool
	Kind         string         // "" = regular strike, "terror" = terror op
	Survivors    []Contribution // forces returning to their contributors (per owner)
}

// RemoteTerror is a terror strike sent to an empire on another board: BRE's
// Terrorist Ops destroy the target's forces rather than capturing land.
type RemoteTerror struct {
	ID           int
	FromBoard    string
	TargetEmpire string
	Agents       int // agents committed; scales the forces destroyed
}

// InFlightStrike is a strike that has left this board and has not been answered.
// An inter-BBS attack is away for a whole packet round trip — out on this board's
// schedule, resolved on the target's, home again on theirs — and packets go
// missing. Holding the committed forces here lets them be given back when no
// result arrives (#96), rather than being lost with the packet.
type InFlightStrike struct {
	ID           int
	Kind         string // "attack" or "terror"
	TargetBoard  string
	TargetEmpire string
	LaunchedDay  int
	Contributors []Contribution // an attack's detachments, by owner
	Owner        string         // a terror op's sender
	Agents       int            // a terror op's committed agents
}

// commitForce deducts the detachment f from e's army; ErrCantAfford if e lacks
// any unit type.
func (e *Empire) commitForce(f AttackForce) error {
	if e.Troopers < f.Troopers || e.Jets < f.Jets || e.Tanks < f.Tanks || e.Bombers < f.Bombers {
		return ErrCantAfford
	}
	e.Troopers -= f.Troopers
	e.Jets -= f.Jets
	e.Tanks -= f.Tanks
	e.Bombers -= f.Bombers
	return nil
}

// CreateGroupAttack starts a new group strike led by e, aimed at targetEmpire
// on targetBoard, leaving on departDay. e commits the detachment f (deducted
// from its army); ErrCantAfford if it lacks the units.
func (w *World) CreateGroupAttack(e *Empire, targetBoard, targetEmpire string, departDay int, f AttackForce) (*GroupAttack, error) {
	if !w.CanGroupAttack(e) {
		return nil, ErrGroupAttacksExhausted
	}
	if err := e.commitForce(f); err != nil {
		return nil, err
	}
	e.GroupAttacksToday++
	w.NextAttackID++
	w.GroupAttacks = append(w.GroupAttacks, GroupAttack{
		ID:           w.NextAttackID,
		TargetBoard:  targetBoard,
		TargetEmpire: targetEmpire,
		DepartDay:    departDay,
		Contributors: []Contribution{{Owner: e.Owner, AttackForce: f}},
	})
	return &w.GroupAttacks[len(w.GroupAttacks)-1], nil
}

// JoinGroupAttack commits e's detachment f to a pending group attack (before it
// leaves). ErrCantAfford if e lacks the units.
func (w *World) JoinGroupAttack(e *Empire, id int, f AttackForce) error {
	for i := range w.GroupAttacks {
		ga := &w.GroupAttacks[i]
		if ga.ID != id {
			continue
		}
		if w.GameDay >= ga.DepartDay {
			return ErrDeparted
		}
		if !w.CanGroupAttack(e) {
			return ErrGroupAttacksExhausted
		}
		if err := e.commitForce(f); err != nil {
			return err
		}
		e.GroupAttacksToday++
		ga.Contributors = append(ga.Contributors, Contribution{Owner: e.Owner, AttackForce: f})
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
		w.InFlight = append(w.InFlight, InFlightStrike{
			ID:           ga.ID,
			Kind:         "attack",
			TargetBoard:  ga.TargetBoard,
			TargetEmpire: ga.TargetEmpire,
			LaunchedDay:  w.GameDay,
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
			scores = append(scores, RemoteScore{Empire: e.Name, NetWorth: w.NetWorth(e), Land: e.Land, Score: e.Score})
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

// SendTerror queues a terror op against targetEmpire on targetBoard, committing
// agents (deducted now). It resolves on the target board's next packet run.
func (w *World) SendTerror(e *Empire, targetBoard, targetEmpire string, agents int) error {
	if !w.CanTerrorOp(e) {
		return ErrTerrorOpsExhausted
	}
	if e.Agents < agents {
		return ErrNoAgents
	}
	e.Agents -= agents
	e.TerrorOpsToday++
	w.NextAttackID++
	t := RemoteTerror{ID: w.NextAttackID, FromBoard: w.Config.BoardID, TargetEmpire: targetEmpire, Agents: agents}
	w.InFlight = append(w.InFlight, InFlightStrike{
		ID:           t.ID,
		Kind:         "terror",
		TargetBoard:  targetBoard,
		TargetEmpire: targetEmpire,
		LaunchedDay:  w.GameDay,
		Owner:        e.Owner,
		Agents:       agents,
	})
	for i := range w.Outbox {
		if w.Outbox[i].ToBoard == targetBoard {
			w.Outbox[i].Terrors = append(w.Outbox[i].Terrors, t)
			return nil
		}
	}
	w.Outbox = append(w.Outbox, Packet{
		FromBoard: w.Config.BoardID,
		ToBoard:   targetBoard,
		Date:      w.LastMaintDate,
		Terrors:   []RemoteTerror{t},
	})
	return nil
}

// resolveRemoteTerror applies a terror op on this board: a protected target is
// untouched (the op fails); otherwise it destroys TerrorTrooperKill troopers per
// committed agent, capped at the target's troopers.
func (w *World) resolveRemoteTerror(t RemoteTerror) AttackResult {
	res := AttackResult{ID: t.ID, TargetBoard: w.Config.BoardID, TargetEmpire: t.TargetEmpire, Kind: "terror"}
	target := w.remoteTarget(t.TargetEmpire)
	if target == nil {
		return res
	}
	res.TargetEmpire = target.Name
	if target.Protection > 0 {
		target.addEvent(fmt.Sprintf("Terrorists from %s were stopped by your New Realm Protection.", t.FromBoard))
		return res
	}
	// BRE terror: each committed agent is an independent hit that removes a
	// fraction (~1/TerrorUnitLossDenom, from BRE's disassembled 6/7 ratio) of one
	// randomly chosen unit type.
	fields := []*int{&target.Troopers, &target.Jets, &target.Turrets, &target.Tanks, &target.Bombers, &target.Carriers}
	destroyed := 0
	for i := 0; i < t.Agents; i++ {
		f := fields[w.rng.Intn(len(fields))]
		loss := *f / TerrorUnitLossDenom
		*f -= loss
		destroyed += loss
	}
	if destroyed == 0 {
		target.addEvent(fmt.Sprintf("Terrorists from %s struck but destroyed nothing.", t.FromBoard))
		return res
	}
	target.addEvent(fmt.Sprintf("Terrorists from %s destroyed %d of your forces!", t.FromBoard, destroyed))
	res.LandTaken = destroyed
	res.Won = true
	return res
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
	// The roster travels the same way and under the same guard (#64).
	if len(p.LeagueNodes) > 0 && p.FromBoard != "" && p.FromBoard == w.CoordinatorBoardID() && !w.IsLeagueCoordinator() {
		w.LeagueNodes = append([]LeagueNode(nil), p.LeagueNodes...)
		w.postNews("The League Coordinator updated the league roster.")
	}
	if len(p.Scores) > 0 {
		w.ImportBoard(RemoteBoard{BoardID: p.FromBoard, Date: p.Date, Scores: p.Scores})
	}
	// Outcomes of our own strikes, returning from the target board.
	for _, res := range p.Results {
		w.clearInFlight(res.ID) // answered, so it can no longer time out
		switch {
		case res.Kind == "terror" && res.Won:
			w.postNews(fmt.Sprintf("Our terror op on %s (%s) destroyed %d troopers!", res.TargetEmpire, res.TargetBoard, res.LandTaken))
		case res.Kind == "terror":
			w.postNews(fmt.Sprintf("Our terror op on %s (%s) was foiled.", res.TargetEmpire, res.TargetBoard))
		case res.Won:
			w.postNews(fmt.Sprintf("Our interplanetary strike on %s (%s) took %d regions!", res.TargetEmpire, res.TargetBoard, res.LandTaken))
		default:
			w.postNews(fmt.Sprintf("Our interplanetary strike on %s (%s) was repelled.", res.TargetEmpire, res.TargetBoard))
		}
		// Return each contributor's surviving forces to their army.
		for _, sv := range res.Survivors {
			e := w.FindByOwner(sv.Owner)
			if e == nil {
				continue
			}
			e.Troopers += sv.Troopers
			e.Jets += sv.Jets
			e.Tanks += sv.Tanks
			e.Bombers += sv.Bombers
		}
	}
	result := Packet{FromBoard: w.Config.BoardID, ToBoard: p.FromBoard, Date: w.LastMaintDate}
	for _, atk := range p.Attacks {
		result.Results = append(result.Results, w.resolveRemoteAttack(atk))
	}
	for _, t := range p.Terrors {
		result.Results = append(result.Results, w.resolveRemoteTerror(t))
	}
	return result
}

// resolveRemoteAttack resolves a remote strike against its target empire (or,
// for a whole-planet attack, this board's strongest baron).
func (w *World) resolveRemoteAttack(atk RemoteAttack) AttackResult {
	target := w.remoteTarget(atk.TargetEmpire)
	res := AttackResult{ID: atk.ID, TargetBoard: w.Config.BoardID, TargetEmpire: atk.TargetEmpire}
	// Survivors return to their contributors whatever the outcome (a share is
	// lost in the strike).
	res.Survivors = survivorsOf(atk.Contributors)
	if target == nil {
		return res
	}
	res.TargetEmpire = target.Name
	// A target still under New Realm Protection is shielded, same as an incoming
	// terror op (resolveRemoteTerror). Protection counts down in transit, so this
	// arrival-time check — not the sender's view days earlier — is authoritative.
	if target.Protection > 0 {
		target.addEvent(fmt.Sprintf("An interplanetary strike from %s was stopped by your New Realm Protection.", atk.FromBoard))
		return res
	}
	def := target.Defense()
	if atk.Offense <= def {
		target.addEvent(fmt.Sprintf("You repelled an interplanetary strike from %s.", atk.FromBoard))
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
	target.addEvent(fmt.Sprintf("An interplanetary strike from %s took %d regions!", atk.FromBoard, land))
	res.LandTaken = land
	res.Won = true
	return res
}

// survivorsOf returns each contributor's detachment reduced by
// GroupAttackLossPct — the forces that come home after the strike.
func survivorsOf(cs []Contribution) []Contribution {
	keep := 100 - GroupAttackLossPct
	out := make([]Contribution, 0, len(cs))
	for _, c := range cs {
		out = append(out, Contribution{Owner: c.Owner, AttackForce: AttackForce{
			Troopers: c.Troopers * keep / 100,
			Jets:     c.Jets * keep / 100,
			Tanks:    c.Tanks * keep / 100,
			Bombers:  c.Bombers * keep / 100,
		}})
	}
	return out
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

// clearInFlight drops the strike with this ID from the waiting list, because its
// result has come home.
func (w *World) clearInFlight(id int) {
	for i, f := range w.InFlight {
		if f.ID == id {
			w.InFlight = append(w.InFlight[:i], w.InFlight[i+1:]...)
			return
		}
	}
}

// ReturnLostForces gives back the forces of any strike still unanswered after
// Config.LostForcesDays, and reports how many strikes it recovered. This is what
// stops a lost packet — a board that stops running transfers, a sysop who drops
// out — from costing a player their army for good (#96). A LostForcesDays of 0
// or less turns the recovery off, and the forces wait indefinitely.
//
// Run from the planetary step, after inbound packets have been applied, so a
// result that did arrive this run is never overtaken by the timer.
func (w *World) ReturnLostForces() int {
	days := w.Config.LostForcesDays
	if days <= 0 {
		return 0
	}
	var waiting []InFlightStrike
	recovered := 0
	for _, f := range w.InFlight {
		if w.GameDay-f.LaunchedDay < days {
			waiting = append(waiting, f)
			continue
		}
		recovered++
		if f.Kind == "terror" {
			if e := w.FindByOwner(f.Owner); e != nil {
				e.Agents += f.Agents
				e.addEvent(fmt.Sprintf("No word came back from %s. Your %d agents have returned home.", f.TargetBoard, f.Agents))
			}
			continue
		}
		for _, c := range f.Contributors {
			e := w.FindByOwner(c.Owner)
			if e == nil {
				continue
			}
			e.Troopers += c.Troopers
			e.Jets += c.Jets
			e.Tanks += c.Tanks
			e.Bombers += c.Bombers
			e.addEvent(fmt.Sprintf("No word came back from %s. Your force sent against %s has returned home.", f.TargetBoard, f.TargetEmpire))
		}
	}
	w.InFlight = waiting
	if recovered > 0 {
		w.postNews(fmt.Sprintf("%d interplanetary force(s) gave up waiting and came home.", recovered))
	}
	return recovered
}
