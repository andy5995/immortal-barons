package ftn

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

const (
	testOutboundDir = "o"
	testNetmailDir  = "n"
)

// hostRoster puts HOST routing in the test roster: Alpha (node 1) forwards for
// Bravo and Charlie, so a packet Bravo sends to Charlie goes up to Alpha first.
// Without it the roster is a mesh and every board links directly.
func hostRoster(t *testing.T, data string) {
	t.Helper()
	roster := "1 HOST 2 3\nAlpha BBS\n1:229/100\nDetroit\nMI\nUSA\n\n" +
		"2\nBravo BBS\n1:229/200\nLansing\nMI\nUSA\n\n" +
		"3\nCharlie BBS\n1:229/300\nFlint\nMI\nUSA\n"
	if err := os.WriteFile(filepath.Join(data, store.NodeListFile), []byte(roster), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunMovesAndRoutesPacket(t *testing.T) {
	data := newTestSetup(t)
	hostRoster(t, data) // Alpha hosts the other two, so Bravo's traffic goes up
	packet := game.Packet{FromBoard: "Bravo BBS", ToBoard: "Old Name", FromNode: 2, ToNode: 3}
	source := writePacket(t, data, packet)

	result, err := Run(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Queued) != 1 {
		t.Fatalf("queued = %d, want 1", len(result.Queued))
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source packet still exists: %v", err)
	}
	queued := result.Queued[0]
	if queued.NextHop != "Alpha BBS" || queued.Address.String() != "1:229/100" {
		t.Errorf("next hop = %q %s, want Alpha BBS 1:229/100", queued.NextHop, queued.Address)
	}
	if filepath.Dir(queued.PacketPath) != filepath.Join(data, testOutboundDir, "fido") {
		t.Errorf("packet moved to %q", queued.PacketPath)
	}
	message, err := os.ReadFile(queued.Message)
	if err != nil {
		t.Fatal(err)
	}
	if got := cString(message[72:144]); got != queued.PacketPath {
		t.Errorf("attachment subject = %q, want %q", got, queued.PacketPath)
	}
	if got := binary.LittleEndian.Uint16(message[166:168]); got != 100 {
		t.Errorf("destination node = %d, want routed node 100", got)
	}
}

func TestConcurrentRunsQueuePacketOnce(t *testing.T) {
	data := newTestSetup(t)
	writePacket(t, data, game.Packet{FromNode: 2, ToNode: 3})

	const runners = 12
	var wg sync.WaitGroup
	results := make(chan Result, runners)
	errs := make(chan error, runners)
	for range runners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := Run(data)
			results <- result
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("Run: %v", err)
		}
	}
	queued := 0
	for result := range results {
		queued += len(result.Queued)
	}
	if queued != 1 {
		t.Fatalf("concurrent runs queued %d messages, want 1", queued)
	}
	messages, err := filepath.Glob(filepath.Join(data, testNetmailDir, "*.msg"))
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("message files = %d, want 1", len(messages))
	}
}

func TestBroadcastGetsOneAttachmentPerOtherBoard(t *testing.T) {
	data := newTestSetup(t)
	// Sparse roster numbers must not make the attachment name longer. The
	// transport copy is identified by its dense recipient position instead.
	roster := "1\nAlpha BBS\n1:229/100\nDetroit\nMI\nUSA\n\n" +
		"2\nBravo BBS\n1:229/200\nLansing\nMI\nUSA\n\n" +
		"999\nCharlie BBS\n1:229/300\nFlint\nMI\nUSA\n"
	if err := os.WriteFile(filepath.Join(data, store.NodeListFile), []byte(roster), 0o644); err != nil {
		t.Fatal(err)
	}
	writePacket(t, data, game.Packet{FromBoard: "Bravo BBS", FromNode: 2})

	result, err := Run(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Queued) != 2 {
		t.Fatalf("broadcast messages = %d, want one for each of two peers", len(result.Queued))
	}
	want := []struct {
		name    string
		address string
		node    uint16
		packet  string
	}{
		{"Alpha BBS", "1:229/100", 100, "packet.brp"},
		{"Charlie BBS", "1:229/300", 300, "packet0.brp"},
	}
	seenPaths := map[string]bool{}
	for i, queued := range result.Queued {
		if queued.NextHop != want[i].name || queued.Address.String() != want[i].address {
			t.Errorf("message %d = %q %s, want %q %s", i, queued.NextHop, queued.Address, want[i].name, want[i].address)
		}
		if seenPaths[queued.PacketPath] {
			t.Errorf("two broadcast messages share attachment %s", queued.PacketPath)
		}
		seenPaths[queued.PacketPath] = true
		if got := filepath.Base(queued.PacketPath); got != want[i].packet {
			t.Errorf("message %d attachment = %q, want %q", i, got, want[i].packet)
		}
		message, err := os.ReadFile(queued.Message)
		if err != nil {
			t.Fatal(err)
		}
		if got := binary.LittleEndian.Uint16(message[166:168]); got != want[i].node {
			t.Errorf("message %d destination node = %d, want %d", i, got, want[i].node)
		}
	}
}

