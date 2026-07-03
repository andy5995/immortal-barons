package main

import (
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// TestThreeBoardIBBSExchange simulates three BBSes running the door with
// separate data directories and distinct BoardIDs, exchanging inter-BBS
// score packets. It drives the real door code paths (runExport/runImport)
// rather than Synchronet: each board exports its packet, the packets are
// carried to the other two boards (as a sysop's mailer would), and each
// board imports the others. Afterwards every board should know the scores
// of the other two.
func TestThreeBoardIBBSExchange(t *testing.T) {
	const today = "2026-07-03"
	root := t.TempDir()

	boards := []struct{ id string }{{"alpha"}, {"beta"}, {"gamma"}}

	// Seed each board: a config with its own BoardID and some AI empires so
	// there are scores to export, then create its world file.
	cfgs := make(map[string]game.Config, len(boards))
	for _, b := range boards {
		cfg := game.DefaultConfig()
		cfg.DataDir = filepath.Join(root, b.id)
		cfg.BoardID = b.id
		cfg.AICount = 3
		if err := store.SaveConfig(cfg); err != nil {
			t.Fatalf("%s SaveConfig: %v", b.id, err)
		}
		if err := runMaint(cfg, today); err != nil { // creates the world (with AI empires)
			t.Fatalf("%s runMaint: %v", b.id, err)
		}
		cfgs[b.id] = cfg
	}

	// Each board exports its packet.
	packets := make(map[string]string, len(boards))
	for _, b := range boards {
		path := filepath.Join(root, b.id+".pkt")
		if err := runExport(cfgs[b.id], path, today); err != nil {
			t.Fatalf("%s runExport: %v", b.id, err)
		}
		packets[b.id] = path
	}

	// Carry every other board's packet in and import it.
	for _, dst := range boards {
		for _, src := range boards {
			if src.id == dst.id {
				continue
			}
			if err := runImport(cfgs[dst.id], packets[src.id]); err != nil {
				t.Fatalf("%s importing %s: %v", dst.id, src.id, err)
			}
		}
	}

	// Every board should now hold both other boards' scores.
	for _, dst := range boards {
		w, err := store.Load(cfgs[dst.id])
		if err != nil {
			t.Fatalf("%s reload: %v", dst.id, err)
		}
		if len(w.RemoteBoards) != len(boards)-1 {
			t.Errorf("%s: want %d remote boards, got %d", dst.id, len(boards)-1, len(w.RemoteBoards))
		}
		for _, src := range boards {
			if src.id == dst.id {
				continue
			}
			rb := findBoard(w.RemoteBoards, src.id)
			if rb == nil {
				t.Errorf("%s: missing remote board %q", dst.id, src.id)
				continue
			}
			if len(rb.Scores) != 3 { // AICount empires, all alive
				t.Errorf("%s: remote board %q has %d scores, want 3", dst.id, src.id, len(rb.Scores))
			}
		}
	}
}

func findBoard(boards []game.RemoteBoard, id string) *game.RemoteBoard {
	for i := range boards {
		if boards[i].BoardID == id {
			return &boards[i]
		}
	}
	return nil
}
