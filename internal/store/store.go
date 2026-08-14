// Package store persists the game world to disk. Save writes the world
// atomically (temp file + rename); Load reads it back, or returns a fresh
// world when no save exists yet.
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/andy5995/immortal-barons/internal/game"
)

// ErrNoWorld is returned by Load when no saved world exists yet. A game must be
// created with -reset (or -reset-from-config) before it can be played or
// maintained; a missing world is a setup error, not a fresh empty game.
var ErrNoWorld = errors.New("no game found — run with -reset to create one")

// NewGame builds a fresh, unsaved world from the config. It is the only path
// that conjures a world from nothing (used by -reset); every other entry point
// loads an existing one and errors with ErrNoWorld if it is missing.
func NewGame(cfg game.Config) *game.World {
	w := game.NewWorld(cfg)
	loadLeagueNodes(w, cfg)
	loadRoutes(w, cfg)
	loadLeagueKeys(w, cfg)
	return w
}

func worldPath(cfg game.Config) string { return filepath.Join(cfg.DataDir, "world.json") }

// Load reads the saved world, or returns a fresh one if none exists.
func Load(cfg game.Config) (*game.World, error) {
	data, err := os.ReadFile(worldPath(cfg))
	if os.IsNotExist(err) {
		return nil, ErrNoWorld
	}
	if err != nil {
		return nil, err
	}
	w := game.NewWorld(cfg) // seeds rng; JSON overwrites exported fields
	if err := json.Unmarshal(data, w); err != nil {
		return nil, err
	}
	repair(w, cfg)
	return w, nil
}

// repair re-runs the migration/normalization Load applies after unmarshalling:
// per-empire Ensure* backfills, world-level backfills, re-pointing Config, and
// loading the league roster. Factored out so FileStore can reload the JSON into
// an EXISTING *World (keeping the caller's pointer valid) without duplicating
// this list.
func repair(w *game.World, cfg game.Config) {
	for _, e := range w.Empires {
		// SDI first: it reads the saved percentage to rebuild a pre-pool save's
		// funding, and EnsureRegions recomputes the percentage from the pool.
		e.EnsureSDIFunding()
		e.EnsureRegions()
		e.EnsureSupport()
		e.EnsureMorale()
		e.EnsureProduction()
	}
	w.EnsureInvestRate()
	w.EnsurePirates()
	w.EnsureTreaties()
	w.EnsureNews()
	w.EnsureEpoch()
	w.Config = cfg
	loadLeagueNodes(w, cfg)
	loadRoutes(w, cfg)
	loadLeagueKeys(w, cfg)
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

// loadRoutes loads this board's routing overrides. Most boards have no such
// file: a league whose Coordinator keeps HOST routing in the roster needs none.
func loadRoutes(w *game.World, cfg game.Config) {
	if rules, err := ParseRouteFile(filepath.Join(cfg.DataDir, RouteFile)); err == nil {
		w.Routes = rules
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
