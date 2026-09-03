//go:build !windows

package store

// isSharingViolation is Windows-only; see sharing_windows.go. On Unix a rename
// is atomic against a concurrent open, so the race it works around cannot
// happen.
func isSharingViolation(error) bool { return false }
