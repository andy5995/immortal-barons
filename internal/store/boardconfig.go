package store

import (
	"bufio"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// BoardConfigFile holds the settings that describe THIS board rather than the
// game: its name in the league, its league number, and where its packets are
// read and written. BRE keeps the same split in BBS.CFG.
//
// They are kept out of config.json for two reasons. A Coordinator's broadcast
// rewrites config.json, so per-board settings have no business living there;
// and a sysop edits these by hand more than anything else in the game, paths
// being the thing that moves when a BBS is reinstalled or a mailer changes.
const BoardConfigFile = "bbs.cfg"

// The keywords, one per line, rather than BRE's seven positional lines.
// Positional cannot express the per-neighbor links at all, and a field left
// blank in one silently shifts every field after it — which is most of what
// BRE's own troubleshooting section is about. Matched case-insensitively;
// written in this casing.
const (
	keyBoardID  = "BoardID"
	keyBBSName  = "BBSName"
	keyBoardURL = "BoardURL"
	keyBullURL  = "BulletinURL"
	keyLeague   = "LeagueNumber"
	keyInbound  = "GameInbound"
	keyOutbound = "GameOutbound"
	keyLottery  = "Lottery"
	keyBulletin = "BulletinDir"
	keyPirate   = "PirateNews"
	keyOnFault  = "OnFault"
)

// boardSetters applies each bbs.cfg keyword this reader recognizes to a
// config. BoardKeys is derived from it, so the list of known keys and the keys
// actually read cannot drift apart. A setter's error is a value it could not
// use and left the default in place for; BoardWarnings reports it.
var boardSetters = map[string]func(cfg *game.Config, value string) error{
	keyBoardID:  func(cfg *game.Config, v string) error { cfg.BoardID = v; return nil },
	keyBBSName:  func(cfg *game.Config, v string) error { cfg.BBSName = v; return nil },
	keyBoardURL: func(cfg *game.Config, v string) error { cfg.BoardURL = v; return nil },
	keyBullURL:  func(cfg *game.Config, v string) error { cfg.BulletinURL = v; return nil },
	keyLeague: func(cfg *game.Config, v string) error {
		// Out of range is left unset (0, "never set") rather than failing the
		// whole import, the way the roster parser drops one bad node line
		// instead of the file. game.MaxLeagueNumber was declared for this
		// bound and nothing had applied it, so any number at all was taken —
		// and the league number reaches packet filenames.
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= game.MaxLeagueNumber {
			cfg.LeagueNumber = n
		}
		return nil
	},
	keyInbound: func(cfg *game.Config, v string) error { cfg.InboundDir = v; return nil },
	keyOutbound: func(cfg *game.Config, v string) error {
		// "GameOutbound <node> <dir>" is one neighbor's own directory (#106);
		// a bare "GameOutbound <dir>" is everyone else's. A directory whose
		// name is a number still works on its own, since only a number
		// followed by more is read as a node.
		if n, dir, ok := perNodeDir(v); ok {
			if cfg.OutboundDirs == nil {
				cfg.OutboundDirs = map[int]string{}
			}
			cfg.OutboundDirs[n] = dir
			return nil
		}
		cfg.OutboundDir = v
		return nil
	},
	keyBulletin: func(cfg *game.Config, v string) error { cfg.BulletinDir = v; return nil },
	keyLottery:  func(cfg *game.Config, v string) error { return setYesNo(&cfg.Lottery, v) },
	keyOnFault:  func(cfg *game.Config, v string) error { cfg.OnFault = v; return nil },
	keyPirate:   func(cfg *game.Config, v string) error { return setYesNo(&cfg.PirateNews, v) },
}

// BoardKeys are the bbs.cfg keywords this reader recognizes, sorted. The
// transport's are ftn.Keys; a key in neither list is a line nothing reads.
func BoardKeys() []string {
	return slices.Sorted(maps.Keys(boardSetters))
}

// ParseYesNo reads a bbs.cfg switch. The original's own configuration file
// spells its booleans "yes" and "no"; the other spellings are the ones a sysop
// is likely to reach for.
func ParseYesNo(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "true", "on", "1":
		return true, nil
	case "no", "false", "off", "0":
		return false, nil
	}
	return false, fmt.Errorf("want Yes or No, got %q", strings.TrimSpace(value))
}

// setYesNo sets *dst from a switch's value. A value that is not a yes or a no
// is returned as an error and leaves *dst alone; a key with no value at all is
// not an error, as the transport's reader skips such a line.
func setYesNo(dst *bool, value string) error {
	if value == "" {
		return nil
	}
	b, err := ParseYesNo(value)
	if err != nil {
		return err
	}
	*dst = b
	return nil
}

