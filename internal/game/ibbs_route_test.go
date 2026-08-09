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

func TestRouteFileOverridesTheHostTree(t *testing.T) {
	w := routingWorld(3) // a leaf whose uplink is 2
	w.Routes = []RouteRule{{Dest: 5, Via: 8}}
	if got := w.NextHop(planetName(5)); got != planetName(8) {
		t.Errorf("NextHop = %q, want %q", got, planetName(8))
	}
	// Anything the file does not name still follows the tree.
	if got := w.NextHop(planetName(7)); got != planetName(2) {
		t.Errorf("unrouted NextHop = %q, want %q", got, planetName(2))
	}
}

// BRE's own example: "Using ROUTE 5 5 after a Route * # command will restore
// BBS #5 to Direct".
func TestRouteStarThenSpecificRestoresDirect(t *testing.T) {
	w := routingWorld(3)
	w.Routes = []RouteRule{{Dest: 0, Via: 8}, {Dest: 5, Via: 5}}
	if got := w.NextHop(planetName(7)); got != planetName(8) {
		t.Errorf("* rule: NextHop = %q, want %q", got, planetName(8))
	}
	if got := w.NextHop(planetName(5)); got != planetName(5) {
		t.Errorf("restored direct: NextHop = %q, want %q", got, planetName(5))
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
func TestADestroyedPacketIsReported(t *testing.T) {
	w := &World{}
	w.ForwardPacket(Packet{FromBoard: "Alpha BBS", ToBoard: "Charlie BBS", Hops: MaxPacketHops})

	if len(w.Transit) != 0 {
		t.Errorf("queued %d packets, want 0", len(w.Transit))
	}
	if len(w.NewsToday) != 1 {
		t.Fatalf("posted %d news lines, want 1", len(w.NewsToday))
	}
	news := w.NewsToday[0]
	for _, want := range []string{"Alpha BBS", "Charlie BBS"} {
		if !strings.Contains(news, want) {
			t.Errorf("news should name %q, got %q", want, news)
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
