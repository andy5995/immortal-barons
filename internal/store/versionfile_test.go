package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The required version lives in a file a sysop can read and edit, not only in
// config.json — that is the whole point of it being separate.
func TestVersionFileRoundTrips(t *testing.T) {
	dir := t.TempDir()
	if got := ReadVersionFile(dir); got != "" {
		t.Errorf("a league with no file should have no requirement, got %q", got)
	}
	if err := WriteVersionFile(dir, "0.0.5"); err != nil {
		t.Fatal(err)
	}
	if got := ReadVersionFile(dir); got != "0.0.5" {
		t.Errorf("read back %q, want 0.0.5", got)
	}
	// Hand-edited forms a sysop is likely to produce.
	for _, body := range []string{"v0.0.7\n", "# a comment\n\n0.0.7\n", "  0.0.7  \n"} {
		if err := os.WriteFile(filepath.Join(dir, VersionFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := ReadVersionFile(dir); got != "0.0.7" {
			t.Errorf("body %q read as %q, want 0.0.7", body, got)
		}
	}
}

// Saving the config writes the file, so a Coordinator's broadcast leaves the
// requirement somewhere legible on every member board.
func TestSaveConfigRecordsTheRequiredVersion(t *testing.T) {
	dir := t.TempDir()
	cfg := game.DefaultConfig()
	cfg.DataDir, cfg.IBBS, cfg.MinBoardVersion = dir, true, "0.0.9"
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if got := ReadVersionFile(dir); got != "0.0.9" {
		t.Errorf("the file says %q after a save, want 0.0.9", got)
	}
	// ...and loading picks it up again.
	got, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.MinBoardVersion != "0.0.9" {
		t.Errorf("LoadConfig gave %q, want 0.0.9", got.MinBoardVersion)
	}
}

// A hand-edited file beats a stale config.json, so editing the file is a real
// way to set the requirement rather than a copy that gets overwritten.
func TestVersionFileBeatsConfigJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := game.DefaultConfig()
	cfg.DataDir, cfg.IBBS, cfg.MinBoardVersion = dir, true, "0.0.5"
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if err := WriteVersionFile(dir, "0.1.0"); err != nil { // sysop edits the file
		t.Fatal(err)
	}
	got, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.MinBoardVersion != "0.1.0" {
		t.Errorf("LoadConfig gave %q, want the hand-edited 0.1.0", got.MinBoardVersion)
	}
}
