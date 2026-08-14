package game

import (
	"errors"
	"fmt"
	"time"
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
	ErrAttacksExhausted      = errors.New("You have already launched all of your attacks for today.")
	ErrNoTarget              = errors.New("An individual attack must name a baron to strike.")
)

// Packet carries inter-BBS actions from one board to another (or, with an empty
// ToBoard, broadcast to the whole league).
type Packet struct {
	FromBoard    string
	ToBoard      string
	Date         string
	Scores       []RemoteScore      // score share (feeds RemoteBoards / IP scores)
	Attacks      []RemoteAttack     // strikes landing on ToBoard
	Terrors      []RemoteTerror     // terror ops landing on ToBoard
	Results      []AttackResult     // outcomes returning to the origin
	LeagueConfig *LeagueConfig      // LC-authored league settings (nil if absent)
	LeagueNodes  []LeagueNode       // LC-authored league roster (nil if absent, #64)
	Recon        []ReconRequest     // scouting asked of ToBoard (#61)
	ReconReports []SpyReport        // answers coming back to the origin (#61)
	Annihilator  *AnnihilatorStatus // a doomsday weapon aimed at ToBoard (#63)
	TimeChecks   []TimeCheck        // round-trip probes, out and echoed back (Travel Times)
	IPMessages   []IPMessage        // interplanetary mail for ToBoard's barons
	// Seq numbers this board's outbound packets so the far side can spot one it
	// has already applied, and Signature authenticates the parts only the
	// Coordinator may author (#53).
	Seq       uint64
	Signature []byte
	// BoardSig is the sending board's own signature over this whole packet, so
	// FromBoard is proven rather than claimed (#118). Verified against the
	// public key the signed roster carries for that board.
	BoardSig []byte
	Reset    *LeagueReset // Coordinator's order to start a new season (#65)
	// League is the Coordinator's league number, so a board playing in two
	// leagues that share one inbound directory can tell the traffic apart.
	League int
	// Hops counts the boards that have forwarded this packet. Outside the
	// signed payload, because every hub changes it.
	Hops int
	// Epoch is the sending board's own generation counter (World.Epoch) at the
	// moment this packet was written. A receiver that has since reset — its own
	// Epoch has advanced past this — recognises the packet as belonging to a
	// game it has already wiped and discards it (#104). 0 means the sender
	// predates the field, and is trusted rather than rejected.
	Epoch int
	// FromNode and ToNode are the roster's node numbers for the packet's two
	// ends, preferred over FromBoard/ToBoard wherever identity actually
	// matters (authentication, addressing, the Coordinator check) because a
	// number cannot collide the way two boards sharing a name can, and survives
	// either end renaming (#105). 0 means unaddressed (ToNode) or unknown
	// (FromNode — no roster loaded yet, or a packet that predates the field);
	// FromBoard/ToBoard are always what's checked in that case.
	FromNode int
	ToNode   int
}

// HasPayload reports whether p carries anything worth sending. The transport
// asks before queueing a reply packet, so an answer that is only recon reports
// or only an echoed probe still goes out.
func (p Packet) HasPayload() bool {
	return len(p.Scores) > 0 || len(p.Attacks) > 0 || len(p.Terrors) > 0 ||
		len(p.Results) > 0 || len(p.Recon) > 0 || len(p.ReconReports) > 0 ||
		len(p.TimeChecks) > 0 || len(p.IPMessages) > 0 ||
		len(p.LeagueNodes) > 0 || p.LeagueConfig != nil || p.Annihilator != nil || p.Reset != nil
}

// LeagueReset is the Coordinator's order for every board to wipe and start a new
// season together. BRE lets the Coordinator reset the whole league in one step;
// without it a new season means every sysop being told out of band and doing it
// by hand on the same evening (#65).
//
// It is one of the payloads the Coordinator has to sign, because a forged one
// would destroy every world in the league.
type LeagueReset struct {
	Season    int    // increments each reset, so a board can tell a new order from an old one
	OnDate    string // ISO date the new season begins
	Announced string // the Coordinator's message to the league
}

// AnnihilatorStatus tells a planet about a Clingy Annihilator aimed at it — while it is
// still being built, and again while it is in the air. BRE broadcasts the same
// thing ("Updating Outgoing Gooie Kablooie Status"), and it is the whole reason a
// target can scramble jets: a weapon nobody can see is one nobody can shoot at
// (#63).
type AnnihilatorStatus struct {
	FromBoard  string
	Funded     bool
	Launched   bool
	ArrivesDay int
	Intact     int
	Dismantled bool // the builders scrapped it; stop watching for it
}

