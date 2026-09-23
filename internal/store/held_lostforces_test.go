package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A strike aimed at a board whose packets this run holds for a protocol
// difference is not given up, and the run report says the recovery is paused
// rather than leaving the sysop to guess (#190). Once the board's traffic
// applies again, the leftover held file no longer pauses anything.
func TestPlanetaryRunPausesLostForcesForAHeldBoard(t *testing.T) {
	inbound, outbound := t.TempDir(), t.TempDir()
	cfg := game.DefaultConfig()
	cfg.IBBS = true
	cfg.BoardID = "Receiver BBS"
	cfg.DataDir = t.TempDir()
	cfg.LostForcesDays = 3
	w := game.NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")
	w.GameDay = 10
	w.InFlight = []game.InFlightStrike{{
		ID: 1, Kind: "terror", TargetBoard: "Far BBS", TargetEmpire: "Rome",
		Owner: e.Owner, Agents: 5, LaunchedDay: 7, // the wait is already up
	}}
	agents := e.Agents

	writePacket(t, inbound, "far-1", game.Packet{FromBoard: "Far BBS", Seq: 1, Protocol: game.Protocol + 1})
	run, err := RunPlanetary(w, inbound, outbound, false)
	if err != nil {
		t.Fatalf("RunPlanetary: %v", err)
	}
	if run.Held != 1 {
		t.Fatalf("held %d packets, want 1", run.Held)
	}
	if len(w.InFlight) != 1 || e.Agents != agents {
		t.Fatalf("the op was given up while its board's packets are held (in flight %d, agents %d)", len(w.InFlight), e.Agents)
	}
	if !reflect.DeepEqual(run.RecoveryPaused, []string{"Far BBS"}) {
		t.Errorf("RecoveryPaused = %v, want [Far BBS]", run.RecoveryPaused)
	}
	log, err := os.ReadFile(filepath.Join(cfg.DataDir, PlanetaryLogFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "Lost-forces recovery is paused for what was sent to Far BBS") ||
		!strings.Contains(string(log), "15 days") {
		t.Errorf("the planetary log does not report the pause:\n%s", log)
	}
	for _, n := range run.NewFaults {
		if strings.Contains(n, "Lost-forces recovery") {
			t.Errorf("the pause was counted as a fault: %q", n)
		}
	}

	// Next run, nothing new arrives: the packet is still held, so still paused.
	if run, err = RunPlanetary(w, inbound, outbound, false); err != nil {
		t.Fatal(err)
	}
	if len(w.InFlight) != 1 || len(run.RecoveryPaused) != 1 {
		t.Fatalf("the pause lapsed while the packet is still held (in flight %d, paused %v)", len(w.InFlight), run.RecoveryPaused)
	}

	// The board's traffic applies again. Its stale held file stays until it ages
	// out, but the link is moving, so the wait resumes: no game day has passed
	// while held, so the original three are up and the agents come home.
	w.ProtocolHeldAt["Far BBS"] = game.Recorded(time.Now().Add(-time.Hour))
	writePacket(t, inbound, "far-2", game.Packet{FromBoard: "Far BBS", Seq: 2})
	if run, err = RunPlanetary(w, inbound, outbound, false); err != nil {
		t.Fatal(err)
	}
	if run.Applied != 1 {
		t.Fatalf("applied %d, want the one current packet", run.Applied)
	}
	if len(w.InFlight) != 0 || e.Agents != agents+5 || run.RecoveryPaused != nil {
		t.Errorf("after the link recovered: in flight %d, agents %d (want %d), paused %v",
			len(w.InFlight), e.Agents, agents+5, run.RecoveryPaused)
	}
}
