//go:build unix

package store

import (
	"errors"
	"os"
	"syscall"
)

// lockFile takes an advisory exclusive flock on f. block=false uses LOCK_NB
// and maps EWOULDBLOCK to ErrBusy.
func lockFile(f *os.File, block bool) error {
	how := syscall.LOCK_EX
	if !block {
		how |= syscall.LOCK_NB
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return ErrBusy
		}
		return err
	}
	return nil
}

func unlockFile(f *os.File) {
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
