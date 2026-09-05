package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// BRE prices Terrorist Ops on the InterPlanetary menu itself, right-aligned
// against the cell's edge: `(2) Terrorist Ops       570,304 (A) SDI Program`.
func TestInterPlanetaryMenuPricesTerroristOps(t *testing.T) {
	menus := BuildMenus()
	cfg := game.DefaultConfig()
	cfg.IBBS, cfg.AICount = true, 0
	w := game.NewWorldSeed(cfg, 1)
	p := w.AddHuman("alice", "Alethia")
	p.Land, p.TerrorOpsToday = 8957, 0
	c := &ctx{World: w, handle: "alice"}

	f := &fakeSession{keys: []rune("0")}
	if err := Run(f, c, menus.InterPlanetary); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := stripANSI(f.out.String())
	var line string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "Terrorist Ops") {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatalf("the menu never drew Terrorist Ops:\n%s", out)
	}
	if !strings.Contains(line, "573,248") {
		t.Errorf("the item should quote the one-agent rate (64 x 8,957):\n%q", line)
	}
	t.Logf("drew: %q", line)
}
