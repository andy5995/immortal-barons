package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// board is one side of a two-planet league, with real packet directories on
// disk.
type board struct {
	w        *game.World
	inbound  string
	outbound string
}

func newBoard(t *testing.T, dir, name string, roster []game.LeagueNode) *board {
	t.Helper()
	cfg := game.DefaultConfig()
	cfg.BoardID = name
	cfg.InboundDir = filepath.Join(dir, "inbound")
	cfg.OutboundDir = filepath.Join(dir, "outbound")
	for _, d := range []string{cfg.InboundDir, cfg.OutboundDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w := game.NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-08-06"
	w.LeagueNodes = roster
	w.AddHuman("baron-"+name, "Realm of "+name)
	return &board{w, cfg.InboundDir, cfg.OutboundDir}
}

func (b *board) run(t *testing.T) {
	t.Helper()
	if _, err := RunPlanetary(b.w, b.inbound, b.outbound, false); err != nil {
		t.Fatal(err)
	}
}

// deliver hands every packet waiting in one board's outbound directory to a
// single peer — the two-board case of broadcast, which is what a league with
// one link looks like.
func deliver(t *testing.T, from, to *board) int {
	t.Helper()
	return broadcast(t, from, to)
}

// TestTwoBoardsExchangeMessagesAndTravelTimes runs the whole inter-BBS loop over
// real directories: a message written on one planet is read on the other, and
// the probes that ride along measure a round trip that shows on Travel Times.
// Nothing here is stubbed — it is the same RunPlanetary the sysop's cron runs.
func TestTwoBoardsExchangeMessagesAndTravelTimes(t *testing.T) {
	dir := t.TempDir()
	roster := []game.LeagueNode{
		{Number: 1, Name: "Nova Hub", City: "Brisbane"},
		{Number: 2, Name: "The Eclipse", City: "Sydney"},
	}
	// SaveConfig writes config.json relative to the data dir, so keep the whole
	// run inside the temp dir.
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	a := newBoard(t, filepath.Join(dir, "a"), "Nova Hub", roster)
	b := newBoard(t, filepath.Join(dir, "b"), "The Eclipse", roster)

	// A baron on Nova Hub writes to the whole of The Eclipse.
	a.w.SendIPMessage(a.w.Empires[0], []string{"The Eclipse"}, false, "We claim the outer belt.")

	a.run(t)
	if n := deliver(t, a, b); n == 0 {
		t.Fatal("Nova Hub wrote no packets")
	}
	b.run(t)

	got := b.w.Empires[0].Mail
	if len(got) != 1 {
		t.Fatalf("The Eclipse received %d messages, want 1", len(got))
	}
	if got[0].Body != "We claim the outer belt." || got[0].FromBoard != "Nova Hub" {
		t.Errorf("delivered message is %+v", got[0])
	}

	// The Eclipse's reply run carries the echoed probe home.
	if n := deliver(t, b, a); n == 0 {
		t.Fatal("The Eclipse wrote no packets back")
	}
	a.run(t)
	if _, ok := a.w.TravelTimes["The Eclipse"]; !ok {
		t.Errorf("no round trip was measured; TravelTimes is %v", a.w.TravelTimes)
	}
	if _, ok := b.w.TravelTimes["Nova Hub"]; ok {
		t.Errorf("The Eclipse recorded a trip it never sent: %v", b.w.TravelTimes)
	}
}

// broadcast hands every packet waiting in one board's outbound directory to each
// of the boards named, then clears the outbound — the sysop's transport, done by
// hand. A mesh league needs the same packet to reach several peers, which is why
// this takes a list; deliver is the one-peer case.
func broadcast(t *testing.T, from *board, to ...*board) int {
	t.Helper()
	entries, err := os.ReadDir(from.outbound)
	if err != nil {
		t.Fatal(err)
	}
	moved := 0
	for _, e := range entries {
		src := filepath.Join(from.outbound, e.Name())
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		for _, dst := range to {
			if err := os.WriteFile(filepath.Join(dst.inbound, e.Name()), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Remove(src); err != nil {
			t.Fatal(err)
		}
		moved++
	}
	return moved
}

// TestThreeBoardsLearnEachOthersScores runs a three-planet league through the
// real transport until every board holds both other boards' score tables.
//
// It replaces a test that drove the removed -export/-import flags, which had
// their own packet format: that pair round-tripped with each other and with
// nothing else, so the test went green while never touching the packets a
// league actually exchanges. The assertion it was making is worth keeping, so
// it is made here against RunPlanetary instead.
func TestThreeBoardsLearnEachOthersScores(t *testing.T) {
	dir := t.TempDir()
	roster := []game.LeagueNode{
		{Number: 1, Name: "Nova Hub", City: "Brisbane"},
		{Number: 2, Name: "The Eclipse", City: "Sydney"},
		{Number: 3, Name: "Red Shift", City: "Perth"},
	}
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}

	boards := map[string]*board{}
	var all []*board
	for i, n := range roster {
		b := newBoard(t, filepath.Join(dir, string(rune('a'+i))), n.Name, roster)
		// newBoard seeds one baron; ExportScores carries only realms with an
		// owner, so give each board three humans rather than AI empires.
		b.w.AddHuman("second-"+n.Name, "Second of "+n.Name)
		b.w.AddHuman("third-"+n.Name, "Third of "+n.Name)
		boards[n.Name] = b
		all = append(all, b)
	}

	// Every board exports, the transport carries each broadcast to both peers,
	// and every board reads what arrived.
	for _, b := range all {
		b.run(t)
	}
	for _, src := range all {
		var peers []*board
		for _, dst := range all {
			if dst != src {
				peers = append(peers, dst)
			}
		}
		if n := broadcast(t, src, peers...); n == 0 {
			t.Fatalf("%s wrote no packets", src.w.Config.BoardID)
		}
	}
	for _, b := range all {
		b.run(t)
	}

	for _, n := range roster {
		me := boards[n.Name]
		if len(me.w.RemoteBoards) != len(roster)-1 {
			t.Errorf("%s knows %d other boards, want %d", n.Name, len(me.w.RemoteBoards), len(roster)-1)
		}
		for _, other := range roster {
			if other.Name == n.Name {
				continue
			}
			rb := findRemoteBoard(me.w.RemoteBoards, other.Name)
			if rb == nil {
				t.Errorf("%s is missing remote board %q", n.Name, other.Name)
				continue
			}
			if len(rb.Scores) != 3 {
				t.Errorf("%s holds %d scores for %q, want 3", n.Name, len(rb.Scores), other.Name)
				continue
			}
			// Content, not just the count: a blank name or a zeroed net worth
			// would travel undetected on a count alone.
			for _, sc := range rb.Scores {
				if sc.Empire == "" || sc.NetWorth == 0 {
					t.Errorf("%s holds a degraded score for %q: %+v", n.Name, other.Name, sc)
				}
			}
		}
	}
}

func findRemoteBoard(boards []game.RemoteBoard, id string) *game.RemoteBoard {
	for i := range boards {
		if boards[i].BoardID == id {
			return &boards[i]
		}
	}
	return nil
}
