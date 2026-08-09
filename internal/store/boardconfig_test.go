package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The file exists to be edited by hand, so it has to survive being edited by
// hand: odd casing, extra spaces, both comment markers, blank lines, and a
// keyword from some other version of the game.
func TestBoardConfigSurvivesHandEditing(t *testing.T) {
	dir := t.TempDir()
	body := "# a comment\n; another\n\n" +
		"boardid   Eye of the Storm  \n" +
		"LEAGUENUMBER 42\n" +
		"Inbound /home/bbs/ftn/in\n" +
		"outbound  /home/bbs/filebox/uplink\n" +
		"Link 3 /home/bbs/filebox/node3\n" +
		"link  5   /home/bbs/filebox/node5\n" +
		"Mailer BINKLEY\n" // a keyword this version knows nothing about
	if err := os.WriteFile(filepath.Join(dir, BoardConfigFile), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := game.DefaultConfig()
	if err := LoadBoardConfig(dir, &cfg); err != nil {
		t.Fatalf("LoadBoardConfig: %v", err)
	}
	// A board name has spaces in it, so only the surrounding whitespace goes.
	if cfg.BoardID != "Eye of the Storm" {
		t.Errorf("BoardID = %q, want %q", cfg.BoardID, "Eye of the Storm")
	}
	if cfg.LeagueNumber != 42 {
		t.Errorf("LeagueNumber = %d, want 42", cfg.LeagueNumber)
	}
	if cfg.InboundDir != "/home/bbs/ftn/in" || cfg.OutboundDir != "/home/bbs/filebox/uplink" {
		t.Errorf("dirs = %q / %q", cfg.InboundDir, cfg.OutboundDir)
	}
	for node, want := range map[int]string{3: "/home/bbs/filebox/node3", 5: "/home/bbs/filebox/node5"} {
		if got := cfg.OutboundDirs[node]; got != want {
			t.Errorf("Link %d = %q, want %q", node, got, want)
		}
	}
}

// A missing file is the normal case for a board that has never been in a
// league, and must not blank out what the defaults set.
func TestMissingBoardConfigChangesNothing(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.BoardID = "Avalon"
	before := cfg
	if err := LoadBoardConfig(t.TempDir(), &cfg); err != nil {
		t.Fatalf("LoadBoardConfig: %v", err)
	}
	if cfg.BoardID != before.BoardID || cfg.InboundDir != before.InboundDir {
		t.Errorf("a missing file changed the config: %+v", cfg)
	}
}

// What the game writes, the game must read back — including the per-neighbour
// links, which BRE's positional format could not carry at all.
func TestBoardConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := game.DefaultConfig()
	cfg.DataDir = dir
	cfg.BoardID = "Bravo BBS"
	cfg.LeagueNumber = 900
	cfg.InboundDir = "ftn/in"
	cfg.OutboundDir = "ftn/out"
	cfg.OutboundDirs = map[int]string{5: "box/five", 3: "box/three"}

	if err := SaveBoardConfig(cfg); err != nil {
		t.Fatalf("SaveBoardConfig: %v", err)
	}
	got := game.DefaultConfig()
	if err := LoadBoardConfig(dir, &got); err != nil {
		t.Fatalf("LoadBoardConfig: %v", err)
	}
	if got.BoardID != cfg.BoardID || got.LeagueNumber != cfg.LeagueNumber ||
		got.InboundDir != cfg.InboundDir || got.OutboundDir != cfg.OutboundDir {
		t.Errorf("round trip = %+v", got)
	}
	if len(got.OutboundDirs) != 2 || got.OutboundDirs[3] != "box/three" || got.OutboundDirs[5] != "box/five" {
		t.Errorf("links = %v, want the two written", got.OutboundDirs)
	}
}

// A board set up before the split has its settings in config.json and nowhere
// else. Losing them would point the door at the wrong directories, which on a
// live board means packets written where no mailer is looking.
func TestPerBoardSettingsMigrateOutOfConfigJSON(t *testing.T) {
	dir := t.TempDir()
	old := `{"TurnsPerDay":12,"BoardID":"Old Board","InboundDir":"ftn/in",` +
		`"OutboundDir":"ftn/out","LeagueNumber":900}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(old), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.BoardID != "Old Board" || cfg.LeagueNumber != 900 ||
		cfg.InboundDir != "ftn/in" || cfg.OutboundDir != "ftn/out" {
		t.Fatalf("settings lost in migration: %+v", cfg)
	}
	if cfg.TurnsPerDay != 12 {
		t.Errorf("TurnsPerDay = %d, want the ruleset untouched at 12", cfg.TurnsPerDay)
	}

	// Saving completes the move: bbs.cfg now holds them and config.json does not.
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	written, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	for _, key := range []string{"BoardID", "InboundDir", "OutboundDir", "LeagueNumber"} {
		if strings.Contains(string(written), key) {
			t.Errorf("config.json still carries %s", key)
		}
	}
	again, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig after save: %v", err)
	}
	if again.BoardID != "Old Board" || again.LeagueNumber != 900 {
		t.Errorf("settings lost after the move: %+v", again)
	}
}

// bbs.cfg is the authority once it exists, so an edit there beats a stale value
// left behind in config.json.
func TestBoardConfigBeatsALeftoverConfigJSONValue(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"),
		[]byte(`{"BoardID":"Stale Name"}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, BoardConfigFile),
		[]byte("BoardID Edited Name\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.BoardID != "Edited Name" {
		t.Errorf("BoardID = %q, want the hand-edited name", cfg.BoardID)
	}
}
