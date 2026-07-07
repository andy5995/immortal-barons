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
		w := game.NewWorld(cfg)
		loadLeagueNodes(w, cfg)
		return w, nil
	}
	if err != nil {
		return nil, err
	}
	w := game.NewWorld(cfg) // seeds rng; JSON overwrites exported fields
	if err := json.Unmarshal(data, w); err != nil {
		return nil, err
	}
	for _, e := range w.Empires {
		e.EnsureRegions()
		e.EnsureSupport()
		e.EnsureMorale()
		e.EnsureProduction()
	}
	w.EnsureInvestRate()
	w.EnsurePirates()
	w.EnsureTreaties()
	w.EnsureNews()
	w.Config = cfg
	loadLeagueNodes(w, cfg)
	return w, nil
}

// NodeListFile is the league roster filename. The clone isn't binary-compatible
// with BRE (its own packet format), so it uses its own name rather than BRE's
// BRNODES.DAT — the format is still the BRNODES layout ParseNodeList reads.
const NodeListFile = "ibnodes.dat"

// loadLeagueNodes loads the league roster from ibnodes.dat in the data dir. A
// missing file just means no roster — fine for a single-BBS game.
func loadLeagueNodes(w *game.World, cfg game.Config) {
	if nodes, err := ParseNodeList(filepath.Join(cfg.DataDir, NodeListFile)); err == nil {
		w.LeagueNodes = nodes
	}
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

// BackupWorld copies the current world file to world.json.bak, so a destructive
// operation (like -reset) is recoverable. It reports whether a backup was made;
// a missing world file (a fresh install) is not an error and returns false.
func BackupWorld(cfg game.Config) (bool, error) {
	data, err := os.ReadFile(worldPath(cfg))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(worldPath(cfg)+".bak", data, 0o644); err != nil {
		return false, err
	}
	return true, nil
}
