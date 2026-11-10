package store

import "github.com/andy5995/immortal-barons/internal/game"

// noSave stands in for the world save in tests that keep the world in memory.
func noSave() error { return nil }

// readInboundAndRemove is ReadInbound for a test with no world to save: the
// files the batch finished with are removed at once.
func readInboundAndRemove(w *game.World, dir string, verbose bool) (InboundResult, error) {
	result, err := ReadInbound(w, dir, verbose)
	if err == nil {
		result.removeTaken()
	}
	return result, err
}
