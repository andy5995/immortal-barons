package store

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// math.MaxUint64 is "3w5e11264sgsf" in base 36: exactly 13 digits.
const maxUint64Base36Digits = 13

// PacketExt is the file extension for inter-BBS packet files. Exported because
// an inbound directory is usually shared with the BBS's own mail, so anything
// inspecting it has to tell a game packet from a mail bundle.
const PacketExt = ".brp"

// IsPacketFile reports whether a directory entry's name is an inter-BBS packet.
// The extension is matched WITHOUT regard to case (#179): FTN transport hands
// files over in upper case routinely — 8.3-era software and several mailers do
// it, and a league carried over FidoNet can meet one at any hop. An exact match
// against the lowercase name left a ".BRP" sitting in the inbound directory
// unread and unreported, so the only symptom a sysop got was packets that never
// arrived. Every scan of a packet directory goes through here, so the two ends
// cannot drift apart on what counts as a packet.
func IsPacketFile(name string) bool {
	return strings.EqualFold(filepath.Ext(name), PacketExt)
}

// RunPlanetary is the inter-BBS maintenance step (BRE's "BRE PLANETARY"): it
// reads and applies inbound packets, launches any group attacks whose day has
// come, exports this board's scores to the league, and writes the outbox. Run
// it on a schedule (or on the door's -maint pass) — can run several times a day.
func RunPlanetary(w *game.World, inboundDir, outboundDir string, verbose bool) (PlanetaryRun, error) {
	var run PlanetaryRun
	before := w.LeagueNodes
	// Before anything else: a packet held for a protocol this build could not
	// read may be readable now, and it has been waiting since it arrived.
	released, err := releaseHeld(w.Config.DataDir, inboundDir)
	if err != nil {
		return run, err
	}
	run.Released = released
	inResult, err := ReadInbound(w, inboundDir, verbose)
	if err != nil {
		return run, err
	}
	run.Applied = inResult.Applied
	run.OtherLeague = inResult.OtherLeague
	run.MeshCopy = inResult.MeshCopy
	run.AlreadySeen = inResult.AlreadySeen
	run.Refused = inResult.Refused
	run.Held = inResult.Held
	run.Quarantined = inResult.Quarantined
	// A member board that just adopted the Coordinator's roster has to persist
	// it: the roster is read from ibnodes.dat at startup, not from the world
	// file (#64).
	if !game.SameRoster(before, w.LeagueNodes) {
		if err := WriteNodeList(filepath.Join(w.Config.DataDir, NodeListFile), w.LeagueNodes); err != nil {
			return run, err
		}
		run.RosterUpdated = true
	}
	// An inbound league-config packet may have updated w.Config; persist it so
	// the adopted settings survive the next load (config.json is authoritative).
	if err := SaveConfig(w.Config); err != nil {
		return run, err
	}
	// The bulletin directories are reconciled after the inbound packets, so a
	// set that arrived this run reaches the disk in the same pass.
	leagueBulletins, err := SyncBulletins(w)
	if err != nil {
		return run, err
	}
	run.Bulletins = len(leagueBulletins)
	// After the inbound packets, so a result that arrived this run is never
	// overtaken by the recovery timer.
	w.ReturnLostForces()
	w.LaunchDueGroupAttacks()
	w.ArriveAnnihilator() // a weapon whose flight is over lands before anything else moves
	w.ExportScores()
	w.ExportNodeList()
	w.ExportBulletins(leagueBulletins)
	w.PingTravelTimes()
	w.ExportAnnihilatorStatus()
	w.StampOutbox()
	run.Forwarded = len(w.Transit)
	// Drained, not copied: they belong to this run, and leaving them on the
	// world would repeat them in the next one.
	run.Notices, w.SysopNotices = w.SysopNotices, nil
	// Also to disk: the run report goes to stdout, which a scheduled run throws
	// away, and a scheduler is how the setup guide says to drive this step.
	AppendPlanetaryLog(w.Config.DataDir, run.Notices, time.Now())
	sent, err := WriteOutbox(w, outboundDir, verbose)
	run.Sent = sent
	return run, err
}

// PlanetaryRun is what one inter-BBS step did, so the command line can report
// it: a run that says nothing is indistinguishable from one that failed to find
// its directories.
type PlanetaryRun struct {
	Applied       int  // packets read from the inbound directory and applied here
	Forwarded     int  // packets that arrived for another board and were passed on
	Sent          int  // packet files written, forwarded ones included
	RosterUpdated bool // the Coordinator's roster replaced this board's copy
	OtherLeague   int  // packets skipped: wrong league number
	MeshCopy      int  // packets skipped: not addressed here, mesh mode
	AlreadySeen   int  // packets skipped: duplicate/replay
	Refused       int  // packets refused: the sender's signature did not match the roster
	Held          int  // packets set aside: they speak a protocol this build cannot read
	Quarantined   int  // packets that could not be parsed at all and were set aside
	Released      int  // held packets this build can now read, returned to inbound
	Bulletins     int  // league bulletins broadcast (Coordinator's board only)
	// Notices are transport faults for the sysop -- an undeliverable packet,
	// orders that failed their check. They are reported here rather than in the
	// planet's news: no player can act on one, and the news cap would let a
	// repeating fault delete the day's real events.
	Notices []string
}

