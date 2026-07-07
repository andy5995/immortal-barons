package menu

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigEditorEditsAndSaves(t *testing.T) {
	dir := t.TempDir()
	w := newWorld()
	w.Config.DataDir = dir
	w.Config.TurnsPerDay = 10

	// Edit item 1 (turns per day) to 20, then S to save and exit.
	f := &fakeSession{keys: []rune("1\r20\rs\r ")}
	configEditor(f, w)

	if w.Config.TurnsPerDay != 20 {
		t.Errorf("expected turns per day 20, got %d", w.Config.TurnsPerDay)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Errorf("config.json should have been saved: %v", err)
	}
}

func TestConfigEditorTurnsFloorAtOne(t *testing.T) {
	dir := t.TempDir()
	w := newWorld()
	w.Config.DataDir = dir

	// Try to set turns per day to 0; it must floor at 1 (a day needs a turn).
	// Value 0, then S to save and exit.
	f := &fakeSession{keys: []rune("1\r0\rs\r ")}
	configEditor(f, w)

	if w.Config.TurnsPerDay != 1 {
		t.Errorf("turns per day should floor at 1, got %d", w.Config.TurnsPerDay)
	}
}
