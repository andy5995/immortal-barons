package store

import (
	"crypto/ed25519"
	"crypto/rand"
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

// TestPlanetaryRunRebroadcastsTheRuleset covers the healing case in #264: the
// Coordinator's ruleset rides every planetary run, so a member that missed the
// one-shot -league-config broadcast still ends up playing the league's rules.
func TestPlanetaryRunRebroadcastsTheRuleset(t *testing.T) {
	dir := t.TempDir()
	roster := []game.LeagueNode{
		{Number: 1, Name: "Nova Hub"},
		{Number: 2, Name: "The Eclipse"},
	}
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	pub, sec, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	a := newBoard(t, filepath.Join(dir, "a"), "Nova Hub", roster)
	b := newBoard(t, filepath.Join(dir, "b"), "The Eclipse", roster)
	a.w.Config.DataDir, b.w.Config.DataDir = filepath.Join(dir, "a"), filepath.Join(dir, "b")
	a.w.CoordKey, a.w.CoordPub = sec, pub
	b.w.CoordPub = pub
	a.w.Config.TurnsPerDay = 12
	b.w.Config.TurnsPerDay = 10

	a.run(t)
	if n := deliver(t, a, b); n == 0 {
		t.Fatal("the Coordinator wrote no packets")
	}
	b.run(t)
	if got := b.w.Config.TurnsPerDay; got != 12 {
		t.Errorf("member plays %d turns a day, want the Coordinator's 12", got)
	}

	// A member never dictates back: its own run must not queue a ruleset.
	b.w.Config.TurnsPerDay = 10
	b.run(t)
	if n := deliver(t, b, a); n > 0 {
		a.run(t)
	}
	if got := a.w.Config.TurnsPerDay; got != 12 {
		t.Errorf("Coordinator adopted a member's ruleset: %d turns a day", got)
	}
}

// TestPacketsFromADivergentBoardAreHeld is the enforcement half of #264. A
// board playing by rules the Coordinator never sent would otherwise feed its
// scores, strikes and trades into everyone else's game.
func TestPacketsFromADivergentBoardAreHeld(t *testing.T) {
	dir := t.TempDir()
	roster := []game.LeagueNode{
		{Number: 1, Name: "Nova Hub"},
		{Number: 2, Name: "The Eclipse"},
	}
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	pub, sec, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	a := newBoard(t, filepath.Join(dir, "a"), "Nova Hub", roster)
	b := newBoard(t, filepath.Join(dir, "b"), "The Eclipse", roster)
	a.w.Config.DataDir, b.w.Config.DataDir = filepath.Join(dir, "a"), filepath.Join(dir, "b")
	a.w.CoordKey, a.w.CoordPub = sec, pub
	b.w.CoordPub = pub

	// The Eclipse plays 12 turns a day; the league plays 10.
	b.w.Config.TurnsPerDay = 12
	b.w.SendIPMessage(b.w.Empires[0], []string{"Nova Hub"}, false, "We claim the outer belt.")
	b.run(t)
	if n := deliver(t, b, a); n == 0 {
		t.Fatal("The Eclipse wrote no packets")
	}
	run, err := RunPlanetary(a.w, a.inbound, a.outbound, false)
	if err != nil {
		t.Fatal(err)
	}
	if run.HeldRules == 0 {
		t.Error("a packet from a board playing its own rules was not held")
	}
	if run.Held != 0 {
		t.Errorf("a ruleset hold was counted as a protocol hold (%d), which names the wrong fault", run.Held)
	}
	if got := len(a.w.Empires[0].Mail); got != 0 {
		t.Errorf("the Coordinator applied %d messages from a divergent board", got)
	}

	// The board comes into line — but the packet it already sent states the
	// rules it was written under, so releasing it must not apply it.
	b.w.Config.TurnsPerDay = 10
	if _, err := RunPlanetary(a.w, a.inbound, a.outbound, false); err != nil {
		t.Fatal(err)
	}
	if got := len(a.w.Empires[0].Mail); got != 0 {
		t.Errorf("a packet written under the wrong rules was applied after release: %d messages", got)
	}

	// What it sends AFTERWARDS is the league's game, and goes through.
	b.w.SendIPMessage(b.w.Empires[0], []string{"Nova Hub"}, false, "Terms accepted.")
	b.run(t)
	deliver(t, b, a)
	if _, err := RunPlanetary(a.w, a.inbound, a.outbound, false); err != nil {
		t.Fatal(err)
	}
	if got := len(a.w.Empires[0].Mail); got != 1 {
		t.Errorf("the Coordinator holds %d messages from a board back in step, want 1", got)
	}

	// And the board is named on BBSINFO throughout, which is the report #264 was
	// filed against: a board whose packets are only ever held has no APPLIED
	// packet to record a fingerprint from, and read "unknown" before the hold
	// recorded one too.
	if got := a.w.BoardRuleset["The Eclipse"]; got == "" {
		t.Error("a divergent board's rules were never recorded, so BBSINFO cannot mark it")
	}
}

// A rules change must not destroy the traffic already in flight when it lands:
// those packets state the rules they were written under, and that fingerprint
// never becomes current again (#264, RulesetGraceDays).
func TestARulesChangeDoesNotDestroyPacketsInFlight(t *testing.T) {
	dir := t.TempDir()
	roster := []game.LeagueNode{
		{Number: 1, Name: "Nova Hub"},
		{Number: 2, Name: "The Eclipse"},
	}
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	pub, sec, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	a := newBoard(t, filepath.Join(dir, "a"), "Nova Hub", roster)
	b := newBoard(t, filepath.Join(dir, "b"), "The Eclipse", roster)
	a.w.Config.DataDir, b.w.Config.DataDir = filepath.Join(dir, "a"), filepath.Join(dir, "b")
	a.w.CoordKey, a.w.CoordPub = sec, pub
	b.w.CoordPub = pub

	// Both boards in step, and the Coordinator has recorded the league's rules.
	a.run(t)
	deliver(t, a, b)
	b.run(t)

	// The Eclipse writes under the rules it has, and the Coordinator changes them
	// while that packet is on the wire.
	b.w.SendIPMessage(b.w.Empires[0], []string{"Nova Hub"}, false, "We claim the outer belt.")
	b.run(t)
	deliver(t, b, a)
	a.w.Config.TurnsPerDay = 12

	run, err := RunPlanetary(a.w, a.inbound, a.outbound, false)
	if err != nil {
		t.Fatal(err)
	}
	if run.HeldRules != 0 {
		t.Errorf("a rules change held %d packets that were already in flight", run.HeldRules)
	}
	if got := len(a.w.Empires[0].Mail); got != 1 {
		t.Errorf("the Coordinator received %d messages across its own rules change, want 1", got)
	}
}

// #187: the alarm fires on a fault the sysop has not been told about, and stays
// quiet while the same fault persists. A run that fails its scheduler's unit
// every fifteen minutes for a week is a unit nobody looks at.
func TestOnlyANewFaultRaisesTheAlarm(t *testing.T) {
	dir := t.TempDir()
	roster := []game.LeagueNode{
		{Number: 1, Name: "Nova Hub"},
		{Number: 2, Name: "The Eclipse"},
	}
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	pub, sec, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	a := newBoard(t, filepath.Join(dir, "a"), "Nova Hub", roster)
	b := newBoard(t, filepath.Join(dir, "b"), "The Eclipse", roster)
	a.w.Config.DataDir, b.w.Config.DataDir = filepath.Join(dir, "a"), filepath.Join(dir, "b")
	a.w.CoordKey, a.w.CoordPub = sec, pub
	b.w.CoordPub = pub
	b.w.Config.TurnsPerDay = 12

	// First offence: the fault is new, so the run reports it as one.
	b.run(t)
	deliver(t, b, a)
	run, err := RunPlanetary(a.w, a.inbound, a.outbound, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.NewFaults) == 0 {
		t.Fatal("the first run to meet a fault raised no alarm")
	}

	// The same board, still out of step: reported in the run's notices as before,
	// but no longer new.
	b.run(t)
	deliver(t, b, a)
	run, err = RunPlanetary(a.w, a.inbound, a.outbound, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Notices) == 0 {
		t.Error("a fault that persists stopped being reported")
	}
	if len(run.NewFaults) != 0 {
		t.Errorf("an unchanged fault raised the alarm again: %v", run.NewFaults)
	}
}
