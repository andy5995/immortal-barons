package store

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/andy5995/immortal-barons/internal/game"
)

func configPath(dataDir string) string { return filepath.Join(dataDir, "config.json") }

// LoadConfig reads <dataDir>/config.json, falling back to defaults for the
// file (or any field) that is absent. DataDir is always set to dataDir.
func LoadConfig(dataDir string) (game.Config, error) {
	def := game.DefaultConfig()
	cfg := def
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
	// The packet directories used to be read relative to the working directory
	// and defaulted to "./data/inbound". They now hang off the data directory, so
	// a config still holding the old default would resolve to <data>/data/inbound.
	if cfg.InboundDir == "./data/inbound" {
		cfg.InboundDir = def.InboundDir
	}
	if cfg.OutboundDir == "./data/outbound" {
		cfg.OutboundDir = def.OutboundDir
	}
	// The doomsday weapon's switch used to be called DoomerKaboomer. A renamed
	// key is not a missing one to a sysop: unmarshalling would leave the default
	// (on) in place and quietly re-enable a weapon they had turned off.
	var legacy struct{ DoomerKaboomer *bool }
	if json.Unmarshal(data, &legacy) == nil && legacy.DoomerKaboomer != nil &&
		!bytes.Contains(data, []byte(`"ClingyAnnihilator"`)) {
		cfg.ClingyAnnihilator = *legacy.DoomerKaboomer
	}
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
