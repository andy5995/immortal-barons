package game

import (
	"errors"
	"fmt"
	"time"
)

// ibbs.go — the inter-BBS packet itself: what a packet carries, how one is
// addressed to a board's outbox, and how an arriving one is applied.
//
// The mechanics that ride in a packet live beside it in the other ibbs_*.go
// files.

var (
	ErrNoAttack = errors.New("no such group attack")
	ErrDeparted = errors.New("that attack force has already left")
	// The per-day interplanetary allowances (Config.MaxGroupAttacks and
	// MaxTerrorOps), the counterparts of the individual-attack cap.
	ErrGroupAttacksExhausted = errors.New("You have already launched all of your group attacks for today.")
	ErrTerrorOpsExhausted    = errors.New("You have already launched all of your terrorist operations for today.")
	ErrAttacksExhausted      = errors.New("You have already launched all of your attacks for today.")
	ErrNoTarget              = errors.New("An individual attack must name a baron to strike.")
	// The interplanetary Special Operations menu's refusals (#49). Bombing Ops
	// and Missile Ops are the sysop's two switches over that menu, and the
	// bomber floor is the original's delivery requirement.
	ErrBombingOpsExhausted = errors.New("You have already launched all of your bombing operations for today.")
	ErrBombingOpsDisabled  = errors.New("Bombing operations are not part of this game.")
	ErrMissileOpsDisabled  = errors.New("Missile operations are not part of this game.")
	ErrNeedBombers         = fmt.Errorf("You need at least %d Bombers to deliver a payload.", BombingBombersRequired)
)

