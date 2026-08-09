package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A three-board league where Alpha and Charlie only talk through Bravo. Alpha's
// packet for Charlie has to survive the hop: before #106 it was skipped and left
// to sit in Bravo's inbound directory until a sysop noticed it.
func TestAHubForwardsAPacketItIsNotAddressedTo(t *testing.T) {
	roster := []game.LeagueNode{
		{Number: 1, Name: "Alpha BBS"},
		{Number: 2, Name: "Bravo BBS", Hosts: []int{1, 3}},
		{Number: 3, Name: "Charlie BBS"},
	}
	inbound, onward := t.TempDir(), t.TempDir()

	cfg := game.DefaultConfig()
	cfg.BoardID = "Bravo BBS"
	hub := game.NewWorldSeed(cfg, 1)
	hub.LeagueNodes = roster

	writePacket(t, inbound, "alpha-to-charlie", game.Packet{
		FromBoard: "Alpha BBS",
		ToBoard:   "Charlie BBS",
		Seq:       9,
		Signature: []byte{7, 7, 7},
		Date:      "2026-08-09",
		Scores:    []game.RemoteScore{{Empire: "Apples", Land: 500}},
	})

	applied, err := ReadInbound(hub, inbound)
	if err != nil {
		t.Fatalf("ReadInbound: %v", err)
	}
	if applied != 0 {
		t.Errorf("a packet in transit is not applied here, got applied=%d", applied)
	}
	if left := packetFiles(t, inbound); len(left) != 0 {
		t.Errorf("transit packet left in the inbound directory: %v", left)
	}

	hub.StampOutbox()
	if _, err := WriteOutbox(hub, onward); err != nil {
		t.Fatalf("WriteOutbox: %v", err)
	}
	files := packetFiles(t, onward)
	if len(files) != 1 {
		t.Fatalf("wrote %d packets onward, want 1", len(files))
	}

	var got game.Packet
	readPacket(t, filepath.Join(onward, files[0]), &got)
	if got.FromBoard != "Alpha BBS" || got.ToBoard != "Charlie BBS" {
		t.Errorf("forwarded packet = %s -> %s, want Alpha BBS -> Charlie BBS", got.FromBoard, got.ToBoard)
	}
	if got.Seq != 9 || string(got.Signature) != string([]byte{7, 7, 7}) {
		t.Errorf("the hub re-stamped the packet: seq=%d sig=%v", got.Seq, got.Signature)
	}
	if got.Hops != 1 {
		t.Errorf("Hops = %d, want 1", got.Hops)
	}
	if len(got.Scores) != 1 || got.Scores[0].Empire != "Apples" {
		t.Errorf("payload did not survive the hop: %+v", got.Scores)
	}
}

// Without routing the transport copies every packet to every board, so one
// addressed elsewhere is a copy its own board already got directly. Forwarding
// it would put it back into the same fan-out, so it is left where it is.
func TestAMeshBoardDoesNotForwardAPacketForSomeoneElse(t *testing.T) {
	inbound := t.TempDir()
	cfg := game.DefaultConfig()
	cfg.BoardID = "Bravo BBS"
	w := game.NewWorldSeed(cfg, 1)
	w.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Alpha BBS"},
		{Number: 2, Name: "Bravo BBS"},
		{Number: 3, Name: "Charlie BBS"},
	}
	writePacket(t, inbound, "alpha-to-charlie", game.Packet{FromBoard: "Alpha BBS", ToBoard: "Charlie BBS"})

	if _, err := ReadInbound(w, inbound); err != nil {
		t.Fatalf("ReadInbound: %v", err)
	}
	if len(w.Transit) != 0 {
		t.Errorf("queued %d packets for forwarding, want 0", len(w.Transit))
	}
	if len(packetFiles(t, inbound)) != 1 {
		t.Error("the packet should have been left where it was")
	}
}

// The other half of the same gate, and the one that protects every league
// already running: without HOST routing a broadcast stays ONE file, because the
// transport is what copies it to every board. Addressing it per planet here
// would have that transport deliver each copy everywhere.
func TestAMeshBoardWritesOneBroadcast(t *testing.T) {
	out := t.TempDir()
	cfg := game.DefaultConfig()
	cfg.BoardID = "Alpha BBS"
	w := game.NewWorldSeed(cfg, 1)
	w.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Alpha BBS"},
		{Number: 2, Name: "Bravo BBS"},
		{Number: 3, Name: "Charlie BBS"},
	}
	w.Outbox = []game.Packet{{FromBoard: "Alpha BBS", Date: "2026-08-09",
		Scores: []game.RemoteScore{{Empire: "Apples", Land: 500}}}}
	w.StampOutbox()
	if _, err := WriteOutbox(w, out); err != nil {
		t.Fatalf("WriteOutbox: %v", err)
	}

	files := packetFiles(t, out)
	if len(files) != 1 {
		t.Fatalf("wrote %d files, want 1: %v", len(files), files)
	}
	if !strings.Contains(files[0], "-to-all-") {
		t.Errorf("the broadcast was addressed to a board: %q", files[0])
	}
	var got game.Packet
	readPacket(t, filepath.Join(out, files[0]), &got)
	if got.ToBoard != "" {
		t.Errorf("ToBoard = %q, want empty (a broadcast)", got.ToBoard)
	}
}

