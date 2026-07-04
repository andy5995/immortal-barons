package store

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/andy5995/immortal-barons/internal/game"
)

// ErrBusy means another session holds the game lock.
var ErrBusy = errors.New("game is busy")

type FileLock struct{ f *os.File }

// Lock takes the exclusive game lock. With block=false it returns ErrBusy
// immediately if the lock is held; with block=true it waits. The actual
// locking primitive is platform-specific (flock on Unix, LockFileEx on
// Windows) — see lock_unix.go and lock_windows.go.
func Lock(cfg game.Config, block bool) (*FileLock, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(cfg.DataDir, "game.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f, block); err != nil {
		f.Close()
		return nil, err
	}
	return &FileLock{f: f}, nil
}

func (l *FileLock) Release() error {
	unlockFile(l.f)
	return l.f.Close()
}
