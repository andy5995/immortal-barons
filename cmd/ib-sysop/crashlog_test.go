package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The crash log is set up in the user's config directory, beside the session
// file, and exists from the start so a sysop can find it before anything goes
// wrong.
func TestKeepCrashLogPointsAtTheConfigDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	if runtime.GOOS == "darwin" {
		t.Setenv("HOME", dir)
	}
	crashLog = ""
	keepCrashLog()
	if crashLog == "" {
		t.Fatal("no crash log was set up")
	}
	if want := filepath.Dir(sessionFile()); filepath.Dir(crashLog) != want {
		t.Errorf("crash log %s is not beside the session file in %s", crashLog, want)
	}
	if _, err := os.Stat(crashLog); err != nil {
		t.Errorf("crash log file was not created: %v", err)
	}
}
