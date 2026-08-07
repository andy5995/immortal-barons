package game

import (
	"path/filepath"
	"testing"
)

// A packet directory hangs off the data directory, so a door launched from a
// node temp dir still finds it; an absolute one is left alone.
//
// Both the separator and what counts as absolute are the host's: a Windows
// board writes C:\bbs\in, and "/var/spool" is a relative path there. So the
// expectations are built with filepath rather than written out, and the
// absolute case uses a directory the OS itself calls absolute.
func TestPacketDirsResolveAgainstDataDir(t *testing.T) {
	c := DefaultConfig()
	c.DataDir = filepath.Join("srv", "bbs", "ib-data")

	want := filepath.Join(c.DataDir, "inbound")
	if got := c.Inbound(); got != want {
		t.Errorf("Inbound() = %q, want %q", got, want)
	}

	abs := t.TempDir() // absolute on every OS the game builds for
	if !filepath.IsAbs(abs) {
		t.Fatalf("t.TempDir() gave a relative path: %q", abs)
	}
	c.OutboundDir = abs
	if got := c.Outbound(); got != abs {
		t.Errorf("an absolute OutboundDir should be used as given, got %q", got)
	}
}
