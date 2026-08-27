package ftn

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

const inSpoolDir = "in"

// inboundReceiptFile is the journal naming what one received bundle still owes.
const inboundReceiptFile = "receipt.json"

type inboundReceipt struct {
	ID       string         `json:"id"`
	Source   string         `json:"source"`
	Envelope string         `json:"envelope,omitempty"`
	Local    []inboundLocal `json:"local"`
	Targets  []batchTarget  `json:"targets"`
	Rejected []string       `json:"rejected,omitempty"`
	Complete bool           `json:"complete"`
	// Created and Progress date the receipt the way a batch plan is dated, and
	// for the same reason: a receipt kept across runs is either waiting on a
	// transit peer or stuck on a collision, and how long it has been that way
	// is the difference (#228). Optional, so an older receipt still loads.
	Created  time.Time `json:"created,omitempty"`
	Progress time.Time `json:"progress,omitempty"`
}

type inboundLocal struct {
	Name      string `json:"name"`
	SpoolFile string `json:"spool_file"`
	Done      bool   `json:"done"`
}

type storedAttach struct {
	Path       string
	Attachment string
	Origin     Address
}

// RunIn removes the FTN transport wrapper. It is intended for a mailer's
// post-session hook: no general FTN inbound locking convention exists.
func RunIn(dataDir string) (Result, error) {
	board, transport, nodes, world, origin, adapterLock, err := transportContext(dataDir)
	if err != nil {
		return Result{}, err
	}
	defer adapterLock.Release()
	if transport.InboundDir == "" {
		return Result{}, fmt.Errorf("%s: InboundDir is not set", filepath.Join(dataDir, ConfigFile))
	}
	root := filepath.Join(dataDir, spoolDir, inSpoolDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Result{}, err
	}
	var result Result
	if err := resumeInboundReceipts(root, board, dataDir, transport, world, nodes, origin, &result); err != nil {
		return result, err
	}
	attaches, referenced, err := scanStoredAttaches(transport, origin, nodes)
	if err != nil {
		return result, err
	}
	for _, attach := range attaches {
		if err := ingestTransportFile(root, board, dataDir, transport, world, nodes, origin, attach.Attachment, attach.Path, "attach", attach.Origin, &result); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", filepath.Base(attach.Path), err))
		}
	}
	entries, err := os.ReadDir(transport.InboundDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return result, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !store.IsPacketFile(entry.Name()) {
			continue
		}
		path := filepath.Join(transport.InboundDir, entry.Name())
		if referenced[cleanAbsolute(path)] {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", entry.Name(), err))
			continue
		}
		manifest, _, err := readTransport(data, entry.Name())
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s is not a complete transport file: %v", entry.Name(), err))
			continue
		}
		if manifest.Delivery == "attach" {
			continue // wait for its stored-message envelope
		}
		if err := ingestTransportFile(root, board, dataDir, transport, world, nodes, origin, path, "", manifest.Delivery, Address{}, &result); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", entry.Name(), err))
		}
	}
	return result, nil
}

func scanStoredAttaches(transport Config, local Address, nodes []game.LeagueNode) ([]storedAttach, map[string]bool, error) {
	referenced := map[string]bool{}
	if transport.InboundNetmailDir == "" {
		return nil, referenced, nil
	}
	entries, err := os.ReadDir(transport.InboundNetmailDir)
	if os.IsNotExist(err) {
		return nil, referenced, nil
	}
	if err != nil {
		return nil, referenced, err
	}
	var out []storedAttach
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".msg") {
			continue
		}
		path := filepath.Join(transport.InboundNetmailDir, entry.Name())
		attach, ok, err := parseStoredAttach(path, transport.InboundDir, local)
		if err != nil {
			continue // unrelated or incomplete netmail is not ours to disturb
		}
		if !ok || nodeByAddress(nodes, attach.Origin) == nil {
			continue
		}
		referenced[cleanAbsolute(attach.Attachment)] = true
		out = append(out, attach)
	}
	return out, referenced, nil
}

