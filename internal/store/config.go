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
	// The per-board settings moved from config.json to bbs.cfg. A board set up
	// before that still has them in config.json and nowhere else, so they are
	// read back here; bbs.cfg then overwrites whatever it names. A sysop who
	// never opens either file notices nothing.
	var moved struct {
		BoardID      *string
		InboundDir   *string
		OutboundDir  *string
		LeagueNumber *int
	}
	if json.Unmarshal(data, &moved) == nil {
		setIf(&cfg.BoardID, moved.BoardID)
		setIf(&cfg.InboundDir, moved.InboundDir)
		setIf(&cfg.OutboundDir, moved.OutboundDir)
		setIf(&cfg.LeagueNumber, moved.LeagueNumber)
	}
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
	if err := LoadBoardConfig(dataDir, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func setIf[T any](dst *T, src *T) {
	if src != nil {
		*dst = *src
	}
}

// SaveConfig writes cfg to <dataDir>/config.json atomically, and the per-board
// settings alongside it in bbs.cfg. Both are written together because a caller
// holds one Config and has no way to know which half it changed — and because
// writing bbs.cfg is what completes the migration for a board set up before the
// two files were split.
func SaveConfig(cfg game.Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	if err := SaveBoardConfig(cfg); err != nil {
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
