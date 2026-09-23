package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A door launched with a -data path that does not exist names the path, not the
// door.json setting the missing directory would otherwise be mistaken for.
func TestDoorNamesAMissingDataDirectory(t *testing.T) {
	if os.Getenv("IB_DOOR_DATADIR_CHILD") == "1" {
		os.Args = []string{"immortal-barons", "-data", os.Getenv("IB_DOOR_DATADIR"), "door32.sys"}
		main()
		return
	}
	missing := filepath.Join(t.TempDir(), "no-such-dir")
	cmd := exec.Command(os.Args[0], "-test.run=^TestDoorNamesAMissingDataDirectory$")
	cmd.Env = append(os.Environ(), "IB_DOOR_DATADIR_CHILD=1", "IB_DOOR_DATADIR="+missing)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("door with a missing data directory exited 0:\n%s", out)
	}
	if !strings.Contains(string(out), missing) || !strings.Contains(string(out), "does not exist") {
		t.Errorf("the refusal does not name the missing directory:\n%s", out)
	}
	if strings.Contains(string(out), "No drop file format") {
		t.Errorf("a missing data directory was reported as an unset drop file format:\n%s", out)
	}
}
