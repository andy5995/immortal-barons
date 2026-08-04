//go:build !windows

package session

// EnableVirtualTerminal is a no-op everywhere but Windows: a terminal that
// needs asking before it will honour ANSI is a Windows console peculiarity.
func EnableVirtualTerminal() (ok bool, restore func()) { return true, func() {} }
