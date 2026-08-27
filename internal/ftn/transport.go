package ftn

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

const (
	spoolDir      = "ftn-spool"
	outSpoolDir   = "out"
	badSpoolDir   = "bad"
	attachSpool   = "attach"
	batchPlanFile = "batch.json"
)

type batchPlan struct {
	ID      string        `json:"id"`
	Targets []batchTarget `json:"targets"`
}

type batchTarget struct {
	Node       int      `json:"node"`
	Name       string   `json:"name"`
	Address    string   `json:"address"`
	Mode       LinkMode `json:"mode"`
	Directory  string   `json:"directory"`
	QueueDir   string   `json:"queue_dir,omitempty"`
	Flavour    string   `json:"flavour,omitempty"`
	Alias      string   `json:"alias"`
	BundleFile string   `json:"bundle_file"`
	Message    string   `json:"message,omitempty"`
	Done       bool     `json:"done"`
}

// RunOut claims one game-outbound snapshot and hands each next hop one opaque
// transport bundle. Existing incomplete batches are resumed first. Attach and
// obox publication is immutable; BSO may merge while it owns the peer's .bsy.
func RunOut(dataDir string) (Result, error) {
	board, transport, nodes, world, origin, adapterLock, err := transportContext(dataDir)
	if err != nil {
		return Result{}, err
	}
	defer adapterLock.Release()
	root := filepath.Join(dataDir, spoolDir, outSpoolDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Result{}, err
	}
	var result Result
	if err := processPendingBatches(root, dataDir, transport, world, nodes, origin, &result); err != nil {
		return result, err
	}
	batch, err := claimOutboundBatch(root, board)
	if err != nil {
		return result, err
	}
	if batch != "" {
		if err := processBatch(batch, dataDir, transport, world, nodes, origin, &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

func transportContext(dataDir string) (game.Config, Config, []game.LeagueNode, *game.World, Address, *store.FileLock, error) {
	board, err := store.LoadConfig(dataDir)
	if err != nil {
		return game.Config{}, Config{}, nil, nil, Address{}, nil, err
	}
	adapterLock, err := store.LockPath(filepath.Join(board.DataDir, "barons-ftn.lock"), true)
	if err != nil {
		return game.Config{}, Config{}, nil, nil, Address{}, nil, err
	}
	fail := func(err error) (game.Config, Config, []game.LeagueNode, *game.World, Address, *store.FileLock, error) {
		adapterLock.Release()
		return game.Config{}, Config{}, nil, nil, Address{}, nil, err
	}
	transport, err := LoadConfig(dataDir)
	if err != nil {
		return fail(err)
	}
	nodes, err := store.ParseNodeList(filepath.Join(dataDir, store.NodeListFile))
	if err != nil {
		return fail(err)
	}
	world := &game.World{Config: board, LeagueNodes: nodes}
	mine := world.NodeNumber(board.BoardID)
	originNode := nodeByNumber(nodes, mine)
	if originNode == nil {
		return fail(fmt.Errorf("this board %q is not in %s", board.BoardID, store.NodeListFile))
	}
	origin, err := ParseAddress(originNode.Address)
	if err != nil {
		return fail(fmt.Errorf("this board %q: %w", board.BoardID, err))
	}
	return board, transport, nodes, world, origin, adapterLock, nil
}

func processPendingBatches(root, dataDir string, transport Config, world *game.World, nodes []game.LeagueNode, origin Address, result *Result) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := processBatch(filepath.Join(root, entry.Name()), dataDir, transport, world, nodes, origin, result); err != nil {
			return err
		}
	}
	return nil
}

func claimOutboundBatch(root string, board game.Config) (string, error) {
	id, err := randomBundleID()
	if err != nil {
		return "", err
	}
	batch := filepath.Join(root, id)
	if err := os.Mkdir(batch, 0o755); err != nil {
		return "", err
	}
	gameLock, err := store.Lock(board, true)
	if err != nil {
		os.Remove(batch)
		return "", err
	}
	claimed := 0
	for _, dir := range outboundDirectories(board) {
		entries, readErr := os.ReadDir(dir)
		if os.IsNotExist(readErr) {
			continue
		}
		if readErr != nil {
			gameLock.Release()
			return "", readErr
		}
		for _, entry := range entries {
			if entry.IsDir() || !store.IsPacketFile(entry.Name()) {
				continue
			}
			source := filepath.Join(dir, entry.Name())
			destination := filepath.Join(batch, fmt.Sprintf("packet-%06d%s", claimed, store.PacketExt))
			if err := os.Rename(source, destination); err != nil {
				gameLock.Release()
				return "", fmt.Errorf("claim %s: %w", source, err)
			}
			claimed++
		}
	}
	if err := gameLock.Release(); err != nil {
		return "", err
	}
	if claimed == 0 {
		os.Remove(batch)
		return "", nil
	}
	return batch, nil
}

func processBatch(batch, dataDir string, transport Config, world *game.World, nodes []game.LeagueNode, origin Address, result *Result) error {
	planPath := filepath.Join(batch, batchPlanFile)
	plan, err := loadBatchPlan(planPath)
	if os.IsNotExist(err) {
		plan, err = buildBatchPlan(batch, dataDir, transport, world, nodes, result)
		if err == nil {
			err = saveBatchPlan(planPath, plan)
		}
	}
	if err != nil {
		return err
	}
	for i := range plan.Targets {
		target := &plan.Targets[i]
		if target.Done {
			continue
		}
		queued, err := publishTarget(batch, dataDir, transport, origin, *target)
		if errors.Is(err, errPeerBusy) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s is busy; its bundle remains in %s", target.Name, batch))
			continue
		}
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v; its bundle remains in %s", target.Name, err, batch))
			continue
		}
		target.Done, target.Message = true, queued.Message
		if err := saveBatchPlan(planPath, plan); err != nil {
			return err
		}
		result.Queued = append(result.Queued, queued)
	}
	for _, target := range plan.Targets {
		if !target.Done {
			return nil
		}
	}
	return os.RemoveAll(batch)
}

