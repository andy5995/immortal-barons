package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// The console report says when lost-forces recovery is paused for a held board,
// so a sysop reading it sees paused rather than broken (#190).
func TestReportPlanetarySaysRecoveryIsPaused(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	cfg := game.DefaultConfig()
	reportPlanetary(cfg, store.PlanetaryRun{RecoveryPaused: []string{"Far BBS", "Near BBS"}})
	os.Stdout = saved
	w.Close()
	out, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(strings.Fields(string(out)), " ")
	if !strings.Contains(got, "Lost-forces recovery is paused for what was sent to Far BBS, Near BBS: those boards' packets are held") ||
		!strings.Contains(got, "15 days") {
		t.Errorf("the report does not say recovery is paused:\n%s", out)
	}
}
