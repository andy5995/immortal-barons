package ftn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// agedPacket writes a packet-named file. It cannot make one OLD: age is taken
// from the change time (see arrivedAt), which the filesystem sets and no call
// can move backwards. Tests that need an aged file shorten the threshold with
// reportEverything instead.
func agedPacket(t *testing.T, dir, name string, _ time.Duration) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("not a real bundle"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// arrivalIsDistinguishable reports whether this platform can tell when a file
// arrived from when its contents were last written. Where it cannot, arrivedAt
// falls back to the modification time and the false alarm below is not
// preventable, so the test has nothing to assert rather than something to fail.
func arrivalIsDistinguishable(t *testing.T) bool {
	t.Helper()
	path := filepath.Join(t.TempDir(), "probe.brp")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-9 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return time.Since(arrivedAt(info)) < time.Hour
}

// reportEverything drops the age threshold for one test.
func reportEverything(t *testing.T) {
	t.Helper()
	previous := unclaimedAfter
	unclaimedAfter = 0
	t.Cleanup(func() { unclaimedAfter = previous })
}

// The failure #236 records: bundles collect in the mailer's inbound and every
// report a sysop reaches for says the transport is healthy.
func TestStatusReportsAPacketNobodyHasClaimed(t *testing.T) {
	reportEverything(t)
	data := newBundledSetup(t, "Bravo BBS", "")
	agedPacket(t, filepath.Join(data, "transport-in"), "stranded.brp", 5*time.Hour)

	status, err := Status(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Unclaimed) != 1 {
		t.Fatalf("unclaimed = %+v, want the one stranded bundle", status.Unclaimed)
	}
	got := status.Unclaimed[0]
	if filepath.Base(got.Path) != "stranded.brp" {
		t.Errorf("path = %q, want stranded.brp", got.Path)
	}
	if got.Age < 0 {
		t.Errorf("age = %v, want a duration since the file arrived", got.Age)
	}
	if got.Subdir != "" {
		t.Errorf("subdir = %q, want empty for a file in InboundDir itself", got.Subdir)
	}
}

// The second route to the same silence, found on the four-board rig: a mailer
// files an unauthenticated session's files into a child directory, and RunIn
// enumerates only InboundDir itself, so no number of later runs will take them.
func TestStatusReportsAPacketInAChildOfTheInboundDirectory(t *testing.T) {
	reportEverything(t)
	data := newBundledSetup(t, "Bravo BBS", "")
	agedPacket(t, filepath.Join(data, "transport-in", "unsecure"), "quarantined.brp", 3*time.Hour)

	status, err := Status(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Unclaimed) != 1 {
		t.Fatalf("unclaimed = %+v, want the quarantined bundle", status.Unclaimed)
	}
	if status.Unclaimed[0].Subdir != "unsecure" {
		t.Errorf("subdir = %q, want the child directory named", status.Unclaimed[0].Subdir)
	}
}

// A bundle and its envelope can arrive in either order and an exchange runs on
// a schedule, so a file that has only just landed is not a fault. Reporting one
// is how a monitor teaches its reader to ignore it.
func TestStatusIgnoresAPacketThatHasOnlyJustArrived(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "")
	agedPacket(t, filepath.Join(data, "transport-in"), "fresh.brp", time.Minute)

	status, err := Status(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Unclaimed) != 0 {
		t.Errorf("unclaimed = %+v, want nothing for a file one minute old", status.Unclaimed)
	}
}

// A receipt is kept until its cleanup succeeds, so a source bundle can still be
// in the inbound while the transfer is healthy. Reporting it as abandoned would
// send a sysop after a file that is mid-flight.
func TestStatusDoesNotReportABundleAReceiptStillHolds(t *testing.T) {
	reportEverything(t)
	data := newBundledSetup(t, "Bravo BBS", "")
	path := agedPacket(t, filepath.Join(data, "transport-in"), "inflight.brp", 6*time.Hour)

	dir := filepath.Join(data, spoolDir, inSpoolDir, "abc123")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	source, err := json.Marshal(path)
	if err != nil {
		t.Fatal(err)
	}
	receipt := `{"id":"abc123","source":` + string(source) + `,"local":[],"targets":[]}`
	if err := os.WriteFile(filepath.Join(dir, inboundReceiptFile), []byte(receipt), 0o644); err != nil {
		t.Fatal(err)
	}

	status, err := Status(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Unclaimed) != 0 {
		t.Errorf("unclaimed = %+v, want nothing: a receipt still accounts for it", status.Unclaimed)
	}
}

// The warning has to reach the run a sysop actually reads, not only --status.
func TestRunInWarnsAboutAPacketItPassedOver(t *testing.T) {
	reportEverything(t)
	data := newBundledSetup(t, "Bravo BBS", "")
	agedPacket(t, filepath.Join(data, "transport-in", "unsecure"), "quarantined.brp", 2*time.Hour)
	writeNamedPacket(t, data, "outgoing.brp", game.Packet{FromNode: 2, ToNode: 1, Seq: 1, League: 100})

	result, err := RunIn(data)
	if err != nil {
		t.Fatal(err)
	}
	var found string
	for _, w := range result.Warnings {
		if strings.Contains(w, "quarantined.brp") {
			found = w
		}
	}
	if found == "" {
		t.Fatalf("warnings = %+v, want one naming the unclaimed bundle", result.Warnings)
	}
	if !strings.Contains(found, "unsecure") {
		t.Errorf("warning does not name the subdirectory: %q", found)
	}
	if !strings.Contains(found, "no later run will take it") {
		t.Errorf("warning does not say that waiting will not help: %q", found)
	}
}