// BoardLine is one setting line of bbs.cfg.
type BoardLine struct {
	N     int    // the line number, from 1
	Key   string // as written; readers match it case-insensitively
	Value string // see SplitKey
	Raw   string // the whole line, trimmed
}

// BoardLines reads <dataDir>/bbs.cfg and returns its setting lines, leaving out
// blank lines and comments (a line starting with # or ;). Every reader of the
// file goes through here, so all of them agree on what a line is. A missing
// file has no lines and is not an error.
func BoardLines(dataDir string) ([]BoardLine, error) {
	f, err := os.Open(boardConfigPath(dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var lines []BoardLine
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, _ := SplitKey(line)
		lines = append(lines, BoardLine{N: n, Key: key, Value: value, Raw: line})
	}
	return lines, sc.Err()
}

// boardSetter finds key's setter, ignoring case.
func boardSetter(key string) func(*game.Config, string) error {
	for k, set := range boardSetters {
		if strings.EqualFold(k, key) {
			return set
		}
	}
	return nil
}

// BoardWarnings names every bbs.cfg line that is read as nothing: a key that is
// neither one of BoardKeys nor one of transportKeys (ftn.Keys, which this
// package cannot import), and a value this reader cannot use.
//
// Both readers skip unknown keys, since they share the file and each skips the
// other's, so without this a misspelled key falls back to its default in
// silence: a misspelled GameInbound leaves the board reading "inbound" and
// going quiet. It warns rather than refuses, because the board still runs on
// its defaults, and a typo in a switch must never stop a caller's session. Old
// spellings are refused before this runs.
func BoardWarnings(dataDir string, transportKeys []string) []string {
	lines, err := BoardLines(dataDir)
	if err != nil {
		return nil
	}
	known := append(BoardKeys(), transportKeys...)
	var warnings []string
	var scratch game.Config
	for _, l := range lines {
		if set := boardSetter(l.Key); set != nil {
			if err := set(&scratch, l.Value); err != nil {
				warnings = append(warnings, fmt.Sprintf("%s line %d: %s: %v; the default stands",
					BoardConfigFile, l.N, l.Key, err))
			}
			continue
		}
		if slices.ContainsFunc(known, func(k string) bool { return strings.EqualFold(k, l.Key) }) {
			continue
		}
		w := fmt.Sprintf("%s line %d: unknown setting %q is ignored", BoardConfigFile, l.N, l.Key)
		if near, ok := NearestName(l.Key, known); ok {
			w += fmt.Sprintf(" (did you mean %s?)", near)
		}
		warnings = append(warnings, w)
	}
	return warnings
}

func boardConfigPath(dataDir string) string { return filepath.Join(dataDir, BoardConfigFile) }

// LoadBoardConfig applies <dataDir>/bbs.cfg to cfg. A missing file leaves cfg
// alone, which is what makes the migration in LoadConfig work: values read from
// an older config.json stand until this file is written for the first time.
//
// An unknown keyword, or a switch that is neither yes nor no, is ignored rather
// than refused (BoardWarnings names both). This file is hand-edited, often by
// someone following a newer version's documentation, and a board that will not
// start is a worse answer than a setting that does nothing.
func LoadBoardConfig(dataDir string, cfg *game.Config) error {
	lines, err := BoardLines(dataDir)
	if err != nil {
		return err
	}
	for _, l := range lines {
		if set := boardSetter(l.Key); set != nil {
			// A value the setter cannot use leaves the default standing, and
			// BoardWarnings reports it to the modes that print warnings.
			_ = set(cfg, l.Value)
		}
	}
	return nil
}

// BoardConfigText renders bbs.cfg as the game would like to see it: bare
// setting lines, no comments. Every key is documented on the website, and a
// generated comment block is one more copy of that prose to keep in step —
// the last one drifted, splitting a sentence across two settings. It is
// printed for the sysop to paste rather than written: this file is the board's
// own identity, a sysop edits it by hand more than anything else in the game,
// and a rules reset that rewrote it put four correct settings back to defaults
// (#152). The one exception is the migration below, which only ever creates
// the file when it is missing.
func BoardConfigText(cfg game.Config) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", keyBoardID, cfg.BoardID)
	fmt.Fprintf(&b, "%s %s\n", keyBBSName, cfg.BBSName)
	fmt.Fprintf(&b, "%s %s\n", keyBoardURL, cfg.BoardURL)
	fmt.Fprintf(&b, "%s %d\n", keyLeague, cfg.LeagueNumber)
	fmt.Fprintf(&b, "%s %s\n", keyInbound, cfg.InboundDir)
	fmt.Fprintf(&b, "%s %s\n", keyOutbound, cfg.OutboundDir)
	fmt.Fprintf(&b, "%s %s\n", keyBullURL, cfg.BulletinURL)
	fmt.Fprintf(&b, "%s %s\n", keyBulletin, cfg.BulletinDir)
	fmt.Fprintf(&b, "%s %s\n", keyLottery, yesNo(cfg.Lottery))
	fmt.Fprintf(&b, "%s %s\n", keyPirate, yesNo(cfg.PirateNews))
	if cfg.OnFault != "" {
		fmt.Fprintf(&b, "%s %s\n", keyOnFault, cfg.OnFault)
	}
	for _, n := range slices.Sorted(maps.Keys(cfg.OutboundDirs)) {
		fmt.Fprintf(&b, "%s %d %s\n", keyOutbound, n, cfg.OutboundDirs[n])
	}
	return b.String()
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// migrateBoardConfig creates bbs.cfg from settings that a board set up before
// the split still holds in config.json, and nowhere else. Without it the first
// save after the upgrade would erase them: they no longer marshal into
// config.json, so the board would quietly fall back to "local" and node 0.
//
// It writes only when the file is absent, so it can never overwrite a working
// board's own file — the whole point of #152. A failure is not the caller's
// problem: loading a game must not fail because a directory is read-only.
func migrateBoardConfig(dataDir string, cfg game.Config) {
	if _, err := os.Stat(boardConfigPath(dataDir)); err == nil || !os.IsNotExist(err) {
		return
	}
	os.WriteFile(boardConfigPath(dataDir), []byte(BoardConfigText(cfg)), 0o644)
}

// SplitKey splits a bbs.cfg line into its key and its value at the first space
// or tab, trimming the value but keeping the spaces inside it (a Windows path
// may hold some). ok is false when the line holds a key alone. A sysop lining
// values up in columns reaches for tabs, and a split on spaces alone read such
// a line as one long unknown key and dropped the setting without a word.
func SplitKey(line string) (key, value string, ok bool) {
	i := strings.IndexAny(line, " \t")
	if i < 0 {
		return line, "", false
	}
	return line[:i], strings.TrimSpace(line[i+1:]), true
}

// perNodeDir splits "<node> <dir>" when value starts with a node number and has
// more after it.
func perNodeDir(value string) (int, string, bool) {
	node, dir, ok := SplitKey(value)
	if !ok {
		return 0, "", false
	}
	n, err := strconv.Atoi(node)
	if err != nil || n < 1 || n > game.MaxNodeNumber || dir == "" {
		return 0, "", false
	}
	return n, dir, true
}

// LegacyBoardRefusal reports bbs.cfg lines written under names this version no
// longer reads -- Inbound, Outbound, and the two-field "Link <node> <dir>" --
// with the lines that replace them, ready to paste. Nil when there are none.
//
// Refused rather than read under both names: an ignored Inbound line would put
// the board back on the default directory without a word, and accepting the
// old spelling forever keeps alive the confusion the rename removed (#241).
// The values are copied byte for byte, so a Windows path keeps its backslashes.
//
// isTransportLink tells the FTN transport's Link lines (ftn.IsLink) from the
// old per-neighbor directory. It is passed in because that grammar belongs to
// the transport, which imports this package.
func LegacyBoardRefusal(dataDir string, isTransportLink func(value string) bool) error {
	lines, err := BoardLines(dataDir)
	if err != nil {
		return nil
	}
	var old, repl []string
	for _, l := range lines {
		switch {
		case strings.EqualFold(l.Key, "Inbound"):
			old, repl = append(old, l.Raw), append(repl, keyInbound+" "+l.Value)
		case strings.EqualFold(l.Key, "Outbound"):
			old, repl = append(old, l.Raw), append(repl, keyOutbound+" "+l.Value)
		case strings.EqualFold(l.Key, "Link"):
			if len(strings.Fields(l.Value)) < 2 || isTransportLink(l.Value) {
				continue
			}
			old, repl = append(old, l.Raw), append(repl, keyOutbound+" "+l.Value)
		}
	}
	if len(old) == 0 {
		return nil
	}
	return fmt.Errorf("%s uses setting names this version no longer reads. Replace these lines:\n  %s\nwith:\n  %s",
		boardConfigPath(dataDir), strings.Join(old, "\n  "), strings.Join(repl, "\n  "))
}
