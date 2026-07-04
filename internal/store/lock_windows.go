//go:build windows

package store

import (
	"os"
	"syscall"
	"unsafe"
)

// Windows has no flock; it locks byte ranges via LockFileEx/UnlockFileEx in
// kernel32. We lock a single byte of the lock file, which is enough for a
// whole-file mutual-exclusion guard. Using the stdlib LazyDLL binding keeps
// this dependency-free.
var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

const (
	lockfileExclusiveLock   = 0x2
	lockfileFailImmediately = 0x1
	errorLockViolation      = 0x21 // ERROR_LOCK_VIOLATION (33)
)

// lockFile takes an exclusive whole-file lock on f. block=false adds
// LOCKFILE_FAIL_IMMEDIATELY and maps the resulting lock violation to ErrBusy.
func lockFile(f *os.File, block bool) error {
	flags := uintptr(lockfileExclusiveLock)
	if !block {
		flags |= lockfileFailImmediately
	}
	ol := new(syscall.Overlapped)
	r1, _, e1 := procLockFileEx.Call(f.Fd(), flags, 0, 1, 0, uintptr(unsafe.Pointer(ol)))
	if r1 == 0 {
		if e1 == syscall.Errno(errorLockViolation) {
			return ErrBusy
		}
		return e1
	}
	return nil
}

func unlockFile(f *os.File) {
	ol := new(syscall.Overlapped)
	procUnlockFileEx.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(ol)))
}
