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
// Positional cannot express the per-neighbour links at all, and a field left
// blank in one silently shifts every field after it — which is most of what
// BRE's own troubleshooting section is about. Matched case-insensitively;
// written in this casing.
const (
	keyBoardID  = "BoardID"
	keyLeague   = "LeagueNumber"
	keyInbound  = "Inbound"
	keyOutbound = "Outbound"
	keyLink     = "Link"
	keyLottery  = "Lottery"
	keyBulletin = "BulletinDir"
	keyPirate   = "PirateNews"
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
			cfg.OutboundDir = value
		case strings.EqualFold(key, keyBulletin):
			cfg.BulletinDir = value
		case strings.EqualFold(key, keyLottery):
			if b, err := strconv.ParseBool(boolWord(value)); err == nil {
				cfg.Lottery = b
			}
		case strings.EqualFold(key, keyPirate):
			if b, err := strconv.ParseBool(boolWord(value)); err == nil {
				cfg.PirateNews = b
			}
		case strings.EqualFold(key, keyLink):
			node, dir, ok := strings.Cut(value, " ")
			n, err := strconv.Atoi(strings.TrimSpace(node))
			if !ok || err != nil {
				continue
			}
			if cfg.OutboundDirs == nil {
				cfg.OutboundDirs = map[int]string{}
			}
			cfg.OutboundDirs[n] = strings.TrimSpace(dir)
		}
	}
	return sc.Err()
}

// BoardConfigText renders bbs.cfg as the game would like to see it, comments
// and all. It is printed for the sysop to paste rather than written: this file
// is the board's own identity, a sysop edits it by hand more than anything else
// in the game, and a rules reset that rewrote it put four correct settings back
// to defaults (#152). The one exception is the migration below, which only ever
// creates the file when it is missing.
func BoardConfigText(cfg game.Config) string {
	var b strings.Builder
	b.WriteString("# This board's own settings. The game's rules are in config.json,\n")
	b.WriteString("# which the League Coordinator's broadcast may rewrite; nothing here\n")
	b.WriteString("# is ever changed from outside. Edit freely, one setting per line.\n\n")

	b.WriteString("# The name this board is known by in the league. It must match the\n")
	b.WriteString("# roster exactly, spelling and spacing included.\n")
	fmt.Fprintf(&b, "%s %s\n\n", keyBoardID, cfg.BoardID)

	b.WriteString("# The number this league runs under, 1-999. Ask your Coordinator for\n")
	b.WriteString("# it; a board playing in a league needs one. Left at 0 this board\n")
	b.WriteString("# would take every league's packets as its own, so the transport\n")
	b.WriteString("# refuses to run until it is set.\n")
	fmt.Fprintf(&b, "%s %d\n\n", keyLeague, cfg.LeagueNumber)

	b.WriteString("# Where packets from the other boards arrive, and where this board\n")
	b.WriteString("# writes packets for its uplink. A path with no drive letter and no\n")
	b.WriteString("# leading slash is read as being inside the data directory.\n")
	fmt.Fprintf(&b, "%s %s\n", keyInbound, cfg.InboundDir)
	fmt.Fprintf(&b, "%s %s\n\n", keyOutbound, cfg.OutboundDir)

	b.WriteString("# Where the game writes its bulletin files: the scoreboard, today's and\n")
	b.WriteString("# yesterday's news, and the world report of the league's battles. Each is\n")
	b.WriteString("# written twice, with colour (.ans) and without (.txt), for a BBS to\n")
	b.WriteString("# show on its own bulletin menu. Leave it blank to write none.\n")
	fmt.Fprintf(&b, "%s %s\n\n", keyBulletin, cfg.BulletinDir)

	b.WriteString("# Whether this board offers the Queen's lottery: a six-letter ticket,\n")
	b.WriteString("# once a day, for 5,000 gold. Yes or no; the league does not decide it\n")
	b.WriteString("# for you.\n")
	fmt.Fprintf(&b, "%s %s\n\n", keyLottery, yesNo(cfg.Lottery))

	b.WriteString("# Whether a pirate raid's outcome is posted to the planet news, in\n")
	b.WriteString("# league and solo play alike. Yes or no; the raids themselves happen\n")
	b.WriteString("# either way, and the raider is always told how theirs went.\n")
	fmt.Fprintf(&b, "%s %s\n\n", keyPirate, yesNo(cfg.PirateNews))

	b.WriteString("# A board that forwards for its neighbours has one link per neighbour:\n")
	b.WriteString("# \"Link <node number> <directory>\". A board with nobody to forward for\n")
	b.WriteString("# needs none of these — anything with no line of its own goes to\n")
	b.WriteString("# Outbound above.\n")
	for _, n := range slices.Sorted(maps.Keys(cfg.OutboundDirs)) {
		fmt.Fprintf(&b, "%s %d %s\n", keyLink, n, cfg.OutboundDirs[n])
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
