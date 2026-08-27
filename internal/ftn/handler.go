package ftn

import (
	"path/filepath"
	"slices"

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