func parseStoredAttach(path, inboundDir string, local Address) (storedAttach, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return storedAttach{}, false, err
	}
	if len(data) < type2HeaderSize {
		return storedAttach{}, false, fmt.Errorf("stored message is shorter than its header")
	}
	attrs := binary.LittleEndian.Uint16(data[186:188])
	if attrs&attributeFileAttach == 0 {
		return storedAttach{}, false, nil
	}
	if cStringField(data[0:36]) != programName && !strings.Contains(string(data[type2HeaderSize:]), "\x01PID: Immortal Barons") {
		return storedAttach{}, false, nil
	}
	destination := Address{
		Zone: binary.LittleEndian.Uint16(data[176:178]), Net: binary.LittleEndian.Uint16(data[174:176]),
		Node: binary.LittleEndian.Uint16(data[166:168]), Point: binary.LittleEndian.Uint16(data[180:182]),
	}
	if destination != local {
		return storedAttach{}, false, fmt.Errorf("stored message is addressed to %s, not this board", destination)
	}
	origin := Address{
		Zone: binary.LittleEndian.Uint16(data[178:180]), Net: binary.LittleEndian.Uint16(data[172:174]),
		Node: binary.LittleEndian.Uint16(data[168:170]), Point: binary.LittleEndian.Uint16(data[182:184]),
	}
	subject := strings.TrimPrefix(cStringField(data[72:144]), "^")
	parts := strings.FieldsFunc(subject, func(r rune) bool { return r == ' ' || r == ',' })
	if len(parts) != 1 {
		return storedAttach{}, false, fmt.Errorf("file-attach subject names %d files, want one", len(parts))
	}
	attachment := parts[0]
	if !filepath.IsAbs(attachment) {
		attachment = filepath.Join(inboundDir, filepath.Base(attachment))
	}
	abs, err := filepath.Abs(attachment)
	if err != nil {
		return storedAttach{}, false, err
	}
	root, err := filepath.Abs(inboundDir)
	if err != nil {
		return storedAttach{}, false, err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return storedAttach{}, false, fmt.Errorf("attachment %s is outside InboundDir", attachment)
	}
	if !store.IsPacketFile(filepath.Base(abs)) {
		return storedAttach{}, false, nil
	}
	return storedAttach{Path: path, Attachment: abs, Origin: origin}, true, nil
}

func ingestTransportFile(root string, board game.Config, dataDir string, transport Config, world *game.World, nodes []game.LeagueNode, origin Address, source, envelope, via string, sender Address, result *Result) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	manifest, entries, err := readTransport(raw, filepath.Base(source))
	if err != nil {
		return err
	}
	if manifest.Format == bundleFormat && manifest.Delivery != via {
		return fmt.Errorf("bundle delivery %q does not match %s handoff", manifest.Delivery, via)
	}
	transmitter, hasTransmitter := bundleTransmitter(entries)
	if manifest.Format == bundleFormat && (!hasTransmitter || nodeByNumber(nodes, transmitter) == nil) {
		return fmt.Errorf("bundle transmitting hop is not in the roster")
	}
	if sender != (Address{}) {
		node := nodeByAddress(nodes, sender)
		if node == nil || hasTransmitter && transmitter != node.Number {
			return fmt.Errorf("bundle transmitter does not match its stored-message origin")
		}
	}
	sum := sha256.Sum256(raw)
	id := hex.EncodeToString(sum[:16])
	dir := filepath.Join(root, id)
	planPath := filepath.Join(dir, inboundReceiptFile)
	receipt, err := loadInboundReceipt(planPath)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		receipt, err = buildInboundReceipt(dir, id, source, envelope, via, entries, board.LeagueNumber, dataDir, transport, world, nodes, result)
		if err == nil {
			receipt.Created = time.Now()
			err = saveInboundReceipt(planPath, receipt)
		} else {
			// No journal refers to partial artifacts, so a later run must be
			// allowed to rebuild this receipt from a clean directory.
			_ = os.RemoveAll(dir)
		}
	}
	if err != nil {
		return err
	}
	return processInboundReceipt(dir, planPath, &receipt, board, dataDir, transport, origin, result)
}

