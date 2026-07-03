// Package store persists the game world to disk. Save writes the world
// atomically (temp file + rename); Load reads it back, or returns a fresh
// world when no save exists yet.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/andy5995/immortal-barons/internal/game"
)

func worldPath(cfg game.Config) string { return filepath.Join(cfg.DataDir, "world.json") }

// Load reads the saved world, or returns a fresh one if none exists.
func Load(cfg game.Config) (*game.World, error) {
	data, err := os.ReadFile(worldPath(cfg))
	if os.IsNotExist(err) {
		return game.NewWorld(cfg), nil
	}
	if err != nil {
		return nil, err
	}
	w := game.NewWorld(cfg) // seeds rng; JSON overwrites exported fields
	if err := json.Unmarshal(data, w); err != nil {
		return nil, err
	}
	w.Config = cfg
	return w, nil
}

// Save writes the world atomically (temp file + rename).
func Save(w *game.World, cfg game.Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	tmp := worldPath(cfg) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, worldPath(cfg))
}
