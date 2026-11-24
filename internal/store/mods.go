package store

import (
	"errors"
	"fmt"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/mod"
)

// ErrModsInLeague refuses a league board choosing mods for itself. Every board
// in a league must play the same rules, so the League Coordinator chooses the
// mods and its -league-reset carries them to every board (#202).
var ErrModsInLeague = errors.New("on a league board the League Coordinator chooses the mods and -league-reset carries them to every board; remove Mods from this board's config.json")

func init() { game.LoadLeagueMods = mod.LoadForLeague }

// ErrWorldMods is wrapped around any failure to restore the mods a saved world
// was reset with. A reset discards them, so it treats this like an unreadable
// world and starts fresh, rather than leaving the board stuck behind a mod
// error that only a reset can clear.
var ErrWorldMods = errors.New("this game's mods could not be loaded")

// loadMods rebuilds the rules w was reset with into w.Rules. A mod changed or
// removed since the reset is reported through mod.Warn (stderr, which reaches
// the sysop's log under a BBS rather than the caller's screen), and the game
// plays on with the copy it saved.
func loadMods(w *game.World, cfg game.Config) error {
	r, warnings, err := mod.Restore(cfg.DataDir, w.Mods, !cfg.IBBS)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWorldMods, err)
	}
	for _, msg := range warnings {
		mod.Warn(msg)
	}
	// A league board's packets state the hash of the copy it plays, not the one
	// recorded with it, so a copy edited in world.json is held by the rest of
	// the league like any other difference in the rules.
	if cfg.IBBS {
		for i := range w.Mods {
			w.Mods[i].Hash = mod.HashFiles(w.Mods[i].Files)
		}
	}
	w.Rules = r
	return nil
}

// ReloadMods replaces the mods w plays by with the ones cfg.Mods names, read
// from their folders as they stand now, without starting a new game. It is for
// testing a mod in a scratch game: everything else in the world, the market
// prices set at the reset included, stays as it was. It returns what changed,
// as -check-mod lists it. A league board refuses it, as it refuses mods.
func ReloadMods(w *game.World, cfg game.Config) ([]string, error) {
	if cfg.IBBS {
		return nil, ErrModsInLeague
	}
	r, refs, err := mod.Resolve(cfg.DataDir, cfg.Mods, cfg.ModOptions)
	if err != nil {
		return nil, fmt.Errorf("the mods were not reloaded: %w", err)
	}
	from, to := w.Rules, r
	if from == nil {
		from = game.BaseRules()
	}
	if to == nil {
		to = game.BaseRules()
	}
	changes := mod.Diff(from, to)
	w.Rules, w.Mods = r, refs
	return changes, nil
}

// CheckResetMods reports whether a reset could apply the mods cfg.Mods names.
// A reset runs it before it backs up the old world, so a mod error does not
// cost the sysop the previous backup.
func CheckResetMods(cfg game.Config) error {
	_, _, err := resolveResetMods(cfg)
	return err
}

// ApplyResetMods installs the mods cfg.Mods names on w, ahead of a sysop reset:
// the rules have to be in place before the reset prices the fresh market.
func ApplyResetMods(w *game.World, cfg game.Config) error {
	r, refs, err := resolveResetMods(cfg)
	if err != nil {
		return err
	}
	w.Rules, w.Mods = r, refs
	return nil
}

func resolveResetMods(cfg game.Config) (*game.Rules, []game.ModRef, error) {
	if len(cfg.Mods) > 0 && cfg.IBBS {
		return nil, nil, ErrModsInLeague
	}
	r, refs, err := mod.Resolve(cfg.DataDir, cfg.Mods, cfg.ModOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("the reset was not made: %w", err)
	}
	return r, refs, nil
}
