package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// testseed.go — IB_ADD_AI, the development hook that seeds computer barons.
//
// Barons were retired as a playable feature in v0.1.3 (see
// internal/game/retire_ai.go), so nothing a sysop can reach makes one: there is
// no flag, no config field and no line in the manual. What they are still good
// for is testing — a populated planet is what a screen with rivals on it, a
// multi-day economy run, or the score table have to be checked against, and
// doing that against the real binary rather than a build of its own is the
// whole point of keeping the hook. It sits on the same shelf as IB_GAME_DATE
// and IB_CLOCK_OFFSET.
//
// Unlike those, this one WRITES: the barons go into world.json and stay there,
// holding slots and taking turns. A banner printed after the fact would not
// take them back, so this says what it is about to do, does it, and exits.

// AddAIVar is the environment variable that seeds computer barons.
const AddAIVar = "IB_ADD_AI"

// addAIRequested reports the number of barons IB_ADD_AI asks for, and whether
// it was set at all. An unparseable or out-of-range value exits rather than
// being rounded into something plausible: the var is typed by hand on a rig,
// and quietly seeding a different number than was asked for is worse than
// refusing.
func addAIRequested() (int, bool) {
	v := os.Getenv(AddAIVar)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > game.PlanetSlots {
		fmt.Fprintf(os.Stderr, "immortal-barons: %s must be a whole number from 1 to %d, not %q\n",
			AddAIVar, game.PlanetSlots, v)
		os.Exit(2)
	}
	return n, true
}

// runAddAI seeds up to n computer barons into the running world and exits. It
// reports how many it actually added and why it stopped short: the planet's
// realm slots run out long before the name pool does.
func runAddAI(cfg game.Config, n int) error {
	// Before the banner: a league board is refused, and announcing a seeding
	// that is about to be refused reads as a failure rather than a rule.
	if cfg.IBBS {
		return fmt.Errorf("a league board has no computer barons")
	}
	fmt.Fprintf(os.Stderr,
		"immortal-barons: %s is set — seeding %d computer baron(s) into %s. They are a "+
			"development hook, not a game feature: they hold realm slots and take a turn "+
			"every day until the game is reset.\n",
		AddAIVar, n, cfg.DataDir)
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()
	w, err := store.Load(cfg)
	if err != nil {
		return err
	}
	added := w.AddAIEmpires(n)
	if err := store.Save(w, cfg); err != nil {
		return err
	}
	fmt.Printf("Added %d AI barons.\n", added)
	switch {
	case added < n && w.PlanetFull():
		fmt.Printf("(Requested %d, but the planet's %d realms are all held.)\n", n, game.PlanetSlots)
	case added < n:
		fmt.Printf("(Requested %d, but the AI name pool is exhausted.)\n", n)
	}
	return nil
}
