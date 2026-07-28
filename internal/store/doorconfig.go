package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// DoorConfig holds machine-local settings for the door front-end — facts about
// THIS installation's BBS, not the game. It lives in door.json, kept separate
// from config.json (the game ruleset, reset to defaults by -reset) and
// world.json (game data), so changing how the door reads callers never touches
// game state. BRE keeps the same split: its drop file setup (SETUP.SR) is a
// per-install file, not part of the game data.
type DoorConfig struct {
	DropfileFormat string // which drop file this BBS writes (a door.Format ID); "" = not configured, run -set-dropfile
}

func doorConfigPath(dataDir string) string { return filepath.Join(dataDir, "door.json") }

// LoadDoorConfig reads <dataDir>/door.json, returning a zero DoorConfig when the
// file is absent (a fresh install that hasn't run -set-dropfile yet).
func LoadDoorConfig(dataDir string) (DoorConfig, error) {
	var dc DoorConfig
	data, err := os.ReadFile(doorConfigPath(dataDir))
	if os.IsNotExist(err) {
		return dc, nil
	}
	if err != nil {
		return dc, err
	}
	if err := json.Unmarshal(data, &dc); err != nil {
		return dc, err
	}
	return dc, nil
}

// SaveDoorConfig writes dc to <dataDir>/door.json atomically.
func SaveDoorConfig(dataDir string, dc DoorConfig) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(dc, "", "  ")
	if err != nil {
		return err
	}
	tmp := doorConfigPath(dataDir) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, doorConfigPath(dataDir))
}
