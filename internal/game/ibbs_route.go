package game

import (
	"encoding/json"
	"time"
)

// Packet routing. Where the Coordinator has arranged the league as a tree, the
// HOST lines in the roster (BRE's BRNODES.DAT) say which neighbor each board
// hands a packet to. A leaf board then configures one link, to its uplink,
// however many boards the league has — which is what makes a twenty-board
// league something a hobbyist sysop can join (#106). Until a Coordinator writes
// a HOST line the league is a mesh, and none of this is in play; see Routed.

// MaxPacketHops bounds how many boards a packet may be forwarded through before
// it is discarded. A routing tree that is mis-typed into a cycle would otherwise
// bounce a packet between two boards forever, on two sysops' systems, with
// nothing to notice it. Any real league is far shallower than this.
const MaxPacketHops = 25

// Routed reports whether this league is arranged as a tree — whether the roster
// carries HOST lines.
//
// It is the switch for every behavior on this page, because the alternative is
// not "no routing" but a different transport. An unrouted league is a mesh whose
// transport copies each outbound file to every board, so a packet addressed
// elsewhere is a copy the addressee already got directly: forwarding it would
// send it back into the same fan-out, and expanding a broadcast into one file
// per board would have the transport deliver each of them everywhere.
func (w *World) Routed() bool {
	for _, n := range w.LeagueNodes {
		if len(n.Hosts) > 0 {
			return true
		}
	}
	return false
}

// NodeNumber is the roster number of the named planet, or 0 if the roster does
// not list it.
func (w *World) NodeNumber(name string) int {
	for _, n := range w.LeagueNodes {
		if n.Name == name {
			return n.Number
		}
	}
	return 0
}

// Routable reports whether a packet addressed to dest can actually be
// delivered from here.
//
// LeaguePlanets lists boards this one has merely HEARD of — learned from a
// packet and never on the roster — giving each an invented number so it can be
// shown and picked. NodeNumber does not know those numbers, so NextHop cannot
// place such a board and hands the packet on unchanged; the next board does the
// same, and it circles the league until the hop cap destroys it. That cost a
// live league a destroyed packet a day, because the travel probe addressed one
// to a board that had once introduced itself under a default name.
//
// A board with no roster entry of its own routes nothing and relies on the
// transport to copy its packets, so everything stays routable there.
func (w *World) Routable(dest string) bool {
	if w.NodeNumber(w.Config.BoardID) == 0 {
		return true
	}
	return dest == "" || dest == w.Config.BoardID || w.NodeNumber(dest) != 0
}

// NodeName is the planet name for a roster number, or "" if unlisted.
func (w *World) NodeName(number int) string {
	for _, n := range w.LeagueNodes {
		if n.Number == number {
			return n.Name
		}
	}
	return ""
}

// AddressedToMe reports whether p is meant for this board: a broadcast, or
// naming this board specifically. The roster's node number is checked first
// when the packet carries one and this board's own number is known (#105) —
// it cannot collide the way two boards sharing a name could, and survives
// either end renaming — falling back to the board name for a packet, or a
// roster, that predates node identity.
func (w *World) AddressedToMe(p Packet) bool {
	if p.ToBoard == "" && p.ToNode == 0 {
		return true
	}
	if p.ToNode != 0 {
		if mine := w.NodeNumber(w.Config.BoardID); mine != 0 {
			return p.ToNode == mine
		}
	}
	return p.ToBoard == w.Config.BoardID
}

// hostOf maps each node number to the node that forwards for it.
func (w *World) hostOf() map[int]int {
	host := map[int]int{}
	for _, n := range w.LeagueNodes {
		for _, child := range n.Hosts {
			if _, dup := host[child]; !dup && child != n.Number {
				host[child] = n.Number
			}
		}
	}
	return host
}

// NextHop is the planet a packet for dest should be handed to. It is dest
// itself when the two boards link directly, which is both the answer for a
// league with no routing at all and the answer for a neighbor in the tree.
// An unroutable destination also comes back as dest: sending it straight at a
// board that may not answer beats holding it here where nobody will look.
func (w *World) NextHop(dest string) string {
	me, to := w.NodeNumber(w.Config.BoardID), w.NodeNumber(dest)
	if me == 0 || to == 0 || me == to {
		return dest
	}
	next := w.routedHop(me, to)
	if name := w.NodeName(next); name != "" {
		return name
	}
	return dest
}

