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
)

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
			if n, err := strconv.Atoi(value); err == nil {
				cfg.LeagueNumber = n
			}
		case strings.EqualFold(key, keyInbound):
			cfg.InboundDir = value
		case strings.EqualFold(key, keyOutbound):
			cfg.OutboundDir = value
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

// SaveBoardConfig writes the per-board settings to <dataDir>/bbs.cfg. The
// comments are written every time: this is the file a sysop opens in an editor,
// and the game is the only thing that knows what belongs in it.
func SaveBoardConfig(cfg game.Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# This board's own settings. The game's rules are in config.json,\n")
	b.WriteString("# which the League Coordinator's broadcast may rewrite; nothing here\n")
	b.WriteString("# is ever changed from outside. Edit freely, one setting per line.\n\n")

	b.WriteString("# The name this board is known by in the league. It must match the\n")
	b.WriteString("# roster exactly, spelling and spacing included.\n")
	fmt.Fprintf(&b, "%s %s\n\n", keyBoardID, cfg.BoardID)

	b.WriteString("# The Coordinator's number for this league, 1-999, or 0 if they have\n")
	b.WriteString("# not given you one. It keeps two leagues apart when they share an\n")
	b.WriteString("# inbound directory.\n")
	fmt.Fprintf(&b, "%s %d\n\n", keyLeague, cfg.LeagueNumber)

	b.WriteString("# Where packets from the other boards arrive, and where this board\n")
	b.WriteString("# writes packets for its uplink. A path with no drive letter and no\n")
	b.WriteString("# leading slash is read as being inside the data directory.\n")
	fmt.Fprintf(&b, "%s %s\n", keyInbound, cfg.InboundDir)
	fmt.Fprintf(&b, "%s %s\n\n", keyOutbound, cfg.OutboundDir)

	b.WriteString("# A board that forwards for its neighbours has one link per neighbour:\n")
	b.WriteString("# \"Link <node number> <directory>\". A board with nobody to forward for\n")
	b.WriteString("# needs none of these — anything with no line of its own goes to\n")
	b.WriteString("# Outbound above.\n")
	for _, n := range slices.Sorted(maps.Keys(cfg.OutboundDirs)) {
		fmt.Fprintf(&b, "%s %d %s\n", keyLink, n, cfg.OutboundDirs[n])
	}

	tmp := boardConfigPath(cfg.DataDir) + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, boardConfigPath(cfg.DataDir))
}
