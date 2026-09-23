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

// boolWord maps the words a sysop is likely to write to what ParseBool takes.
// The original's own configuration file spells its booleans "yes" and "no".
func boolWord(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "yes", "on":
		return "true"
	case "no", "off":
		return "false"
	}
	return v
}

func boardConfigPath(dataDir string) string { return filepath.Join(dataDir, BoardConfigFile) }

// LoadBoardConfig applies <dataDir>/bbs.cfg to cfg. A missing file leaves cfg
// alone, which is what makes the migration in LoadConfig work: values read from
// an older config.json stand until this file is written for the first time.
//
// An unknown keyword is ignored rather than refused. This file is hand-edited,
// often by someone following a newer version's documentation, and a board that
// will not start is a worse answer than a setting that does nothing.
func LoadBoardConfig(dataDir string, cfg *game.Config) error {
	f, err := os.Open(boardConfigPath(dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, rest, _ := strings.Cut(line, " ")
		value := strings.TrimSpace(rest)
		switch {
		case strings.EqualFold(key, keyBoardID):
			cfg.BoardID = value
		case strings.EqualFold(key, keyBBSName):
			cfg.BBSName = value
		case strings.EqualFold(key, keyBoardURL):
			cfg.BoardURL = value
		case strings.EqualFold(key, keyBullURL):
			cfg.BulletinURL = value
		case strings.EqualFold(key, keyLeague):
			// Out of range is left unset (0, "never set") rather than failing the
			// whole import, the way the roster parser drops one bad node line
			// instead of the file. game.MaxLeagueNumber was declared for this
			// bound and nothing had applied it, so any number at all was taken —
			// and the league number reaches packet filenames.
			if n, err := strconv.Atoi(value); err == nil && n >= 1 && n <= game.MaxLeagueNumber {
				cfg.LeagueNumber = n
			}
		case strings.EqualFold(key, keyInbound):
			cfg.InboundDir = value
		case strings.EqualFold(key, keyOutbound):
			// "GameOutbound <node> <dir>" is one neighbor's own directory (#106);
			// a bare "GameOutbound <dir>" is everyone else's. A directory whose
			// name is a number still works on its own, since only a number
			// followed by more is read as a node.
			if n, dir, ok := perNodeDir(value); ok {
				if cfg.OutboundDirs == nil {
					cfg.OutboundDirs = map[int]string{}
				}
				cfg.OutboundDirs[n] = dir
				continue
			}
			cfg.OutboundDir = value
		case strings.EqualFold(key, keyBulletin):
			cfg.BulletinDir = value
		case strings.EqualFold(key, keyLottery):
			if b, err := strconv.ParseBool(boolWord(value)); err == nil {
				cfg.Lottery = b
			}
		case strings.EqualFold(key, keyOnFault):
			cfg.OnFault = value
		case strings.EqualFold(key, keyPirate):
			if b, err := strconv.ParseBool(boolWord(value)); err == nil {
				cfg.PirateNews = b
			}
		}
	}
	return sc.Err()
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

// perNodeDir splits "<node> <dir>" when value starts with a node number and has
// more after it.
func perNodeDir(value string) (int, string, bool) {
	node, dir, ok := strings.Cut(value, " ")
	if !ok {
		return 0, "", false
	}
	n, err := strconv.Atoi(node)
	dir = strings.TrimSpace(dir)
	if err != nil || n < 1 || n > game.MaxNodeNumber || dir == "" {
		return 0, "", false
	}
	return n, dir, true
}

// linkModes are the words that make a Link line the FTN transport's. A Link
// line without one is the per-neighbor directory as this file spelled it
// before that became "GameOutbound <node> <dir>".
var linkModes = []string{"attach", "obox", "bso"}

// LegacyBoardRefusal reports bbs.cfg lines written under names this version no
// longer reads -- Inbound, Outbound, and the two-field "Link <node> <dir>" --
// with the lines that replace them, ready to paste. Nil when there are none.
//
// Refused rather than read under both names: an ignored Inbound line would put
// the board back on the default directory without a word, and accepting the
// old spelling forever keeps alive the confusion the rename removed (#241).
// The values are copied byte for byte, so a Windows path keeps its backslashes.
func LegacyBoardRefusal(dataDir string) error {
	path := boardConfigPath(dataDir)
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var old, repl []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		key, rest, _ := strings.Cut(line, " ")
		value := strings.TrimSpace(rest)
		switch {
		case strings.EqualFold(key, "Inbound"):
			old, repl = append(old, line), append(repl, keyInbound+" "+value)
		case strings.EqualFold(key, "Outbound"):
			old, repl = append(old, line), append(repl, keyOutbound+" "+value)
		case strings.EqualFold(key, "Link"):
			fields := strings.Fields(value)
			if len(fields) < 2 || slices.ContainsFunc(linkModes, func(m string) bool {
				return strings.EqualFold(m, fields[1])
			}) {
				continue
			}
			old, repl = append(old, line), append(repl, keyOutbound+" "+value)
		}
	}
	if len(old) == 0 {
		return nil
	}
	return fmt.Errorf("%s uses setting names this version no longer reads. Replace these lines:\n  %s\nwith:\n  %s",
		path, strings.Join(old, "\n  "), strings.Join(repl, "\n  "))
}
