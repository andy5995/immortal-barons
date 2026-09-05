package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// dropCtx is a lone realm with land to give up, on the in-memory store.
func dropCtx(t *testing.T) (*ctx, *game.Empire) {
	t.Helper()
	cfg := game.DefaultConfig()
	cfg.AICount = 0
	w := game.NewWorldSeed(cfg, 1)
	p := w.AddHuman("alice", "Alicia")
	w.GrantRegions(p, game.Desert.Count(&p.Regions), 40)
	return &ctx{World: w, handle: "alice"}, p
}

// The Regions item on the SELL menu drops rather than sells, and the drop is
// irreversible, so it asks first and the default is no (#257). Answering
// anything but 'y' must leave the realm's land alone.
func TestDropRegionsDefaultsToNo(t *testing.T) {
	c, p := dropCtx(t)
	before := p.Land
	// Enter alone: the default answer, which must be "no".
	f := &fakeSession{keys: []rune("\r")}
	sellLand(f, c)

	out := f.out.String()
	if !strings.Contains(out, "Regions may not be sold") {
		t.Fatalf("the notice must still be shown first, got: %q", out)
	}
	if !strings.Contains(out, "Drop regions?") {
		t.Fatalf("expected the confirmation, got: %q", out)
	}
	if strings.Contains(out, "Drop Regions *-") {
		t.Errorf("a declined confirmation must not reach the picker, got: %q", out)
	}
	if p.Land != before {
		t.Errorf("land moved on a declined drop: %d -> %d", before, p.Land)
	}
}

// Saying yes reaches the picker, which names itself, and the drop goes through.
func TestDropRegionsAfterConfirmation(t *testing.T) {
	c, p := dropCtx(t)
	before, desertBefore := p.Land, p.Regions.Desert
	// The Desert row's own key on the picker, then how many to give up.
	f := &fakeSession{keys: []rune("y" + string(rune(game.Desert.Key)) + "10\r")}
	sellLand(f, c)

	out := f.out.String()
	if !strings.Contains(out, "Drop Regions *-") {
		t.Fatalf("the picker must name itself as the drop screen, got: %q", out)
	}
	if p.Regions.Desert != desertBefore-10 {
		t.Errorf("Desert = %d after dropping 10, want %d", p.Regions.Desert, desertBefore-10)
	}
	if p.Land != before-10 {
		t.Errorf("land = %d, want %d", p.Land, before-10)
	}
}
