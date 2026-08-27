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
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

const (
	// spoolDir stays "ftn-spool" (#231): out/bad/in are pure internal
	// bookkeeping, never seen by a mailer, so renaming this buys the Type-2
	// Subject field nothing -- attachmentDirectory below doesn't nest under
	// it at all. Renaming it WOULD cost something real: Status walks only
	// the current root, so an in-place upgrade would silently lose sight of
	// whatever a sysop already has queued under the old name.
	spoolDir    = "ftn-spool"
	outSpoolDir = "out"
	badSpoolDir = "bad"
	// attachSpool is short (#231): the path it forms part of counts against
	// the 70-byte FTN Type-2 Subject field, and dataDir itself is often
	// already most of that budget on a Synchronet install, where a door's
	// data directory is a fixed xtrn/<door>/data path the sysop cannot
	// shorten. A sysop who still doesn't fit after this has AttachDir,
	// which is a real directory choice rather than a fixed name this
	// package controls.
	attachSpool   = "att"
	batchPlanFile = "batch.json"
)

type batchPlan struct {
	ID      string        `json:"id"`
	Targets []batchTarget `json:"targets"`
	// Created and Progress date the snapshot: when it was claimed, and when a
	// target in it last published. Both are optional so a journal written
	// before they existed still loads; countPendingOutbound falls back to the
	// file's own mtime, which is what a sysop would otherwise read by hand
	// (#228).
	Created  time.Time `json:"created,omitempty"`
	Progress time.Time `json:"progress,omitempty"`
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
	// LastError keeps the most recent publication failure for this target, so a
	// later run or check can say why a peer is behind without the sysop having
	// kept the output of the run that met it.
	LastError string `json:"last_error,omitempty"`
	// Raw marks a target carrying one unbundled packet, for a peer that cannot
	// read a bundle. Journaled rather than re-read from the config, so a plan
	// written before the setting changed still publishes what it planned.
	Raw bool `json:"raw,omitempty"`
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
	if err := RequireNetmail(transport, dataDir); err != nil {
		return Result{}, err
	}
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
	countPendingOutbound(dataDir, &result)
	return result, nil
}

// noteTargetFailure records why a target is behind, for a later run or check to
// report. A journal that cannot be rewritten is not worth failing the run over:
// the packets are still queued and the next run meets the same peer, so the
// note is a convenience and its loss costs only the explanation.
func noteTargetFailure(planPath string, plan batchPlan, target *batchTarget, cause error) {
	if target.LastError == cause.Error() {
		return // unchanged since the last run; rewriting says nothing new
	}
	target.LastError = cause.Error()
	_ = saveBatchPlan(planPath, plan)
}

// countPendingOutbound fills in what this run leaves behind, from the same
// journal walk the status report uses: one definition of "still waiting"
// rather than two that can drift apart (#228).
func countPendingOutbound(dataDir string, result *Result) {
	status, err := Status(dataDir)
	if err != nil {
		return
	}
	for _, peer := range status.Peers {
		result.Snapshots += peer.Snapshots
		result.Waiting = append(result.Waiting, peer.Name)
		if peer.LastError != "" {
			result.Stalled = append(result.Stalled, peer.Name+": "+peer.LastError)
		}
		if peer.Oldest > result.OldestWait {
			result.OldestWait = peer.Oldest
		}
	}
}

// lastAdvanced dates a snapshot's most recent progress: when a target in it
// last published, or when it was claimed if none ever has. A journal written
// before those fields existed carries neither, so the file's own mtime stands
// in — approximate, but it moves whenever the journal is rewritten, which is
// exactly when progress happens.
func lastAdvanced(plan batchPlan, planPath string) time.Time {
	if !plan.Progress.IsZero() {
		return plan.Progress
	}
	if !plan.Created.IsZero() {
		return plan.Created
	}
	if info, err := os.Stat(planPath); err == nil {
		return info.ModTime()
	}
	return time.Now()
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
	// Before any packet moves: a board with no league number accepts every
	// league's packets and has its own accepted everywhere, so it is not
	// configured to be exchanging mail yet (#227).
	if err := store.CheckLeagueNumber(board); err != nil {
		return fail(err)
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
			plan.Created = time.Now()
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
			noteTargetFailure(planPath, plan, target, err)
			continue
		}
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v; its bundle remains in %s", target.Name, err, batch))
			noteTargetFailure(planPath, plan, target, err)
			continue
		}
		target.Done, target.Message, target.LastError = true, queued.Message, ""
		plan.Progress = time.Now()
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

