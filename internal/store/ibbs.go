package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// PacketExt is the file extension for inter-BBS packet files. Exported because
// an inbound directory is usually shared with the BBS's own mail, so anything
// inspecting it has to tell a game packet from a mail bundle.
const PacketExt = ".brp"

// RunPlanetary is the inter-BBS maintenance step (BRE's "BRE PLANETARY"): it
// reads and applies inbound packets, launches any group attacks whose day has
// come, exports this board's scores to the league, and writes the outbox. Run
// it on a schedule (or on the door's -maint pass) — can run several times a day.
func RunPlanetary(w *game.World, inboundDir, outboundDir string) (PlanetaryRun, error) {
	var run PlanetaryRun
	before := w.LeagueNodes
	applied, err := ReadInbound(w, inboundDir)
	if err != nil {
		return run, err
	}
	run.Applied = applied
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
	// After the inbound packets, so a result that arrived this run is never
	// overtaken by the recovery timer.
	w.ReturnLostForces()
	w.LaunchDueGroupAttacks()
	w.ArriveAnnihilator() // a weapon whose flight is over lands before anything else moves
	w.ExportScores()
	w.ExportNodeList()
	w.PingTravelTimes()
	w.ExportAnnihilatorStatus()
	w.StampOutbox()
	run.Forwarded = len(w.Transit)
	sent, err := WriteOutbox(w, outboundDir)
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
}

// WriteOutbox writes each queued packet to a JSON file and clears the world's
// Outbox and Transit queues. How the files then reach other boards (a sync tool,
// FidoNet, scp, a shared mount) is the operator's concern — this is the Option A
// transport from the design spec.
//
// A packet goes to the link for its NEXT hop, not for its final destination:
// with HOST routing in the roster, everything a leaf board sends lands in the
// one directory its uplink collects from (#106). dir is that directory; a board
// that hosts others configures a separate one per neighbour.
func WriteOutbox(w *game.World, dir string) (int, error) {
	packets := addressBroadcasts(w, append(append([]game.Packet(nil), w.Outbox...), w.Transit...))
	if len(packets) == 0 {
		return 0, nil
	}
	for i, p := range packets {
		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return 0, err
		}
		to := p.ToBoard
		if to == "" {
			to = "all"
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
		name := fmt.Sprintf("%s%s-to-%s-%s-%d-%d%s",
			leaguePrefix(p.League), sanitize(w.Config.BoardID), sanitize(to), sanitize(p.Date), p.Seq, i, PacketExt)
		if err := os.WriteFile(filepath.Join(target, name), data, 0o644); err != nil {
			return 0, err
		}
	}
	w.Outbox, w.Transit = nil, nil
	return len(packets), nil
}

// addressBroadcasts turns each broadcast into one packet per planet on the
// roster. A broadcast is one file that the transport is expected to copy to
// every board — which only works where every board links to every other one.
// Once a league routes, only the game knows the shape of it, so the roster is
// where the copies have to be made; each one then follows the ordinary path to
// its board. An unrouted league sends the single broadcast as before.
//
// The Coordinator's signature covers the payload and the sender, not the
// destination, so an addressed copy verifies exactly as the broadcast did.
func addressBroadcasts(w *game.World, packets []game.Packet) []game.Packet {
	boards := w.KnownBoards()
	if !w.Routed() || len(boards) == 0 {
		return packets
	}
	out := make([]game.Packet, 0, len(packets))
	for _, p := range packets {
		if p.ToBoard != "" {
			out = append(out, p)
			continue
		}
		for _, b := range boards {
			copied := p
			copied.ToBoard = b
			out = append(out, copied)
		}
	}
	return out
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

// ReadInbound reads every packet file in dir addressed to this board (or
// broadcast), applies it, queues any result packet to the world's Outbox, and
// removes the consumed files. In a routed league a packet for another board is
// picked up and queued for forwarding, so a hub passes traffic on instead of
// collecting it (#106); in a mesh it is left alone, being a copy of one the
// addressee got directly. A packet stamped with a different league's number is
// left alone too: it belongs to the other game sharing this directory.
// Returns the number of packets applied.
func ReadInbound(w *game.World, dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	applied := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != PacketExt {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return applied, err
		}
		var p game.Packet
		if err := json.Unmarshal(data, &p); err != nil {
			return applied, fmt.Errorf("packet %s: %w", e.Name(), err)
		}
		if w.Config.LeagueNumber != 0 && p.League != 0 && p.League != w.Config.LeagueNumber {
			continue // another league's game, sharing this directory
		}
		if p.ToBoard != "" && p.ToBoard != w.Config.BoardID {
			if !w.Routed() {
				continue // a mesh fans every packet out; this is a copy for someone else
			}
			// In transit. The file goes either way, so a packet that has run out
			// of hops is not re-read on every later run.
			w.ForwardPacket(p)
			if err := os.Remove(path); err != nil {
				return applied, err
			}
			continue
		}
		result := w.ApplyPacket(p)
		if result.HasPayload() {
			w.Outbox = append(w.Outbox, result)
		}
		if err := os.Remove(path); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, nil
}

// sanitize keeps packet filenames safe (no path separators or spaces).
func sanitize(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_")
	return r.Replace(s)
}
