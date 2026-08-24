package game

// Packet routing. Where the Coordinator has arranged the league as a tree, the
// HOST lines in the roster (BRE's BRNODES.DAT) say which neighbour each board
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
// It is the switch for every behaviour on this page, because the alternative is
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
// league with no routing at all and the answer for a neighbour in the tree.
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
// It is passed on byte for byte: the sequence number and the Coordinator's
// signature belong to the board that wrote it, so a hub that re-stamped a
// packet in transit would be vouching for someone else's orders.
//
// A packet that has been forwarded too many times is destroyed instead, and the
// news says so. Saying so is the point: a cycle in the roster is one sysop's
// typo that every board in the league obeys, and a hop count that quietly ate
// the traffic would leave nobody anything to go on. BRE reported the same thing
// as "Illegal Route Found from BBS #".
func (w *World) ForwardPacket(p Packet) {
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
	w.Transit = append(w.Transit, p)
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