// WriteOutbox atomically publishes each queued packet as a JSON file and clears
// the world's Outbox and Transit queues. How the files then reach other boards
// (a sync tool, FidoNet, scp, a shared mount) is the operator's concern — this
// is the Option A transport from the design spec.
//
// A packet goes to the link for its NEXT hop, not for its final destination:
// with HOST routing in the roster, everything a leaf board sends lands in the
// one directory its uplink collects from (#106). dir is that directory; a board
// that hosts others configures a separate one per neighbour.
func WriteOutbox(w *game.World, dir string, verbose bool) (int, error) {
	packets := append(append([]game.Packet(nil), w.Outbox...), w.Transit...)
	// Backstop the protocol stamp. StampOutbox sets it on everything this board
	// authored, and every production path calls it — but this is the last point
	// before bytes reach disk, and a packet that goes out stating no protocol is
	// held by every board that receives it, for a reason nobody can see from the
	// packet. Transit packets keep the stamp of the board that wrote them: a
	// forwarded packet is relayed byte for byte and is not ours to re-label.
	for i := range packets[:len(w.Outbox)] {
		if packets[i].Protocol == 0 {
			packets[i].Protocol = game.Protocol
		}
	}
	if len(packets) == 0 {
		return 0, nil
	}
	for _, p := range packets {
		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return 0, err
		}
		target := dir
		// A broadcast that got this far has no roster to address it from, so
		// there is no next hop to compute: it goes out on the default link for
		// the transport to fan out.
		if p.ToBoard != "" {
			if link, ok := w.Config.OutboundLink(w.NodeNumber(w.NextHop(p.ToBoard))); ok {
				target = link
			}
		}
		if err := os.MkdirAll(target, 0o755); err != nil {
			return 0, err
		}
		name := packetFilename(p, data)
		if err := writePacketAtomic(filepath.Join(target, name), data); err != nil {
			return 0, err
		}
		if verbose {
			board := p.ToBoard
			if board == "" {
				board = "(broadcast)"
			}
			fmt.Printf("  Wrote packet to %s (%s, dated %s)\n", board, p.PacketType(), p.Date)
		}
	}
	w.Outbox, w.Transit = nil, nil
	return len(packets), nil
}

// packetFilename keeps the transport name short enough to leave room for an
// absolute directory in an FTN Type-2 subject. Modern packets are identified
// exactly by origin node, final destination node, and the origin's monotonic
// sequence number. Older packets without that identity use a stable 128-bit
// content digest instead.
func packetFilename(p game.Packet, data []byte) string {
	var identity string
	if p.FromNode > 0 && p.Seq > 0 {
		sequence := strconv.FormatUint(p.Seq, 36)
		if padding := maxUint64Base36Digits - len(sequence); padding > 0 {
			sequence = strings.Repeat("0", padding) + sequence
		}
		identity = strconv.FormatInt(int64(p.FromNode), 36) + "-" + sequence + "-" +
			strconv.FormatInt(int64(p.ToNode), 36)
		if p.League <= 0 {
			// With no league prefix, the same node numbers in two leagues sharing
			// a directory need a short discriminator. Matching board names still
			// denote the same origin, as they did in the old descriptive names.
			identity = shortDigest([]byte(p.FromBoard), 10) + "-" + identity
		}
	} else {
		identity = shortDigest(data, 16)
	}
	return leaguePrefix(p.League) + identity + PacketExt
}

func shortDigest(data []byte, size int) string {
	digest := sha256.Sum256(data)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(digest[:size])
}

// writePacketAtomic keeps an incomplete packet under a non-.brp temporary
// name. Consumers only discover the final name after the signed JSON has been
// completely written and closed.
func writePacketAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if existing, readErr := os.ReadFile(path); readErr == nil {
		if bytes.Equal(existing, data) {
			return nil // an earlier attempt already published this packet
		}
		return fmt.Errorf("packet filename collision at %s", path)
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	if err := os.Rename(tmpPath, path); err == nil {
		return nil
	} else if existing, readErr := os.ReadFile(path); readErr == nil && bytes.Equal(existing, data) {
		// Windows will not rename over an existing target. An identical target
		// means this packet was already published by an earlier attempt.
		return nil
	} else {
		return err
	}
}

