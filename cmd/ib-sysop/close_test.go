package main

import "testing"

// Closing a tab to the left of the active one must leave the same board
// showing: the Run view acts on whichever board is active.
func TestCloseKeepsTheActiveBoard(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // close saves the session
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir()) // os.UserConfigDir on Windows
	u := &ui{}
	for _, d := range []string{"A", "B", "C", "D"} {
		u.tabs = append(u.tabs, &boardTab{u: u, dir: d})
	}
	u.active = 2
	u.close(0)
	if got := u.tabs[u.active].dir; got != "C" {
		t.Errorf("after closing A with C active, the active tab is %s", got)
	}
	u.close(u.active)
	if got := u.tabs[u.active].dir; got != "D" {
		t.Errorf("after closing the active C, the active tab is %s, want the next one, D", got)
	}
}