const aliasWrapWarning = "the four-character FTN attachment counter wrapped; verify that no peer still holds aliases from the preceding cycle"

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
		te := transportEntry{Name: file.Name(), Raw: raw, Packet: packet, Route: []int{mine}}
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
			result.Warnings = append(result.Warnings, aliasWrapWarning)
		}
		delivery := "direct"
		if link.Mode == LinkAttach {
			delivery = "attach"
			if err := checkSubjectMargin(transport, filepath.Join(dir, alias), result); err != nil {
				return batchPlan{}, err
			}
		}
		target := batchTarget{
			Node: nodeNumber, Name: node.Name, Address: address.String(), Mode: link.Mode,
			Directory: dir, QueueDir: link.Directory, Flavour: link.Flavour, Alias: alias,
		}

		// A raw peer gets one file per packet, because a raw file IS one packet:
		// there is no envelope to hold a second. That costs an alias each and
		// gives up coalescing, which is the price of reaching a board that
		// cannot read a bundle at all.
		if rawFor(transport, link) {
			for i, entry := range groups[nodeNumber] {
				rawTarget := target
				rawTarget.Raw = true
				if i > 0 {
					rawTarget.Alias, wrapped, err = nextAlias(dataDir, dir, world.Config.LeagueNumber, mine)
					if err != nil {
						return batchPlan{}, err
					}
					if wrapped {
						result.Warnings = append(result.Warnings, aliasWrapWarning)
					}
				}
				if link.Mode == LinkAttach {
					if err := checkSubjectMargin(transport, filepath.Join(dir, rawTarget.Alias), result); err != nil {
						return batchPlan{}, err
					}
				}
				rawTarget.BundleFile = fmt.Sprintf("target-%03d-%03d.raw", nodeNumber, i)
				if err := replaceFileAtomic(filepath.Join(batch, rawTarget.BundleFile), entry.Raw, 0o644); err != nil {
					return batchPlan{}, err
				}
				plan.Targets = append(plan.Targets, rawTarget)
			}
			continue
		}

		body, _, err := makeBundle(world.NodeNumber(world.Config.BoardID), delivery, groups[nodeNumber])
		if err != nil {
			return batchPlan{}, err
		}
		target.BundleFile = fmt.Sprintf("target-%03d.bundle", nodeNumber)
		if err := replaceFileAtomic(filepath.Join(batch, target.BundleFile), body, 0o644); err != nil {
			return batchPlan{}, err
		}
		plan.Targets = append(plan.Targets, target)
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

// attachmentDirectory's default deliberately does NOT nest under spoolDir
// the way out/bad/in do (#231): attach is the one spool directory whose path
// leaks into an external protocol field with a hard byte budget (the FTN
// Type-2 Subject) -- every byte the other three spend on organization is a
// byte this one can't afford to spend the same way. Nesting it under
// spoolDir would add a whole "ftn-spool/" segment to that budget for no
// benefit, since out/bad/in never appear in a mailer-facing Subject at all.
func attachmentDirectory(dataDir string, transport Config) string {
	if transport.AttachDir != "" {
		return transport.AttachDir
	}
	return filepath.Join(dataDir, attachSpool)
}

func checkSubjectMargin(transport Config, attached string, result *Result) error {
	_, spare, err := fileAttachSubject(transport, attached)
	if err != nil {
		return err
	}
	if spare < subjectMarginBytes {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"attachment subjects have %d byte(s) to spare in the FTN Type-2 field; %s",
			spare, subjectAdvice(transport.SubjectMode)))
	}
	return nil
}

func publishTarget(batch, dataDir string, transport Config, origin Address, target batchTarget) (Queued, error) {
	// Named before it is reached: creating the netmail with no directory
	// configured fails as `open : no such file or directory`, an error whose
	// blank filename says nothing about which setting is missing. --in hits
	// this too when a routing board forwards transit, which is where RunOut's
	// own check cannot help (three-board rig, 2026-08-27).
	if target.Mode == LinkAttach && transport.NetmailDir == "" {
		return Queued{}, fmt.Errorf("%s: NetmailDir is not set, and %s takes an attach handoff",
			filepath.Join(dataDir, ConfigFile), target.Name)
	}
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
		// A raw packet is never merged: appendBSOBundle would read it as a
		// one-entry legacy bundle and fold it into a ZIP, which is exactly what
		// this peer cannot read.
		actual, appended := "", false
		var publishErr error
		if !target.Raw {
			actual, appended, publishErr = appendBSOBundle(flow, target.Directory, body)
		}
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
