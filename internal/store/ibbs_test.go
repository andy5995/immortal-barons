package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	target.Protection = 0 // else the strike is turned away and never fights

	// Board A launches a group attack and writes its outbox to the exchange.
	ga, cErr := wA.CreateGroupAttack(leader, "boardB", "Victim", game.GroupAttackHoursMin, game.AttackForce{Troopers: 100_000})
	if cErr != nil {
		t.Fatalf("create: %v", cErr)
	}
	wA.LaunchDueGroupAttacksAt(time.Now().Add((game.GroupAttackHoursMin + 1) * time.Hour))
	if _, err := WriteOutbox(wA, exchange, false); err != nil {
		t.Fatalf("WriteOutbox A: %v", err)
	}

	// Board B reads the exchange, applies the attack, and queues its result.
	result, err := ReadInbound(wB, exchange, false)
	if err != nil || result.Applied != 1 {
		t.Fatalf("ReadInbound B: applied=%d err=%v", result.Applied, err)
	}
	if target.Land >= 100 {
		t.Errorf("target should have lost land, has %d", target.Land)
	}
	if _, err := WriteOutbox(wB, exchange, false); err != nil {
		t.Fatalf("WriteOutbox B: %v", err)
	}

	// The launch must have debited the committed force from the leader.
	if leader.Troopers != 900_000 {
		t.Errorf("leader should hold 900,000 troopers after committing 100,000, got %d", leader.Troopers)
	}

	// Board A reads the result back; its bulletin records the outcome, and the
	// survivors come home. Content, not just presence: a result packet that
	// lost its outcome fields would still land "a bulletin line".
	if _, err := ReadInbound(wA, exchange, false); err != nil {
		t.Fatalf("ReadInbound A: %v", err)
	}
	if len(wA.NewsToday) == 0 {
		t.Fatalf("board A should have a bulletin entry for the strike outcome")
	}
	if news := wA.NewsToday[0]; !strings.Contains(news, "Victim") || !strings.Contains(news, "boardB") {
		t.Errorf("bulletin should name the target and board, got %q", news)
	}
	// The target had nothing standing, so the force is barely touched and the
	// whole 100,000 comes home to the 900,000 left behind.
	if leader.Troopers != 1_000_000 {
		t.Errorf("leader should hold 1,000,000 troopers after the survivors return, got %d", leader.Troopers)
	}
	_ = ga
}

func TestRunPlanetaryExportsScores(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	cfg := game.DefaultConfig()
	cfg.BoardID = "boardA"
	w := game.NewWorldSeed(cfg, 1)
	w.AddHuman("p", "Player")

	if _, err := RunPlanetary(w, in, out, false); err != nil {
		t.Fatalf("RunPlanetary: %v", err)
	}
	// A broadcast score packet should have been written to the outbound dir —
	// and it must CARRY the scores: an empty or corrupt packet also makes the
	// dir non-empty, so unmarshal and check the content.
	entries, _ := os.ReadDir(out)
	if len(entries) == 0 {
		t.Fatal("RunPlanetary should have written a score packet")
	}
	var found bool
	for _, ent := range entries {
		b, err := os.ReadFile(filepath.Join(out, ent.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var pkt game.Packet
		if err := json.Unmarshal(b, &pkt); err != nil {
			t.Fatalf("outbound %s is not a packet: %v", ent.Name(), err)
		}
		if pkt.FromBoard != "boardA" {
			t.Errorf("packet FromBoard = %q, want boardA", pkt.FromBoard)
		}
		for _, sc := range pkt.Scores {
			if sc.Empire == "Player" {
				found = true
			}
		}
	}
	if !found {
		t.Error(`no outbound packet scored the "Player" empire`)
	}
}
