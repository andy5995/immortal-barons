package store

import (
	"errors"
	"fmt"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/mod"
)

// ErrModsInLeague refuses mods on a league board. Every board in a league must
// play the same rules, and nothing yet carries a mod to the other members: a
// mod enabled on one board would be that board quietly playing a different
// game (#202).
var ErrModsInLeague = errors.New("mods cannot be used on a league board yet; remove Mods from config.json")

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
	if len(w.Mods) > 0 && cfg.IBBS {
		return fmt.Errorf("%w: %w", ErrWorldMods, ErrModsInLeague)
	}
	r, warnings, err := mod.Restore(cfg.DataDir, w.Mods)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWorldMods, err)
	}
	for _, msg := range warnings {
		mod.Warn(msg)
	}
	w.Rules = r
	return nil
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
	r, refs, err := mod.Resolve(cfg.DataDir, cfg.Mods)
	if err != nil {
		return nil, nil, fmt.Errorf("the reset was not made: %w", err)
	}
	return r, refs, nil
}
