package store

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// Check is one line of a setup report: what was examined, whether it is
// usable, and what to do when it is not.
type Check struct {
	Name   string
	OK     bool
	Detail string
}

// CheckBoardInRoster reports whether this board's own name is in the league
// roster. The name is typed twice, in two files, and compared byte for byte at
// transport time — so a missed capital fails three steps later, on the one
// board that is wrong (#154). A roster this board has not been sent yet is not
// an error: a member is told to reset before the Coordinator's first packet. A
// roster that is there but unusable is a different answer from an absent one,
// and saying so here is the whole point of checking at setup.
func CheckBoardInRoster(dataDir, boardID string) error {
	path := filepath.Join(dataDir, NodeListFile)
	nodes, err := ParseNodeList(path)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return fmt.Errorf("%s: %w", path, err)
	case len(nodes) == 0:
		return fmt.Errorf("%s lists no boards", path)
	}
	names := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.Name == boardID {
			return nil
		}
		names = append(names, n.Name)
	}
	if near, ok := nearestName(boardID, names); ok {
		return fmt.Errorf("this board %q is not in %s. The closest entry is %q — the two must match exactly, spelling and spacing included",
			boardID, NodeListFile, near)
	}
	return fmt.Errorf("this board %q is not in %s, which lists: %s",
		boardID, NodeListFile, strings.Join(names, ", "))
}

// nearestName picks the roster entry a misspelled board name most likely meant.
// Most failures here are a capital or a stray space, so an exact match after
// folding case and collapsing whitespace wins outright; otherwise a name is
// only offered when it is closer than a quarter of its own length, which keeps
// two genuinely different boards from being suggested for each other.
func nearestName(want string, names []string) (string, bool) {
	folded := foldName(want)
	for _, name := range names {
		if foldName(name) == folded {
			return name, true
		}
	}
	best, bestDistance := "", 0
	for _, name := range names {
		d := editDistance(folded, foldName(name))
		limit := len([]rune(name))/4 + 1
		if d > limit {
			continue
		}
		if best == "" || d < bestDistance {
			best, bestDistance = name, d
		}
	}
	return best, best != ""
}

func foldName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// editDistance is Levenshtein distance over runes, on two board names — short
// enough that the full matrix is not worth avoiding.
func editDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, min(curr[j-1]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}

// Checkup examines everything a league board needs to be set up correctly and
// returns one Check per item, so a sysop sees all of it at once rather than one
// failure per transport run (#154).
func Checkup(cfg game.Config) []Check {
	var checks []Check
	add := func(name string, ok bool, detail string) {
		checks = append(checks, Check{Name: name, OK: ok, Detail: detail})
	}

	if !cfg.InterBBSEnabled() {
		add("Inter-BBS play", false, "off — this board plays alone. Run -ibbs-reset to join a league")
		return checks
	}
	add("Inter-BBS play", true, "on")

	rosterPath := filepath.Join(cfg.DataDir, NodeListFile)
	nodes, problems, err := ParseNodeListReport(rosterPath)
	switch {
	case err != nil:
		add("League roster", false, fmt.Sprintf("%s: %v", rosterPath, err))
	case len(nodes) == 0:
		add("League roster", false, rosterPath+" lists no boards")
	default:
		add("League roster", true, fmt.Sprintf("%d board(s) in %s", len(nodes), rosterPath))
	}
	// A skipped block is a board this league cannot reach and whose packets are
	// refused, so it is reported on its own line rather than left to be inferred
	// from a board count (#180).
	if len(problems) > 0 {
		add("Roster entries", false, fmt.Sprintf("%s in %s", strings.Join(problems, "; "), rosterPath))
	}
	switch {
	case cfg.BoardID == "":
		add("Board name", false, "not set. Give -board-id when you run -ibbs-reset")
	case len(nodes) == 0:
		// The roster line above already says why. Repeating it here would
		// report one problem as two.
		add("Board name", false, fmt.Sprintf("%q — cannot be checked until the roster loads", cfg.BoardID))
	default:
		if err := CheckBoardInRoster(cfg.DataDir, cfg.BoardID); err != nil {
			add("Board name", false, err.Error())
		} else {
			add("Board name", true, fmt.Sprintf("%q", cfg.BoardID))
		}
	}

	if cfg.LeagueNumber == 0 {
		add("League number", true, "not set — fine unless two leagues share an inbound directory")
	} else {
		add("League number", true, fmt.Sprint(cfg.LeagueNumber))
	}

	add("Inbound directory", dirUsable(cfg.Inbound(), false), cfg.Inbound())
	add("Outbound directory", dirUsable(cfg.Outbound(), true), cfg.Outbound())

	// Test the keys the way loadLeagueKeys does — decode them and check the
	// length — rather than that a file is present. A coord.pub that does not
	// decode to a full key loads as no key at all and every league order is
	// refused, so a file-exists check answers "ok" to the one question the sysop
	// is asking and sends them looking somewhere else.
	coordPub := filepath.Join(cfg.DataDir, CoordPubFile)
	coordKey := filepath.Join(cfg.DataDir, CoordKeyFile)
	switch {
	case fileExists(coordKey):
		if readHexKey(coordKey, ed25519.PrivateKeySize) == nil {
			add("Coordinator key", false, CoordKeyFile+" is there but does not read as a key, so this board cannot sign league orders and sends them unsigned")
		} else {
			add("Coordinator key", true, "this board is the League Coordinator ("+CoordKeyFile+")")
		}
	case fileExists(coordPub):
		if readHexKey(coordPub, ed25519.PublicKeySize) == nil {
			add("Coordinator key", false, CoordPubFile+" is there but does not read as a key, so league orders will be refused. Run -coord-key again with the key the Coordinator gave you")
		} else {
			add("Coordinator key", true, "recorded in "+CoordPubFile)
		}
	default:
		add("Coordinator key", false, "not recorded — league orders will be refused. Run -coord-key with the key the Coordinator gave you")
	}

	boardKey := filepath.Join(cfg.DataDir, BoardKeyFile)
	switch {
	case !fileExists(boardKey):
		add("Board signing key", true, "none — optional; -gen-board-key creates one")
	case readHexKey(boardKey, ed25519.PrivateKeySize) == nil:
		add("Board signing key", false, BoardKeyFile+" is there but does not read as a key, so this board's packets go out unsigned")
	default:
		add("Board signing key", true, BoardKeyFile)
	}

	return checks
}

// dirUsable reports whether a packet directory is there and can be used. Only
// the outbound one is written by the game; an inbound directory belongs to the
// mailer and needs reading alone.
func dirUsable(dir string, write bool) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	if !write {
		return true
	}
	probe := filepath.Join(dir, ".barons-write-probe")
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(probe)
	return true
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
