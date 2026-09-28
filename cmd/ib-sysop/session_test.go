package main

import (
	"slices"
	"testing"
)

// A saved directory that would not open at startup stays in the session, so a
// drive mounted late is back on the next launch.
func TestSaveSessionKeepsMissingDirs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	u := &ui{zoom: defaultZoom, missing: []string{"/mnt/bbs/data"}}
	u.tabs = []*boardTab{{u: u, dir: "/srv/door/data"}}
	u.saveSession()
	if got, want := loadSession().Dirs, []string{"/srv/door/data", "/mnt/bbs/data"}; !slices.Equal(got, want) {
		t.Errorf("saved dirs = %v, want %v", got, want)
	}
}
