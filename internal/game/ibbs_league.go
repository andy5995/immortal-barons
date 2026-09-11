package game

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

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
	SabreConstantDial    int       `json:",omitempty"`
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
		SabreConstantDial:    c.SabreConstantDial,
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
	c.SabreConstantDial = lc.SabreConstantDial
}

// CoordinatorBoardID is the name of node #1 in the roster — the League
// Coordinator's board. Empty if no roster is loaded.
func (w *World) CoordinatorBoardID() string { return w.NodeName(1) }

// IsLeagueCoordinator reports whether this board is node #1 (the LC).
func (w *World) IsLeagueCoordinator() bool {
	return w.Config.BoardID != "" && w.Config.BoardID == w.CoordinatorBoardID()
}

// RulesetFingerprint identifies the league rules this board is actually playing
// by. It is taken over the broadcast ruleset alone, never the whole config:
// perBoardConfigFields are MEANT to differ from board to board, so hashing them
// would report every board in a healthy league as divergent (#264).
//
// Short by design — it is compared, never inverted, and it is printed on a
// report a sysop reads.
func (c Config) RulesetFingerprint() string {
	b, err := json.Marshal(c.leagueRuleset())
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:4])
}

// ExportLeagueConfig queues a broadcast packet carrying this board's league
// rules. Only the Coordinator sends it; member boards accept it only when it
// comes from node #1 (see ApplyPacket). It goes out on every planetary run,
// like the roster, so a board that was down for one broadcast — or joined the
// league after it — heals on the next run instead of playing its own numbers
// for the rest of the season (#264). The original does the same on its
// planetary step.
//
// Prepended, not appended — see ExportNodeList's doc comment for why: the
// same Seq-ordering reasoning applies to every packet type
// CarriesCoordinatorOrders recognizes, this one included.
func (w *World) ExportLeagueConfig() {
	if !w.IsLeagueCoordinator() {
		return
	}
	w.Outbox = append([]Packet{{
		FromBoard:    w.Config.BoardID,
		Date:         w.LastMaintDate,
		LeagueConfig: w.Config.leagueRuleset(),
	}}, w.Outbox...)
}

// RulesetDivergent reports whether a packet comes from a board playing by rules
// that are not the league's, which is grounds to HOLD it: a board running its
// own turns-per-day or attack limits feeds scores, strikes and trades into
// everyone else's game (#264).
//
// Three cases are deliberately NOT divergent, and each of them would break the
// league if they were:
//
//   - a packet from the COORDINATOR, whose ruleset is the league's by
//     definition. It is also the packet that carries a change, so holding it
//     would throw away the only thing that heals a board that has fallen behind;
//   - a packet from a board that states no fingerprint at all, which is a board
//     older than the field;
//   - anything at all until this board knows what the league's rules are — on a
//     member that is the Coordinator's last-reported fingerprint, so a board
//     that has not heard from the Coordinator yet holds nobody.
func (w *World) RulesetDivergent(p Packet) bool {
	if p.Ruleset == "" || w.senderIsCoordinator(p) {
		return false
	}
	league := w.leagueRulesetFingerprint()
	if league == "" || p.Ruleset == league {
		return false
	}
	return !w.withinRulesetGrace(p.Ruleset)
}

// RulesetGraceDays is how long a packet stating the PREVIOUS league ruleset is
// still applied after a change. Without it a rules change destroys the traffic
// already in flight when it lands: those packets state the rules they were
// written under, that fingerprint never becomes current again, and they would be
// re-held on every run until they expired. Round trips between boards are
// measured in days (Travel Times), and boards adopt a change on their own next
// planetary run, so a league always has such packets in flight at a change.
//
// A week is long enough for a slow link and a board that runs its planetary step
// once a day, and short enough that it cannot cover a board simply playing its
// own game.
const RulesetGraceDays = 7

// withinRulesetGrace reports whether a fingerprint is the ruleset this board
// held until recently. Both halves are needed: an unset date means no change has
// been recorded, and an unparseable one is a corrupt clock, neither of which may
// silently widen the gate.
func (w *World) withinRulesetGrace(fp string) bool {
	if fp == "" || fp != w.PrevLeagueRuleset || w.PrevRulesetAt == "" {
		return false
	}
	// A stamp written before stamps carried a zone cannot be placed on a clock
	// (ParseStamp), so the grace closes rather than widening by whatever the
	// board's offset from UTC happens to be.
	changed, ok := ParseStamp(w.PrevRulesetAt)
	if !ok {
		return false
	}
	return time.Since(changed) <= RulesetGraceDays*24*time.Hour
}

// NoteLeagueRuleset records the rules this board now takes to be the league's,
// keeping the one before it so packets written under it are still applied for
// RulesetGraceDays. Called at the top of a planetary run, which is both where a
// change from the Coordinator lands and where the gate is applied.
func (w *World) NoteLeagueRuleset() {
	fp := w.leagueRulesetFingerprint()
	if fp == "" || fp == w.LeagueRuleset {
		return
	}
	if w.LeagueRuleset != "" {
		w.PrevLeagueRuleset = w.LeagueRuleset
		w.PrevRulesetAt = Recorded(time.Now())
	}
	w.LeagueRuleset = fp
}

// NoteBoardRuleset records what a board says it is playing by. Called on a
// packet that is APPLIED and on one that is HELD for divergence: a board that
// has never played the league's rules has no applied packet to learn it from,
// and leaving it unrecorded is what kept BBSINFO reporting the very board this
// was built for as "unknown" rather than divergent.
func (w *World) NoteBoardRuleset(board, fingerprint string) {
	if board == "" || fingerprint == "" || board == w.Config.BoardID {
		return
	}
	if w.BoardRuleset == nil {
		w.BoardRuleset = map[string]string{}
	}
	w.BoardRuleset[board] = fingerprint
}

// senderIsCoordinator reports whether a packet was written by node #1. It is
// deliberately NOT fromCoordinator, which answers a different question — that
// one is false on the Coordinator's own board, because nobody dictates to it.
func (w *World) senderIsCoordinator(p Packet) bool {
	if p.FromNode != 0 {
		return p.FromNode == 1
	}
	return p.FromBoard != "" && p.FromBoard == w.CoordinatorBoardID()
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

// ScoredBoards names every planet this board holds SCORES for — the realms on
// it are known by name, so it can be attacked, terrorised, traded with or
// messaged by baron.
//
// This is a SMALLER set than KnownBoards and the difference matters. A planet
// reaches the roster the moment the Coordinator lists it, and can be addressed
// from then on; its scores arrive only when it next exchanges packets. So a
// freshly listed planet is known and unscored, and an action that needs a baron
// there has nobody to offer. Three screens derived this set by hand, which said
// nothing about why it differed from the other one.
func (w *World) ScoredBoards() []string {
	boards := make([]string, 0, len(w.RemoteBoards))
	for _, b := range w.RemoteBoards {
		boards = append(boards, b.BoardID)
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
				Protected: e.Protection > 0, FormerName: e.FormerName,
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
