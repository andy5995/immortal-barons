//go:build windows

package session

import (
	"os"

	"golang.org/x/sys/windows"
)

// EnableVirtualTerminal turns on ENABLE_VIRTUAL_TERMINAL_PROCESSING for this
// process's own console output, so the ANSI we write is interpreted instead of
// printed as literal escape text. It reports whether ANSI output is usable and
// returns a function that puts the console mode back the way we found it.
//
// The legacy console host (HKCU\Console\ForceV2=0) has no VT support at all and
// refuses the mode; no per-process call can change that, so the honest answer
// there is false and the caller must fall back to plain text (issue #98). A
// sysop should not have to flip ForceV2 globally to read our screens — on that
// BBS it broke the NTVDM 16-bit utilities sharing the machine.
//
// Stdout that is not a console (redirected to a file or pipe) reports true:
// the escapes reach the consumer unmangled, which is what it asked for.
func EnableVirtualTerminal() (ok bool, restore func()) {
	noop := func() {}
	h := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return true, noop // not a console
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true, noop
	}
	if err := windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return false, noop
	}
	// Read the mode back rather than trusting the call: the bit is what decides
	// whether a screenful of escapes renders or lands on the sysop as garbage.
	var got uint32
	if err := windows.GetConsoleMode(h, &got); err != nil || got&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING == 0 {
		windows.SetConsoleMode(h, mode)
		return false, noop
	}
	return true, func() { windows.SetConsoleMode(h, mode) }
}
