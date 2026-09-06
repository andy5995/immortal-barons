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

// inboundFile keeps a directory entry with the directory it came from. The two
// scans below read several directories into one list, and pairing them by index
// or by filename would put a file's path together from the wrong half.
type inboundFile struct {
	dir   string
	entry os.DirEntry
}

// RunIn removes the FTN transport wrapper. It is intended for a mailer's
// post-session hook: no general FTN inbound locking convention exists.
func RunIn(dataDir string) (Result, error) {
	board, transport, nodes, world, origin, adapterLock, err := transportContext(dataDir)
	if err != nil {
		return Result{}, err
	}
	defer adapterLock.Release()
	if len(transport.InboundDirs) == 0 {
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
	// Every directory the mailer delivers into, not just the first: one that is
	// named but never read looks exactly like a transport that is working.
	var entries []inboundFile
	for _, dir := range transport.InboundDirs {
		found, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return result, err
		}
		for _, entry := range found {
			entries = append(entries, inboundFile{dir: dir, entry: entry})
		}
	}
	for _, found := range entries {
		entry := found.entry
		if entry.IsDir() || !store.IsPacketFile(entry.Name()) {
			continue
		}
		path := filepath.Join(found.dir, entry.Name())
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
	// Anything still sitting in the mailer's inbound after this run had its
	// chance. Saying so is the whole of #236: the run above passes over a
	// bundle it cannot claim in silence, so a board can collect them for days
	// while every report -- this count, --status, -league-check -- reads
	// healthy.
	claimed := claimedSources(dataDir)
	for path := range referenced {
		claimed[path] = true
	}
	for _, waiting := range scanUnclaimed(transport.InboundDirs, claimed, time.Now()) {
		result.Warnings = append(result.Warnings, unclaimedWarning(waiting))
	}
	return result, nil
}

// unclaimedWarning says what is waiting and, where the location settles it, why
// no later run will help. It names no cause otherwise: a bundle can go unclaimed
// for reasons this build has never met, and a wrong cause sends a sysop looking
// in the wrong place -- which is the failure #236 is about, not an improvement
// on it.
func unclaimedWarning(waiting Unclaimed) string {
	if waiting.Subdir != "" {
		return fmt.Sprintf("%s has waited %s in the %s subdirectory of the mailer's inbound, which is not scanned; "+
			"no later run will take it. A mailer files an unauthenticated session's files apart from the rest, "+
			"so give that directory its own InboundDir line in ftn.cfg, or check the session password for "+
			"that peer and move the file up.",
			filepath.Base(waiting.Path), waiting.Age.Round(time.Minute), waiting.Subdir)
	}
	waited := fmt.Sprintf("%s has waited %s in the mailer's inbound and no run has claimed it.",
		filepath.Base(waiting.Path), waiting.Age.Round(time.Minute))
	if isBundle(waiting.Path) {
		return waited + fmt.Sprintf(" Read what it is with: unzip -p %s manifest.json",
			shellQuote(waiting.Path))
	}
	return waited
}

// envelopeAttachment resolves the one file a stored message attaches, and
// refuses a name that points outside InboundDir. It is the half of
// parseStoredAttach that does not depend on who the message is addressed to, so
// a REPORT can ask "does an envelope name this file?" without the roster and
// board address a claim needs.
func envelopeAttachment(data []byte, inboundDirs []string) (string, error) {
	subject := strings.TrimPrefix(cStringField(data[72:144]), "^")
	parts := strings.FieldsFunc(subject, func(r rune) bool { return r == ' ' || r == ',' })
	if len(parts) != 1 {
		return "", fmt.Errorf("file-attach subject names %d files, want one", len(parts))
	}
	attachment := parts[0]
	if len(inboundDirs) == 0 {
		return "", fmt.Errorf("no InboundDir is set")
	}
	if !filepath.IsAbs(attachment) {
		// The envelope names a file, not a directory, and the mailer may have
		// filed it in any of the inbound directories -- an authenticated
		// session and an unauthenticated one land apart. Resolving against the
		// first listed would name a path that does not exist, and the direct
		// scan cannot rescue it because an attach bundle is deliberately
		// skipped there.
		name := filepath.Base(attachment)
		attachment = filepath.Join(inboundDirs[0], name)
		for _, dir := range inboundDirs {
			candidate := filepath.Join(dir, name)
			if _, err := os.Stat(candidate); err == nil {
				attachment = candidate
				break
			}
		}
	}
	abs, err := filepath.Abs(attachment)
	if err != nil {
		return "", err
	}
	for _, dir := range inboundDirs {
		root, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(root, abs)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("attachment %s is outside every InboundDir", attachment)
}

// envelopeReferenced names every inbound file a stored-message envelope points
// at, without checking who sent it or who it is addressed to. RunIn makes those
// checks before it will CLAIM a bundle; a report only needs to know that
// something already points at the file, so that one waiting for the next -in is
// not called abandoned. An envelope that fails the fuller checks is reported by
// RunIn's own warning instead, so nothing goes unmentioned by both.
func envelopeReferenced(transport Config) map[string]bool {
	referenced := map[string]bool{}
	for _, dir := range transport.netmailDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".msg") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil || len(data) < type2HeaderSize {
				continue
			}
			if binary.LittleEndian.Uint16(data[186:188])&attributeFileAttach == 0 {
				continue
			}
			if abs, err := envelopeAttachment(data, transport.InboundDirs); err == nil {
				referenced[cleanAbsolute(abs)] = true
			}
		}
	}
	return referenced
}

