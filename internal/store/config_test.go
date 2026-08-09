package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestLoadConfig_MissingFile_ReturnsDefaults(t *testing.T) {
	dir := t.TempDir()

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	want := game.DefaultConfig()
	want.DataDir = dir
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("LoadConfig(%q) = %+v, want %+v", dir, cfg, want)
	}
}

func TestSaveConfig_LoadConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Fill EVERY field non-default (same helper as the world round trip), so a
	// Config field that stops serializing cannot hide by round-tripping as the
	// default the test happened not to change.
	cfg := game.DefaultConfig()
	fillValue(reflect.ValueOf(&cfg).Elem())
	cfg.DataDir = dir
	cfg.TurnsPerDay = 15
	cfg.AICount = 3
	cfg.GameLength = 30

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	got, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !reflect.DeepEqual(got, cfg) {
		t.Errorf("round trip = %+v, want %+v", got, cfg)
	}
	if got.DataDir != dir {
		t.Errorf("DataDir = %q, want %q", got.DataDir, dir)
	}
}

func TestLoadConfig_PartialFile_KeepsDefaultsForMissingFields(t *testing.T) {
	dir := t.TempDir()

	partial, err := json.Marshal(map[string]int{"TurnsPerDay": 20})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), partial, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	want := game.DefaultConfig()
	want.DataDir = dir
	want.TurnsPerDay = 20

	if !reflect.DeepEqual(got, want) {
		t.Errorf("LoadConfig(partial) = %+v, want %+v", got, want)
	}
}

// A config written before the packet directories became data-relative still
// points at the right place: the old default is migrated, a custom path is not.
func TestLoadConfigMigratesOldPacketDirDefaults(t *testing.T) {
	dir := t.TempDir()
	body := `{"InboundDir":"./data/inbound","OutboundDir":"/var/spool/out"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.InboundDir != "inbound" {
		t.Errorf("InboundDir = %q, want the new default %q", cfg.InboundDir, "inbound")
	}
	if cfg.OutboundDir != "/var/spool/out" {
		t.Errorf("a custom OutboundDir should be left alone, got %q", cfg.OutboundDir)
	}
}