// routedHop answers NextHop in roster numbers: the HOST tree, then a direct
// link.
func (w *World) routedHop(me, to int) int {
	host := w.hostOf()

	// Walk from the destination towards the root. If this board is on that
	// path, the packet goes down to whichever of its children leads there.
	seen := map[int]bool{to: true}
	for at := to; ; {
		up, ok := host[at]
		if !ok || seen[up] {
			break
		}
		if up == me {
			return at
		}
		seen[up] = true
		at = up
	}
	// Otherwise it goes up: the uplink knows the rest of the league.
	if up, ok := host[me]; ok {
		return up
	}
	return to
}

// ForwardPacket queues a packet that arrived here but is addressed elsewhere.
// It is passed on as it came: the sequence number and the Coordinator's
// signature belong to the board that wrote it, so a hub that re-stamped a
// packet in transit would be vouching for someone else's orders.
//
// raw is the packet as it arrived, and it is what goes out again, with only its
// Hops changed. Re-encoding p instead dropped every field this build does not
// know, which a newer board's packet carries, and the signature the board that
// wrote it made over them then failed at the destination: the hub was never
// told, and the destination could only refuse it. nil re-encodes p, for a
// packet that has no arriving bytes.
//
// A packet that has been forwarded too many times is destroyed instead, and the
// news says so. Saying so is the point: a cycle in the roster is one sysop's
// typo that every board in the league obeys, and a hop count that quietly ate
// the traffic would leave nobody anything to go on. BRE reported the same thing
// as "Illegal Route Found from BBS #".
func (w *World) ForwardPacket(p Packet, raw []byte) {
	// An unroutable destination is hopeless on the first hop, not the
	// twenty-fifth: no board on the way can place it either. Saying which board
	// is missing from the roster beats reporting a circle, which describes what
	// the packet did rather than why.
	if !w.Routable(p.ToBoard) {
		w.noteSysop("A packet from %s for %s was destroyed: no board of that name is on the league roster.",
			p.FromBoard, p.ToBoard)
		return
	}
	if p.Hops >= MaxPacketHops {
		w.noteSysop("A packet from %s bound for %s has been passed between boards %d times and was destroyed. The league's routing sends it in a circle.",
			p.FromBoard, p.ToBoard, p.Hops)
		return
	}
	p.Hops++
	out, err := relayBytes(p, raw)
	if err != nil {
		w.noteSysop("A packet from %s for %s was destroyed: it could not be re-encoded to pass on (%v).",
			p.FromBoard, p.ToBoard, err)
		return
	}
	w.Transit = append(w.Transit, out)
}

