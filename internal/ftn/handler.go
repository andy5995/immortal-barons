package ftn

import (
	"path/filepath"
	"slices"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// Queued records one transport bundle handed to the FTN mailer.
type Queued struct {
	PacketPath string
	NextHop    string
	Address    Address
	Message    string
}

// Result describes one inbound or outbound transport scan.
type Result struct {
	Queued    []Queued
	Delivered int
	// Snapshots and Waiting describe outbound work this run could not finish:
	// how many claimed snapshots still hold an unpublished target, and which
	// peers those targets are for. A snapshot is kept whole until every one of
	// its targets is published, so one unreachable peer retains the packets and
	// bundles of the peers that did go out — which makes a raw file count
	// overstate the backlog and a bare "queued 0" indistinguishable from an
	// empty system (#228).
	Snapshots int
	Waiting   []string
	// Stalled names the peers that have a recorded reason for being behind,
	// each with that reason, and OldestWait is how long the least recently
	// advanced snapshot has gone without publishing anything. Together they
	// separate "a peer is offline this hour" from "nothing has moved for days",
	// which a count alone cannot (#228).
	Stalled    []string
	OldestWait time.Duration
	// Warnings report recoverable packet or configuration problems.
	Warnings []string
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

func nodeByNumber(nodes []game.LeagueNode, number int) *game.LeagueNode {
	for i := range nodes {
		if nodes[i].Number == number {
			return &nodes[i]
		}
	}
	return nil
}
