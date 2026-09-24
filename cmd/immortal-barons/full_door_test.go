package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// -full launched as a door reads the drop file named by -dropfile, as the door
// does without it. It used to take its -local branch whenever a player name was
// set, and -name defaults to the OS user, so a door caller got nothing and the
// game ran on the BBS machine's console as that user.
func TestFullAsADoorReadsTheDropFile(t *testing.T) {
	if os.Getenv("IB_FULL_DOOR_CHILD") == "1" {
		os.Args = []string{"immortal-barons", "-full", "-data", os.Getenv("IB_FULL_DOOR_DATA"),
			"-dropfile", os.Getenv("IB_FULL_DOOR_DROP")}
		main()
		return
	}
	root := t.TempDir()
	cfg := game.DefaultConfig()
	cfg.DataDir = filepath.Join(root, "data")
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(store.NewGame(cfg), cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DataDir, "door.json"), []byte(`{"DropfileFormat":"door32"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// In a node directory, not the working directory: -dropfile is the only way
	// to it, which is how Synchronet launches a door.
	node := filepath.Join(root, "node1")
	if err := os.MkdirAll(node, 0o755); err != nil {
		t.Fatal(err)
	}
	drop := filepath.Join(node, "door32.sys")
	body := "0\r\n0\r\n38400\r\nTest BBS\r\n1\r\nReal Name\r\nDropCaller\r\n100\r\n60\r\n1\r\n1\r\n"
	if err := os.WriteFile(drop, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestFullAsADoorReadsTheDropFile$")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "IB_FULL_DOOR_CHILD=1", "IB_FULL_DOOR_DATA="+cfg.DataDir, "IB_FULL_DOOR_DROP="+drop)
	out, _ := cmd.CombinedOutput()

	log, err := os.ReadFile(filepath.Join(cfg.DataDir, "ib-door.log"))
	if err != nil {
		t.Fatalf("-full wrote no door log, so it never took the door path:\n%s", out)
	}
	if !strings.Contains(string(log), `launch handle="DropCaller"`) {
		t.Errorf("-full did not launch as the drop file's caller:\n%s\noutput:\n%s", log, out)
	}
}