// shellQuote makes a path safe to paste. InboundDir comes from ftn.cfg and may
// hold spaces, and a command a sysop cannot paste is not advice.
func shellQuote(path string) string {
	if !strings.ContainsAny(path, " \t'\"\\$`&;|<>()*?[]#~") {
		return path
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

func scanStoredAttaches(transport Config, local Address, nodes []game.LeagueNode) ([]storedAttach, map[string]bool, error) {
	referenced := map[string]bool{}
	var out []storedAttach
	var entries []inboundFile
	for _, dir := range transport.netmailDirs() {
		found, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, referenced, err
		}
		for _, entry := range found {
			entries = append(entries, inboundFile{dir: dir, entry: entry})
		}
	}
	for _, found := range entries {
		entry := found.entry
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".msg") {
			continue
		}
		path := filepath.Join(found.dir, entry.Name())
		attach, ok, err := parseStoredAttach(path, transport.InboundDirs, local)
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

func parseStoredAttach(path string, inboundDirs []string, local Address) (storedAttach, bool, error) {
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
	abs, err := envelopeAttachment(data, inboundDirs)
	if err != nil {
		return storedAttach{}, false, err
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
		target := batchTarget{
			Node: number, Name: node.Name, Address: address.String(), Mode: link.Mode,
			Directory: publishDir, QueueDir: link.Directory, Flavour: link.Flavour, Alias: alias,
		}

		// A forwarded packet obeys the same posture an originated one does: a
		// peer that cannot read a bundle cannot read one it is only relaying
		// through us either, and it arrives under the same .BRP alias, so the
		// board has no way to tell it apart from a packet it can parse (#230).
		if rawFor(transport, link) {
			for i, entry := range groups[number] {
				rawTarget := target
				rawTarget.Raw = true
				if i > 0 {
					rawTarget.Alias, wrapped, err = nextAlias(dataDir, publishDir, world.Config.LeagueNumber, mine)
					if err != nil {
						return receipt, err
					}
					if wrapped {
						result.Warnings = append(result.Warnings, "the four-character FTN attachment counter wrapped")
					}
					if link.Mode == LinkAttach {
						if err := checkSubjectMargin(transport, filepath.Join(publishDir, rawTarget.Alias), result); err != nil {
							return receipt, err
						}
					}
				}
				rawTarget.BundleFile = fmt.Sprintf("target-%03d-%03d.raw", number, i)
				if err := replaceFileAtomic(filepath.Join(dir, rawTarget.BundleFile), entry.Raw, 0o644); err != nil {
					return receipt, err
				}
				receipt.Targets = append(receipt.Targets, rawTarget)
			}
			continue
		}

		body, _, err := makeBundle(mine, delivery, groups[number])
		if err != nil {
			return receipt, err
		}
		target.BundleFile = fmt.Sprintf("target-%03d.bundle", number)
		if err := replaceFileAtomic(filepath.Join(dir, target.BundleFile), body, 0o644); err != nil {
			return receipt, err
		}
		receipt.Targets = append(receipt.Targets, target)
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
