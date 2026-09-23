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
		"GameInbound /home/bbs/ftn/in\n" +
		"gameoutbound  /home/bbs/filebox/uplink\n" +
		"GameOutbound 3 /home/bbs/filebox/node3\n" +
		"gameOutbound  5   /home/bbs/filebox/node5\n" +
		"SomeFutureKey BINKLEY\n" // a keyword this version knows nothing about
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
			t.Errorf("GameOutbound %d = %q, want %q", node, got, want)
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

// What the game offers the sysop to paste, the game must read back — including the per-neighbor
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
	cfg.BulletinDir = "bbs/bulletins" // where the game writes its own bulletins (#233)
	cfg.BBSName = "The Dog House BBS" // the web pages' own name for this board (#245)
	cfg.BoardURL = "https://doghouse.example/"
	cfg.BulletinURL = "https://doghouse.example/bulletins"

	if err := os.WriteFile(filepath.Join(dir, BoardConfigFile), []byte(BoardConfigText(cfg)), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got := game.DefaultConfig()
	if err := LoadBoardConfig(dir, &got); err != nil {
		t.Fatalf("LoadBoardConfig: %v", err)
	}
	if got.BoardID != cfg.BoardID || got.LeagueNumber != cfg.LeagueNumber ||
		got.InboundDir != cfg.InboundDir || got.OutboundDir != cfg.OutboundDir ||
		got.BulletinDir != cfg.BulletinDir || got.BBSName != cfg.BBSName ||
		got.BoardURL != cfg.BoardURL || got.BulletinURL != cfg.BulletinURL {
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

// The reset that started #152 wrote bbs.cfg from a defaults-seeded config and
// returned a working board to "local" and node 0. Nothing the game saves may
// touch that file once it exists.
func TestSavingConfigLeavesBoardConfigAlone(t *testing.T) {
	dir := t.TempDir()
	own := "BoardID Alpha BBS\nLeagueNumber 900\nGameInbound ftn/in\nGameOutbound ftn/out\n"
	path := filepath.Join(dir, BoardConfigFile)
	if err := os.WriteFile(path, []byte(own), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := game.DefaultConfig() // as a reset seeds it: BoardID "local", league 0
	cfg.DataDir = dir
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != own {
		t.Errorf("%s was rewritten:\n%s", BoardConfigFile, got)
	}
}

// bbs.cfg is hand-written and may be in place before the game has ever written
// a config.json — a sysop preparing a board, or a data directory restored from
// a backup that only kept the files they edit. LoadConfig used to return the
// moment config.json was missing, so every setting in bbs.cfg went unread and
// the board came up as "local" on node 0.
func TestBoardConfigIsReadWithNoConfigJSON(t *testing.T) {
	dir := t.TempDir()
	body := "BoardID Avalon\nLeagueNumber 900\n"
	if err := os.WriteFile(filepath.Join(dir, BoardConfigFile), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.BoardID != "Avalon" || cfg.LeagueNumber != 900 {
		t.Errorf("bbs.cfg went unread: BoardID %q, league %d", cfg.BoardID, cfg.LeagueNumber)
	}
}

// The lottery switch is the one bbs.cfg setting spelled as a word, so take the
// spellings a sysop is likely to write, and leave the default alone for one
// that means nothing.
func TestLotterySwitchSpellings(t *testing.T) {
	for _, c := range []struct {
		value string
		want  bool
	}{
		{"no", false}, {"No", false}, {"off", false}, {"false", false},
		{"yes", true}, {"YES", true}, {"on", true}, {"true", true},
		{"maybe", true}, // unparseable: the default stands
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, BoardConfigFile),
			[]byte("Lottery "+c.value+"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.Lottery != c.want {
			t.Errorf("Lottery %q gave %v, want %v", c.value, cfg.Lottery, c.want)
		}
	}
}

// The league number reaches packet filenames and decides which packets in a
// shared inbound directory belong to this game, so a number outside BRE's
// documented 1-999 is refused. It leaves the field unset rather than failing
// the whole file, matching how the roster parser drops one bad node line.
func TestLeagueNumberOutsideTheRangeIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  int
	}{
		{"in range", "42", 42},
		{"lowest allowed", "1", 1},
		{"highest allowed", "999", 999},
		{"above the ceiling", "1000", 0},
		{"zero", "0", 0},
		{"negative", "-3", 0},
		{"not a number", "seven", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "BoardID Test Board\nLeagueNumber " + tc.value + "\n"
			if err := os.WriteFile(filepath.Join(dir, BoardConfigFile), []byte(body), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			cfg := game.DefaultConfig()
			cfg.LeagueNumber = 0
			if err := LoadBoardConfig(dir, &cfg); err != nil {
				t.Fatalf("LoadBoardConfig: %v", err)
			}
			if cfg.LeagueNumber != tc.want {
				t.Errorf("LeagueNumber = %d, want %d", cfg.LeagueNumber, tc.want)
			}
		})
	}
}

// The pirate-news switch takes the same word spellings as the lottery, and
// suppressing the news leaves the raids themselves alone (internal/game).
func TestPirateNewsSwitch(t *testing.T) {
	for _, c := range []struct {
		value string
		want  bool
	}{
		{"no", false}, {"off", false}, {"false", false},
		{"yes", true}, {"YES", true},
		{"", true}, // unparseable: the default stands
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, BoardConfigFile),
			[]byte("PirateNews "+c.value+"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.PirateNews != c.want {
			t.Errorf("PirateNews %q gave %v, want %v", c.value, cfg.PirateNews, c.want)
		}
	}
}

