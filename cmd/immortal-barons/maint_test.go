package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// plantBadPacket puts an unparseable packet in the inbound directory, old enough
// not to be mid-transfer, so the next planetary step quarantines it and records
// a fault.
func plantBadPacket(t *testing.T, cfg game.Config) {
	t.Helper()
	bad := filepath.Join(cfg.Inbound(), "L900-0001.brp")
	if err := os.WriteFile(bad, []byte("not a packet"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(bad, old, old); err != nil {
		t.Fatal(err)
	}
}

// leagueBoard is a saved league game in a temporary data directory, with its
// inbound directory made.
func leagueBoard(t *testing.T) game.Config {
	t.Helper()
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.IBBS = true
	cfg.BoardID = "Test Board"
	cfg.LeagueNumber = 900
	if err := os.MkdirAll(cfg.Inbound(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(store.NewGame(cfg), cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// captureStdout runs fn with stdout redirected and returns what it printed,
// for the modes whose printout is the only place their answer lands.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = saved
	w.Close()
	out, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// A league board's timer runs -maint (#289), which runs the planetary step. The
// step marks what it met as reported, so -maint has to raise the alarm itself or
// a fault first met here is never raised at all.
func TestMaintRaisesTheAlarmOnANewFault(t *testing.T) {
	cfg := leagueBoard(t)
	hookOut := filepath.Join(cfg.DataDir, "hook.out")
	cfg.OnFault = `printf '%s' "$IB_FAULTS" > "` + hookOut + `"`

	plantBadPacket(t, cfg)

	if err := runMaint(cfg, "2026-09-22"); !errors.Is(err, errFaults) {
		t.Fatalf("-maint that met a new fault returned %v, want errFaults so the scheduler sees it", err)
	}
	// The hook command is a sh one-liner; the Windows branch runs cmd /c instead.
	if runtime.GOOS != "windows" {
		got, err := os.ReadFile(hookOut)
		if err != nil {
			t.Fatalf("the OnFault hook did not run: %v", err)
		}
		if len(strings.TrimSpace(string(got))) == 0 {
			t.Error("the OnFault hook ran without the faults in $IB_FAULTS")
		}
	}

	// Nothing new on the next run, so no alarm.
	if err := runMaint(cfg, "2026-09-22"); err != nil {
		t.Errorf("a -maint run with nothing new returned %v", err)
	}
}

// Every mode that runs the planetary step refuses a league board with no league
// number, as -planetary does: such a board would take every league's packets as
// its own.
func TestModesThatRunThePlanetaryStepRefuseANoLeagueNumberBoard(t *testing.T) {
	cfg := leagueBoard(t)
	cfg.LeagueNumber = 0
	for mode, run := range map[string]func() error{
		"-maint":        func() error { return runMaint(cfg, "2026-09-22") },
		"-full":         func() error { return runFull(cfg, "tester", "2026-09-22", 0, true, false) },
		"-league-reset": func() error { return runLeagueReset(cfg, "2026-10-01") },
	} {
		if err := run(); err == nil || !strings.Contains(err.Error(), "no league number") {
			t.Errorf("%s on a league board with no league number returned %v", mode, err)
		}
	}
}

// -league-reset runs the planetary step too, so a fault it meets is raised there
// or never: the step records it as reported.
func TestLeagueResetRaisesTheAlarmOnANewFault(t *testing.T) {
	cfg := leagueBoard(t)
	// Every field filled: the roster parser skips an entry with an empty one.
	roster := []game.LeagueNode{
		{Number: 1, Name: cfg.BoardID, Address: "1:1/1", City: "Here", State: "ST", Country: "USA"},
		{Number: 2, Name: "Other Board", Address: "1:1/2", City: "There", State: "ST", Country: "USA"},
	}
	if err := store.WriteNodeList(filepath.Join(cfg.DataDir, store.NodeListFile), roster); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GenerateCoordKey(cfg.DataDir); err != nil {
		t.Fatal(err)
	}
	plantBadPacket(t, cfg)
	if err := runLeagueReset(cfg, "2026-10-01"); !errors.Is(err, errFaults) {
		t.Fatalf("-league-reset that met a new fault returned %v, want errFaults", err)
	}
}

// The alarm names the command that met the fault, so the scheduler's mail points
// at the job that actually ran.
func TestTheAlarmNamesTheMode(t *testing.T) {
	r, wpipe, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := os.Stderr
	os.Stderr = wpipe
	got := reportFaults(game.DefaultConfig(), store.PlanetaryRun{NewFaults: []string{"a fault"}}, "-maint")
	os.Stderr = stderr
	wpipe.Close()
	out, _ := io.ReadAll(r)
	if !errors.Is(got, errFaults) {
		t.Errorf("reportFaults returned %v, want errFaults", got)
	}
	if !strings.HasPrefix(string(out), "immortal-barons -maint: ") {
		t.Errorf("the alarm reads %q, want it to name -maint", out)
	}
}
