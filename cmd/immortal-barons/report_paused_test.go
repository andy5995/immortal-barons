package main

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// The console report says when lost-forces recovery is paused for a held board,
// so a sysop reading it sees paused rather than broken (#190).
func TestReportPlanetarySaysRecoveryIsPaused(t *testing.T) {
	cfg := game.DefaultConfig()
	out := captureStdout(t, func() {
		reportPlanetary(cfg, store.PlanetaryRun{RecoveryPaused: []string{"Far BBS", "Near BBS"}})
	})
	got := strings.Join(strings.Fields(out), " ")
	if !strings.Contains(got, "Lost-forces recovery is paused for what was sent to Far BBS, Near BBS: those boards' packets are held") ||
		!strings.Contains(got, "15 days") {
		t.Errorf("the report does not say recovery is paused:\n%s", out)
	}
}