// A directory named by a bare number is still the board-wide directory; only a
// number followed by more is one neighbor's.
func TestGameOutboundNamedByANumberIsBoardWide(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, BoardConfigFile), []byte("GameOutbound 7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := game.DefaultConfig()
	if err := LoadBoardConfig(dir, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.OutboundDir != "7" || len(cfg.OutboundDirs) != 0 {
		t.Errorf("GameOutbound 7 read as %q / %v", cfg.OutboundDir, cfg.OutboundDirs)
	}
}

// A key no reader knows is named with its line and, when one is close, the key
// it was probably meant to be; a switch that is neither yes nor no is named with
// its value. Known keys in any case, comments and the transport's own keys are
// not flagged.
func TestBoardWarningsNameUnknownKeysAndBadSwitches(t *testing.T) {
	dir := t.TempDir()
	body := "# notes\n" +
		"boardid Alpha\n" +
		"GameInbond\tin\n" +
		"; more notes\n" +
		"IncomingFileDir /mail/in\n" +
		"Colour blue\n" +
		"MAILER Binkley\n" +
		"Lottery maybe\n"
	if err := os.WriteFile(filepath.Join(dir, BoardConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := BoardWarnings(dir, []string{"IncomingFileDir", "Mailer"})
	if len(got) != 3 {
		t.Fatalf("got %d warnings, want 3: %q", len(got), got)
	}
	if !strings.Contains(got[0], "line 3") || !strings.Contains(got[0], `"GameInbond"`) ||
		!strings.Contains(got[0], "did you mean GameInbound?") {
		t.Errorf("misspelled key warning = %q", got[0])
	}
	if !strings.Contains(got[1], "line 6") || !strings.Contains(got[1], `"Colour"`) ||
		strings.Contains(got[1], "did you mean") {
		t.Errorf("unrelated key warning = %q; it should name the line and suggest nothing", got[1])
	}
	if !strings.Contains(got[2], "line 8") || !strings.Contains(got[2], "Lottery") ||
		!strings.Contains(got[2], `"maybe"`) || !strings.Contains(got[2], "Yes or No") {
		t.Errorf("bad switch warning = %q; it should name the line, key, value and what is accepted", got[2])
	}
}

// Every key the reader knows, in any case, raises no warning, and each one is
// read: BoardKeys is derived from the setters, so this holds by construction,
// and the count pins it against a key being added to the constants alone.
func TestKnownBoardKeysRaiseNoWarning(t *testing.T) {
	keys := BoardKeys()
	if len(keys) != 11 {
		t.Errorf("BoardKeys has %d keys, want 11: %q", len(keys), keys)
	}
	dir := t.TempDir()
	var body strings.Builder
	for i, k := range keys {
		if i%2 == 1 {
			k = strings.ToLower(k)
		}
		body.WriteString(k + " yes\n")
	}
	if err := os.WriteFile(filepath.Join(dir, BoardConfigFile), []byte(body.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := BoardWarnings(dir, nil); len(got) != 0 {
		t.Errorf("known keys were warned about: %q", got)
	}
}

// A sysop lining values up in columns uses tabs or runs of spaces. Either
// separates a key from its value, and a value keeps the spaces inside it.
func TestBoardConfigAcceptsTabsAndRunsOfSpaces(t *testing.T) {
	dir := t.TempDir()
	body := "BoardID\tThe X-Bit BBS\n" +
		"LeagueNumber    777\n" +
		"GameInbound\t \t" + `C:\SBBS\ib in` + "\n" +
		"GameOutbound 3\t/srv/filebox/node 3\n"
	if err := os.WriteFile(filepath.Join(dir, BoardConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := game.DefaultConfig()
	cfg.DataDir = dir
	if err := LoadBoardConfig(dir, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.BoardID != "The X-Bit BBS" {
		t.Errorf("BoardID = %q, want %q", cfg.BoardID, "The X-Bit BBS")
	}
	if cfg.LeagueNumber != 777 {
		t.Errorf("LeagueNumber = %d, want 777", cfg.LeagueNumber)
	}
	if !strings.HasSuffix(cfg.Inbound(), `C:\SBBS\ib in`) {
		t.Errorf("GameInbound = %q, want it to end in %q", cfg.Inbound(), `C:\SBBS\ib in`)
	}
	if got := cfg.OutboundDirs[3]; got != "/srv/filebox/node 3" {
		t.Errorf("GameOutbound 3 = %q, want %q", got, "/srv/filebox/node 3")
	}
}
