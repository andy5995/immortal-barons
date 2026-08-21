package ftn

import (
	"encoding/json"
	"fmt"
	"io"
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
	Queued []Queued
	// Warnings report a configuration that works now but is close to failing.
	Warnings []string
}

// Run scans every configured outbound directory. A packet is first moved into
// that directory's fido subdirectory; only the process which successfully
// claims that move creates its file-attach netmail message.
func Run(dataDir string) (Result, error) {
	board, err := store.LoadConfig(dataDir)
	if err != nil {
		return Result{}, err
	}
	// This lock serializes only copies of barons-ftn. It never blocks the game;
	// WriteOutbox publishes complete packets atomically under their .brp names.
	adapterLock, err := store.LockPath(filepath.Join(board.DataDir, "barons-ftn.lock"), true)
	if err != nil {
		return Result{}, err
	}
	defer adapterLock.Release()
	transport, err := LoadConfig(dataDir)
	if err != nil {
		return Result{}, err
	}
	nodes, err := store.ParseNodeList(filepath.Join(dataDir, store.NodeListFile))
	if err != nil {
		return Result{}, err
	}
	world := &game.World{Config: board, LeagueNodes: nodes}
	originNode := nodeByNumber(nodes, world.NodeNumber(board.BoardID))
	if originNode == nil {
		if err := store.CheckBoardInRoster(board.DataDir, board.BoardID); err != nil {
			return Result{}, err
		}
		return Result{}, fmt.Errorf("this board %q is not in %s", board.BoardID, store.NodeListFile)
	}
	origin, err := ParseAddress(originNode.Address)
	if err != nil {
		return Result{}, fmt.Errorf("this board %q: %w", board.BoardID, err)
	}

	dirs := outboundDirectories(board)
	candidates, margin, err := preflightDirectories(dirs, transport, world, nodes)
	if err != nil {
		return Result{}, err
	}
	var result Result
	if len(candidates) > 0 && margin < subjectMarginBytes {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"attachment subjects have %d byte(s) to spare in the FTN Type-2 field; %s",
			margin, subjectAdvice(transport.SubjectMode)))
	}
	for _, candidate := range candidates {
		if err := os.MkdirAll(filepath.Dir(candidate.claimed), 0o755); err != nil {
			return result, err
		}
		won, err := claimPacket(candidate.source, candidate.claimed)
		if err != nil {
			return result, fmt.Errorf("claim %s: %w", candidate.source, err)
		}
		if !won {
			continue
		}
		queued, err := queueClaimed(candidate.claimed, candidate.packet, transport, origin, world, nodes)
		if err != nil {
			if restoreErr := restorePacket(candidate.claimed, candidate.source); restoreErr != nil {
				return result, fmt.Errorf("queue %s: %v (packet remains at %s: rollback failed: %v)",
					filepath.Base(candidate.source), err, candidate.claimed, restoreErr)
			}
			return result, fmt.Errorf("queue %s: %w", filepath.Base(candidate.source), err)
		}
		result.Queued = append(result.Queued, queued...)
	}
	return result, nil
}

type packetCandidate struct {
	source  string
	claimed string
	packet  game.Packet
}

// preflightDirectories checks every pathname that this run could put in an
// FTN subject before it moves even the first packet. This makes an overlong
// outbound path a configuration error instead of a partial handoff. It returns
// the exact snapshot it checked, so a packet atomically published while the
// helper is running waits for the next run rather than bypassing preflight.
// The second return is the smallest number of unused subject bytes any of the
// checked pathnames left.
func preflightDirectories(dirs []string, transport Config, world *game.World, nodes []game.LeagueNode) ([]packetCandidate, int, error) {
	var candidates []packetCandidate
	margin := type2SubjectSize
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, 0, err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != store.PacketExt {
				continue
			}
			info, err := entry.Info()
			if os.IsNotExist(err) {
				continue // another helper claimed it after ReadDir
			}
			if err != nil {
				return nil, 0, err
			}
			if !info.Mode().IsRegular() {
				continue
			}
			source := filepath.Join(dir, entry.Name())
			claimed := transport.attachPath(dir, entry.Name())
			packet, found, spare, err := preflightPacket(source, claimed, transport, world, nodes)
			if err != nil {
				return nil, 0, fmt.Errorf("preflight %s: %w", source, err)
			}
			if found {
				candidates = append(candidates, packetCandidate{source: source, claimed: claimed, packet: packet})
				margin = min(margin, spare)
			}
		}
	}
	return candidates, margin, nil
}

