//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package mod

// protectInheritedFDs: Windows needs nothing here. Go's os/exec starts a child
// with a handle list (PROC_THREAD_ATTRIBUTE_HANDLE_LIST), so only the standard
// handles are inherited, whatever the game has open.
func protectInheritedFDs() {}