func buildInboundReceipt(dir, id, source, envelope, via string, entries []transportEntry, boardLeague int, dataDir string, transport Config, world *game.World, nodes []game.LeagueNode, result *Result) (inboundReceipt, error) {
	receipt := inboundReceipt{ID: id, Source: source, Envelope: envelope}
	groups := map[int][]transportEntry{}
	mine := world.NodeNumber(world.Config.BoardID)
	reject := func(entry transportEntry, reason error) {
		receipt.Rejected = append(receipt.Rejected, fmt.Sprintf("%s: %v", entry.Name, reason))
	}
	for i, entry := range entries {
		if boardLeague != 0 && entry.Packet.League != 0 && entry.Packet.League != boardLeague {
			reject(entry, fmt.Errorf("belongs to league %d, not league %d", entry.Packet.League, boardLeague))
			continue
		}
		addressedToMe := world.AddressedToMe(entry.Packet)
		if addressedToMe {
			spoolFile := fmt.Sprintf("local-%06d.brp", i)
			if err := writeFileAtomic(filepath.Join(dir, spoolFile), entry.Raw, 0o644); err != nil {
				return receipt, err
			}
			receipt.Local = append(receipt.Local, inboundLocal{Name: entry.Name, SpoolFile: spoolFile})
		}
		if entry.Packet.ToNode == 0 && entry.Packet.ToBoard == "" {
			if via == "direct" && transport.OboxMeshFanout {
				hops := transportHops(entry)
				if hops >= game.MaxPacketHops {
					result.Warnings = append(result.Warnings, fmt.Sprintf("did not fan out %s after %d transport hops", entry.Name, hops))
					continue
				}
				if slices.Contains(entry.Route, mine) {
					reject(entry, fmt.Errorf("transport route already contains this node"))
					continue
				}
				var targets []int
				for _, node := range nodes {
					if node.Number == mine || slices.Contains(entry.Route, node.Number) || slices.Contains(entry.Covered, node.Number) {
						continue
					}
					if len(transport.Links) > 0 {
						if _, connected := transport.Links[node.Number]; !connected {
							continue
						}
					}
					targets = append(targets, node.Number)
				}
				covered := appendUnique(entry.Covered, mine)
				for _, target := range targets {
					covered = appendUnique(covered, target)
				}
				for _, target := range targets {
					forwarded := entry
					forwarded.Route = append(append([]int(nil), entry.Route...), mine)
					forwarded.Covered = append([]int(nil), covered...)
					groups[target] = append(groups[target], forwarded)
				}
			}
			continue
		}
		if !addressedToMe && world.Routed() {
			hops := transportHops(entry)
			if hops >= game.MaxPacketHops {
				result.Warnings = append(result.Warnings, fmt.Sprintf("did not forward %s after %d transport hops", entry.Name, hops))
				continue
			}
			if slices.Contains(entry.Route, mine) {
				reject(entry, fmt.Errorf("routing cycle returns to this node"))
				continue
			}
			targets, err := packetTargets(entry.Packet, world, nodes)
			if err != nil {
				reject(entry, err)
				continue
			}
			forwarded := entry
			forwarded.Route = append(append([]int(nil), entry.Route...), mine)
			for _, target := range targets {
				if slices.Contains(forwarded.Route, target) {
					reject(entry, fmt.Errorf("routing cycle sends it back through node %d", target))
					forwarded.Route = nil
					break
				}
			}
			if forwarded.Route != nil {
				for _, target := range targets {
					groups[target] = append(groups[target], forwarded)
				}
			}
		}
	}
	groupNodes := mapsKeys(groups)
	slices.Sort(groupNodes)
	for _, number := range groupNodes {
		node := nodeByNumber(nodes, number)
		address, err := ParseAddress(node.Address)
		if err != nil {
			return receipt, err
		}
		link := linkFor(transport, number)
		publishDir := link.Directory
		if link.Mode != LinkObox {
			publishDir = attachmentDirectory(dataDir, transport)
		}
		if err := os.MkdirAll(publishDir, 0o755); err != nil {
			return receipt, err
		}
		alias, wrapped, err := nextAlias(dataDir, publishDir, world.Config.LeagueNumber, mine)
		if err != nil {
			return receipt, err
		}
		if wrapped {
			result.Warnings = append(result.Warnings, "the four-character FTN attachment counter wrapped")
		}
		delivery := "direct"
		if link.Mode == LinkAttach {
			delivery = "attach"
			if err := checkSubjectMargin(transport, filepath.Join(publishDir, alias), result); err != nil {
				return receipt, err
			}
		}
		body, _, err := makeBundle(mine, delivery, groups[number])
		if err != nil {
			return receipt, err
		}
		bundleFile := fmt.Sprintf("target-%03d.bundle", number)
		if err := replaceFileAtomic(filepath.Join(dir, bundleFile), body, 0o644); err != nil {
			return receipt, err
		}
		receipt.Targets = append(receipt.Targets, batchTarget{
			Node: number, Name: node.Name, Address: address.String(), Mode: link.Mode,
			Directory: publishDir, QueueDir: link.Directory, Flavour: link.Flavour, Alias: alias, BundleFile: bundleFile,
		})
	}
	return receipt, nil
}

