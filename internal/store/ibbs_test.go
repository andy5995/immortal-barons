package store

import (
	"os"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestPacketFileRoundTrip(t *testing.T) {
	exchange := t.TempDir() // stands in for the transport that moves files between boards

	cfgA := game.DefaultConfig()
	cfgA.BoardID = "boardA"
	wA := game.NewWorldSeed(cfgA, 1)
	leader := wA.AddHuman("leader", "Alpha")
	leader.Troopers = 1_000_000 // troops to commit

	cfgB := game.DefaultConfig()
	cfgB.BoardID = "boardB"
	wB := game.NewWorldSeed(cfgB, 1)
	target := wB.AddHuman("victim", "Victim")
	target.Regions = game.RegionMix{Coastal: 100}
	target.EnsureRegions()
	target.Troopers, target.Turrets, target.Tanks = 0, 0, 0

	// Board A launches a group attack and writes its outbox to the exchange.
	ga, cErr := wA.CreateGroupAttack(leader, "boardB", "Victim", wA.GameDay+1, 100_000)
	if cErr != nil {
		t.Fatalf("create: %v", cErr)
	}
	wA.GameDay++
	wA.LaunchDueGroupAttacks()
	if err := WriteOutbox(wA, exchange); err != nil {
		t.Fatalf("WriteOutbox A: %v", err)
	}

	// Board B reads the exchange, applies the attack, and queues its result.
	n, err := ReadInbound(wB, exchange)
	if err != nil || n != 1 {
		t.Fatalf("ReadInbound B: n=%d err=%v", n, err)
	}
	if target.Land >= 100 {
		t.Errorf("target should have lost land, has %d", target.Land)
	}
	if err := WriteOutbox(wB, exchange); err != nil {
		t.Fatalf("WriteOutbox B: %v", err)
	}

	// Board A reads the result back; its bulletin records the outcome.
	if _, err := ReadInbound(wA, exchange); err != nil {
		t.Fatalf("ReadInbound A: %v", err)
	}
	if len(wA.NewsToday) == 0 {
		t.Errorf("board A should have a bulletin entry for the strike outcome")
	}
	_ = ga
}

func TestRunPlanetaryExportsScores(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	cfg := game.DefaultConfig()
	cfg.BoardID = "boardA"
	w := game.NewWorldSeed(cfg, 1)
	w.AddHuman("p", "Player")

	if err := RunPlanetary(w, in, out); err != nil {
		t.Fatalf("RunPlanetary: %v", err)
	}
	// A broadcast score packet should have been written to the outbound dir.
	entries, _ := os.ReadDir(out)
	if len(entries) == 0 {
		t.Fatal("RunPlanetary should have written a score packet")
	}
}