func preflightPacket(source, claimed string, transport Config, world *game.World, nodes []game.LeagueNode) (game.Packet, bool, int, error) {
	data, err := os.ReadFile(source)
	if os.IsNotExist(err) {
		return game.Packet{}, false, 0, nil // another helper claimed it
	}
	if err != nil {
		return game.Packet{}, false, 0, err
	}
	var packet game.Packet
	if err := json.Unmarshal(data, &packet); err != nil {
		return game.Packet{}, false, 0, err
	}
	attached, err := filepath.Abs(claimed)
	if err != nil {
		return game.Packet{}, false, 0, err
	}
	_, spare, err := fileAttachSubject(transport, attached)
	if err != nil {
		return game.Packet{}, false, 0, err
	}
	destinationNumber := packet.ToNode
	if destinationNumber == 0 && packet.ToBoard != "" {
		destinationNumber = world.NodeNumber(packet.ToBoard)
	}
	if destinationNumber != 0 {
		return packet, true, spare, nil
	}
	recipients := broadcastRecipients(world, nodes)
	if len(recipients) == 0 {
		return game.Packet{}, false, 0, fmt.Errorf("broadcast has no other board in %s", store.NodeListFile)
	}
	for _, node := range recipients[1:] {
		copyPath := broadcastCopyPath(attached, node.Number)
		_, copySpare, err := fileAttachSubject(transport, copyPath)
		if err != nil {
			return game.Packet{}, false, 0, fmt.Errorf("broadcast attachment for node %d: %w", node.Number, err)
		}
		spare = min(spare, copySpare)
	}
	return packet, true, spare, nil
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

// claimPacket uses the source rename as the ownership claim. Run's adapter-only
// lock gives same-path rename one portable winner; an existing destination is
// never overwritten.
func claimPacket(source, destination string) (bool, error) {
	if _, err := os.Lstat(destination); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.Rename(source, destination); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func restorePacket(claimed, source string) error {
	if _, err := os.Lstat(source); err == nil {
		return fmt.Errorf("source path already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(claimed, source)
}

func queueClaimed(path string, packet game.Packet, transport Config, origin Address, world *game.World, nodes []game.LeagueNode) ([]Queued, error) {
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
	recipients := broadcastRecipients(world, nodes)
	if len(recipients) == 0 {
		return nil, fmt.Errorf("broadcast has no other board in %s", store.NodeListFile)
	}

	paths := []string{path}
	for _, node := range recipients[1:] {
		copyPath := broadcastCopyPath(path, node.Number)
		if err := copyFileExclusive(path, copyPath); err != nil {
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

func broadcastRecipients(world *game.World, nodes []game.LeagueNode) []game.LeagueNode {
	mine := world.NodeNumber(world.Config.BoardID)
	var recipients []game.LeagueNode
	for _, node := range nodes {
		if node.Number != mine {
			recipients = append(recipients, node)
		}
	}
	return recipients
}

func copyFileExclusive(source, destination string) (err error) {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if closeErr := out.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if !ok || err != nil {
			os.Remove(destination)
		}
	}()
	_, err = io.Copy(out, in)
	ok = err == nil
	return err
}

func broadcastCopyPath(path string, node int) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + fmt.Sprintf("-n%d", node) + ext
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
	message, err := createFileAttach(transport, attached, origin, address)
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