// Packet carries inter-BBS actions from one board to another (or, with an empty
// ToBoard, broadcast to the whole league).
type Packet struct {
	FromBoard string
	ToBoard   string
	Date      string
	Scores    []RemoteScore  // score share (feeds RemoteBoards / IP scores)
	Attacks   []RemoteAttack // strikes landing on ToBoard
	Terrors   []RemoteTerror // terror ops landing on ToBoard
	// SpecialOps are the InterPlanetary Special Operations menu's strikes (#49),
	// carried apart from Terrors: a different menu, a different per-day
	// allowance, and no committed agents.
	SpecialOps   []RemoteSpecialOp  `json:",omitempty"`
	Results      []AttackResult     // outcomes returning to the origin
	LeagueConfig *LeagueConfig      // LC-authored league settings (nil if absent)
	LeagueNodes  []LeagueNode       // LC-authored league roster (nil if absent, #64)
	Recon        []ReconRequest     // scouting asked of ToBoard (#61)
	ReconReports []SpyReport        // answers coming back to the origin (#61)
	Annihilator  *AnnihilatorStatus // a doomsday weapon aimed at ToBoard (#63)
	TimeChecks   []TimeCheck        // round-trip probes, out and echoed back (Travel Times)
	IPMessages   []IPMessage        // interplanetary mail for ToBoard's barons
	// SpyGuys are watchers posted TO ToBoard, and News the lines they send home:
	// a SpyGuy's report is planet news on the planet that paid for him, which is
	// what BRE's NEWS_DATA record makes it. Both omitempty for the reason the
	// trading fields below give — an older board must still verify a packet that
	// carries neither.
	SpyGuys []SpyGuyDispatch `json:",omitempty"`
	News    []string         `json:",omitempty"`
	// Interplanetary trading (IB's own). All three are omitempty ON PURPOSE: the
	// origin signature is taken over the marshalled packet, so a board too old to
	// know these fields would drop them on unmarshal and then fail to verify a
	// packet that carried them. Omitting them when empty keeps every packet that
	// does NOT trade byte-identical across versions, which is every packet an old
	// board can act on anyway.
	TradeBids  []IPTradeBid    `json:",omitempty"` // buy orders landing on ToBoard's market
	TradeFills []IPTradeFill   `json:",omitempty"` // their answers coming home
	Market     []RemoteListing `json:",omitempty"` // FromBoard's market, riding its scores
	// TradeDeals are one-way shipments of goods to a named realm on ToBoard
	// (#195) — a different mechanic from the bids above, which buy against a
	// market and get an answer back. Nothing returns for these: the goods are
	// paid for and gone when the deal is sent. omitempty for the same reason as
	// every field around it.
	TradeDeals []IPTradeDeal `json:",omitempty"`
	// Notice is a plain-text bounce: this board refused a packet and is telling
	// the sender why. It carries NO payload, deliberately — see bounceVersion.
	Notice string `json:",omitempty"`
	// Version is the game version the sender is running, for the BBSINFO report.
	// omitempty on the same grounds as the fields above: the origin signature is
	// taken over the marshalled packet, so a board that does not know this field
	// must still see byte-identical bytes for every packet that omits it.
	Version string `json:",omitempty"`
	// Seq numbers this board's outbound packets so the far side can spot one it
	// has already applied, and Signature authenticates the parts only the
	// Coordinator may author (#53).
	Seq       uint64
	Signature []byte
	// BoardSig is the sending board's own signature over this whole packet, so
	// FromBoard is proven rather than claimed (#118). Verified against the
	// public key the signed roster carries for that board.
	BoardSig []byte
	// Bulletins is the Coordinator's complete league bulletin set (see
	// bulletin.go). A pointer with omitempty for the reason the trading fields
	// give: a board too old to know the field must still see byte-identical
	// bytes for every packet that does not carry one.
	Bulletins *BulletinSet `json:",omitempty"`
	Reset     *LeagueReset // Coordinator's order to start a new season (#65)
	// League is the Coordinator's league number, so a board playing in two
	// leagues that share one inbound directory can tell the traffic apart.
	League int
	// Hops counts the boards that have forwarded this packet. Outside the
	// signed payload, because every hub changes it.
	Hops int
	// Protocol is the packet format this board speaks, and the ONLY thing a
	// board must match to exchange packets. It moves when the wire format moves,
	// not when the game version does, so a board can take a menu fix or a
	// balance change without the league having to move in lockstep.
	//
	// omitempty AND excluded from the origin signature (boardSigningBytes), both
	// deliberately: a board that predates the field omits it, and one that has it
	// signs as though it did not, so the two verify each other's packets. Without
	// that, introducing this field would break every board pair across the
	// change — the exact fault it exists to prevent.
	//
	// It is deliberately NOT signed. Tampering with it can only make the far side
	// hold a packet it would otherwise apply, which is a denial of service a
	// dropped file achieves anyway; signing it would buy nothing and cost the
	// cross-version compatibility above.
	Protocol int `json:",omitempty"`
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

// PacketType returns a short human-readable label for the packet's primary
// content, for verbose logging (-detailed).
func (p Packet) PacketType() string {
	switch {
	case p.LeagueConfig != nil:
		return "league config"
	case len(p.LeagueNodes) > 0:
		return "roster"
	case p.Reset != nil:
		return "reset"
	case p.Bulletins != nil:
		return "bulletins"
	case len(p.Attacks) > 0 && len(p.Results) > 0:
		return "attacks, results"
	case len(p.Attacks) > 0:
		return "attacks"
	case len(p.Terrors) > 0:
		return "terror ops"
	case len(p.SpecialOps) > 0:
		return "special ops"
	case len(p.Results) > 0:
		return "results"
	case len(p.Scores) > 0:
		return "scores"
	case len(p.Recon) > 0:
		return "recon"
	case len(p.ReconReports) > 0:
		return "recon reports"
	case p.Annihilator != nil:
		return "annihilator"
	case len(p.TimeChecks) > 0:
		return "time checks"
	case len(p.IPMessages) > 0:
		return "ip messages"
	case len(p.SpyGuys) > 0:
		return "spyguys"
	case len(p.News) > 0:
		return "news"
	case len(p.TradeBids) > 0 || len(p.TradeFills) > 0:
		return "trade"
	case len(p.TradeDeals) > 0:
		return "trade deals"
	case len(p.Market) > 0:
		return "market"
	case p.Notice != "":
		return "notice"
	default:
		return "empty"
	}
}

// HasPayload reports whether p carries anything worth sending. The transport
// asks before queueing a reply packet, so an answer that is only recon reports
// or only an echoed probe still goes out.
func (p Packet) HasPayload() bool {
	return len(p.Scores) > 0 || len(p.Attacks) > 0 || len(p.Terrors) > 0 ||
		len(p.SpecialOps) > 0 ||
		len(p.Results) > 0 || len(p.Recon) > 0 || len(p.ReconReports) > 0 ||
		len(p.TimeChecks) > 0 || len(p.IPMessages) > 0 ||
		len(p.SpyGuys) > 0 || len(p.News) > 0 ||
		len(p.TradeBids) > 0 || len(p.TradeFills) > 0 || len(p.TradeDeals) > 0 || p.Notice != "" ||
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

// AnnihilatorStatus tells a planet about a Gooie Kablooie aimed at it — while it is
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

// outboxFor returns the queued packet bound for board, creating it if this is
// the first payload addressed there this run. The pointer is only good until
// the next call — appending to the Outbox can move the packets — so fill it in
// before asking for another.
func (w *World) outboxFor(board string) *Packet {
	// The backstop for every way a destination can be named. A board the roster
	// cannot place is unroutable by construction (see Routable): the packet would
	// be handed from board to board unchanged until the hop cap destroyed it. It
	// is refused here rather than at each call site so that a call site added
	// later cannot put one on the wire. The caller still gets somewhere to write,
	// and what it writes is thrown away.
	if !w.Routable(board) {
		if !w.unroutableNoted[board] {
			if w.unroutableNoted == nil {
				w.unroutableNoted = map[string]bool{}
			}
			w.unroutableNoted[board] = true
			w.noteSysop("Nothing can be sent to %s: no board of that name is on the league roster, so no route to it exists. Anything addressed there is being discarded here rather than circling the league.", board)
		}
		w.unroutableSink = Packet{FromBoard: w.Config.BoardID, ToBoard: board, Date: w.LastMaintDate}
		return &w.unroutableSink
	}
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

// enqueueTradeBid queues a buy order for another planet's market.
func (w *World) enqueueTradeBid(toBoard string, b IPTradeBid) {
	p := w.outboxFor(toBoard)
	p.TradeBids = append(p.TradeBids, b)
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
		w.noteSysop("A packet claiming to be from %s did not match that board's key and was refused.", p.FromBoard)
		return Packet{}
	}
	// Applying the same packet twice would pay out a strike's results, a
	// broadcast or a reset all over again, so a packet already seen here is
	// dropped whole (#53).
	if w.SeenPacket(p) {
		return Packet{}
	}
	// The Coordinator may require a version of the whole league. A board below it
	// has its packets refused here — the only lever a Coordinator has once the
	// packet format has moved on, since an old board cannot even verify the
	// signature on a packet carrying fields it does not know.
	//
	// IB refuses only the OFFENDING board. The original is said to stop the
	// Coordinator processing outbound traffic at all until the laggard upgrades;
	// that is a recollection, not something read out of the binary, and holding a
	// whole league hostage to one stale board is too destructive to copy on a
	// maybe. Recorded in docs/mechanics-reference.md as unverified.
	if p.FromBoard != "" && p.FromBoard != w.Config.BoardID && !w.BoardMeetsMinVersion(p.Version) {
		return w.bounceVersion(p)
	}
	// A bounce carries no payload, so there is nothing to apply and nothing to
	// answer. Delivering the notice and stopping is what keeps two boards that
	// both refuse each other from bouncing the same packet back and forth.
	if p.Notice != "" {
		w.noteSysop("%s refused our packet: %s", p.FromBoard, p.Notice)
		return Packet{}
	}
	// Past every guard, so this records packets actually ACCEPTED — a forged or
	// replayed one must not make a silent board look like it is still talking,
	// which is the whole question LastPacketReport answers.
	if p.FromBoard != "" && p.FromBoard != w.Config.BoardID {
		if w.LastPacketFrom == nil {
			w.LastPacketFrom = map[string]string{}
		}
		// A wall-clock stamp, not the game date: the original's own BBSINFO.LST
		// shows MM/DD/YYYY HH:MM:SS, and the question ("has this board gone
		// quiet?") is about real elapsed time, which a game date cannot answer on
		// a league whose clock has stalled.
		w.LastPacketFrom[p.FromBoard] = time.Now().Format(RecordedTimeFormat)
		if p.Version != "" {
			if w.BoardVersion == nil {
				w.BoardVersion = map[string]string{}
			}
			w.BoardVersion[p.FromBoard] = p.Version
		}
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
	// The roster travels the same way and under the same guard (#64). It arrives
	// as a struct rather than through the roster PARSER, so the node-number
	// range is applied here as well (#180) — otherwise a number the file format
	// rejects could still reach this board over the wire and last until the next
	// restart, when re-reading the written file would silently drop it.
	if len(p.LeagueNodes) > 0 && orders {
		if nodes := usableNodes(p.LeagueNodes); !SameRoster(w.LeagueNodes, nodes) {
			w.LeagueNodes = nodes
			w.postNews("The League Coordinator updated the league roster.")
		}
	}
	if p.Reset != nil && orders {
		w.applyLeagueReset(p.Reset)
	}
	// The league's bulletins travel with the ruleset and the roster, under the
	// same guard: the Coordinator sends the whole set every run, and this board
	// files news only for what actually changed here (see bulletin.go).
	if p.Bulletins != nil && orders {
		w.applyBulletins(*p.Bulletins)
	}
	if carriesCoordinatorOrders(p) && !orders {
		w.noteSysop("A packet from %s claimed to carry League Coordinator orders and was refused: %s.",
			p.FromBoard, w.CoordRefusalReason(p))
	}
	if len(p.Scores) > 0 {
		w.ImportBoard(RemoteBoard{BoardID: p.FromBoard, Date: p.Date, Scores: p.Scores, Market: p.Market})
		w.applyDupeCheck(p.FromBoard, p.Scores)
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
	// One-way shipments land straight on the realm they name (#195). Nothing goes
	// back: the sender paid on the way out and the recipient gets no say.
	for _, d := range p.TradeDeals {
		w.deliverIPTradeDeal(d)
	}
	// Answers to our own bids: goods or gold, straight to the baron who bid (#47).
	for _, f := range p.TradeFills {
		w.applyTradeFill(f)
	}
	result := Packet{FromBoard: w.Config.BoardID, ToBoard: p.FromBoard, Date: w.LastMaintDate}
	// Bids landing HERE are filled or refused against this board's market now,
	// and the answer rides the reply home.
	for _, b := range p.TradeBids {
		result.TradeFills = append(result.TradeFills, w.resolveRemoteTradeBid(b))
	}
	// A probe naming us goes straight back; one of ours coming home is measured.
	result.TimeChecks = w.applyTimeChecks(p.TimeChecks)
	// A watcher posted here settles in and answers at once with whatever this
	// planet already has aimed at his; his own reports go out later, as the
	// strikes are prepared. News coming the other way IS his report — it is
	// planet news here, which is what the original's NEWS_DATA record makes it.
	for _, d := range p.SpyGuys {
		w.receiveSpyGuy(d)
	}
	for _, line := range p.News {
		w.postNews(line)
	}
	// Scouting asked of us: answer with what is true here and now.
	for _, req := range p.Recon {
		// An empty TargetEmpire is a GLOBAL request — the Coordinator's sweep of
		// the whole league (#48) — and is answered with every living realm here
		// rather than one. A request naming a realm is answered with that one.
		if req.TargetEmpire == "" {
			for _, e := range w.Empires {
				if !e.Alive || e.Owner == "" {
					continue
				}
				result.ReconReports = append(result.ReconReports, w.spyReport(e))
			}
			continue
		}
		if e := w.remoteTarget(req.TargetEmpire); e != nil {
			result.ReconReports = append(result.ReconReports, w.spyReport(e))
			e.addEvent("Foreign agents were seen taking an interest in your realm.")
		}
	}
	for _, atk := range p.Attacks {
		result.Results = append(result.Results, w.resolveRemoteAttack(atk))
	}
	// A covert operation landing here reports the state it found its target in,
	// and that is what fills the sender's Spy Database — the original's own
	// arrangement, where resolve_received_covert_operation calls write_spy_report
	// and the answer reaches the sender as "Information added to Global Spy Data
	// Bank". Intelligence is a by-product of acting, not an errand of its own.
	for _, t := range p.Terrors {
		result.Results = append(result.Results, w.resolveRemoteTerror(t))
		if e := w.remoteTarget(t.TargetEmpire); e != nil {
			result.ReconReports = append(result.ReconReports, w.spyReport(e))
		}
	}
	for _, op := range p.SpecialOps {
		result.Results = append(result.Results, w.resolveRemoteSpecialOp(op))
	}
	return result
}

// bounceVersion refuses a packet from a board below the league's required
// version and tells the sender so, rather than dropping it in silence — a board
// whose packets vanish has no way to learn that it is the one at fault.
//
// The reply carries a NOTICE and nothing else. It is tempting to return the
// packet itself, which is how "send it back" is usually described, but that
// packet holds attacks, bids and mail aimed at THIS board: handing it back would
// have the origin apply its own strikes against its own realms. What the sender
// needs is the reason, and the reason is one line.
//
// The sender's own in-flight forces and escrowed gold are not stranded by this:
// nothing was applied here, so its lost-packet timer returns them on schedule
// (ReturnLostForces).
func (w *World) bounceVersion(p Packet) Packet {
	ver := "an unstated version"
	if p.Version != "" {
		ver = "v" + p.Version
	}
	w.noteSysop("A packet from %s was refused: it runs %s, and this league requires v%s.",
		p.FromBoard, ver, w.Config.MinBoardVersion)
	return Packet{
		FromBoard: w.Config.BoardID, ToBoard: p.FromBoard, Date: w.LastMaintDate,
		Notice: fmt.Sprintf("this league requires v%s and your board runs %s; upgrade and the packet will be accepted",
			w.Config.MinBoardVersion, ver),
	}
}

// FitColumn cuts text to width runes so an over-long name cannot push a table's
// later columns off the row. A `%-*s` verb pads but never trims, so without this
// one long board name shifts every column after it and wraps the line.
//
// Presentational only — the stored name is untouched. Packets are routed by
// board NAME when they carry no node number, so a shortened name could match the
// wrong board; the only cap applied to the stored value is MaxNodeNameLen, which
// sits far above any real one.
func FitColumn(text string, width int) string { return FitColumnMark(text, width, "…") }

// FitColumnMark is FitColumn with the truncation marker named by the caller.
// The marker's own width is part of the arithmetic, which is what makes this
// worth a parameter: the CP437 and plain-ASCII writers rewrite "…" as three
// dots BELOW every layer that counts columns, so a caller on either of those
// has to fit the cell with the marker it will really get (#196).
func FitColumnMark(text string, width int, mark string) string {
	r := []rune(text)
	if len(r) <= width {
		return text
	}
	m := []rune(mark)
	if width <= len(m) {
		return string(r[:max(0, width)])
	}
	return string(r[:width-len(m)]) + mark
}
