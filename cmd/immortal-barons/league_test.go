package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// #228: the run that met a failure is long gone by the time a sysop asks why a
// board has gone quiet, so -league-check reads the spool journals rather than
// relying on anyone having kept that output.
func TestSpoolChecksReportTheBacklogAndItsReason(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()

	// A board with no transport spool has nothing to say.
	if checks := spoolChecks(cfg); len(checks) != 0 {
		t.Fatalf("a board with no spool reported %+v", checks)
	}

	batch := filepath.Join(cfg.DataDir, "ftn-spool", "out", "0001")
	if err := os.MkdirAll(batch, 0o755); err != nil {
		t.Fatal(err)
	}
	plan := map[string]any{
		"id": "0001",
		"targets": []map[string]any{{
			"node": 1, "name": "Alpha BBS", "done": false,
			"last_error": "peer BSO queue is busy",
		}},
	}
	body, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(batch, "batch.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	checks := spoolChecks(cfg)
	if len(checks) != 1 {
		t.Fatalf("checks = %+v, want one waiting peer", checks)
	}
	if !checks[0].OK {
		t.Error("a peer that is merely offline was reported as a setup fault")
	}
	for _, want := range []string{"Alpha BBS", "peer BSO queue is busy", "without progress"} {
		if !strings.Contains(checks[0].Detail, want) {
			t.Errorf("the report does not mention %q: %q", want, checks[0].Detail)
		}
	}
}

// A journal nothing can read is the one spool state that IS a fault: it is
// neither retry state nor deliberate quarantine, so nothing else mentions it.
func TestSpoolChecksFailOnAnUnreadableJournal(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	dir := filepath.Join(cfg.DataDir, "ftn-spool", "in", "deadbeef")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "receipt.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	checks := spoolChecks(cfg)
	if len(checks) != 1 || checks[0].OK {
		t.Fatalf("checks = %+v, want one failure", checks)
	}
	if !strings.Contains(checks[0].Detail, "deadbeef") {
		t.Errorf("the failure does not name the directory: %q", checks[0].Detail)
	}
}

// The one-skip case printed its count twice ("skipped 1: 1: ...") because a
// singular branch spelled the prefix that the format string then repeated. It
// survived because every other count reads correctly, and only a run that skips
// exactly one packet shows it.
func TestSkipSummaryCountsOnce(t *testing.T) {
	for _, tc := range []struct {
		run  store.PlanetaryRun
		want string
	}{
		{store.PlanetaryRun{Held: 1}, "skipped 1: 1 held for a protocol this build does not read"},
		{store.PlanetaryRun{Held: 2}, "skipped 2: 2 held for a protocol this build does not read"},
		{store.PlanetaryRun{Held: 1, AlreadySeen: 1}, "skipped 2: 1 held for a protocol this build does not read, 1 already seen"},
	} {
		if got := skipSummary(tc.run); got != tc.want {
			t.Errorf("skipSummary = %q, want %q", got, tc.want)
		}
	}
}

// An unquoted message is refused, not cut to its first word. The flag package
// stops at the first stray word, so everything after it — -data included — was
// ignored, and the Coordinator's board froze the league with a one-word message
// from whatever ./data happened to be.
func TestLeagueFreezeRefusesAnUnquotedMessage(t *testing.T) {
	if os.Getenv("IB_FREEZE_ARGS_CHILD") == "1" {
		os.Args = []string{"immortal-barons", "-league-freeze", "Updating", "please", "-data", os.Getenv("IB_FREEZE_DATADIR")}
		main()
		return
	}
	cwd := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestLeagueFreezeRefusesAnUnquotedMessage$")
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "IB_FREEZE_ARGS_CHILD=1", "IB_FREEZE_DATADIR="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), `unknown argument "please"`) {
		t.Errorf("an unquoted freeze message was not refused (err %v):\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(cwd, "data")); err == nil {
		t.Error("the refused command still created ./data")
	}
}
