package store

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// ParseNodeList reads a BRNODES-style league node list. Each node is a block of
// six lines — number, name, FidoNet address, city, state, country — separated
// by one or more blank lines (matching docs/brnodes.sam in the original). Nodes
// with an unparseable number are skipped.
func ParseNodeList(path string) ([]game.LeagueNode, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var nodes []game.LeagueNode
	var block []string
	flush := func() {
		if len(block) >= 6 {
			n, err := strconv.Atoi(strings.TrimSpace(block[0]))
			if err == nil {
				nodes = append(nodes, game.LeagueNode{
					Number:  n,
					Name:    strings.TrimSpace(block[1]),
					Address: strings.TrimSpace(block[2]),
					City:    strings.TrimSpace(block[3]),
					State:   strings.TrimSpace(block[4]),
					Country: strings.TrimSpace(block[5]),
				})
			}
		}
		block = nil
	}

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		block = append(block, line)
	}
	flush()
	return nodes, sc.Err()
}

// BoardConfig is a board's inter-BBS configuration (BRE's BBS.CFG): the
// seven fields identify this board in the league and tell the game where its
// packet directories are.
type BoardConfig struct {
	Sysop      string
	PlanetName string
	Address    string
	InboundDir string
	NetmailDir string
	League     int
	Mailer     string
}

// ParseBoardConfig reads a BBS.CFG-style file: seven lines, in order — sysop
// name, planet name, node address, inbound-file dir, netmail dir, league
// number, mailer. Missing trailing lines are left blank.
func ParseBoardConfig(path string) (BoardConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return BoardConfig{}, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, strings.TrimSpace(sc.Text()))
		if len(lines) == 7 {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return BoardConfig{}, err
	}
	get := func(i int) string {
		if i < len(lines) {
			return lines[i]
		}
		return ""
	}
	league, _ := strconv.Atoi(get(5))
	return BoardConfig{
		Sysop:      get(0),
		PlanetName: get(1),
		Address:    get(2),
		InboundDir: get(3),
		NetmailDir: get(4),
		League:     league,
		Mailer:     get(6),
	}, nil
}