// leaguePrefix marks a packet's filename with the league it belongs to, so a
// sysop looking at an inbound directory shared by two leagues can see which is
// which. League 0 means the number was never set, and the prefix is left off.
func leaguePrefix(league int) string {
	if league <= 0 {
		return ""
	}
	return fmt.Sprintf("L%03d-", league)
}

// InboundResult is what ReadInbound did: how many packets were applied and how
// many were skipped, broken down by reason. The per-reason breakdown matters
// more than the total — "already seen" and "another league" send the sysop to
// completely different places.
type InboundResult struct {
	Applied     int
	OtherLeague int
	MeshCopy    int
	AlreadySeen int
	Refused     int
	// Held counts packets set aside for a protocol this build cannot read.
	Held int
	// Quarantined counts packets that could not even be parsed as JSON and
	// were set aside instead of stopping the run. See quarantinePacket.
	Quarantined int
}

// stagedPacket is an inbound file that parsed cleanly, waiting to be applied
// in the order ReadInbound decides rather than the order os.ReadDir returned
// it in.
type stagedPacket struct {
	path   string
	packet game.Packet
}

// originKey identifies a packet's sender for grouping. It keys on FromBoard
// first, matching IsPacketSeen/SeenPacket's replay tracking (HighSeq is keyed
// by FromBoard alone) — NOT preferring FromNode the way fromCoordinator does,
// because the two keys disagreeing is exactly what would split one origin's
// packets across two groups. A board's own packets do not all carry the same
// FromNode within one batch: an older or backlogged packet can predate the
// board's roster entry (FromNode 0) while a newer one from the same board
// carries it, and grouping those two apart lets the group with the higher Seq
// apply first, marking the other group's lower Seq a false replay — a packet
// silently lost, not just misordered. FromNode is still the fallback for a
// packet old or malformed enough to carry no board name at all.
func originKey(p game.Packet) string {
	if p.FromBoard != "" {
		return "b" + p.FromBoard
	}
	if p.FromNode != 0 {
		return "n" + strconv.Itoa(p.FromNode)
	}
	return ""
}

// shuffleGroupOrder puts keys in a fresh order every call, reading randomness
// from src (crypto/rand.Reader in production, something that can be made to
// fail in tests) rather than anything derived from packet content — so no
// origin can grind for a favorable position by crafting what it sends (#178).
// It is all-or-nothing: the shuffle happens on a scratch copy, and keys is
// touched only once the WHOLE shuffle has succeeded. A source of randomness
// that fails partway through a Fisher-Yates in place would otherwise leave
// keys neither in this run's random order nor its original scan order, just
// whatever the swaps before the failure happened to produce.
func shuffleGroupOrder(keys []string, src io.Reader) {
	shuffled := append([]string(nil), keys...)
	for i := len(shuffled) - 1; i > 0; i-- {
		j, err := rand.Int(src, big.NewInt(int64(i+1)))
		if err != nil {
			return
		}
		shuffled[i], shuffled[int(j.Int64())] = shuffled[int(j.Int64())], shuffled[i]
	}
	copy(keys, shuffled)
}

