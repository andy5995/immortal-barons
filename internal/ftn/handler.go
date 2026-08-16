package ftn

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// Queued records one packet handed to the FTN mailer.
type Queued struct {
	PacketPath string
	NextHop    string
	Address    Address
	Message    string
}

// Result describes one outbound scan.
type Result struct {
	Queued         []Queued
	LostClaimRaces int
}

// Run scans every configured outbound directory. A packet is first moved into
// that directory's fido subdirectory; only the process which successfully
// claims that move creates its file-attach netmail message.
func Run(dataDir string) (Result, error) {
	board, err := store.LoadConfig(dataDir)
	if err != nil {
		return Result{}, err
	}
	transport, err := LoadConfig(dataDir)
	if err != nil {
		return Result{}, err
	}
	nodes, err := store.ParseNodeList(filepath.Join(dataDir, store.NodeListFile))
	if err != nil {
		return Result{}, err
	}
	routes, err := store.ParseRouteFile(filepath.Join(dataDir, store.RouteFile))
	if err != nil && !os.IsNotExist(err) {
		return Result{}, err
	}
	world := &game.World{Config: board, LeagueNodes: nodes, Routes: routes}
	originNode := nodeByNumber(nodes, world.NodeNumber(board.BoardID))
	if originNode == nil {
		return Result{}, fmt.Errorf("this board %q is not in %s", board.BoardID, store.NodeListFile)
	}
	origin, err := ParseAddress(originNode.Address)
	if err != nil {
		return Result{}, fmt.Errorf("this board %q: %w", board.BoardID, err)
	}

	var result Result
	for _, dir := range outboundDirectories(board) {
		if err := scanDirectory(dir, transport, origin, world, nodes, &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

func outboundDirectories(cfg game.Config) []string {
	seen := map[string]bool{}
	var dirs []string
	add := func(dir string) {
		clean := filepath.Clean(dir)
		if !seen[clean] {
			seen[clean] = true
			dirs = append(dirs, clean)
		}
	}
	add(cfg.Outbound())
	var numbers []int
	for number := range cfg.OutboundDirs {
		numbers = append(numbers, number)
	}
	slices.Sort(numbers)
	for _, number := range numbers {
		if dir, ok := cfg.OutboundLink(number); ok {
			add(dir)
		}
	}
	return dirs
}

func scanDirectory(dir string, transport Config, origin Address, world *game.World, nodes []game.LeagueNode, result *Result) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	fidoDir := filepath.Join(dir, "fido")
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != store.PacketExt {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := os.MkdirAll(fidoDir, 0o755); err != nil {
			return err
		}
		source := filepath.Join(dir, entry.Name())
		claimed := filepath.Join(fidoDir, entry.Name())
		won, err := claimPacket(source, claimed)
		if err != nil {
			return fmt.Errorf("claim %s: %w", source, err)
		}
		if !won {
			result.LostClaimRaces++
			continue
		}
		queued, err := queueClaimed(claimed, transport, origin, world, nodes)
		if err != nil {
			if restoreErr := restorePacket(claimed, source); restoreErr != nil {
				return fmt.Errorf("queue %s: %v (packet remains at %s: rollback failed: %v)", entry.Name(), err, claimed, restoreErr)
			}
			return fmt.Errorf("queue %s: %w", entry.Name(), err)
		}
		result.Queued = append(result.Queued, queued...)
	}
	return nil
}

// claimPacket uses link+unlink as an atomic, no-replace move. The fido target
// is beneath the source directory and therefore on the same filesystem. If two
// copies of the handler race, only one can create the destination link.
func claimPacket(source, destination string) (bool, error) {
	if err := os.Link(source, destination); err != nil {
		if os.IsExist(err) || os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if err := os.Remove(source); err != nil && !os.IsNotExist(err) {
		if rollbackErr := os.Remove(destination); rollbackErr != nil {
			return false, fmt.Errorf("remove source: %v (remove claim: %v)", err, rollbackErr)
		}
		return false, err
	}
	return true, nil
}

func restorePacket(claimed, source string) error {
	if err := os.Link(claimed, source); err != nil {
		return err
	}
	if err := os.Remove(claimed); err != nil {
		return err
	}
	return nil
}

func queueClaimed(path string, transport Config, origin Address, world *game.World, nodes []game.LeagueNode) ([]Queued, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var packet game.Packet
	if err := json.Unmarshal(data, &packet); err != nil {
		return nil, err
	}
	destinationNumber := packet.ToNode
	if destinationNumber == 0 && packet.ToBoard != "" {
		destinationNumber = world.NodeNumber(packet.ToBoard)
	}
	if destinationNumber == 0 {
		return queueBroadcast(path, transport, origin, world, nodes)
	}
	destination := nodeByNumber(nodes, destinationNumber)
	if destination == nil {
		return nil, fmt.Errorf("destination node %d is not in %s", destinationNumber, store.NodeListFile)
	}
	nextHopName := world.NextHop(destination.Name)
	nextHop := nodeByNumber(nodes, world.NodeNumber(nextHopName))
	if nextHop == nil {
		return nil, fmt.Errorf("next hop %q is not in %s", nextHopName, store.NodeListFile)
	}
	queued, err := queueForNode(path, transport, origin, *nextHop)
	if err != nil {
		return nil, fmt.Errorf("next hop %q: %w", nextHop.Name, err)
	}
	return []Queued{queued}, nil
}

// queueBroadcast fans one unaddressed packet out to every other board. Each
// .msg needs its own pathname because KFS may remove an attachment as soon as
// that message is sent. Broadcasts go directly to their recipients: assigning
// ToNode here to route them would change the already-signed packet bytes.
func queueBroadcast(path string, transport Config, origin Address, world *game.World, nodes []game.LeagueNode) ([]Queued, error) {
	mine := world.NodeNumber(world.Config.BoardID)
	var recipients []game.LeagueNode
	for _, node := range nodes {
		if node.Number != mine {
			recipients = append(recipients, node)
		}
	}
	if len(recipients) == 0 {
		return nil, fmt.Errorf("broadcast has no other board in %s", store.NodeListFile)
	}

	paths := []string{path}
	for _, node := range recipients[1:] {
		copyPath := broadcastCopyPath(path, node.Number)
		if err := os.Link(path, copyPath); err != nil {
			removeFiles(paths[1:])
			return nil, fmt.Errorf("copy broadcast for node %d: %w", node.Number, err)
		}
		paths = append(paths, copyPath)
	}
	var queued []Queued
	for i, node := range recipients {
		item, err := queueForNode(paths[i], transport, origin, node)
		if err != nil {
			for _, prior := range queued {
				os.Remove(prior.Message)
			}
			removeFiles(paths[1:])
			return nil, fmt.Errorf("broadcast to %q: %w", node.Name, err)
		}
		queued = append(queued, item)
	}
	return queued, nil
}

func broadcastCopyPath(path string, node int) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + fmt.Sprintf("-node-%d", node) + ext
}

func queueForNode(path string, transport Config, origin Address, node game.LeagueNode) (Queued, error) {
	address, err := ParseAddress(node.Address)
	if err != nil {
		return Queued{}, err
	}
	attached, err := filepath.Abs(path)
	if err != nil {
		return Queued{}, err
	}
	message, err := createFileAttach(transport.NetmailDir, attached, origin, address, transport.Binkley)
	if err != nil {
		return Queued{}, err
	}
	return Queued{PacketPath: attached, NextHop: node.Name, Address: address, Message: message}, nil
}

func removeFiles(paths []string) {
	for _, path := range paths {
		os.Remove(path)
	}
}

func nodeByNumber(nodes []game.LeagueNode, number int) *game.LeagueNode {
	for i := range nodes {
		if nodes[i].Number == number {
			return &nodes[i]
		}
	}
	return nil
}
