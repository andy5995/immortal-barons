package store

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/andy5995/immortal-barons/internal/game"
)

// ErrBusy means another session holds the game lock.
var ErrBusy = errors.New("game is busy")

type FileLock struct{ f *os.File }

// Lock takes the exclusive game lock. With block=false it returns ErrBusy
// immediately if the lock is held; with block=true it waits.
func Lock(cfg game.Config, block bool) (*FileLock, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(cfg.DataDir, "game.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	how := syscall.LOCK_EX
	if !block {
		how |= syscall.LOCK_NB
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, err
	}
	return &FileLock{f: f}, nil
}

func (l *FileLock) Release() error {
	syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	return l.f.Close()
}
