package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
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
