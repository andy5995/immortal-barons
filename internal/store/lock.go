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
//
// SCOPE: this guards several nodes of ONE BBS, on ONE host, against a LOCAL
// filesystem. It is not a general distributed lock, and the docs say so (#135).
//
// Do not "improve" multi-node reach by pointing DataDir at a network share.
// Two things break there, and neither announces itself:
//
//   - The two primitives are unrelated. flock is advisory and whole-file;
//     LockFileEx is mandatory and byte-range, with its own bookkeeping. A
//     Windows node and a Unix node on one directory are not expected to exclude
//     each other at all — both would take "the lock" and proceed. (Reasoned
//     from the two APIs; not reproduced. #135 names the experiment.)
//   - flock over NFS is emulated and depends on the server's configuration, and
//     over SMB the semantics do not map cleanly.
//
// The failure is silent. Save is atomic, so each node writes a complete,
// well-formed world and the last writer wins: no error, no corruption, just a
// turn that is gone. That is why this is a comment and not a runtime check —
// there is nothing here that could detect it.
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
