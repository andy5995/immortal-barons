package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/andy5995/immortal-barons/internal/sysop"
)

// Only an existing folder with no game gets a next step, and with no game
// program found that step is to download the game first.
func TestOpenAdvice(t *testing.T) {
	wrapped := fmt.Errorf("opening: %w", sysop.ErrNoWorld)
	for _, c := range []struct {
		name   string
		err    error
		exists bool
		prog   string
		want   string
	}{
		{"no game, no program", sysop.ErrNoWorld, true, "", adviseDownload},
		{"no game, program found", sysop.ErrNoWorld, true, "/opt/ib/immortal-barons", adviseReset},
		{"wrapped error", wrapped, true, "/opt/ib/immortal-barons", adviseReset},
		{"missing folder", sysop.ErrNoWorld, false, "", ""},
		{"other error", errors.New("permission denied"), true, "", ""},
	} {
		if got := openAdvice(c.err, c.exists, c.prog); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
