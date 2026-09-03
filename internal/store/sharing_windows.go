//go:build windows

package store

import (
	"errors"
	"syscall"
)

// syscall names ERROR_ACCESS_DENIED but not the sharing violation, the same
// reason lock_windows.go spells ERROR_LOCK_VIOLATION out.
const errorSharingViolation = syscall.Errno(32)

// isSharingViolation reports whether err is Windows refusing an operation
// because another process holds the file open. Go opens files with
// FILE_SHARE_READ|FILE_SHARE_WRITE and NOT FILE_SHARE_DELETE, so an open and a
// replacing rename of the same path exclude each other for the instant they
// overlap: the open fails with ERROR_SHARING_VIOLATION, the rename with
// ERROR_ACCESS_DENIED. Unix has no equivalent — rename there is atomic against
// an open, which is why this file has no Unix counterpart beyond a false.
func isSharingViolation(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	return errno == errorSharingViolation || errno == syscall.ERROR_ACCESS_DENIED
}