// relayBytes is raw with its Hops set to p.Hops, every other field kept
// exactly, known to this build or not. The keys come back sorted, which is
// harmless: a receiver verifies what it decodes, never the bytes as sent.
func relayBytes(p Packet, raw []byte) (json.RawMessage, error) {
	if raw == nil {
		return json.Marshal(p)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	hops, err := json.Marshal(p.Hops)
	if err != nil {
		return nil, err
	}
	fields["Hops"] = hops
	return json.Marshal(fields)
}

// addressBroadcasts turns each broadcast into one packet per planet on the
// roster. A broadcast is one file that the transport is expected to copy to
// every board — which only works where every board links to every other one.
// Once a league routes, only the game knows the shape of it, so the roster is
// where the copies have to be made; each one then follows the ordinary path to
// its board. An unrouted league sends the single broadcast as before.
//
// Each copy is a separate packet from here on: StampOutbox gives it its own
// sequence number and its own origin signature, covering the destination it
// actually names.
func (w *World) addressBroadcasts(packets []Packet) []Packet {
	boards := w.KnownBoards()
	if !w.Routed() || len(boards) == 0 {
		return packets
	}
	out := make([]Packet, 0, len(packets))
	for _, p := range packets {
		if p.ToBoard != "" {
			out = append(out, p)
			continue
		}
		for _, b := range boards {
			// Skip a board the roster cannot place: addressing it would put one
			// undeliverable copy of every broadcast on the wire.
			if !w.Routable(b) {
				continue
			}
			copied := p
			copied.ToBoard = b
			copied.ToNode = w.NodeNumber(b)
			out = append(out, copied)
		}
	}
	return out
}

// LinkSilentDays is how many whole days it has been since a packet from board
// was processed here, or -1 when this board has never heard from it at all.
//
// Silence is a DIFFERENT question from routability (#187): Routable asks whether
// this board can address a packet to that one, which the roster answers, while
// this asks whether anything is actually coming back. A planet can be perfectly
// routable and utterly unreachable — its sysop's mailer down, its outbound never
// collected — and until now nothing anywhere said so.
func (w *World) LinkSilentDays(board string, now time.Time) int {
	at, ok := ParseStamp(w.LastPacketFrom[board])
	if !ok {
		return -1
	}
	if d := int(now.Sub(at) / (24 * time.Hour)); d > 0 {
		return d
	}
	return 0
}

// LinkSilentMax is how long a planet may go quiet before a player addressing it
// is warned. Packets move on the sysop's schedule — a board that exchanges mail
// once a day is normal, and one that polls twice a week is a setup, not a fault
// — so this is deliberately several days rather than hours.
const LinkSilentMax = 3

// LinkSilentAlarmDays is how long a board may go quiet before the SYSOP is told,
// as against LinkSilentMax, which warns a PLAYER addressing it. The two differ
// on purpose. A player's warning is cheap to be wrong about — it says a message
// may sit a while, and a board that polls every few days will trip it harmlessly
// — where a sysop notice that cries wolf at every slow-but-working link is noise
// in the one channel that is supposed to mean something is broken. A full week
// of silence is not a polling schedule.
const LinkSilentAlarmDays = 7

// NoteSilentLinks raises one sysop notice per board that has stopped answering,
// so the operator meets a dead link in the planetary run's own output rather
// than by opening a report they have no reason to suspect. The existing fault
// plumbing does the rest: the run reports it, newNotices counts it once when it
// first appears rather than once per run, and it keeps appearing in the log for
// as long as it is true.
//
// The transport fault counter cannot see this on its own — a fault there is a
// packet that ARRIVED and could not be read, so a board sending nothing at all
// produces none, and silence reads as health. That is how a board sat three
// days without traffic while its Travel Times figure still showed 40 minutes.
//
// A board never heard from is not reported, matching LinkSilent: that is every
// league before its first exchange, and a notice on every fresh setup would
// teach the sysop to ignore this one.
func (w *World) NoteSilentLinks(now time.Time) {
	for _, board := range w.knownPeers() {
		if days := w.LinkSilentDays(board, now); days > LinkSilentAlarmDays {
			w.noteSysop("No packet has been processed from %s in %d days. Its mailer, or ours, may not be moving files.", board, days)
		}
	}
}

// NoteUnansweredProbes tells the sysop about a board whose packets still arrive
// while no probe sent to it has come back for RoundTripAlarmDays. Its mail
// reaching us shows it runs its planetary step, and every build echoes a probe,
// so the break is in what THIS board sends: its mailer or the route out. That
// is the fault NoteSilentLinks cannot see, because the board is not silent.
// A board whose outbound netmail named files its mailer could not find sent
// nothing for days while receiving normally, and nothing said so.
//
// A board with no completed round trip on record is not reported, as in
// NoteSilentLinks: that is every link before its first exchange.
func (w *World) NoteUnansweredProbes(now time.Time) {
	for _, board := range w.knownPeers() {
		if !w.Routable(board) {
			continue
		}
		heard := w.LinkSilentDays(board, now)
		if heard < 0 || heard > RoundTripAlarmDays {
			continue // silent, which NoteSilentLinks reports
		}
		seen, err := time.Parse(time.RFC3339, w.TravelSeen[board])
		if err != nil {
			continue
		}
		if days := int(now.Sub(seen) / (24 * time.Hour)); days > RoundTripAlarmDays {
			w.noteSysop("Packets from %s arrive, but no probe sent to it has come back in %d days. What this board sends may not be reaching it: check this board's mailer and outbound first.", board, days)
		}
	}
}

// LinkQuiet reports that a planet this board HAS heard from has since gone
// quiet for longer than LinkSilentMax. A planet never heard from is not quiet,
// it is new: every league starts that way, and warning about it would make the
// warning meaningless on the day a board joins.
func (w *World) LinkQuiet(board string, now time.Time) bool {
	d := w.LinkSilentDays(board, now)
	return d > LinkSilentMax
}
