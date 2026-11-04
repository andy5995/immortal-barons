package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// PacketLogFile records every packet the planetary step writes and every packet
// it finds in inbound, with what became of it. A lost strike or trade deal is
// otherwise untraceable (#311): the run report's counts go to stdout, which a
// scheduler throws away, and no board kept the packet numbers it sent or took
// in. With both boards' logs, a missing packet number shows which side lost it.
//
// Kept apart from planetary.log, which holds faults: one line per packet buries
// a fault there within a few hours on a busy league.
const PacketLogFile = "packets.log"

// packetLogMaxBytes is where packets.log is moved to packets.log.1 and started
// again, so the pair stays under twice this. An estimate, not a measurement: at
// about 100 bytes a line and 30 packets a run, a board running the step every
// fifteen minutes fills it in three to four days, longer than a trade deal
// waits for its receipt.
const packetLogMaxBytes = 1 << 20

// AppendPacketLog adds one timestamped line per entry. A failure is ignored for
// the reason AppendPlanetaryLog gives.
func AppendPacketLog(dataDir string, lines []string, when time.Time) {
	if len(lines) == 0 {
		return
	}
	lock, err := lockLogs(dataDir)
	if err != nil {
		return
	}
	defer lock.Release()
	path := filepath.Join(dataDir, PacketLogFile)
	if info, err := os.Stat(path); err == nil && info.Size() >= packetLogMaxBytes {
		// A rename that fails (on Windows, a viewer holding the file open) would
		// leave the file growing on every run; past twice the cap it stops
		// taking lines instead, until a later run can move it.
		if os.Rename(path, path+".1") != nil && info.Size() >= 2*packetLogMaxBytes {
			return
		}
	}
	stamp := when.UTC().Format("2006-01-02 15:04:05 MST")
	var b strings.Builder
	for _, l := range lines {
		fmt.Fprintf(&b, "%s  %s\n", stamp, logSafe(l))
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	f.WriteString(b.String())
	f.Close()
}

// settle records an inbound packet's fate once the move or removal that
// carries it out has run, and returns that step's error. A step that failed
// left the file in inbound, and the line says so rather than claiming it.
func (r *InboundResult) settle(p game.Packet, fate string, err error) error {
	if err != nil {
		fate += fmt.Sprintf(", but the file stays in inbound: %v", err)
	}
	r.Log = append(r.Log, inLine(p, fate))
	return err
}

// PacketLogLine is one packets.log entry: its kind ("in", "out", "pass", or
// "ftn" ahead of one of those) in a fixed column, then the text. Every entry is
// built here, so the columns line up whichever part of the game wrote it.
func PacketLogLine(kind, format string, a ...any) string {
	return fmt.Sprintf("%-4s %s", kind, fmt.Sprintf(format, a...))
}

// inLine words one inbound packet's fate for the packet log.
func inLine(p game.Packet, what string) string {
	return PacketLogLine("in", "#%d from %s (%s): %s", p.Seq, p.FromBoard, p.Contents(), what)
}
