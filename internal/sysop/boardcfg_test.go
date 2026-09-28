package sysop

import (
	"os"
	"strings"
	"testing"
)

func TestEnsureBoardConfig(t *testing.T) {
	cfg := leagueBoard(t)
	path := BoardConfigPath(cfg.DataDir)

	// An existing file is left exactly as the sysop wrote it.
	if err := os.WriteFile(path, []byte("BoardID Hand Edited\n# a comment\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if created, err := EnsureBoardConfig(cfg.DataDir); err != nil || created {
		t.Fatalf("EnsureBoardConfig on an existing file = %v, %v", created, err)
	}
	if got, _ := os.ReadFile(path); string(got) != "BoardID Hand Edited\n# a comment\n" {
		t.Errorf("an existing bbs.cfg was rewritten:\n%s", got)
	}

	// A missing one is written from the game's own template.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if created, err := EnsureBoardConfig(cfg.DataDir); err != nil || !created {
		t.Fatalf("EnsureBoardConfig on a missing file = %v, %v", created, err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "BoardID ") || !strings.Contains(string(got), "\nLeagueNumber ") {
		t.Errorf("the written bbs.cfg is not the game's template:\n%s", got)
	}
}
