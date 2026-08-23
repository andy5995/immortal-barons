package game

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

// TestAdoptedRosterHoldsTheNodeNumberRange is the wire half of #180: a roster
// arrives as a struct, not through the file parser, so a number the roster FILE
// refuses could otherwise reach a board over the wire and last until the next
// restart re-read the file and silently dropped it.
func TestAdoptedRosterHoldsTheNodeNumberRange(t *testing.T) {
	pub, sec, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.BoardID = "The Eclipse"
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-08-23"
	w.CoordPub = pub
	w.LeagueNodes = []LeagueNode{{Number: 1, Name: "Nova Hub"}, {Number: 2, Name: "The Eclipse"}}

	lc := NewWorldSeed(func() Config { c := DefaultConfig(); c.BoardID = "Nova Hub"; return c }(), 1)
	lc.LastMaintDate = "2026-08-23"
	lc.CoordKey, lc.CoordPub = sec, pub
	lc.LeagueNodes = []LeagueNode{
		{Number: 1, Name: "Nova Hub"},
		{Number: 2, Name: "The Eclipse"},
		{Number: 1000, Name: "Oversized BBS"},
		{Number: 0, Name: "Ghost BBS"},
	}
	lc.ExportNodeList()
	lc.StampOutbox() // signs it, and fans a broadcast out to one copy per member
	if len(lc.Outbox) == 0 {
		t.Fatal("the Coordinator queued no roster packet")
	}
	for _, p := range lc.Outbox {
		w.ApplyPacket(p)
	}

	if len(w.LeagueNodes) != 2 {
		t.Fatalf("adopted roster is %+v", w.LeagueNodes)
	}
	for _, n := range w.LeagueNodes {
		if n.Number < 1 || n.Number > MaxNodeNumber {
			t.Errorf("node %q was adopted with number %d", n.Name, n.Number)
		}
	}
}
