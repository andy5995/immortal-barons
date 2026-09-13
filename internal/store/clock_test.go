package store

import (
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A world written under a shifted test clock must not load on a smaller shift.
// Its instants are in the future from there: a weapon would never land, a
// Travel Times probe would never return, and nothing in the game would say why.
func TestAWorldWrittenAheadRefusesToLoadBehind(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	t.Cleanup(func() { game.SetClockOffset(0) })
	game.SetClockOffset(72 * time.Hour)
	w := game.NewWorld(cfg)
	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(cfg); err != nil {
		t.Fatalf("loading under the SAME shift must work: %v", err)
	} else if got.ClockOffset != "72h0m0s" {
		t.Errorf("the shift was not recorded, got %q", got.ClockOffset)
	}

	// Back to a real clock: the save is ahead of us and must be refused.
	game.SetClockOffset(0)
	if _, err := Load(cfg); err == nil {
		t.Fatal("a world written 72h ahead loaded on an unshifted clock")
	} else if !strings.Contains(err.Error(), ClockRewindVar) {
		t.Errorf("the refusal does not name the override variable, so nobody can act on it: %v", err)
	}

	// ...unless the operator says so outright.
	t.Setenv(ClockRewindVar, "1")
	got, err := Load(cfg)
	if err != nil {
		t.Fatalf("%s did not let it load: %v", ClockRewindVar, err)
	}
	if got == nil {
		t.Fatal("no world came back")
	}
}

// A larger shift is fine: the game's clock only ever moves forward on a rig.
func TestAWorldWrittenBehindLoadsAhead(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	t.Cleanup(func() { game.SetClockOffset(0) })
	game.SetClockOffset(24 * time.Hour)
	w := game.NewWorld(cfg)
	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	game.SetClockOffset(96 * time.Hour)
	if _, err := Load(cfg); err != nil {
		t.Fatalf("winding the rig further forward was refused: %v", err)
	}
}