// ReadInbound reads every packet file in dir addressed to this board (or
// broadcast) and applies it, queuing any result packet to the world's Outbox
// and removing the consumed files. In a routed league a packet for another
// board is picked up and queued for forwarding, so a hub passes traffic on
// instead of collecting it (#106); in a mesh it is left alone, being a copy of
// one the addressee got directly. A packet stamped with a different league's
// number is left alone too: it belongs to the other game sharing this
// directory.
//
// Application order is NOT filename order (#178): every packet in dir is
// parsed and staged first, the Coordinator's own packets (there is normally
// at most one) are applied ahead of everything else — the rest of this run's
// checks read the roster those packets can update — and the remainder is
// grouped by origin and applied group by group in an order reshuffled every
// run. Each origin's own packets stay in their own Seq order within their
// group: only the order BETWEEN origins was ever the problem. Base-36 encodes
// the origin node near the front of every filename, so alphabetical order
// gave the same origin first place in every batch for as long as the roster
// stood — a fixed, permanent advantage on anything two origins contest in the
// same run, such as a trade bid or a land claim.
func ReadInbound(w *game.World, dir string, verbose bool) (InboundResult, error) {
	var result InboundResult
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return result, err
	}

	var coordPackets []stagedPacket
	groups := map[string][]stagedPacket{}
	var groupOrder []string

	for _, e := range entries {
		if e.IsDir() || !IsPacketFile(e.Name()) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return result, err
		}
		var p game.Packet
		if err := json.Unmarshal(data, &p); err != nil {
			// Corrupt JSON, a truncated transfer, or a foreign file dropped in
			// the wrong directory must not block every other packet behind it
			// (#178) — set it aside for a sysop to look at and move on.
			result.Quarantined++
			w.SysopNotices = append(w.SysopNotices, fmt.Sprintf(
				"Packet %s could not be read and has been set aside in %s: %v", e.Name(), BadDir, err))
			if qerr := quarantinePacket(w.Config.DataDir, path); qerr != nil {
				return result, qerr
			}
			if verbose {
				fmt.Printf("  Quarantined packet %s (unreadable: %v)\n", e.Name(), err)
			}
			continue
		}
		if w.IsFromCoordinator(p) {
			coordPackets = append(coordPackets, stagedPacket{path, p})
			continue
		}
		key := originKey(p)
		if _, seen := groups[key]; !seen {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], stagedPacket{path, p})
	}

	bySeq := func(s []stagedPacket) func(a, b int) bool {
		return func(a, b int) bool { return s[a].packet.Seq < s[b].packet.Seq }
	}
	sort.SliceStable(coordPackets, bySeq(coordPackets))
	for _, g := range groups {
		sort.SliceStable(g, bySeq(g))
	}

	for _, sp := range coordPackets {
		if err := applyStagedPacket(w, &result, sp.path, sp.packet, verbose); err != nil {
			return result, err
		}
	}

	shuffleGroupOrder(groupOrder, rand.Reader)
	// Only worth a line in the log when there was an actual choice to make: one
	// origin's batch applies in the same order regardless, and logging that
	// every run buries the runs where the shuffle did something.
	if len(groupOrder) > 1 {
		w.SysopNotices = append(w.SysopNotices, fmt.Sprintf(
			"Inbound batch: %d origins applied in this order: %v", len(groupOrder), groupOrder))
	}
	for _, key := range groupOrder {
		for _, sp := range groups[key] {
			if err := applyStagedPacket(w, &result, sp.path, sp.packet, verbose); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

// applyStagedPacket runs the checks and effects for one already-parsed
// inbound packet: the league check, the addressed-here/forward/mesh check,
// the protocol-hold check, the replay check, and finally OriginRefused plus
// ApplyPacket itself. Factored out of ReadInbound so staging and ordering
// packets ahead of applying them did not have to change what happens to any
// one packet, only when.
func applyStagedPacket(w *game.World, result *InboundResult, path string, p game.Packet, verbose bool) error {
	if w.Config.LeagueNumber != 0 && p.League != 0 && p.League != w.Config.LeagueNumber {
		result.OtherLeague++
		if verbose {
			fmt.Printf("  Skipped packet from %s (wrong league)\n", p.FromBoard)
		}
		return nil // another league's game, sharing this directory
	}
	if !w.AddressedToMe(p) {
		if !w.Routed() {
			result.MeshCopy++
			if verbose {
				fmt.Printf("  Skipped packet from %s (not addressed here)\n", p.FromBoard)
			}
			return nil // a mesh fans every packet out; this is a copy for someone else
		}
		// In transit. The file goes either way, so a packet that has run out
		// of hops is not re-read on every later run.
		w.ForwardPacket(p)
		if verbose {
			fmt.Printf("  Forwarded packet from %s to %s (%s, dated %s)\n",
				p.FromBoard, p.ToBoard, p.PacketType(), p.Date)
		}
		return os.Remove(path)
	}
	// Held, not applied: this build cannot read the format. See HeldDir.
	// Checked only for a packet addressed HERE — a hub passes on bytes it
	// never interprets, and holding one in transit would stall delivery to a
	// board that reads it perfectly well.
	if !game.SpeaksOurProtocol(p.Protocol) {
		result.Held++
		w.NoteProtocolHold(p.FromBoard, p.Protocol)
		return holdPacket(w.Config.DataDir, path)
	}
	if w.IsPacketSeen(p) {
		result.AlreadySeen++
		if verbose {
			fmt.Printf("  Skipped packet from %s (already seen)\n", p.FromBoard)
		}
		os.Remove(path) // clean up; SeenPacket already recorded it
		return nil
	}
	// Asked BEFORE ApplyPacket, which posts the refusal to the planet's
	// news and then returns the same empty packet it returns for a
	// replay or for anything with no reply to send.
	refused := w.OriginRefused(p)
	applyResult := w.ApplyPacket(p)
	if applyResult.HasPayload() {
		w.Outbox = append(w.Outbox, applyResult)
	}
	if refused {
		result.Refused++
		if verbose {
			fmt.Printf("  Refused packet from %s (%s): it does not match that board's key\n",
				p.FromBoard, p.PacketType())
		}
	} else {
		result.Applied++
		if verbose {
			fmt.Printf("  Applied packet from %s (%s, dated %s)\n", p.FromBoard, p.PacketType(), p.Date)
		}
	}
	return os.Remove(path)
}
