package game

import (
	"strings"
	"testing"
)

// The routing tree from BRE's own documentation (docs/bre.doc, "Host Routing
// System"), which draws it as:
//
//	        BBS #1
//	      /        \
//	   BBS #2     BBS #4
//	 / |  |  \    |  |  \
//	3  5  7  8    6  9  11
//	                       |
//	                      10
func breRoutingExample() []LeagueNode {
	nodes := []LeagueNode{
		{Number: 1, Hosts: []int{2, 4}},
		{Number: 2, Hosts: []int{3, 5, 7, 8}},
		{Number: 4, Hosts: []int{6, 9, 11}},
		{Number: 11, Hosts: []int{10}},
	}
	have := map[int]bool{}
	for _, n := range nodes {
		have[n.Number] = true
	}
	for i := 1; i <= 11; i++ {
		if !have[i] {
			nodes = append(nodes, LeagueNode{Number: i})
		}
	}
	for i := range nodes {
		nodes[i].Name = planetName(nodes[i].Number)
	}
	return nodes
}

func planetName(n int) string {
	return string(rune('A'+n-1)) + " BBS"
}

func routingWorld(me int) *World {
	w := &World{}
	w.LeagueNodes = breRoutingExample()
	w.Config.BoardID = planetName(me)
	return w
}

func TestNextHopFollowsTheHostTree(t *testing.T) {
	cases := []struct {
		from, to, want int
	}{
		// A leaf sends everything to its uplink, whoever the target is.
		{3, 5, 2}, {3, 1, 2}, {3, 10, 2},
		// The Coordinator picks the branch the target sits under.
		{1, 3, 2}, {1, 10, 4}, {1, 2, 2},
		// A hub delivers into its own subtree and passes the rest upward.
		{2, 7, 7}, {2, 10, 1},
		{4, 10, 11}, {4, 3, 1},
		// Two hops down: 10 hangs off 11, which hangs off 4.
		{1, 11, 4}, {11, 3, 4},
	}
	for _, c := range cases {
		w := routingWorld(c.from)
		if got := w.NextHop(planetName(c.to)); got != planetName(c.want) {
			t.Errorf("from %d to %d: NextHop = %q, want %q", c.from, c.to, got, planetName(c.want))
		}
	}
}

// With no HOST lines anywhere, every board still reaches every other board —
// the league is a full mesh, which is what a small one already is.
func TestNextHopWithNoRoutingIsDirect(t *testing.T) {
	w := &World{}
	w.Config.BoardID = "Alpha BBS"
	w.LeagueNodes = []LeagueNode{{Number: 1, Name: "Alpha BBS"}, {Number: 2, Name: "Bravo BBS"}}
	if got := w.NextHop("Bravo BBS"); got != "Bravo BBS" {
		t.Errorf("NextHop = %q, want Bravo BBS", got)
	}
}

// A board with no roster at all — the common single-board case — must not have
// its packets sent somewhere else.
func TestNextHopWithNoRosterIsDirect(t *testing.T) {
	w := &World{}
	w.Config.BoardID = "Alpha BBS"
	if got := w.NextHop("Somewhere Else"); got != "Somewhere Else" {
		t.Errorf("NextHop = %q, want Somewhere Else", got)
	}
}

// A roster typed into a cycle must not hang the game working out a next hop.
func TestNextHopSurvivesACircularRoster(t *testing.T) {
	w := &World{}
	w.Config.BoardID = "A"
	w.LeagueNodes = []LeagueNode{
		{Number: 1, Name: "A"},
		{Number: 2, Name: "B", Hosts: []int{3}},
		{Number: 3, Name: "C", Hosts: []int{2}},
	}
	if got := w.NextHop("C"); got == "" {
		t.Error("NextHop returned nothing for a circular roster")
	}
}

