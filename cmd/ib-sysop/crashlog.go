package main

import (
	"os"
	"path/filepath"
	"runtime/debug"
)

// crashLog is where a fatal crash's message and stack trace are written, or ""
// when there is nowhere to write them. A panel started by double-click has a
// console window of its own, which closes the moment it crashes and takes the
// trace with it; this keeps a copy. It lives beside the session file, in the
// user's config directory, because the program's own folder may not be
// writable.
var crashLog string

// keepCrashLog makes the runtime copy any fatal crash, on any goroutine, to
// crashLog. The file is appended to, so an earlier crash is not lost, and it
// stays empty until something actually crashes.
func keepCrashLog() {
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	path := filepath.Join(dir, "immortal-barons", "ib-sysop-crash.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		f.Close()
		return
	}
	f.Close() // SetCrashOutput keeps its own duplicate of the file
	crashLog = path
}
