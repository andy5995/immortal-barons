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
	if err := RunPlanetary(b.w, b.inbound, b.outbound); err != nil {
		t.Fatal(err)
	}
}

// deliver moves every packet waiting in one board's outbound directory into the
// other's inbound — the sysop's transport, done by hand.
func deliver(t *testing.T, from, to *board) int {
	t.Helper()
	entries, err := os.ReadDir(from.outbound)
	if err != nil {
		t.Fatal(err)
	}
	moved := 0
	for _, e := range entries {
		src := filepath.Join(from.outbound, e.Name())
		if err := os.Rename(src, filepath.Join(to.inbound, e.Name())); err != nil {
			t.Fatal(err)
		}
		moved++
	}
	return moved
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
