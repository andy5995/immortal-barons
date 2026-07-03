package store

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/andy5995/immortal-barons/internal/game"
)

func configPath(dataDir string) string { return filepath.Join(dataDir, "config.json") }

// LoadConfig reads <dataDir>/config.json, falling back to defaults for the
// file (or any field) that is absent. DataDir is always set to dataDir.
func LoadConfig(dataDir string) (game.Config, error) {
	cfg := game.DefaultConfig()
	cfg.DataDir = dataDir
	data, err := os.ReadFile(configPath(dataDir))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.DataDir = dataDir
	return cfg, nil
}

// SaveConfig writes cfg to <dataDir>/config.json atomically.
func SaveConfig(cfg game.Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := configPath(cfg.DataDir) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, configPath(cfg.DataDir))
}