// A packet must not be forwarded round a loop forever. Nothing catches this at
// the routing layer — a cycle in the roster is a typo one sysop makes and every
// board obeys — so the hop count is what stops it.
func TestForwardPacketStopsAtTheHopLimit(t *testing.T) {
	w := &World{}
	for i := range MaxPacketHops + 5 {
		w.ForwardPacket(Packet{FromBoard: "Alpha BBS", ToBoard: "Bravo BBS", Hops: i})
	}
	if len(w.Transit) != MaxPacketHops {
		t.Errorf("queued %d packets, want %d", len(w.Transit), MaxPacketHops)
	}
}

// Destroying the packet is only half the job. A hop count that ate the traffic
// in silence would leave a mis-routed league with no symptom but disappearing
// mail, which is the failure the count exists to make survivable.
//
// It is reported to the SYSOP, not to the planet: a player can do nothing about
// a routing cycle, and the news cap is 20 lines, so a fault that repeats every
// exchange would delete the day's real events instead.
func TestADestroyedPacketIsReported(t *testing.T) {
	w := &World{}
	w.ForwardPacket(Packet{FromBoard: "Alpha BBS", ToBoard: "Charlie BBS", Hops: MaxPacketHops})

	if len(w.Transit) != 0 {
		t.Errorf("queued %d packets, want 0", len(w.Transit))
	}
	if len(w.NewsToday) != 0 {
		t.Errorf("a transport fault reached the planet's news: %q", w.NewsToday)
	}
	if len(w.SysopNotices) != 1 {
		t.Fatalf("recorded %d sysop notices, want 1", len(w.SysopNotices))
	}
	note := w.SysopNotices[0]
	for _, want := range []string{"Alpha BBS", "Charlie BBS"} {
		if !strings.Contains(note, want) {
			t.Errorf("the notice should name %q, got %q", want, note)
		}
	}
}

// A forwarded packet keeps the sequence number and signature of the board that
// wrote it: a hub that re-stamped one would be vouching for another board's
// orders.
func TestForwardPacketDoesNotRestampTheOriginal(t *testing.T) {
	w := &World{}
	w.Config.BoardID = "Hub BBS"
	w.Config.LeagueNumber = 7
	sig := []byte{1, 2, 3}
	w.ForwardPacket(Packet{FromBoard: "Alpha BBS", ToBoard: "Bravo BBS", Seq: 42, Signature: sig, League: 7})
	w.StampOutbox() // stamps the Outbox only

	got := w.Transit[0]
	if got.Seq != 42 {
		t.Errorf("Seq = %d, want 42", got.Seq)
	}
	if string(got.Signature) != string(sig) {
		t.Errorf("Signature = %v, want %v", got.Signature, sig)
	}
	if got.Hops != 1 {
		t.Errorf("Hops = %d, want 1", got.Hops)
	}
}

// The live fault this guard exists for: a board that once introduced itself
// under the default BoardID ("local") is recorded in RemoteBoards, and
// LeaguePlanets hands it an invented node number so it can be listed and
// picked. NodeNumber never learns that number, so anything addressed to it is
// unroutable by construction -- and the travel probe addressed one every day,
// which circled the league and was destroyed, once a day, on every board.
func TestTheTravelProbeSkipsABoardTheRosterCannotPlace(t *testing.T) {
	w := &World{}
	w.Config.BoardID = "The X-Bit BBS"
	w.LeagueNodes = []LeagueNode{
		{Number: 1, Name: "The X-Bit BBS", Hosts: []int{2}},
		{Number: 2, Name: "Nite Eyes BBS"},
	}
	w.RemoteBoards = []RemoteBoard{{BoardID: "local"}}
	w.LastMaintDate = "2026-08-23"

	// It is still a planet as far as the screens are concerned -- the guard must
	// stop it being ADDRESSED, not make it vanish.
	var listed bool
	for _, p := range w.LeaguePlanets() {
		if p.Name == "local" {
			listed = true
		}
	}
	if !listed {
		t.Fatal("LeaguePlanets should still list a board heard from off-roster")
	}

	w.PingTravelTimes()
	for _, p := range w.Outbox {
		if p.ToBoard == "local" {
			t.Error("the travel probe addressed a board the roster cannot place")
		}
	}
	if len(w.Outbox) == 0 {
		t.Error("no probe was sent at all; the guard is too broad")
	}
}

