package game

import (
	"fmt"
	"sort"
	"strings"
)

// RecordedTimeFormat is how these reports stamp a time, copied from the
// original's own BBSINFO.LST: MM/DD/YYYY HH:MM:SS.
const RecordedTimeFormat = "01/02/2006 15:04:05"

// The three sysop reports the original offers on its command line, described in
// its own documentation (docs/bre.doc, "Command-Line Options"):
//
//	LASTPACKET  the last date a packet from every other BBS was processed
//	BBSINFO     every BBS, its version, and the date of its last recon
//	PLAYERLIST  every player on every BBS — League Coordinator only
//
// The original writes each to a text file, and so does IB. They answer the
// questions a league sysop actually asks when traffic stops: who has gone
// quiet, who is running something too old to understand our packets, and who is
// playing. Nothing here changes game state — they are pure reads over what the
// inbound packets already told this board.

// reportHeader is the common two-line heading: what the report is, and which
// board and day produced it, since these files get mailed between sysops and a
// bare table says nothing about where it came from.
func (w *World) reportHeader(title string) string {
	board := w.Config.BoardID
	if board == "" {
		board = "this board"
	}
	day := w.LastMaintDate
	if day == "" {
		day = "no game day yet"
	}
	return fmt.Sprintf("%s\n%s\nfor %s, %s\n\n", title, strings.Repeat("=", len(title)), board, day)
}

// knownPeers is every other board this one has heard of, roster first so the
// order is the league's own, then any board that has written to us without
// being on it — which is itself worth seeing.
func (w *World) knownPeers() []string {
	var out []string
	seen := map[string]bool{w.Config.BoardID: true}
	for _, n := range w.LeagueNodes {
		if !seen[n.Name] {
			seen[n.Name] = true
			out = append(out, n.Name)
		}
	}
	var extra []string
	for _, b := range w.RemoteBoards {
		if !seen[b.BoardID] {
			seen[b.BoardID] = true
			extra = append(extra, b.BoardID)
		}
	}
	sort.Strings(extra)
	return append(out, extra...)
}

// LastPacketReport lists when a packet from each other board was last
// PROCESSED here — not the date the sender stamped on it. The two differ
// exactly when traffic is stuck, which is the case the report exists for.
func (w *World) LastPacketReport() string {
	var b strings.Builder
	b.WriteString(w.reportHeader("Last Packet Processed"))
	peers := w.knownPeers()
	if len(peers) == 0 {
		b.WriteString("No other boards are known yet.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%-28s %-12s %s\n", "Planet", "Processed", "Sequence")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 52))
	for _, name := range peers {
		when := w.LastPacketFrom[name]
		if when == "" {
			when = "never"
		}
		seq := "-"
		if n, ok := w.HighSeq[name]; ok {
			seq = fmt.Sprintf("%d", n)
		}
		fmt.Fprintf(&b, "%-28s %-12s %s\n", name, when, seq)
	}
	return b.String()
}

// BBSInfoReport is the original's BBSINFO.LST: every board, when we last heard
// from it, and the version it is running.
//
// The layout is from a live capture of the original's own file (a 12-board
// league): a right-aligned number and ")", the board name, a MM/DD/YYYY
// HH:MM:SS timestamp under "Last Recon", and a "v"-prefixed version. One row in
// that capture is printed in red; what marks a row that way is NOT known, so IB
// colours nothing rather than invent a rule.
//
// A version older than this board's is worth a sysop's eye: the packet format
// has gained fields, and a board that predates them cannot verify a packet
// carrying them (docs/dev/ibbs-packet-format.md).
func (w *World) BBSInfoReport() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%3s %-28s %-22s %s\n", "###", "BBS Name", "Last Recon", "BRE Version")
	peers := w.knownPeers()
	if len(peers) == 0 {
		b.WriteString("No other boards are known yet.\n")
		return b.String()
	}
	node := map[string]int{}
	for _, n := range w.LeagueNodes {
		node[n.Name] = n.Number
	}
	for i, name := range peers {
		num := i + 1
		if n, ok := node[name]; ok {
			num = n
		}
		when := w.LastPacketFrom[name]
		if when == "" {
			when = "never"
		}
		raw := w.BoardVersion[name]
		ver := "unknown"
		if raw != "" {
			ver = "v" + raw
		}
		// The report is where a Coordinator looks to find out WHO is holding the
		// league up, so a board failing the requirement says so on its own row
		// rather than only in the news when a packet bounces.
		if !w.BoardMeetsMinVersion(raw) {
			ver += fmt.Sprintf("  (below v%s)", w.Config.MinBoardVersion)
		}
		fmt.Fprintf(&b, "%2d) %-28s %-22s %s\n", num, name, when, ver)
	}
	return b.String()
}

// PlayerListReport lists every realm on every board — this one from its own
// empires, the others from the scores each last shared. The original restricts
// it to the League Coordinator; the caller enforces that, because the report
// itself is just a read.
func (w *World) PlayerListReport() string {
	var b strings.Builder
	b.WriteString(w.reportHeader("Players In This League"))
	fmt.Fprintf(&b, "%-28s %-20s %10s %10s\n", "Planet", "Realm", "Net Worth", "Score")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 72))

	local := w.Config.BoardID
	if local == "" {
		local = "this board"
	}
	rows := 0
	for _, e := range w.Empires {
		if e.Alive && e.Owner != "" {
			fmt.Fprintf(&b, "%-28s %-20s %10d %10d\n", local, e.Name, w.NetWorth(e), e.Score)
			rows++
		}
	}
	boards := append([]RemoteBoard(nil), w.RemoteBoards...)
	sort.Slice(boards, func(i, j int) bool { return boards[i].BoardID < boards[j].BoardID })
	for _, rb := range boards {
		for _, s := range rb.Scores {
			fmt.Fprintf(&b, "%-28s %-20s %10d %10d\n", rb.BoardID, s.Empire, s.NetWorth, s.Score)
			rows++
		}
	}
	if rows == 0 {
		b.WriteString("No realms are known yet.\n")
	}
	return b.String()
}

// versionAtLeast reports whether have is the same as, or newer than, want.
// Both are dotted numbers ("0.0.5"); a missing or unparseable part counts as 0,
// so "0.1" reads as "0.1.0" and a board that sends nonsense reads as ancient
// rather than as passing.
func versionAtLeast(have, want string) bool {
	part := func(v string, i int) int {
		f := strings.Split(v, ".")
		if i >= len(f) {
			return 0
		}
		n := 0
		for _, r := range f[i] {
			if r < '0' || r > '9' {
				return n
			}
			n = n*10 + int(r-'0')
		}
		return n
	}
	for i := 0; i < 3; i++ {
		h, wnt := part(have, i), part(want, i)
		if h != wnt {
			return h > wnt
		}
	}
	return true
}

// BoardMeetsMinVersion reports whether a board running `version` clears the
// League Coordinator's requirement, and is the single place that decides it.
//
// A board that states NO version fails a set requirement. That is the point
// rather than a side effect: it predates boards saying so at all, which puts it
// below any version a Coordinator would think to require, and a board that
// cannot state its version certainly cannot prove it meets the bar.
func (w *World) BoardMeetsMinVersion(version string) bool {
	if w.Config.MinBoardVersion == "" {
		return true
	}
	if version == "" {
		return false
	}
	return versionAtLeast(version, w.Config.MinBoardVersion)
}
