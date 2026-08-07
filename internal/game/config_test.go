package game

import "testing"

// A packet directory hangs off the data directory, so a door launched from a
// node temp dir still finds it; an absolute one is left alone.
func TestPacketDirsResolveAgainstDataDir(t *testing.T) {
	c := DefaultConfig()
	c.DataDir = "/srv/bbs/ib-data"
	if got, want := c.Inbound(), "/srv/bbs/ib-data/inbound"; got != want {
		t.Errorf("Inbound() = %q, want %q", got, want)
	}
	c.OutboundDir = "/var/spool/ibbs/out"
	if got := c.Outbound(); got != "/var/spool/ibbs/out" {
		t.Errorf("an absolute OutboundDir should be used as given, got %q", got)
	}
}
