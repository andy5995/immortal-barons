package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseNodeList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brnodes.dat")
	// Two node blocks separated by a blank line (docs/brnodes.sam format).
	content := "1\nAvalon\n363/277\nOrlando\nFL\nUSA\n\n5\nThe Holodeck BBS\n260/249\nRochester\nNY\nUSA\n\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes, err := ParseNodeList(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("want 2 nodes, got %d: %+v", len(nodes), nodes)
	}
	if nodes[0].Number != 1 || nodes[0].Name != "Avalon" || nodes[0].Address != "363/277" {
		t.Errorf("node 0 mismatch: %+v", nodes[0])
	}
	if nodes[1].Number != 5 || nodes[1].City != "Rochester" || nodes[1].State != "NY" {
		t.Errorf("node 1 mismatch: %+v", nodes[1])
	}
}

func TestParseBoardConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bbs.cfg")
	content := "John Dailey\nAvalon\n363/277\nC:\\FD\\FILES\nC:\\FD\\NETMAIL\n900\nFRONTDOOR\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseBoardConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PlanetName != "Avalon" || cfg.Address != "363/277" || cfg.League != 900 || cfg.Mailer != "FRONTDOOR" {
		t.Errorf("board config mismatch: %+v", cfg)
	}
}
