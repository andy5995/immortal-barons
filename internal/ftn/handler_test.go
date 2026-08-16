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

func TestRunMovesAndRoutesPacket(t *testing.T) {
	data := newTestSetup(t)
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
	if err := os.Remove(filepath.Join(data, store.RouteFile)); err != nil {
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
	}{
		{"Alpha BBS", "1:229/100", 100},
		{"Charlie BBS", "1:229/300", 300},
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
		message, err := os.ReadFile(queued.Message)
		if err != nil {
			t.Fatal(err)
		}
		if got := binary.LittleEndian.Uint16(message[166:168]); got != want[i].node {
			t.Errorf("message %d destination node = %d, want %d", i, got, want[i].node)
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

	// Make the base attachment fit exactly. The -n3 copy needed by the second
	// broadcast recipient then exceeds the Type-2 subject field.
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

func newTestSetup(t *testing.T) string {
	t.Helper()
	data, err := os.MkdirTemp("", "i")
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
		ConfigFile:            "NetmailDir " + testNetmailDir + "\n",
		store.RouteFile:       "ROUTE * 1\n",
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