func loadBatchPlan(path string) (batchPlan, error) {
	var plan batchPlan
	data, err := os.ReadFile(path)
	if err != nil {
		return plan, err
	}
	err = json.Unmarshal(data, &plan)
	return plan, err
}

func saveBatchPlan(path string, plan batchPlan) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return replaceFileAtomic(path, data, 0o644)
}

func buildBatchPlan(batch, dataDir string, transport Config, world *game.World, nodes []game.LeagueNode, result *Result) (batchPlan, error) {
	entries, err := os.ReadDir(batch)
	if err != nil {
		return batchPlan{}, err
	}
	groups := map[int][]transportEntry{}
	mine := world.NodeNumber(world.Config.BoardID)
	for _, file := range entries {
		if file.IsDir() || !store.IsPacketFile(file.Name()) {
			continue
		}
		path := filepath.Join(batch, file.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return batchPlan{}, err
		}
		var packet game.Packet
		if err := json.Unmarshal(raw, &packet); err != nil {
			if moveErr := quarantineTransport(dataDir, path); moveErr != nil {
				return batchPlan{}, fmt.Errorf("packet %s: %v; quarantine: %w", file.Name(), err, moveErr)
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf("quarantined malformed outbound packet %s: %v", file.Name(), err))
			continue
		}
		te := transportEntry{Name: file.Name(), Raw: raw, Packet: packet, Route: []int{mine}, PriorHops: packet.Hops}
		targets, err := packetTargets(packet, world, nodes)
		if err != nil {
			if moveErr := quarantineTransport(dataDir, path); moveErr != nil {
				return batchPlan{}, moveErr
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf("quarantined unroutable packet %s: %v", file.Name(), err))
			continue
		}
		if packet.ToNode == 0 && packet.ToBoard == "" {
			targets = configuredFanoutTargets(transport, targets)
			if len(targets) == 0 {
				if moveErr := quarantineTransport(dataDir, path); moveErr != nil {
					return batchPlan{}, moveErr
				}
				result.Warnings = append(result.Warnings, fmt.Sprintf("quarantined broadcast packet %s: no configured fanout peers", file.Name()))
				continue
			}
			te.Covered = []int{mine}
			for _, target := range targets {
				te.Covered = appendUnique(te.Covered, target)
			}
		}
		for _, target := range targets {
			groups[target] = append(groups[target], te)
		}
	}
	plan := batchPlan{ID: filepath.Base(batch)}
	groupNodes := mapsKeys(groups)
	slices.Sort(groupNodes)
	for _, nodeNumber := range groupNodes {
		node := nodeByNumber(nodes, nodeNumber)
		if node == nil {
			return batchPlan{}, fmt.Errorf("next hop node %d is not in %s", nodeNumber, store.NodeListFile)
		}
		address, err := ParseAddress(node.Address)
		if err != nil {
			return batchPlan{}, err
		}
		link := linkFor(transport, nodeNumber)
		dir := link.Directory
		if link.Mode != LinkObox {
			dir = attachmentDirectory(dataDir, transport)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return batchPlan{}, err
		}
		alias, wrapped, err := nextAlias(dataDir, dir, world.Config.LeagueNumber, world.NodeNumber(world.Config.BoardID))
		if err != nil {
			return batchPlan{}, err
		}
		if wrapped {
			result.Warnings = append(result.Warnings, "the four-character FTN attachment counter wrapped; verify that no peer still holds aliases from the preceding cycle")
		}
		delivery := "direct"
		if link.Mode == LinkAttach {
			delivery = "attach"
		}
		body, _, err := makeBundle(world.NodeNumber(world.Config.BoardID), delivery, groups[nodeNumber])
		if err != nil {
			return batchPlan{}, err
		}
		bundleFile := fmt.Sprintf("target-%03d.bundle", nodeNumber)
		if err := writeFileAtomic(filepath.Join(batch, bundleFile), body, 0o644); err != nil {
			return batchPlan{}, err
		}
		plan.Targets = append(plan.Targets, batchTarget{
			Node: nodeNumber, Name: node.Name, Address: address.String(), Mode: link.Mode,
			Directory: dir, QueueDir: link.Directory, Flavour: link.Flavour, Alias: alias, BundleFile: bundleFile,
		})
	}
	return plan, nil
}