// An undeliverable packet dies on arrival rather than after 25 hops, and the
// notice says which name is missing instead of describing a circle.
func TestAnUnroutablePacketIsDroppedAtOnce(t *testing.T) {
	w := &World{}
	w.Config.BoardID = "The X-Bit BBS"
	w.LeagueNodes = []LeagueNode{{Number: 1, Name: "The X-Bit BBS", Hosts: []int{2}}, {Number: 2, Name: "Nite Eyes BBS"}}

	w.ForwardPacket(Packet{FromBoard: "Nite Eyes BBS", ToBoard: "local"})

	if len(w.Transit) != 0 {
		t.Errorf("queued %d packets, want 0", len(w.Transit))
	}
	if len(w.SysopNotices) != 1 {
		t.Fatalf("recorded %d notices, want 1", len(w.SysopNotices))
	}
	if !strings.Contains(w.SysopNotices[0], "roster") {
		t.Errorf("the notice should name the roster as the problem, got %q", w.SysopNotices[0])
	}
}

// A board with no roster entry of its own routes nothing and leans on the
// transport, so nothing there is unroutable -- the guard must not strand it.
func TestAnUnrosteredBoardStillAddressesEveryone(t *testing.T) {
	w := &World{}
	w.Config.BoardID = "Lonely BBS"
	w.RemoteBoards = []RemoteBoard{{BoardID: "Nite Eyes BBS"}}
	if !w.Routable("Nite Eyes BBS") {
		t.Error("a board that is not on the roster itself must still address its peers")
	}
}

// The backstop under every call site that addresses a board by name: a
// destination the roster cannot place never reaches the Outbox at all, and the
// fault is reported to the sysop rather than to the planet, which can do
// nothing about a routing problem.
func TestTheOutboxRefusesAnUnroutableDestination(t *testing.T) {
	w := &World{}
	w.Config.BoardID = "The X-Bit BBS"
	w.LeagueNodes = []LeagueNode{
		{Number: 1, Name: "The X-Bit BBS", Hosts: []int{2}},
		{Number: 2, Name: "Nite Eyes BBS"},
	}

	w.enqueue("local", RemoteAttack{})
	w.enqueue("local", RemoteAttack{})
	w.enqueue("Nite Eyes BBS", RemoteAttack{})

	if len(w.Outbox) != 1 || w.Outbox[0].ToBoard != "Nite Eyes BBS" {
		t.Fatalf("Outbox = %+v, want one packet, for Nite Eyes BBS", w.Outbox)
	}
	if len(w.Outbox[0].Attacks) != 1 {
		t.Errorf("the routable board lost its payload: %+v", w.Outbox[0])
	}
	if len(w.NewsToday) != 0 {
		t.Errorf("a transport fault reached the planet's news: %q", w.NewsToday)
	}
	if len(w.SysopNotices) != 1 {
		t.Fatalf("recorded %d sysop notices, want 1 — one per board, not one per payload", len(w.SysopNotices))
	}
	for _, want := range []string{"local", "roster"} {
		if !strings.Contains(w.SysopNotices[0], want) {
			t.Errorf("the notice should mention %q, got %q", want, w.SysopNotices[0])
		}
	}
}

// A board with no roster entry of its own routes nothing, so the backstop must
// not stop it queueing anything (the mirror of Routable's own exemption).
func TestTheOutboxBackstopSparesAnUnrosteredBoard(t *testing.T) {
	w := &World{}
	w.Config.BoardID = "Lonely BBS"
	w.RemoteBoards = []RemoteBoard{{BoardID: "Nite Eyes BBS"}}

	w.enqueue("Nite Eyes BBS", RemoteAttack{})

	if len(w.Outbox) != 1 || len(w.SysopNotices) != 0 {
		t.Errorf("Outbox = %+v, notices = %q; the backstop stranded an unrostered board", w.Outbox, w.SysopNotices)
	}
}
