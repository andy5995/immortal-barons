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
		// No config.json yet. bbs.cfg still has to be read: it is hand-written and
		// may well be in place first, and returning here left a board with its
		// name, league number and packet paths all silently at their defaults.
		err := LoadBoardConfig(dataDir, &cfg)
		return cfg, err
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.DataDir = dataDir
	// The required version has its own file for the same reason bbs.cfg does: it
	// is a thing a sysop edits and greps. A Coordinator's broadcast rewrites
	// config.json, and SaveConfig then rewrites this file to match, so the two
	// agree; where only the file has been edited by hand, the file wins.
	if v := ReadVersionFile(dataDir); v != "" {
		cfg.MinBoardVersion = v
	}
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
	legacyBoard := false
	if json.Unmarshal(data, &moved) == nil {
		setIf(&cfg.BoardID, moved.BoardID)
		setIf(&cfg.InboundDir, moved.InboundDir)
		setIf(&cfg.OutboundDir, moved.OutboundDir)
		setIf(&cfg.LeagueNumber, moved.LeagueNumber)
		legacyBoard = moved.BoardID != nil || moved.InboundDir != nil ||
			moved.OutboundDir != nil || moved.LeagueNumber != nil
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
	// (on) in place and quietly re-enable a weapon they had turned off. The key
	// written today is still ClingyAnnihilator (see Config.GooieKablooie).
	var legacy struct{ DoomerKaboomer *bool }
	if json.Unmarshal(data, &legacy) == nil && legacy.DoomerKaboomer != nil &&
		!bytes.Contains(data, []byte(`"ClingyAnnihilator"`)) {
		cfg.GooieKablooie = *legacy.DoomerKaboomer
	}
	if err := LoadBoardConfig(dataDir, &cfg); err != nil {
		return cfg, err
	}
	// Give those legacy settings a bbs.cfg of their own before the next save
	// drops them: they no longer marshal into config.json. This is the only
	// place the game writes that file, and only when it does not exist.
	if legacyBoard {
		migrateBoardConfig(dataDir, cfg)
	}
	return cfg, nil
}

func setIf[T any](dst *T, src *T) {
	if src != nil {
		*dst = *src
	}
}

// SaveConfig writes cfg to <dataDir>/config.json atomically. It leaves bbs.cfg
// alone: that file is the board's own identity, and a caller holding one Config
// cannot tell which half of it changed, so a rules reset used to return a
// working board's name and directories to defaults (#152).
func SaveConfig(cfg game.Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	// The required version is kept in its own readable file too, so a Coordinator's
	// broadcast leaves something a sysop can find without opening the game.
	if err := syncVersionFile(cfg); err != nil {
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