// #236 names -league-check among the reports that stayed silent, so the check
// that a bundle is a bundle before advising unzip belongs beside it: a plain
// packet and a truncated download both land here too.
func TestUnclaimedWarningOffersTheManifestOnlyForABundle(t *testing.T) {
	reportEverything(t)
	data := newBundledSetup(t, "Bravo BBS", "")
	in := filepath.Join(data, "transport-in")
	plain := agedPacket(t, in, "plain.brp", 4*time.Hour)
	if err := os.WriteFile(plain, []byte(`{"FromNode":2,"ToNode":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle := agedPacket(t, in, "bundle.brp", 4*time.Hour)
	if err := os.WriteFile(bundle, []byte("PK\x03\x04rest of a zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path      string
		wantUnzip bool
	}{{plain, false}, {bundle, true}} {
		got := unclaimedWarning(Unclaimed{Path: tc.path, Age: 4 * time.Hour})
		if strings.Contains(got, "unzip -p") != tc.wantUnzip {
			t.Errorf("%s: unzip advice = %v, want %v\n%s",
				filepath.Base(tc.path), !tc.wantUnzip, tc.wantUnzip, got)
		}
	}
}

// A path with a space has to survive being pasted, and an attach bundle whose
// envelope is already here is waiting for the next run, not abandoned.
func TestStatusQuotesPathsAndSpareAnEnvelopedBundle(t *testing.T) {
	if got := shellQuote("/var/spool/ftn in/X.brp"); got != `'/var/spool/ftn in/X.brp'` {
		t.Errorf("shellQuote = %s, want the path in single quotes", got)
	}
	if got := shellQuote("/var/spool/ftn/X.brp"); got != "/var/spool/ftn/X.brp" {
		t.Errorf("shellQuote = %s, want a plain path left alone", got)
	}
}

// Age comes from the change time, so a file the mailer delivered a moment ago
// with an old modification time -- which is what binkp gives it -- is not
// reported. This is the false alarm the threshold exists to prevent.
func TestAFreshlyDeliveredFileWithAnOldModTimeIsNotReported(t *testing.T) {
	if !arrivalIsDistinguishable(t) {
		t.Skip("this platform has no arrival time apart from the modification time")
	}
	data := newBundledSetup(t, "Bravo BBS", "")
	path := agedPacket(t, filepath.Join(data, "transport-in"), "justarrived.brp", 0)
	old := time.Now().Add(-9 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	status, err := Status(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Unclaimed) != 0 {
		t.Errorf("unclaimed = %+v, want nothing: the file arrived seconds ago", status.Unclaimed)
	}
}

// The ageing itself, which no filesystem call can set up: move the clock
// instead of the file.
func TestScanUnclaimedAgesFromArrival(t *testing.T) {
	dir := t.TempDir()
	agedPacket(t, dir, "waiting.brp", 0)

	if got := scanUnclaimed([]string{dir}, nil, time.Now()); len(got) != 0 {
		t.Errorf("unclaimed = %+v, want nothing for a file that just arrived", got)
	}
	later := time.Now().Add(unclaimedAfter + time.Minute)
	got := scanUnclaimed([]string{dir}, nil, later)
	if len(got) != 1 || got[0].Age < unclaimedAfter {
		t.Errorf("unclaimed = %+v, want the file once it is past the threshold", got)
	}
}

// A mailer can deliver into more than one directory: ENiGMA files an
// authenticated session into secInbound and an unauthenticated one into
// inbound. Naming only the first reads nothing from the second while every
// report stays healthy, which is how 43 packets collected on the test rig.
func TestRunInReadsEveryInboundDirectory(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "InboundDir transport-sec\n")
	secure := filepath.Join(data, "transport-sec")
	if err := os.MkdirAll(secure, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(game.Packet{FromNode: 1, ToNode: 2, Seq: 7, League: 100})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secure, "fromsecure.brp"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := RunIn(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Delivered != 1 {
		t.Fatalf("delivered = %d, want the packet from the second inbound directory", result.Delivered)
	}
}

// The same directories are watched by the report, so a packet stranded in the
// second one is named rather than silently skipped.
func TestStatusWatchesEveryInboundDirectory(t *testing.T) {
	reportEverything(t)
	data := newBundledSetup(t, "Bravo BBS", "InboundDir transport-sec\n")
	agedPacket(t, filepath.Join(data, "transport-sec"), "stranded.brp", 0)

	status, err := Status(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Unclaimed) != 1 || filepath.Base(status.Unclaimed[0].Path) != "stranded.brp" {
		t.Errorf("unclaimed = %+v, want the packet in the second inbound directory", status.Unclaimed)
	}
}

// The envelope names a file, not a directory: an attach bundle filed into the
// second inbound directory must still be found, or it is skipped by the direct
// scan (attach is deliberately passed over there) and never ingested at all.
func TestEnvelopeAttachmentFindsTheFileInAnyInboundDirectory(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	name := "abcd0001.brp"
	if err := os.WriteFile(filepath.Join(second, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, type2HeaderSize)
	copy(data[72:144], name)

	got, err := envelopeAttachment(data, []string{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(second, name) {
		t.Errorf("resolved to %s, want the copy that exists in %s", got, second)
	}
}

// A directory named in its own right and also reachable as a child of another
// is one fault, not two.
func TestScanUnclaimedReportsAFileOnce(t *testing.T) {
	reportEverything(t)
	parent := t.TempDir()
	child := filepath.Join(parent, "unsecure")
	agedPacket(t, child, "once.brp", 0)

	got := scanUnclaimed([]string{parent, child}, nil, time.Now())
	if len(got) != 1 {
		t.Errorf("unclaimed = %+v, want the file reported once", got)
	}
}
