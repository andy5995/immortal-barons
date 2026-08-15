package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// VersionFile records the game version the League Coordinator requires of every
// board. It is a plain one-line text file so it can be read, set and mailed
// without opening the game — the Coordinator maintains it, and every member
// board keeps its own copy so a sysop can see what is being asked of them.
//
// It sits beside the roster and bbs.cfg rather than inside config.json for the
// same reason those do: a sysop chasing "why are my packets bouncing?" should be
// able to answer it with cat, and a broadcast that rewrites config.json should
// leave something legible behind.
const VersionFile = "ibversion.cfg"

func versionFilePath(dataDir string) string { return filepath.Join(dataDir, VersionFile) }

// ReadVersionFile returns the required version recorded on disk, or "" when the
// file is absent or says nothing — which is "no requirement", the state every
// league starts in. Blank lines and #-comments are ignored, and a leading "v"
// is accepted since that is how the reports print it.
func ReadVersionFile(dataDir string) string {
	data, err := os.ReadFile(versionFilePath(dataDir))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return strings.TrimPrefix(line, "v")
	}
	return ""
}

// WriteVersionFile records the requirement on disk. An empty version writes the
// file with no value rather than deleting it, so the file is always there to be
// found and edited — a missing file reads as "no requirement" either way, but a
// sysop cannot edit a file that is not there.
func WriteVersionFile(dataDir, version string) error {
	body := "# The game version every board in this league must run.\n" +
		"# Set by the League Coordinator and sent to every board.\n" +
		"# Blank means no requirement. A board below it has its packets refused.\n"
	if version == "" {
		body += "\n"
	} else {
		body += version + "\n"
	}
	return os.WriteFile(versionFilePath(dataDir), []byte(body), 0o644)
}

// syncVersionFile keeps the file and the config agreeing after the config has
// been written. The config is authoritative at this point: a Coordinator's
// broadcast has already been applied to it, so the file follows.
func syncVersionFile(cfg game.Config) error {
	if !cfg.IBBS {
		return nil
	}
	if ReadVersionFile(cfg.DataDir) == cfg.MinBoardVersion {
		return nil
	}
	if err := WriteVersionFile(cfg.DataDir, cfg.MinBoardVersion); err != nil {
		return fmt.Errorf("recording the required version: %w", err)
	}
	return nil
}