// Once the Coordinator writes a HOST line, the same broadcast is addressed per
// planet instead: only the game knows the tree, so the transport cannot fan out
// along it.
func TestARoutedBoardAddressesABroadcastPerPlanet(t *testing.T) {
	out := t.TempDir()
	cfg := game.DefaultConfig()
	cfg.BoardID = "Alpha BBS"
	w := game.NewWorldSeed(cfg, 1)
	w.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Alpha BBS", Hosts: []int{2, 3}},
		{Number: 2, Name: "Bravo BBS"},
		{Number: 3, Name: "Charlie BBS"},
	}
	w.Outbox = []game.Packet{{FromBoard: "Alpha BBS", Date: "2026-08-09",
		Scores: []game.RemoteScore{{Empire: "Apples", Land: 500}}}}
	w.StampOutbox()
	if _, err := WriteOutbox(w, out); err != nil {
		t.Fatalf("WriteOutbox: %v", err)
	}

	files := packetFiles(t, out)
	if len(files) != 2 {
		t.Fatalf("wrote %d files, want one per other planet: %v", len(files), files)
	}
	for _, f := range files {
		if strings.Contains(f, "-to-all-") {
			t.Errorf("a routed league should address every copy, got %q", f)
		}
	}
}

// A board hosting two others sends each neighbour's traffic to that
// neighbour's own link, because a mailer collects one directory per link.
func TestPacketsGoToTheLinkForTheirNextHop(t *testing.T) {
	base, toCharlie := t.TempDir(), t.TempDir()

	cfg := game.DefaultConfig()
	cfg.BoardID = "Bravo BBS"
	cfg.OutboundDirs = map[int]string{3: toCharlie}
	hub := game.NewWorldSeed(cfg, 1)
	hub.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Alpha BBS"},
		{Number: 2, Name: "Bravo BBS", Hosts: []int{1, 3}},
		{Number: 3, Name: "Charlie BBS"},
		{Number: 4, Name: "Delta BBS", Hosts: []int{2}},
	}
	hub.Outbox = []game.Packet{
		{FromBoard: "Bravo BBS", ToBoard: "Charlie BBS", Date: "2026-08-09"},
		{FromBoard: "Bravo BBS", ToBoard: "Alpha BBS", Date: "2026-08-09"},
		// Delta is this board's uplink, so its traffic takes the default link.
		{FromBoard: "Bravo BBS", ToBoard: "Delta BBS", Date: "2026-08-09"},
	}
	hub.StampOutbox()
	if _, err := WriteOutbox(hub, base); err != nil {
		t.Fatalf("WriteOutbox: %v", err)
	}

	if got := len(packetFiles(t, toCharlie)); got != 1 {
		t.Errorf("Charlie's link holds %d packets, want 1", got)
	}
	if got := len(packetFiles(t, base)); got != 2 {
		t.Errorf("the default link holds %d packets, want 2", got)
	}
}

// Two leagues sharing one inbound directory must not eat each other's packets.
func TestAPacketFromAnotherLeagueIsLeftAlone(t *testing.T) {
	inbound := t.TempDir()
	cfg := game.DefaultConfig()
	cfg.BoardID = "Alpha BBS"
	cfg.LeagueNumber = 42
	w := game.NewWorldSeed(cfg, 1)

	writePacket(t, inbound, "ours", game.Packet{
		FromBoard: "Bravo BBS", ToBoard: "Alpha BBS", League: 42,
		Scores: []game.RemoteScore{{Empire: "Apples", Land: 500}},
	})
	writePacket(t, inbound, "theirs", game.Packet{
		FromBoard: "Bravo BBS", ToBoard: "Alpha BBS", League: 900,
		Scores: []game.RemoteScore{{Empire: "Oranges", Land: 500}},
	})

	applied, err := ReadInbound(w, inbound)
	if err != nil {
		t.Fatalf("ReadInbound: %v", err)
	}
	if applied != 1 {
		t.Errorf("applied %d packets, want 1", applied)
	}
	left := packetFiles(t, inbound)
	if len(left) != 1 || !strings.HasPrefix(left[0], "theirs") {
		t.Errorf("the other league's packet should still be there, directory holds %v", left)
	}
}

// The league number is on the filename as well as inside the packet, so a sysop
// looking at a shared directory can see which game a file belongs to.
func TestPacketFilenameCarriesTheLeagueNumber(t *testing.T) {
	out := t.TempDir()
	cfg := game.DefaultConfig()
	cfg.BoardID = "Alpha BBS"
	cfg.LeagueNumber = 42
	w := game.NewWorldSeed(cfg, 1)
	w.Outbox = []game.Packet{{FromBoard: "Alpha BBS", ToBoard: "Bravo BBS", Date: "2026-08-09"}}
	w.StampOutbox()
	if _, err := WriteOutbox(w, out); err != nil {
		t.Fatalf("WriteOutbox: %v", err)
	}
	files := packetFiles(t, out)
	if len(files) != 1 || !strings.HasPrefix(files[0], "L042-") {
		t.Errorf("packet filenames = %v, want one prefixed L042-", files)
	}
}

func writePacket(t *testing.T, dir, name string, p game.Packet) {
	t.Helper()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+PacketExt), data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func readPacket(t *testing.T, path string, p *game.Packet) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := json.Unmarshal(data, p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
}

func packetFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == PacketExt {
			names = append(names, e.Name())
		}
	}
	return names
}