func processInboundReceipt(dir, planPath string, receipt *inboundReceipt, board game.Config, dataDir string, transport Config, origin Address, result *Result) error {
	for _, rejected := range receipt.Rejected {
		result.Warnings = append(result.Warnings, "quarantined rejected inbound packet "+rejected)
	}
	if !receipt.Complete {
		if len(receipt.Local) > 0 {
			gameLock, err := store.Lock(board, true)
			if err != nil {
				return err
			}
			for i := range receipt.Local {
				local := &receipt.Local[i]
				if local.Done {
					continue
				}
				body, err := os.ReadFile(filepath.Join(dir, local.SpoolFile))
				if err != nil {
					gameLock.Release()
					return err
				}
				name := local.Name
				target := filepath.Join(board.Inbound(), name)
				if existing, err := os.ReadFile(target); err == nil {
					if !slices.Equal(existing, body) {
						gameLock.Release()
						return fmt.Errorf("canonical packet filename collision at %s: existing and received bytes differ", target)
					}
					local.Done = true
					receipt.Progress = time.Now()
					result.Warnings = append(result.Warnings, fmt.Sprintf("ignored duplicate packet %s: canonical name and bytes are identical", name))
					if err := saveInboundReceipt(planPath, *receipt); err != nil {
						gameLock.Release()
						return err
					}
					continue
				} else if !os.IsNotExist(err) {
					gameLock.Release()
					return err
				}
				if err := writeFileAtomic(target, body, 0o644); err != nil {
					gameLock.Release()
					return err
				}
				local.Done = true
				receipt.Progress = time.Now()
				result.Delivered++
				if err := saveInboundReceipt(planPath, *receipt); err != nil {
					gameLock.Release()
					return err
				}
			}
			if err := gameLock.Release(); err != nil {
				return err
			}
		}
		for i := range receipt.Targets {
			target := &receipt.Targets[i]
			if target.Done {
				continue
			}
			queued, err := publishTarget(dir, dataDir, transport, origin, *target)
			if errors.Is(err, errPeerBusy) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s is busy; inbound forwarding remains in %s", target.Name, dir))
				continue
			}
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v; inbound forwarding remains in %s", target.Name, err, dir))
				continue
			}
			target.Done, target.Message = true, queued.Message
			result.Queued = append(result.Queued, queued)
			if err := saveInboundReceipt(planPath, *receipt); err != nil {
				return err
			}
		}
		for _, target := range receipt.Targets {
			if !target.Done {
				return nil
			}
		}
		receipt.Complete = true
		if err := saveInboundReceipt(planPath, *receipt); err != nil {
			return err
		}
	}
	return cleanupInboundReceipt(dir, dataDir, *receipt)
}

func loadInboundReceipt(path string) (inboundReceipt, error) {
	var receipt inboundReceipt
	data, err := os.ReadFile(path)
	if err != nil {
		return receipt, err
	}
	err = json.Unmarshal(data, &receipt)
	return receipt, err
}

func saveInboundReceipt(path string, receipt inboundReceipt) error {
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	return replaceFileAtomic(path, data, 0o644)
}

func resumeInboundReceipts(root string, board game.Config, dataDir string, transport Config, world *game.World, nodes []game.LeagueNode, origin Address, result *Result) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		planPath := filepath.Join(dir, inboundReceiptFile)
		receipt, err := loadInboundReceipt(planPath)
		if os.IsNotExist(err) {
			continue // a directory being built by a run in progress
		}
		if err != nil {
			// Neither retry state nor deliberate quarantine: a receipt that
			// cannot be read is work nobody will ever finish, and skipping it
			// silently is how it stays that way (#228).
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"inbound spool %s has an unreadable %s and is being left alone: %v",
				entry.Name(), filepath.Base(planPath), err))
			continue
		}
		if err := processInboundReceipt(dir, planPath, &receipt, board, dataDir, transport, origin, result); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("resume inbound %s: %v", entry.Name(), err))
		}
	}
	return nil
}

func cleanupInboundReceipt(dir, dataDir string, receipt inboundReceipt) error {
	if receipt.Envelope != "" {
		if err := os.Remove(receipt.Envelope); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if len(receipt.Rejected) > 0 {
		if err := quarantineTransport(dataDir, receipt.Source); err != nil && !os.IsNotExist(err) {
			return err
		}
	} else if err := os.Remove(receipt.Source); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.RemoveAll(dir)
}

func appendUnique(values []int, value int) []int {
	out := append([]int(nil), values...)
	if !slices.Contains(out, value) {
		out = append(out, value)
	}
	return out
}

func nodeByAddress(nodes []game.LeagueNode, address Address) *game.LeagueNode {
	for i := range nodes {
		parsed, err := ParseAddress(nodes[i].Address)
		if err == nil && parsed == address {
			return &nodes[i]
		}
	}
	return nil
}

func cleanAbsolute(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}