func TestBroadcastCopyPathUsesDenseBase36Index(t *testing.T) {
	path := filepath.Join("out", "packet.brp")
	for _, tc := range []struct {
		index int
		name  string
	}{
		{0, "packet0.brp"},
		{35, "packetz.brp"},
		{36, "packet10.brp"},
	} {
		want := filepath.Join("out", tc.name)
		if got := broadcastCopyPath(path, tc.index); got != want {
			t.Errorf("broadcastCopyPath(%d) = %q, want %q", tc.index, got, want)
		}
	}
}

func TestBadPacketIsRestoredToOutbound(t *testing.T) {
	data := newTestSetup(t)
	source := filepath.Join(data, testOutboundDir, "bad.brp")
	if err := os.WriteFile(source, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(data); err == nil {
		t.Fatal("Run accepted malformed packet")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("malformed packet was not restored: %v", err)
	}
}

func TestRunPreflightsEverySubjectBeforeMovingPackets(t *testing.T) {
	data := newTestSetup(t)
	outbound := filepath.Join(data, testOutboundDir)
	good := filepath.Join(outbound, "a.brp")
	body, err := json.Marshal(game.Packet{FromNode: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(good, body, 0o644); err != nil {
		t.Fatal(err)
	}

	// Make the base attachment fit exactly. The one-character copy index needed
	// by the second broadcast recipient then exceeds the Type-2 subject field.
	fidoDir := filepath.Join(outbound, "fido")
	stemBytes := type2SubjectSize - 1 - len(fidoDir) - 1 - len(store.PacketExt)
	if stemBytes < 1 {
		t.Fatalf("test path %q is already too long", fidoDir)
	}
	longSource := filepath.Join(outbound, strings.Repeat("z", stemBytes)+store.PacketExt)
	if err := os.WriteFile(longSource, body, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(data); err == nil {
		t.Fatal("Run accepted an overlong broadcast attachment")
	} else if !strings.Contains(err.Error(), "broadcast attachment for node 3") ||
		!strings.Contains(err.Error(), "permits at most 71") {
		t.Fatalf("Run error did not identify the subject limit: %v", err)
	}
	for _, path := range []string{good, longSource} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("preflight moved %s: %v", path, err)
		}
	}
	if messages, err := filepath.Glob(filepath.Join(data, testNetmailDir, "*.msg")); err != nil {
		t.Fatal(err)
	} else if len(messages) != 0 {
		t.Errorf("preflight created messages: %v", messages)
	}
}

func TestAttachDirAndBasenameSubject(t *testing.T) {
	data := newTestSetup(t, "AttachDir hold\nSubjectPath Basename\n")
	writePacket(t, data, game.Packet{FromBoard: "Bravo BBS", ToBoard: "Old Name", FromNode: 2, ToNode: 3})

	result, err := Run(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Queued) != 1 {
		t.Fatalf("queued = %d, want 1", len(result.Queued))
	}
	queued := result.Queued[0]
	if want := filepath.Join(data, "hold"); filepath.Dir(queued.PacketPath) != want {
		t.Errorf("packet moved to %q, want a file in %q", queued.PacketPath, want)
	}
	if _, err := os.Stat(queued.PacketPath); err != nil {
		t.Errorf("attachment is not where the subject says: %v", err)
	}
	message, err := os.ReadFile(queued.Message)
	if err != nil {
		t.Fatal(err)
	}
	if got := cString(message[72:144]); got != "packet.brp" {
		t.Errorf("attachment subject = %q, want the basename alone", got)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("basename subjects warned about their margin: %v", result.Warnings)
	}
}

func TestSubjectPrefixIsWrittenNotResolved(t *testing.T) {
	data := newTestSetup(t, "AttachDir hold\nSubjectPath ib/out\n")
	writePacket(t, data, game.Packet{FromBoard: "Bravo BBS", FromNode: 2, ToNode: 3})

	result, err := Run(data)
	if err != nil {
		t.Fatal(err)
	}
	message, err := os.ReadFile(result.Queued[0].Message)
	if err != nil {
		t.Fatal(err)
	}
	if got := cString(message[72:144]); got != "ib/out/packet.brp" {
		t.Errorf("attachment subject = %q, want the operator's prefix and the basename", got)
	}
}

// The incident from #141: an absolute subject that fitted at outbound sequence
// 99 broke at 100, with nothing reconfigured. The queued name is the one that
// took the live board down.
func TestBasenameSubjectSurvivesTheSequenceDigit(t *testing.T) {
	const (
		attachDir = "AttachDir " + "ibout-a-conventional-enough-directory\n"
		binkley   = "Binkley Yes\n"
		name      = "L100-Bravo_BBS-to-all-2026-08-16-100-0.brp"
	)
	packet := game.Packet{FromBoard: "Bravo BBS", FromNode: 2, ToNode: 3}
	body, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}

	absolute := newTestSetup(t, attachDir+binkley)
	source := filepath.Join(absolute, testOutboundDir, name)
	if err := os.WriteFile(source, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(absolute); err == nil {
		t.Fatal("the absolute spelling fitted; this test no longer reproduces the incident")
	} else if !strings.Contains(err.Error(), "SubjectPath") {
		t.Errorf("error does not name the setting that fixes it: %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Errorf("preflight moved the packet before refusing: %v", err)
	}

	short := newTestSetup(t, attachDir+binkley+"SubjectPath Basename\n")
	if err := os.WriteFile(filepath.Join(short, testOutboundDir, name), body, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Run(short)
	if err != nil {
		t.Fatalf("the same packet still fails under basename subjects: %v", err)
	}
	if len(result.Queued) != 1 {
		t.Fatalf("queued = %d, want 1", len(result.Queued))
	}
	message, err := os.ReadFile(result.Queued[0].Message)
	if err != nil {
		t.Fatal(err)
	}
	if got := cString(message[72:144]); got != "^"+name {
		t.Errorf("attachment subject = %q, want the Binkley-prefixed basename", got)
	}
}

func TestThinSubjectMarginIsReported(t *testing.T) {
	data := newTestSetup(t)
	const spare = 3
	fido := filepath.Join(data, testOutboundDir, fidoSubdir)
	stem := type2SubjectSize - 1 - spare - len(fido) - 1 - len(store.PacketExt)
	if stem < 1 {
		t.Fatalf("test path %q is already too long", fido)
	}
	body, err := json.Marshal(game.Packet{FromBoard: "Bravo BBS", FromNode: 2, ToNode: 3})
	if err != nil {
		t.Fatal(err)
	}
	name := strings.Repeat("z", stem) + store.PacketExt
	if err := os.WriteFile(filepath.Join(data, testOutboundDir, name), body, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Queued) != 1 {
		t.Fatalf("queued = %d, want 1", len(result.Queued))
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "3 byte") {
		t.Fatalf("warnings = %v, want one naming the 3 bytes left", result.Warnings)
	}
}

// TestCopyFileExclusiveReusesByteIdenticalExistingFile is half of #198: a
// file already sitting at destination that matches source byte-for-byte is
// an earlier attempt at this exact copy, not a collision -- reuse it rather
// than failing.
func TestCopyFileExclusiveReusesByteIdenticalExistingFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "packet.brp")
	destination := filepath.Join(dir, "packet0.brp")
	data := []byte(`{"fromBoard":"Bravo BBS"}`)
	if err := os.WriteFile(source, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyFileExclusive(source, destination); err != nil {
		t.Fatalf("copyFileExclusive refused a byte-identical existing copy: %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Errorf("destination content = %q, want unchanged %q", got, data)
	}
}

// TestCopyFileExclusiveReplacesStaleDifferentContent is the other half of
// #198: nothing but this exact call ever writes to a broadcastCopyPath
// destination, so content that does not match source is provably stale --
// most plausibly left by a run that was killed before its own cleanup ran,
// then collided identically on every later run since broadcastCopyPath's
// naming is deterministic from source's own filename. It is safe, and
// necessary, to clear and replace rather than fail forever.
func TestCopyFileExclusiveReplacesStaleDifferentContent(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "packet.brp")
	destination := filepath.Join(dir, "packet0.brp")
	data := []byte(`{"fromBoard":"Bravo BBS","seq":9}`)
	if err := os.WriteFile(source, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("stale garbage from before a routing change"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyFileExclusive(source, destination); err != nil {
		t.Fatalf("copyFileExclusive did not recover from a stale leftover: %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Errorf("destination content = %q, want the current source content %q", got, data)
	}
}

// TestCopyFileExclusiveReplacesUnreadableExistingFile is review on #225:
// copyFileExclusive originally gave up and returned the original EEXIST
// error the moment os.ReadFile(destination) failed for ANY reason --
// permission, a directory sitting at that path, a disk fault -- leaving the
// file untouched. Since the destination is deterministic from source's own
// filename, the next run hits the exact same unreadable file and fails
// identically: the same "collide forever" failure #198 exists to fix, just
// reached through an unreadable file instead of a readable-but-different
// one. An unreadable destination must be cleared and replaced the same way,
// not given up on.
func TestCopyFileExclusiveReplacesUnreadableExistingFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores the read-permission bit this test relies on")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "packet.brp")
	destination := filepath.Join(dir, "packet0.brp")
	data := []byte(`{"fromBoard":"Bravo BBS","seq":9}`)
	if err := os.WriteFile(source, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("unreadable leftover"), 0o000); err != nil {
		t.Fatal(err)
	}
	if err := copyFileExclusive(source, destination); err != nil {
		t.Fatalf("copyFileExclusive gave up on an unreadable leftover instead of clearing it: %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Errorf("destination content = %q, want the current source content %q", got, data)
	}
}

// TestRunSkipsPacketWithBadDestinationButStillQueuesOthers is #198's other
// half: not just the copyFileExclusive collision itself, but the surrounding
// per-candidate loop in Run that used to abort the WHOLE run on any one
// candidate's queueClaimed failure. A packet addressed to a node no longer in
// the roster is a real, still-possible failure independent of the
// copyFileExclusive fix above -- it must not stop an unrelated packet in the
// same run from going out, and the failed one must come back to source
// intact to retry, not be left stranded at its claimed path.
func TestRunSkipsPacketWithBadDestinationButStillQueuesOthers(t *testing.T) {
	data := newTestSetup(t)
	outbound := filepath.Join(data, testOutboundDir)

	goodBody, err := json.Marshal(game.Packet{FromBoard: "Bravo BBS", FromNode: 2, ToNode: 1})
	if err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(outbound, "good.brp")
	if err := os.WriteFile(good, goodBody, 0o644); err != nil {
		t.Fatal(err)
	}

	// Node 999 is not in this test's roster (Alpha/Bravo/Charlie, 1-3):
	// preflightPacket does not resolve destinations against the roster, so
	// this reaches queueClaimed as a genuine, real failure there.
	badBody, err := json.Marshal(game.Packet{FromBoard: "Bravo BBS", FromNode: 2, ToNode: 999})
	if err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(outbound, "bad.brp")
	if err := os.WriteFile(bad, badBody, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(data)
	if err != nil {
		t.Fatalf("Run refused the whole batch over one packet's bad destination: %v", err)
	}
	if len(result.Queued) != 1 {
		t.Fatalf("queued = %d, want 1 (the packet with a real destination)", len(result.Queued))
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "destination node 999 is not in") {
		t.Fatalf("warnings = %v, want one naming the bad destination", result.Warnings)
	}
	if _, err := os.Stat(good); !os.IsNotExist(err) {
		t.Errorf("the good packet was not moved: %v", err)
	}
	if _, err := os.Stat(bad); err != nil {
		t.Errorf("the packet with a bad destination was not restored to source: %v", err)
	}
}

func newTestSetup(t *testing.T, extraFTN ...string) string {
	t.Helper()
	tempRoot := os.TempDir()
	if filepath.VolumeName(tempRoot) == "" {
		// Darwin's system temp directory alone exceeds the 71-byte Subject
		// limit. Use the short conventional Unix path; Windows needs its
		// volume-qualified temp directory.
		tempRoot = "/tmp"
	}
	data, err := os.MkdirTemp(tempRoot, "i")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(data) })
	for _, dir := range []string{testOutboundDir, testNetmailDir} {
		if err := os.Mkdir(filepath.Join(data, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"config.json":         "{}\n",
		store.BoardConfigFile: "BoardID Bravo BBS\nOutbound " + testOutboundDir + "\n",
		ConfigFile:            "NetmailDir " + testNetmailDir + "\n" + strings.Join(extraFTN, ""),
		store.NodeListFile: "1\nAlpha BBS\n1:229/100\nDetroit\nMI\nUSA\n\n" +
			"2\nBravo BBS\n1:229/200\nLansing\nMI\nUSA\n\n" +
			"3\nCharlie BBS\n1:229/300\nFlint\nMI\nUSA\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(data, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return data
}

func writePacket(t *testing.T, data string, packet game.Packet) string {
	t.Helper()
	body, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(data, testOutboundDir, "packet.brp")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
