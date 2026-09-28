package sysop

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/andy5995/immortal-barons/internal/store"
)

// BoardConfigPath is the board's bbs.cfg in data directory dir.
func BoardConfigPath(dir string) string { return filepath.Join(dir, store.BoardConfigFile) }

// EnsureBoardConfig makes sure dir has a bbs.cfg to edit. A missing one is
// written from the settings the game is running with now, the same text the
// game prints for a sysop to paste, so an editor opens on the real values
// rather than an empty file. An existing file is never touched: it is the
// board's identity, and a sysop edits it by hand (#152).
func EnsureBoardConfig(dir string) (created bool, err error) {
	path := BoardConfigPath(dir)
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	cfg, err := store.LoadConfig(dir)
	if err != nil {
		return false, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return false, nil // written between the check and here; leave it be
	}
	if err != nil {
		return false, err
	}
	if _, err := f.WriteString(store.BoardConfigText(cfg)); err != nil {
		f.Close()
		return true, err
	}
	return true, f.Close()
}
