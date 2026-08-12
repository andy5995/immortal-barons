package store

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// ParseNodeList reads a BRNODES-style league node list. Each node is a block of
// six lines — number, name, FidoNet address, city, state, country — separated
// by one or more blank lines (matching docs/brnodes.sam in the original), with
// an OPTIONAL seventh line carrying that board's packet-signing public key
// (#118). Nodes with an unparseable number are skipped.
//
// The key line is seventh rather than woven in so a roster written before it
// existed still parses unchanged, and one written with it is ignored gracefully
// by an older board: the block is read by index, and index six is simply absent.
//
// Every one of the six lines must carry a value. The format cannot express an
// empty field: a blank line is what ends a block, so a roster written with one
// missing (no city, say) loses that board here. That is deliberate — the loss
// happens on the Coordinator's own board, where they notice, rather than
// silently on everyone else's.
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
			n, hosts, err := parseNodeNumber(block[0])
			if err == nil {
				var key string
				if len(block) >= 7 {
					key = strings.TrimSpace(block[6])
				}
				nodes = append(nodes, game.LeagueNode{
					Number:    n,
					Hosts:     hosts,
					Name:      strings.TrimSpace(block[1]),
					Address:   strings.TrimSpace(block[2]),
					City:      strings.TrimSpace(block[3]),
					State:     strings.TrimSpace(block[4]),
					Country:   strings.TrimSpace(block[5]),
					PublicKey: key,
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

// parseNodeNumber reads a roster block's first line: a node number, optionally
// followed by "HOST" and the numbers this board forwards for ("2 HOST 3 4 8").
func parseNodeNumber(line string) (int, []int, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return 0, nil, strconv.ErrSyntax
	}
	n, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, nil, err
	}
	var hosts []int
	for i := 1; i < len(fields); i++ {
		if strings.EqualFold(fields[i], "HOST") {
			continue
		}
		if h, err := strconv.Atoi(fields[i]); err == nil {
			hosts = append(hosts, h)
		}
	}
	return n, hosts, nil
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

// WriteNodeList writes the roster back in the same block format ParseNodeList
// reads. A member board calls this after adopting the Coordinator's broadcast
// (#64), so the roster survives a restart — World.LeagueNodes is not part of the
// saved world, it is loaded from this file.
func WriteNodeList(path string, nodes []game.LeagueNode) error {
	var b strings.Builder
	for i, n := range nodes {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%d", n.Number)
		if len(n.Hosts) > 0 {
			b.WriteString(" HOST")
			for _, h := range n.Hosts {
				fmt.Fprintf(&b, " %d", h)
			}
		}
		fmt.Fprintf(&b, "\n%s\n%s\n%s\n%s\n%s\n", n.Name, n.Address, n.City, n.State, n.Country)
		// Only written when set, so a keyless roster round-trips byte for byte.
		if n.PublicKey != "" {
			fmt.Fprintf(&b, "%s\n", n.PublicKey)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