// ReconRequest asks another board what it knows about one of its own barons.
// BRE exchanges these as "Global Recon Requests" out and "Local Recon Info"
// back; the answer is real figures read on the target's board, which is what
// separates it from reading a shared score table (#61).
type ReconRequest struct {
	ID           int
	FromBoard    string
	FromOwner    string // the baron who paid the agent, so the answer can reach them
	TargetEmpire string
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
	ClingyAnnihilator     bool
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
		ClingyAnnihilator:     c.ClingyAnnihilator,
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
	c.ClingyAnnihilator = lc.ClingyAnnihilator
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
func (w *World) CoordinatorBoardID() string { return w.NodeName(1) }

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
func (f AttackForce) Empty() bool { return f.units() == 0 }

// units counts the whole detachment, whatever the type.
func (f AttackForce) units() int { return f.Troopers + f.Jets + f.Tanks + f.Bombers }

// GoldCost is what launching this detachment as an individual strike costs.
// BRE quotes it before asking to confirm ("This attack will cost 100 gold.").
func (f AttackForce) GoldCost() int64 { return int64(f.units()) * IndividualAttackGoldPerUnit }

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

// GroupAttack is a strike being assembled on this board. Until it leaves, other
// barons here may join it.
type GroupAttack struct {
	ID           int
	TargetBoard  string
	TargetEmpire string // "" = the whole planet (its strongest baron)
	// DepartAt is the instant the force leaves. BRE asks for a delay in HOURS
	// (12-120) and stores the answer as a wall-clock departure, which is what
	// lets a strike be timed to land before an opponent's next turn; a day
	// number cannot express that (#124).
	DepartAt time.Time
	// DepartDay is the pre-hours field, kept so a world saved before the change
	// still knows when its pending attacks leave. Only read when DepartAt is
	// zero; never written for a new attack.
	DepartDay    int `json:",omitempty"`
	Contributors []Contribution
}

// Due reports whether this attack's force has left by now. A pre-hours attack
// has no DepartAt, so it falls back to the game day it was filed against.
func (g GroupAttack) Due(now time.Time, gameDay int) bool {
	if g.DepartAt.IsZero() {
		return gameDay >= g.DepartDay
	}
	return !now.Before(g.DepartAt)
}

// DepartureAfter is the instant a group attack filed now leaves, given a delay
// in hours. It clamps to BRE's own window rather than trusting the caller: a
// packet-driven or scripted launch must not slip past the bounds the prompt
// enforces.
func DepartureAfter(now time.Time, hours int) time.Time {
	if hours < GroupAttackHoursMin {
		hours = GroupAttackHoursMin
	}
	if hours > GroupAttackHoursMax {
		hours = GroupAttackHoursMax
	}
	return now.Add(time.Duration(hours) * time.Hour)
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

// AttackKind is how an individual interplanetary strike is pressed. The values
// are BRE's own: its IBBS attack record stores Quick=0, Normal=1, Extended=2,
// so a packet stays readable to the original. The zero value is QuickStrike
// rather than the common case, which is BRE's encoding and not a default —
// every send names its kind.
type AttackKind int

const (
	QuickStrike AttackKind = iota
	NormalAttack
	ExtendedBattle
)

// attackKindRates holds each variant's published figures in one row, so adding
// a variant is one entry rather than an arm in four parallel switches. The
// figures themselves live in balance.go; this only names them.
var attackKindRates = map[AttackKind]struct {
	name                    string
	strength, capture, loss int
}{
	QuickStrike:    {"Quick Strike", QuickStrikeStrengthPct, QuickStrikeCapturePct, QuickStrikeLossPct},
	NormalAttack:   {"Normal Attack", NormalAttackStrengthPct, NormalAttackCapturePct, NormalAttackLossPct},
	ExtendedBattle: {"Extended Battle", ExtendedBattleStrengthPct, ExtendedBattleCapturePct, ExtendedBattleLossPct},
}

// rates returns k's row, falling back to the normal attack for a value that is
// not one of the three — an unreadable kind in an arriving packet resolves as
// an ordinary attack rather than crashing the far board's maintenance run.
func (k AttackKind) rates() (name string, strength, capture, loss int) {
	r, ok := attackKindRates[k]
	if !ok {
		r = attackKindRates[NormalAttack]
	}
	return r.name, r.strength, r.capture, r.loss
}

// String names the kind as BRE's screens and result headers do.
func (k AttackKind) String() string { name, _, _, _ := k.rates(); return name }

// strengthPct is how hard the attacker fights, as a percentage of its offense.
func (k AttackKind) strengthPct() int { _, s, _, _ := k.rates(); return s }

// capturePct is how much land the strike takes, relative to a normal attack.
func (k AttackKind) capturePct() int { _, _, c, _ := k.rates(); return c }

// lossPct is the share of each side's committed force spent before it retreats.
func (k AttackKind) lossPct() int { _, _, _, l := k.rates(); return l }

// RemoteAttack is a departed strike aimed at an empire on another board.
type RemoteAttack struct {
	ID           int
	FromBoard    string
	TargetEmpire string
	Offense      int
	Contributors []Contribution
	// Kind is how an individual strike is pressed. A group attack leaves it at
	// the zero value and is resolved as a normal attack (see Group below), which
	// is why resolution reads Group first.
	Kind AttackKind
	// Group marks a strike assembled by CreateGroupAttack. BRE gives group
	// attacks no type choice — only individual strikes pick one — so this
	// distinguishes "a group attack" from "an individual quick strike", which
	// share Kind's zero value.
	Group bool
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
	Gold    int64
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
	Survivors    []Contribution // forces returning to their contributors (per owner)
	// Kind is what came home: "terror" for a terror op, otherwise how the strike
	// was pressed — "Quick Strike", "Normal Attack", "Extended Battle" or
	// "Group Attack", which is what BRE heads each returning report with.
	Kind string
	// Outcome says WHY, which Won alone cannot: BRE's returning report prints
	// SUCCESS, FAILURE, NOT FOUND or PROTECTED, and a baron whose force never
	// found its target is owed a different sentence from one that was beaten.
	// Empty in a packet written before this existed, which reads as Won deciding
	// between success and failure — the behaviour that packet was written under.
	Outcome AttackOutcome `json:",omitempty"`
	// Enemy is what the strike destroyed on the defender, by unit type. BRE's
	// returning report itemises it beside the attacker's own casualties; without
	// it the origin board can only say whether the strike won.
	Enemy UnitLoss `json:",omitempty"`
}

// AttackOutcome is a returning strike's verdict, matching the four BRE prints
// on its result header.
type AttackOutcome string

const (
	OutcomeWon       AttackOutcome = "success"
	OutcomeRepelled  AttackOutcome = "failure"
	OutcomeNotFound  AttackOutcome = "notfound"  // no such realm on the target board
	OutcomeProtected AttackOutcome = "protected" // shielded by New Realm Protection
)

// outcome reads a result's verdict, falling back to Won for a packet written
// before Outcome existed.
func (r AttackResult) outcome() AttackOutcome {
	if r.Outcome != "" {
		return r.Outcome
	}
	if r.Won {
		return OutcomeWon
	}
	return OutcomeRepelled
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
	// Group marks a strike assembled by CreateGroupAttack, and Whole a strike
	// aimed at the planet rather than a named baron. Both are needed to word the
	// returning report and its news line: BRE keeps separate copy for an
	// individual strike, a group strike on one realm, and a group strike on a
	// whole planet, and the target board's answer carries none of that.
	Group bool `json:",omitempty"`
	Whole bool `json:",omitempty"`
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
// on targetBoard, leaving after hours hours (BRE's 12-120 window). e commits the
// detachment f (deducted from its army); ErrCantAfford if it lacks the units.
func (w *World) CreateGroupAttack(e *Empire, targetBoard, targetEmpire string, hours int, f AttackForce) (*GroupAttack, error) {
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
		DepartAt:     DepartureAfter(time.Now(), hours),
		Contributors: []Contribution{{Owner: e.Owner, AttackForce: f}},
	})
	return &w.GroupAttacks[len(w.GroupAttacks)-1], nil
}

// CreateIndividualAttack sends one baron's detachment against one named remote
// baron, BRE's "Indiv. Attack Force". Unlike a group attack it assembles nothing
// and waits for nobody: the force leaves on this run and the strike is in flight
// straight away. It spends one of the day's individual attacks, the same
// allowance a conventional attack at home draws on, and it must name a target —
// there is no whole-planet form.
//
// kind picks how the strike is pressed; it scales the offense that leaves here
// and travels with the packet so the target board can apply the matching
// capture and casualty rates.
func (w *World) CreateIndividualAttack(e *Empire, targetBoard, targetEmpire string, kind AttackKind, f AttackForce) (int, error) {
	if targetEmpire == "" {
		return 0, ErrNoTarget
	}
	if !w.CanAttack(e) {
		return 0, ErrAttacksExhausted
	}
	cost := f.GoldCost()
	if e.Gold < cost {
		return 0, ErrCantAfford
	}
	if err := e.commitForce(f); err != nil {
		return 0, err
	}
	e.Gold -= cost
	e.AttacksToday++
	w.NextAttackID++
	id := w.NextAttackID
	contributors := []Contribution{{Owner: e.Owner, AttackForce: f}}
	w.enqueue(targetBoard, RemoteAttack{
		ID:           id,
		FromBoard:    w.Config.BoardID,
		TargetEmpire: targetEmpire,
		Offense:      f.offense() * kind.strengthPct() / 100,
		Contributors: contributors,
		Kind:         kind,
	})
	w.InFlight = append(w.InFlight, InFlightStrike{
		ID:           id,
		Kind:         "attack",
		TargetBoard:  targetBoard,
		TargetEmpire: targetEmpire,
		LaunchedDay:  w.GameDay,
		Contributors: contributors,
	})
	return id, nil
}

// JoinGroupAttack commits e's detachment f to a pending group attack (before it
// leaves). ErrCantAfford if e lacks the units.
func (w *World) JoinGroupAttack(e *Empire, id int, f AttackForce) error {
	for i := range w.GroupAttacks {
		ga := &w.GroupAttacks[i]
		if ga.ID != id {
			continue
		}
		if ga.Due(time.Now(), w.GameDay) {
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

// LaunchDueGroupAttacks turns every group attack whose departure has arrived
// into an outbound RemoteAttack and removes it from the pending list. Run
// during the PLANETARY maintenance step, which is why the window is worth
// having in hours: the step runs several times a day, so a 12-hour delay really
// does leave before a day-long one.
func (w *World) LaunchDueGroupAttacks() { w.LaunchDueGroupAttacksAt(time.Now()) }

// LaunchDueGroupAttacksAt is LaunchDueGroupAttacks against a given instant, so a
// test can watch a strike sit and then leave without waiting for the clock.
func (w *World) LaunchDueGroupAttacksAt(now time.Time) {
	var remaining []GroupAttack
	for _, ga := range w.GroupAttacks {
		if !ga.Due(now, w.GameDay) {
			remaining = append(remaining, ga)
			continue
		}
		w.enqueue(ga.TargetBoard, RemoteAttack{
			ID:           ga.ID,
			FromBoard:    w.Config.BoardID,
			TargetEmpire: ga.TargetEmpire,
			Offense:      ga.Offense(),
			Contributors: ga.Contributors,
			Group:        true,
		})
		w.InFlight = append(w.InFlight, InFlightStrike{
			ID:           ga.ID,
			Kind:         "attack",
			TargetBoard:  ga.TargetBoard,
			TargetEmpire: ga.TargetEmpire,
			LaunchedDay:  w.GameDay,
			Contributors: ga.Contributors,
			Group:        true,
			Whole:        ga.TargetEmpire == "",
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
			scores = append(scores, RemoteScore{
				Empire: e.Name, NetWorth: w.NetWorth(e), Land: e.Land, Score: e.Score,
				Protected: e.Protection > 0,
			})
		}
	}
	if len(scores) == 0 {
		return
	}
	w.Outbox = append(w.Outbox, Packet{FromBoard: w.Config.BoardID, Date: w.LastMaintDate, Scores: scores})
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

// outboxFor returns the queued packet bound for board, creating it if this is
// the first payload addressed there this run. The pointer is only good until
// the next call — appending to the Outbox can move the packets — so fill it in
// before asking for another.
func (w *World) outboxFor(board string) *Packet {
	for i := range w.Outbox {
		if w.Outbox[i].ToBoard == board {
			return &w.Outbox[i]
		}
	}
	w.Outbox = append(w.Outbox, Packet{
		FromBoard: w.Config.BoardID,
		ToBoard:   board,
		Date:      w.LastMaintDate,
	})
	return &w.Outbox[len(w.Outbox)-1]
}

// enqueue appends atk to the outbound packet for toBoard, creating it if needed.
func (w *World) enqueue(toBoard string, atk RemoteAttack) {
	p := w.outboxFor(toBoard)
	p.Attacks = append(p.Attacks, atk)
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
	p := w.outboxFor(targetBoard)
	p.Terrors = append(p.Terrors, t)
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
	// Origin BEFORE anything is recorded about the packet, including replay
	// bookkeeping (#118). SeenPacket raises HighSeq[FromBoard], so checking it
	// first would let a forged packet with a large sequence number poison the
	// counter and silently drop every genuine packet from the board it
	// impersonates — a cheaper attack than the forgery this exists to stop.
	//
	// A packet from a board the roster names a key for has to prove it is that
	// board, or none of it is applied. Everything below — scores, strikes,
	// results, mail — was previously believed on the strength of the FromBoard
	// string alone, so one file dropped in the inbound directory could grant a
	// realm an army or take its regions.
	//
	// A board the roster names NO key for is still applied, because that is
	// every league until its Coordinator publishes one, and refusing would break
	// a working league on upgrade rather than securing it. That transition state
	// is the remaining gap, and it closes as rosters gain keys.
	if ok, checked := w.VerifyBoardOrigin(p); checked && !ok {
		w.postNews(fmt.Sprintf("A packet claiming to be from %s did not match that board's key and was refused.", p.FromBoard))
		return Packet{}
	}
	// A packet stamped with an Epoch older than this board's current one was
	// written for a game this board has since wiped — most often its own
	// unprocessed leftovers surviving a -reset. Applying it would grant units,
	// take regions, or deliver mail against empires that no longer exist, so it
	// is discarded whole rather than applied to the fresh world (#104). Checked
	// before SeenPacket for the same reason as the origin check above: a stale
	// packet should not get to poison the replay counter either.
	if p.Epoch > 0 && p.Epoch < w.Epoch {
		w.postNews(fmt.Sprintf("A packet from %s predates this world's current game and was discarded.", p.FromBoard))
		return Packet{}
	}
	// Applying the same packet twice would pay out a strike's results, a
	// broadcast or a reset all over again, so a packet already seen here is
	// dropped whole (#53).
	if w.SeenPacket(p) {
		return Packet{}
	}
	// Anything that dictates to this board has to be signed by the Coordinator.
	// Positional trust — believing whoever names themselves node 1 — is what this
	// replaces, because the board name in a packet is just a string a file can
	// claim (#53).
	orders := w.fromCoordinator(p) && w.VerifyCoordinatorOrders(p)
	// The Coordinator rebroadcasts on every run, so the news only carries what
	// actually changed here — otherwise a quiet league fills the planet's paper
	// with the same line once per exchange.
	if p.LeagueConfig != nil && orders {
		if *p.LeagueConfig != *w.Config.leagueRuleset() {
			w.Config.applyLeagueRuleset(p.LeagueConfig)
			w.postNews("The League Coordinator updated the league settings.")
		}
	}
	// The roster travels the same way and under the same guard (#64).
	if len(p.LeagueNodes) > 0 && orders && !SameRoster(w.LeagueNodes, p.LeagueNodes) {
		w.LeagueNodes = append([]LeagueNode(nil), p.LeagueNodes...)
		w.postNews("The League Coordinator updated the league roster.")
	}
	if p.Reset != nil && orders {
		w.applyLeagueReset(p.Reset)
	}
	if carriesCoordinatorOrders(p) && !orders {
		w.postNews(fmt.Sprintf("A packet from %s claimed to carry League Coordinator orders and was refused.", p.FromBoard))
	}
	if len(p.Scores) > 0 {
		w.ImportBoard(RemoteBoard{BoardID: p.FromBoard, Date: p.Date, Scores: p.Scores})
	}
	// Outcomes of our own strikes, returning from the target board.
	for _, res := range p.Results {
		w.applyAttackResult(res)
	}
	if p.Annihilator != nil && p.FromBoard != "" {
		w.applyAnnihilatorStatus(p.Annihilator)
	}
	// Scouting answers coming home. They land in the planet-wide Spy Database,
	// so the whole board benefits from one baron's agent (#61).
	for _, r := range p.ReconReports {
		w.SpyDatabase = append(w.SpyDatabase, r)
		w.postNews(fmt.Sprintf("Our agents reported back on %s of %s.", r.Empire, r.Board))
	}
	for _, m := range p.IPMessages {
		w.deliverIPMessage(m)
	}
	result := Packet{FromBoard: w.Config.BoardID, ToBoard: p.FromBoard, Date: w.LastMaintDate}
	// A probe naming us goes straight back; one of ours coming home is measured.
	result.TimeChecks = w.applyTimeChecks(p.TimeChecks)
	// Scouting asked of us: answer with what is true here and now.
	for _, req := range p.Recon {
		if e := w.remoteTarget(req.TargetEmpire); e != nil {
			result.ReconReports = append(result.ReconReports, SpyReport{
				Board:   w.Config.BoardID,
				Empire:  e.Name,
				Date:    w.LastMaintDate,
				Land:    e.Land,
				Offense: e.Offense(),
				Defense: e.Defense(),
				Gold:    e.Gold,
			})
			e.addEvent("Foreign agents were seen taking an interest in your realm.")
		}
	}
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
//
// Both armies press until they have spent the attack type's share of what they
// brought and then break off — BRE states that for the interplanetary variants
// outright ("both sides will only fight until they suffer 8% losses",
// game/attack.hlp), which is why this does not run the local Regular Attack's
// round-by-round attrition. That resolver exists to make the WINNER's casualties
// an outcome rather than a rate; here the rate is the published mechanic. The
// sysop's Attack Damage rescales it (Level.InterplanetaryLossPct).
func (w *World) resolveRemoteAttack(atk RemoteAttack) AttackResult {
	target := w.remoteTarget(atk.TargetEmpire)
	res := AttackResult{ID: atk.ID, TargetBoard: w.Config.BoardID, TargetEmpire: atk.TargetEmpire}
	kind := atk.Kind
	// A group attack has no type of its own, so it fights on the normal
	// attack's terms — and it is the baseline the individual strike's doubled
	// returns are measured against.
	returnsPct := 100
	if atk.Group {
		kind = NormalAttack
		res.Kind = "Group Attack"
	} else {
		res.Kind = kind.String()
		returnsPct = IndividualAttackReturnsPct
	}
	lossPct := w.Config.AttackDamage.InterplanetaryLossPct(kind.lossPct())
	// Survivors return to their contributors whatever the outcome — the force
	// bleeds on the way in, not only when it wins.
	res.Survivors = survivorsOf(atk.Contributors, lossPct)
	if target == nil {
		res.Outcome = OutcomeNotFound
		return res
	}
	res.TargetEmpire = target.Name
	// A target still under New Realm Protection is shielded, same as an incoming
	// terror op (resolveRemoteTerror). Protection counts down in transit, so this
	// arrival-time check — not the sender's view days earlier — is authoritative.
	if target.Protection > 0 {
		res.Outcome = OutcomeProtected
		target.addEvent(fmt.Sprintf("An interplanetary strike from %s was stopped by your New Realm Protection.", atk.FromBoard))
		return res
	}
	// Measure the defence BEFORE the battle costs it — the fight is decided by
	// what was standing when the force arrived, not by what is left afterwards.
	def := target.Defense()
	// The defender spends the same share of its own forces holding the line, win
	// or lose. loseForces is the local battle's own helper, so a turret lost to an
	// invader is accounted exactly as one lost at home.
	res.Enemy = loseForces(target, float64(lossPct)/100)
	if atk.Offense <= def {
		res.Outcome = OutcomeRepelled
		target.addEvent(fmt.Sprintf("You repelled an interplanetary strike from %s. You lost %d of your forces.",
			atk.FromBoard, res.Enemy.Total()))
		return res
	}
	// Overwhelmed: take a bite of land proportional to the margin, scaled by what
	// this kind of strike can carry off (capped). The margin is widened to int64
	// first: a league-sized strike can pass 21 million offense, and multiplying
	// that by 100 wraps a 32-bit int on the door builds this project supports.
	frac := int64(atk.Offense-def) * 100 / int64(max(atk.Offense, 1))
	land := int(int64(target.Land) * frac / 100 / 4 * int64(kind.capturePct()) / 100 * int64(returnsPct) / 100)
	if land > target.Land {
		land = target.Land
	}
	if land > 0 {
		target.Regions.remove(land)
		target.syncLand()
	}
	target.addEvent(fmt.Sprintf("An interplanetary strike from %s took %d regions and %d of your forces!",
		atk.FromBoard, land, res.Enemy.Total()))
	res.LandTaken = land
	res.Won = true
	res.Outcome = OutcomeWon
	return res
}

// survivorsOf returns each contributor's detachment reduced by lossPct — the
// forces that come home after the strike.
func survivorsOf(cs []Contribution, lossPct int) []Contribution {
	keep := 100 - lossPct
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

// takeInFlight removes the strike with this ID from the waiting list and returns
// it, because its result has come home. ok is false when nothing was waiting —
// see applyAttackResult for why that is not the same as "harmless".
func (w *World) takeInFlight(id int) (InFlightStrike, bool) {
	for i, f := range w.InFlight {
		if f.ID == id {
			w.InFlight = append(w.InFlight[:i], w.InFlight[i+1:]...)
			return f, true
		}
	}
	return InFlightStrike{}, false
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

// SendRecon queues a scouting request at targetEmpire on targetBoard, spending
// one of e's agents. The answer arrives with a later packet and lands in the
// planet-wide Spy Database (#61).
func (w *World) SendRecon(e *Empire, targetBoard, targetEmpire string) error {
	if e.Agents < 1 {
		return ErrNoAgents
	}
	if targetEmpire == "" {
		return ErrNoTarget
	}
	e.Agents--
	w.NextAttackID++
	req := ReconRequest{ID: w.NextAttackID, FromBoard: w.Config.BoardID, FromOwner: e.Owner, TargetEmpire: targetEmpire}
	p := w.outboxFor(targetBoard)
	p.Recon = append(p.Recon, req)
	return nil
}

// ExportAnnihilatorStatus tells the targeted planet about this one's weapon, whether
// it is still being funded or already in the air (#63).
func (w *World) ExportAnnihilatorStatus() {
	if w.Annihilator == nil {
		return
	}
	d := w.Annihilator
	w.enqueueAnnihilator(d.TargetBoard, &AnnihilatorStatus{
		FromBoard:  w.Config.BoardID,
		Funded:     d.Funded,
		Launched:   d.Launched,
		ArrivesDay: d.ArrivesDay,
		Intact:     d.Intact,
	})
}

// ExportAnnihilatorGone tells the targeted planet to stop watching, because the
// weapon aimed at it was dismantled.
func (w *World) ExportAnnihilatorGone(board string) {
	w.enqueueAnnihilator(board, &AnnihilatorStatus{FromBoard: w.Config.BoardID, Dismantled: true})
}

func (w *World) enqueueAnnihilator(board string, st *AnnihilatorStatus) {
	w.outboxFor(board).Annihilator = st
}

// applyAnnihilatorStatus takes in what another planet says about the weapon it is
// pointing at us, and posts the warning its barons need.
func (w *World) applyAnnihilatorStatus(st *AnnihilatorStatus) {
	if st.Dismantled {
		if w.Incoming != nil && w.Incoming.Creator == st.FromBoard {
			w.Incoming = nil
			w.postNews(fmt.Sprintf("The Clingy Annihilator being built at %s has been dismantled.", st.FromBoard))
		}
		return
	}
	first := w.Incoming == nil
	if first {
		w.Incoming = &Annihilator{Creator: st.FromBoard, Intact: 100}
	}
	in := w.Incoming
	wasFlying := in.Launched
	in.TargetBoard = w.Config.BoardID
	in.ArrivesDay = st.ArrivesDay
	if st.Launched {
		in.Launched = true
	}
	if st.Intact > 0 && st.Intact < in.Intact {
		in.Intact = st.Intact
	}
	switch {
	case st.Launched && !wasFlying:
		hours := (st.ArrivesDay - w.GameDay) * 24
		if hours < 0 {
			hours = 0
		}
		w.postNews(fmt.Sprintf("A Clingy Annihilator arrives from %s in %d hours.", st.FromBoard, hours))
	case first:
		w.postNews(fmt.Sprintf("A Clingy Annihilator destined for our planet is under construction at %s.", st.FromBoard))
	}
}

// ArriveAnnihilator detonates an incoming weapon whose flight is over. Run from the
// planetary step, so the jets get every day of the flight to shoot at it.
func (w *World) ArriveAnnihilator() {
	if w.Incoming == nil || !w.Incoming.Launched || w.GameDay < w.Incoming.ArrivesDay {
		return
	}
	intact := w.Incoming.Intact
	w.Incoming = nil
	w.DetonateAnnihilator(intact)
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