func configuredFanoutTargets(transport Config, candidates []int) []int {
	if len(transport.Links) == 0 {
		return candidates
	}
	out := make([]int, 0, len(candidates))
	for _, node := range candidates {
		if _, ok := transport.Links[node]; ok {
			out = append(out, node)
		}
	}
	return out
}

func mapsKeys[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func packetTargets(packet game.Packet, world *game.World, nodes []game.LeagueNode) ([]int, error) {
	destination := packet.ToNode
	if destination == 0 && packet.ToBoard != "" {
		destination = world.NodeNumber(packet.ToBoard)
	}
	if destination == 0 {
		mine := world.NodeNumber(world.Config.BoardID)
		var out []int
		for _, node := range nodes {
			if node.Number != mine {
				out = append(out, node.Number)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("broadcast has no recipient")
		}
		return out, nil
	}
	node := nodeByNumber(nodes, destination)
	if node == nil {
		return nil, fmt.Errorf("destination node %d is not in the roster", destination)
	}
	hopName := world.NextHop(node.Name)
	hop := world.NodeNumber(hopName)
	if hop == 0 {
		return nil, fmt.Errorf("next hop %q is not in the roster", hopName)
	}
	return []int{hop}, nil
}

func linkFor(config Config, node int) Link {
	if link, ok := config.Links[node]; ok {
		return link
	}
	return Link{Mode: LinkAttach, Flavour: "Normal"}
}

func attachmentDirectory(dataDir string, transport Config) string {
	if transport.AttachDir != "" {
		return transport.AttachDir
	}
	return filepath.Join(dataDir, spoolDir, attachSpool)
}

func publishTarget(batch, dataDir string, transport Config, origin Address, target batchTarget) (Queued, error) {
	address, err := ParseAddress(target.Address)
	if err != nil {
		return Queued{}, err
	}
	body, err := os.ReadFile(filepath.Join(batch, target.BundleFile))
	if err != nil {
		return Queued{}, err
	}
	final := filepath.Join(target.Directory, target.Alias)
	queued := Queued{PacketPath: final, NextHop: target.Name, Address: address}
	switch target.Mode {
	case LinkAttach:
		if _, _, err := fileAttachSubject(transport, final); err != nil {
			return Queued{}, err
		}
		if err := writeFileAtomic(final, body, 0o644); err != nil {
			return Queued{}, err
		}
		if existing := messageForAttachment(transport.NetmailDir, final); existing != "" {
			queued.Message = existing
			return queued, nil
		}
		message, err := createFileAttach(transport, final, origin, address)
		if err != nil {
			return Queued{}, err
		}
		queued.Message = message
	case LinkObox:
		if err := writeFileAtomic(final, body, 0o644); err != nil {
			return Queued{}, err
		}
		queued.Message = "obox"
	case LinkBSO:
		busy, flow, err := bsoPaths(target.QueueDir, address, target.Flavour)
		if err != nil {
			return Queued{}, err
		}
		lock, err := acquireBSY(busy)
		if err != nil {
			return Queued{}, err
		}
		actual, appended, publishErr := appendBSOBundle(flow, target.Directory, body)
		if publishErr == nil && appended {
			queued.PacketPath = actual
		}
		if publishErr == nil && !appended {
			publishErr = writeFileAtomic(final, body, 0o644)
			if publishErr == nil {
				var absolute string
				absolute, publishErr = filepath.Abs(final)
				if publishErr == nil {
					publishErr = addFlowEntry(flow, absolute)
				}
			}
		}
		releaseErr := releaseBSY(busy, lock)
		if publishErr != nil {
			return Queued{}, publishErr
		}
		if releaseErr != nil {
			return Queued{}, releaseErr
		}
		queued.Message = flow
	default:
		return Queued{}, fmt.Errorf("unsupported link mode %d", target.Mode)
	}
	return queued, nil
}

func messageForAttachment(dir, attachment string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	want, _, err := fileAttachSubject(Config{SubjectMode: SubjectAbsolute}, attachment)
	if err != nil {
		want = attachment
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".msg") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err == nil && len(data) >= type2HeaderSize {
			subject := cStringField(data[72:144])
			if strings.TrimPrefix(subject, "^") == strings.TrimPrefix(want, "^") || filepath.Base(subject) == filepath.Base(attachment) {
				return filepath.Join(dir, entry.Name())
			}
		}
	}
	return ""
}

func quarantineTransport(dataDir, source string) error {
	dir := filepath.Join(dataDir, spoolDir, badSpoolDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := filepath.Base(source)
	for i := 0; ; i++ {
		name := base
		if i > 0 {
			name = strings.TrimSuffix(base, filepath.Ext(base)) + "-" + strconv.Itoa(i) + filepath.Ext(base)
		}
		target := filepath.Join(dir, name)
		if _, err := os.Lstat(target); os.IsNotExist(err) {
			return os.Rename(source, target)
		} else if err != nil {
			return err
		}
	}
}

// cString is kept here too because transport recovery has to identify an
// already-created message after a crash before its journal update.
func cStringField(b []byte) string {
	if i := slices.Index(b, byte(0)); i >= 0 {
		b = b[:i]
	}
	return string(b)
}
