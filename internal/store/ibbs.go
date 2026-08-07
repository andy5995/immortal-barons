package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// packetExt is the file extension for inter-BBS packet files.
const packetExt = ".brp"

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
	if !sameRoster(before, w.LeagueNodes) {
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
	w.ArriveDoomer() // a weapon whose flight is over lands before anything else moves
	w.ExportScores()
	w.ExportNodeList()
	w.PingTravelTimes()
	w.ExportDoomerStatus()
	w.StampOutbox()
	run.Sent = len(w.Outbox)
	return run, WriteOutbox(w, outboundDir)
}

// PlanetaryRun is what one inter-BBS step did, so the command line can report
// it: a run that says nothing is indistinguishable from one that failed to find
// its directories.
type PlanetaryRun struct {
	Applied       int  // packets read from the inbound directory
	Sent          int  // packets written to the outbound directory
	RosterUpdated bool // the Coordinator's roster replaced this board's copy
}

// WriteOutbox writes each queued packet to a JSON file in dir and clears the
// world's Outbox. How the files then reach other boards (a sync tool, FidoNet,
// scp, a shared mount) is the operator's concern — this is the Option A
// transport from the design spec.
func WriteOutbox(w *game.World, dir string) error {
	if len(w.Outbox) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, p := range w.Outbox {
		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		to := p.ToBoard
		if to == "" {
			to = "all"
		}
		name := fmt.Sprintf("%s-to-%s-%s-%d%s", sanitize(w.Config.BoardID), sanitize(to), sanitize(p.Date), i, packetExt)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return err
		}
	}
	w.Outbox = nil
	return nil
}

// ReadInbound reads every packet file in dir addressed to this board (or
// broadcast), applies it, queues any result packet to the world's Outbox, and
// removes the consumed files. Packets addressed elsewhere are left in place.
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
		if e.IsDir() || filepath.Ext(e.Name()) != packetExt {
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
		if p.ToBoard != "" && p.ToBoard != w.Config.BoardID {
			continue // not for us; leave it for the transport to route onward
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

// sameRoster reports whether two league rosters are identical, so the node-list
// file is only rewritten when a broadcast actually changed it.
func sameRoster(a, b []game.LeagueNode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
