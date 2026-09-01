package ftn

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

func TestBundlePreservesPacketBytes(t *testing.T) {
	raw := []byte("{\n  \"FromBoard\": \"Alpha BBS\",\n  \"Seq\": 7\n}\n")
	var packet game.Packet
	if err := json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	body, manifest, err := makeBundle(1, "direct", []transportEntry{{Name: "original.brp", Raw: raw, Packet: packet, Route: []int{1}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entries) != 1 {
		t.Fatalf("bundle routing entries = %+v", manifest.Entries)
	}
	decoded, entries, err := readTransport(body, "alias.BRP")
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Delivery != "direct" || len(entries) != 1 {
		t.Fatalf("decoded manifest = %+v, entries=%d", decoded, len(entries))
	}
	if entries[0].Name != store.PacketFilename(packet, raw) {
		t.Fatalf("decoded packet name = %q", entries[0].Name)
	}
	if string(entries[0].Raw) != string(raw) {
		t.Fatalf("inner bytes changed:\n%s", entries[0].Raw)
	}
}

func TestReadTransportAcceptsLegacyJSON(t *testing.T) {
	raw, _ := json.Marshal(game.Packet{FromBoard: "Alpha BBS", FromNode: 1, Seq: 9})
	manifest, entries, err := readTransport(raw, "OLD.BRP")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Format != 0 || len(entries) != 1 || string(entries[0].Raw) != string(raw) {
		t.Fatalf("legacy decode = %+v %#v", manifest, entries)
	}
}

func TestLegacyHopCountComesFromUnchangedPacket(t *testing.T) {
	packet := game.Packet{FromBoard: "Alpha BBS", FromNode: 1, ToNode: 3, Seq: 20, League: 100, Hops: 7}
	raw, _ := json.Marshal(packet)
	_, legacy, err := readTransport(raw, "OLD.BRP")
	if err != nil {
		t.Fatal(err)
	}
	if len(legacy) != 1 || transportHops(legacy[0]) != 7 {
		t.Fatalf("legacy hops = %+v", legacy)
	}
	body, _, err := makeBundle(2, "direct", legacy)
	if err != nil {
		t.Fatal(err)
	}
	_, forwarded, err := readTransport(body, "ALIAS.BRP")
	if err != nil {
		t.Fatal(err)
	}
	if len(forwarded) != 1 || transportHops(forwarded[0]) != 8 || string(forwarded[0].Raw) != string(raw) {
		t.Fatalf("forwarded legacy entry = %+v", forwarded)
	}
}

func TestReadTransportDerivesCanonicalPacketName(t *testing.T) {
	packet := game.Packet{FromBoard: "Alpha BBS", FromNode: 1, ToNode: 2, Seq: 9, League: 100}
	raw, _ := json.Marshal(packet)
	manifest, _ := json.Marshal(bundleManifest{
		Format: 1, Delivery: "direct",
		Entries: []bundleManifestEntry{{Route: []int{1}}},
	})
	var body bytes.Buffer
	zw := zip.NewWriter(&body)
	if err := writeZipMember(zw, bundleManifestName, manifest); err != nil {
		t.Fatal(err)
	}
	if err := writeZipMember(zw, "packets/000000/EXTERNAL-LONG-NAME.BRP", raw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_, entries, err := readTransport(body.Bytes(), "alias.BRP")
	if err != nil {
		t.Fatal(err)
	}
	want := store.PacketFilename(packet, raw)
	if len(entries) != 1 || entries[0].Name != want {
		t.Fatalf("decoded names = %+v, want %q", entries, want)
	}
}

func TestReadTransportRejectsReorderedPacketMembers(t *testing.T) {
	packets := []game.Packet{
		{FromBoard: "Alpha BBS", FromNode: 11, ToNode: 3, Seq: 1, League: 100},
		{FromBoard: "Bravo BBS", FromNode: 22, ToNode: 3, Seq: 1, League: 100},
	}
	entries := make([]transportEntry, len(packets))
	for i, packet := range packets {
		raw, err := json.Marshal(packet)
		if err != nil {
			t.Fatal(err)
		}
		entries[i] = transportEntry{Raw: raw, Packet: packet, Route: []int{packet.FromNode, 1}}
	}
	body, _, err := makeBundle(1, "direct", entries)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 3 {
		t.Fatalf("bundle members = %d, want manifest and two packets", len(zr.File))
	}
	var reordered bytes.Buffer
	zw := zip.NewWriter(&reordered)
	for _, index := range []int{0, 2, 1} {
		member := zr.File[index]
		raw, err := readZipMember(member)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeZipMember(zw, member.Name, raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readTransport(reordered.Bytes(), "alias.BRP"); err == nil || !strings.Contains(err.Error(), "unexpected packet member") {
		t.Fatalf("reordered packet members were accepted: %v", err)
	}
}

func TestRunOutCoalescesOneBundlePerNextHop(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "")
	writeNamedPacket(t, data, "a.brp", game.Packet{FromBoard: "Bravo BBS", ToBoard: "Charlie BBS", FromNode: 2, ToNode: 3, Seq: 1, League: 100})
	writeNamedPacket(t, data, "b.brp", game.Packet{FromBoard: "Bravo BBS", ToBoard: "Alpha BBS", FromNode: 2, ToNode: 1, Seq: 2, League: 100})

	result, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Queued) != 1 {
		t.Fatalf("queued = %d, want one bundle for Alpha", len(result.Queued))
	}
	queued := result.Queued[0]
	if queued.NextHop != "Alpha BBS" || len(filepath.Base(queued.PacketPath)) != 12 || filepath.Ext(queued.PacketPath) != ".BRP" {
		t.Fatalf("queued = %+v", queued)
	}
	body, err := os.ReadFile(queued.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	_, entries, err := readTransport(body, filepath.Base(queued.PacketPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("bundle entries = %d, want 2", len(entries))
	}
}

// #227 made a league number required of a board, so RunOut can no longer reach
// nextAlias with league 0 — see TestTransportRefusesABoardWithNoLeagueNumber.
// The league-0 alias namespace stays covered here, directly: it exists for a
// legacy packet that carries no league of its own, and must still be distinct
// from any real league's namespace.
func TestAliasKeepsALegacyLeagueZeroNamespace(t *testing.T) {
	dir := t.TempDir()
	zero, _, err := nextAlias(dir, dir, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(zero, "0002") {
		t.Fatalf("league-0/node-2 alias = %q, want the 0002 namespace", zero)
	}
	one, _, err := nextAlias(dir, dir, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if one[:4] == zero[:4] {
		t.Fatalf("league 1 shares the legacy namespace %q", zero[:4])
	}
}

func TestRunOutReportsThinSubjectMargin(t *testing.T) {
	const spare = 3
	prefix := strings.Repeat("p", type2SubjectSize-1-spare-1-len("00000000.BRP"))
	data := newBundledSetup(t, "Bravo BBS", "SubjectPath "+prefix+"\n")
	writeNamedPacket(t, data, "thin.brp", game.Packet{FromNode: 2, ToNode: 1, Seq: 1, League: 100})

	result, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Queued) != 1 || len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "3 byte") {
		t.Fatalf("thin-margin result = %+v", result)
	}
}

func TestBuildBatchPlanReplacesPartialBundleOnRetry(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "")
	board, err := store.LoadConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	transport, err := LoadConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := store.ParseNodeList(filepath.Join(data, store.NodeListFile))
	if err != nil {
		t.Fatal(err)
	}
	for i := range nodes {
		nodes[i].Hosts = nil
	}
	world := &game.World{Config: board, LeagueNodes: nodes}
	batch := filepath.Join(data, spoolDir, outSpoolDir, "partial")
	if err := os.MkdirAll(batch, 0o755); err != nil {
		t.Fatal(err)
	}
	writeClaimed := func(name string, packet game.Packet) {
		raw, err := json.Marshal(packet)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(batch, name), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeClaimed("packet-000000.brp", game.Packet{FromNode: 2, ToNode: 1, Seq: 1, League: 100})
	writeClaimed("packet-000001.brp", game.Packet{FromNode: 2, ToNode: 1, Seq: 2, League: 100})
	writeClaimed("packet-000002.brp", game.Packet{FromNode: 2, ToNode: 3, Seq: 3, League: 100})

	brokenNodes := append([]game.LeagueNode(nil), nodes...)
	brokenNodes[2].Address = "not-an-ftn-address"
	if _, err := buildBatchPlan(batch, data, transport, world, brokenNodes, &Result{}); err == nil {
		t.Fatal("partial plan unexpectedly succeeded")
	}
	if err := quarantineTransport(data, filepath.Join(batch, "packet-000001.brp")); err != nil {
		t.Fatal(err)
	}
	plan, err := buildBatchPlan(batch, data, transport, world, nodes, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Targets) != 2 {
		t.Fatalf("retry targets = %+v", plan.Targets)
	}
	body, err := os.ReadFile(filepath.Join(batch, "target-001.bundle"))
	if err != nil {
		t.Fatal(err)
	}
	_, entries, err := readTransport(body, "target-001.bundle")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Packet.Seq != 1 {
		t.Fatalf("replaced partial target = %+v", entries)
	}
}

func TestRunOutBroadcastMarksEverySiblingTargetCovered(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "")
	writeNamedPacket(t, data, "broadcast.brp", game.Packet{FromBoard: "Bravo BBS", FromNode: 2, Seq: 17, League: 100})
	result, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Queued) != 2 {
		t.Fatalf("broadcast queued = %d, want 2", len(result.Queued))
	}
	for _, queued := range result.Queued {
		body, err := os.ReadFile(queued.PacketPath)
		if err != nil {
			t.Fatal(err)
		}
		_, entries, err := readTransport(body, filepath.Base(queued.PacketPath))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || !slices.Equal(entries[0].Route, []int{2}) ||
			!slices.Contains(entries[0].Covered, 1) || !slices.Contains(entries[0].Covered, 2) || !slices.Contains(entries[0].Covered, 3) {
			t.Fatalf("broadcast transport entry = %+v", entries)
		}
	}
}

func TestRunOutBroadcastFansOutToConfiguredBSOPeers(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "Link 1 BSO bso Normal Bundled\n")
	writeNamedPacket(t, data, "broadcast.brp", game.Packet{FromBoard: "Bravo BBS", FromNode: 2, Seq: 19, League: 100})
	result, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Queued) != 1 || result.Queued[0].NextHop != "Alpha BBS" || result.Queued[0].Message != filepath.Join(data, "bso", "00e50064.flo") {
		t.Fatalf("BSO broadcast fanout = %+v", result)
	}
	body, err := os.ReadFile(result.Queued[0].PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	_, entries, err := readTransport(body, filepath.Base(result.Queued[0].PacketPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !slices.Equal(entries[0].Covered, []int{2, 1}) {
		t.Fatalf("BSO covered set = %+v", entries)
	}
}

func TestConcurrentRunOutClaimsOneSnapshot(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "")
	writeNamedPacket(t, data, "a.brp", game.Packet{FromNode: 2, ToNode: 1, Seq: 1, League: 100})
	const runners = 8
	var wg sync.WaitGroup
	results := make(chan Result, runners)
	for range runners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := RunOut(data)
			if err != nil {
				t.Errorf("RunOut: %v", err)
			}
			results <- result
		}()
	}
	wg.Wait()
	close(results)
	queued := 0
	for result := range results {
		queued += len(result.Queued)
	}
	if queued != 1 {
		t.Fatalf("queued bundles = %d, want 1", queued)
	}
}

func TestBSOBusyDefersOnlyThatTarget(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "Link 1 BSO bso Normal Bundled\n")
	writeNamedPacket(t, data, "a.brp", game.Packet{FromNode: 2, ToNode: 1, Seq: 1, League: 100})
	busy := filepath.Join(data, "bso", "00e50064.bsy")
	if err := os.MkdirAll(filepath.Dir(busy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(busy, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Queued) != 0 || len(first.Warnings) == 0 {
		t.Fatalf("busy result = %+v", first)
	}
	if err := os.Remove(busy); err != nil {
		t.Fatal(err)
	}
	second, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Queued) != 1 {
		t.Fatalf("resumed queued = %d, want 1", len(second.Queued))
	}
	flow := filepath.Join(data, "bso", "00e50064.flo")
	flowData, err := os.ReadFile(flow)
	if err != nil || !strings.HasPrefix(string(flowData), "^") {
		t.Fatalf("flow = %q, %v", flowData, err)
	}
}

func TestBSOAppendsToAdvertisedBundleUnderBusyLock(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "Link 1 BSO bso Normal Bundled\n")
	writeNamedPacket(t, data, "first.brp", game.Packet{FromNode: 2, ToNode: 1, Seq: 1, League: 100})
	first, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Queued) != 1 {
		t.Fatalf("first queued = %d, want 1", len(first.Queued))
	}
	firstPath := first.Queued[0].PacketPath

	writeNamedPacket(t, data, "second.brp", game.Packet{FromNode: 2, ToNode: 1, Seq: 2, League: 100})
	second, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Queued) != 1 || second.Queued[0].PacketPath != firstPath {
		t.Fatalf("second queued = %+v, want append to %s", second.Queued, firstPath)
	}
	body, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest, entries, err := readTransport(body, filepath.Base(firstPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Packet.Seq != 1 || entries[1].Packet.Seq != 2 {
		t.Fatalf("merged bundle = %+v, entries=%+v", manifest, entries)
	}
	flowData, err := os.ReadFile(filepath.Join(data, "bso", "00e50064.flo"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(strings.TrimSpace(string(flowData)), "\n") != 0 {
		t.Fatalf("flow contains more than one reference: %q", flowData)
	}
	files, err := filepath.Glob(filepath.Join(data, "attach", "*.BRP"))
	if err != nil || len(files) != 1 {
		t.Fatalf("BSO attachment files = %v, %v; want one", files, err)
	}
}

func TestBSOBundleAppendIsReplaySafe(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "00000001.BRP")
	flow := filepath.Join(dir, "00e50064.flo")
	packet1 := game.Packet{FromNode: 2, ToNode: 1, Seq: 1, League: 100}
	packet2 := game.Packet{FromNode: 2, ToNode: 1, Seq: 2, League: 100}
	raw1, _ := json.Marshal(packet1)
	raw2, _ := json.Marshal(packet2)
	existing, _, err := makeBundle(2, "direct", []transportEntry{{Name: "one.brp", Raw: raw1, Packet: packet1}})
	if err != nil {
		t.Fatal(err)
	}
	incoming, _, err := makeBundle(2, "direct", []transportEntry{{Name: "two.brp", Raw: raw2, Packet: packet2}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existingPath, existing, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := addFlowEntry(flow, existingPath); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		path, appended, err := appendBSOBundle(flow, dir, incoming)
		if err != nil || !appended || path != existingPath {
			t.Fatalf("append %d = %q, %t, %v", i, path, appended, err)
		}
	}
	merged, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatal(err)
	}
	_, entries, err := readTransport(merged, filepath.Base(existingPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("replayed append produced %d entries, want 2", len(entries))
	}
}

func TestDocumentedBSOPaths(t *testing.T) {
	for _, tc := range []struct {
		address Address
		busy    string
		flow    string
	}{
		{Address{Zone: 1, Net: 229, Node: 300}, "00e5012c.bsy", "00e5012c.flo"},
		{Address{Zone: 1, Net: 229, Node: 300, Point: 4}, filepath.Join("00e5012c.pnt", "00000004.bsy"), filepath.Join("00e5012c.pnt", "00000004.flo")},
	} {
		busy, flow, err := bsoPaths("outbound", tc.address, "Normal")
		if err != nil {
			t.Fatal(err)
		}
		if busy != filepath.Join("outbound", tc.busy) || flow != filepath.Join("outbound", tc.flow) {
			t.Errorf("bsoPaths(%s) = %q, %q", tc.address.String(), busy, flow)
		}
	}
}

func TestRunInUnwrapsToPrivateGameInbound(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "")
	packetRaw := []byte("{\n \"FromBoard\":\"Alpha BBS\", \"ToBoard\":\"Bravo BBS\", \"FromNode\":1, \"ToNode\":2, \"Seq\":11, \"League\":100\n}\n")
	var packet game.Packet
	if err := json.Unmarshal(packetRaw, &packet); err != nil {
		t.Fatal(err)
	}
	body, _, err := makeBundle(1, "direct", []transportEntry{{Name: "inside.brp", Raw: packetRaw, Packet: packet, Route: []int{1}}})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(data, "transport-in", "10000000.BRP")
	if err := os.WriteFile(source, body, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := RunIn(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Delivered != 1 {
		t.Fatalf("delivered = %d, want 1; warnings=%v", result.Delivered, result.Warnings)
	}
	delivered, err := os.ReadFile(filepath.Join(data, "door-in", store.PacketFilename(packet, packetRaw)))
	if err != nil {
		t.Fatal(err)
	}
	if string(delivered) != string(packetRaw) {
		t.Fatal("RunIn changed the signed packet bytes")
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("transport bundle remains: %v", err)
	}
}

func TestRunInDeliversAtMaximumHopCount(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "")
	packet := game.Packet{
		FromBoard: "Alpha BBS", ToBoard: "Bravo BBS", FromNode: 1, ToNode: 2,
		Seq: 21, League: 100, Hops: game.MaxPacketHops,
	}
	raw, _ := json.Marshal(packet)
	body, _, err := makeBundle(1, "direct", []transportEntry{{Raw: raw, Packet: packet, Route: []int{1}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "transport-in", "10000005.BRP"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := RunIn(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Delivered != 1 {
		t.Fatalf("maximum-hop delivery = %+v", result)
	}
}

func TestRunInQuarantinesRejectedMembersAfterDeliveringValidOnes(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "")
	packets := []game.Packet{
		{FromBoard: "Alpha BBS", ToBoard: "Bravo BBS", FromNode: 1, ToNode: 2, Seq: 22, League: 100},
		{FromBoard: "Alpha BBS", ToBoard: "Missing BBS", FromNode: 1, ToNode: 999, Seq: 23, League: 100},
		{FromBoard: "Alpha BBS", ToBoard: "Bravo BBS", FromNode: 1, ToNode: 2, Seq: 24, League: 200},
	}
	entries := make([]transportEntry, len(packets))
	for i, packet := range packets {
		raw, err := json.Marshal(packet)
		if err != nil {
			t.Fatal(err)
		}
		entries[i] = transportEntry{Raw: raw, Packet: packet, Route: []int{1}}
	}
	body, _, err := makeBundle(1, "direct", entries)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(data, "transport-in", "10000006.BRP")
	if err := os.WriteFile(source, body, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := RunIn(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Delivered != 1 || len(result.Warnings) != 2 {
		t.Fatalf("mixed inbound result = %+v", result)
	}
	warnings := strings.Join(result.Warnings, " ")
	if !strings.Contains(warnings, "destination node 999") || !strings.Contains(warnings, "league 200") {
		t.Fatalf("mixed inbound warnings = %v", result.Warnings)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("rejected source remains in inbound: %v", err)
	}
	quarantined := filepath.Join(data, spoolDir, badSpoolDir, filepath.Base(source))
	if got, err := os.ReadFile(quarantined); err != nil || !bytes.Equal(got, body) {
		t.Fatalf("quarantined bundle = %d bytes, %v", len(got), err)
	}
	second, err := RunIn(data)
	if err != nil || second.Delivered != 0 || len(second.Warnings) != 0 {
		t.Fatalf("second inbound pass = %+v, %v", second, err)
	}
}

func TestRunInLogsIdenticalCanonicalDuplicate(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "")
	raw, _ := json.Marshal(game.Packet{FromBoard: "Alpha BBS", ToBoard: "Bravo BBS", FromNode: 1, ToNode: 2, Seq: 16, League: 100})
	var packet game.Packet
	if err := json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	body, _, err := makeBundle(1, "direct", []transportEntry{{Raw: raw, Packet: packet, Route: []int{1}}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		source := filepath.Join(data, "transport-in", fmt.Sprintf("1000001%d.BRP", i))
		if err := os.WriteFile(source, body, 0o644); err != nil {
			t.Fatal(err)
		}
		result, err := RunIn(data)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && result.Delivered != 1 {
			t.Fatalf("first delivery = %+v", result)
		}
		if i == 1 && (result.Delivered != 0 || len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "duplicate")) {
			t.Fatalf("duplicate delivery = %+v", result)
		}
	}
}

func TestRunInRetainsDifferentBytesAtCanonicalCollision(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "")
	firstPacket := game.Packet{FromBoard: "Alpha BBS", ToBoard: "Bravo BBS", FromNode: 1, ToNode: 2, Seq: 18, League: 100}
	secondPacket := firstPacket
	secondPacket.Notice = "different signed packet contents"
	for i, packet := range []game.Packet{firstPacket, secondPacket} {
		raw, _ := json.Marshal(packet)
		body, _, err := makeBundle(1, "direct", []transportEntry{{Raw: raw, Packet: packet, Route: []int{1}}})
		if err != nil {
			t.Fatal(err)
		}
		source := filepath.Join(data, "transport-in", fmt.Sprintf("1000002%d.BRP", i))
		if err := os.WriteFile(source, body, 0o644); err != nil {
			t.Fatal(err)
		}
		result, err := RunIn(data)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && result.Delivered != 1 {
			t.Fatalf("first delivery = %+v", result)
		}
		if i == 1 {
			if result.Delivered != 0 || len(result.Warnings) == 0 || !strings.Contains(strings.Join(result.Warnings, " "), "collision") {
				t.Fatalf("collision delivery = %+v", result)
			}
			if _, err := os.Stat(source); err != nil {
				t.Fatalf("collision source was not retained: %v", err)
			}
		}
	}
}

func TestRunInValidatesAndDeletesStoredAttach(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "")
	raw, _ := json.Marshal(game.Packet{FromBoard: "Alpha BBS", ToBoard: "Bravo BBS", FromNode: 1, ToNode: 2, Seq: 12, League: 100})
	var packet game.Packet
	if err := json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	body, _, err := makeBundle(1, "attach", []transportEntry{{Name: "mail.brp", Raw: raw, Packet: packet}})
	if err != nil {
		t.Fatal(err)
	}
	attachment := filepath.Join(data, "transport-in", "10000001.BRP")
	if err := os.WriteFile(attachment, body, 0o644); err != nil {
		t.Fatal(err)
	}
	messageConfig := Config{NetmailDir: filepath.Join(data, "transport-in"), SubjectMode: SubjectBasename}
	message, err := createFileAttach(messageConfig, attachment, Address{Zone: 1, Net: 229, Node: 100}, Address{Zone: 1, Net: 229, Node: 200})
	if err != nil {
		t.Fatal(err)
	}
	result, err := RunIn(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Delivered != 1 {
		t.Fatalf("delivered = %d, warnings=%v", result.Delivered, result.Warnings)
	}
	for _, path := range []string{attachment, message} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("consumed attach file remains at %s: %v", path, err)
		}
	}
}

func TestRunInForwardsOpaquePacketAtHub(t *testing.T) {
	data := newBundledSetup(t, "Alpha BBS", "Link 3 Obox obox3 Bundled\n")
	if err := os.Mkdir(filepath.Join(data, "obox3"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte("{\n \"FromBoard\":\"Bravo BBS\", \"ToBoard\":\"Charlie BBS\", \"FromNode\":2, \"ToNode\":3, \"Seq\":13, \"League\":100\n}\n")
	var packet game.Packet
	if err := json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	body, _, err := makeBundle(2, "direct", []transportEntry{{Name: "transit.brp", Raw: raw, Packet: packet, Route: []int{2}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "transport-in", "10000002.BRP"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	board, err := store.LoadConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	gameLock, err := store.Lock(board, true)
	if err != nil {
		t.Fatal(err)
	}
	type runResult struct {
		result Result
		err    error
	}
	done := make(chan runResult, 1)
	go func() {
		result, err := RunIn(data)
		done <- runResult{result: result, err: err}
	}()
	var result Result
	select {
	case completed := <-done:
		result, err = completed.result, completed.err
		if releaseErr := gameLock.Release(); releaseErr != nil {
			t.Fatal(releaseErr)
		}
	case <-time.After(2 * time.Second):
		_ = gameLock.Release()
		<-done
		t.Fatal("pure-transit RunIn waited for the game lock")
	}
	if err != nil {
		t.Fatal(err)
	}
	if result.Delivered != 0 || len(result.Queued) != 1 || result.Queued[0].NextHop != "Charlie BBS" {
		t.Fatalf("forward result = %+v", result)
	}
	forwarded, err := os.ReadFile(result.Queued[0].PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	_, entries, err := readTransport(forwarded, filepath.Base(result.Queued[0].PacketPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || string(entries[0].Raw) != string(raw) || transportHops(entries[0]) != 1 {
		t.Fatalf("forwarded entry changed: %+v", entries)
	}
}

// A raw peer must get a bare packet when we are only RELAYING one to it, not
// just when we originate it: it arrives under the same .BRP alias either way,
// so a bundle it cannot unwrap is a file its game reads as a corrupt packet.
func TestRunInForwardsRawToARawPeer(t *testing.T) {
	data := newBundledSetup(t, "Alpha BBS", "Link 3 Obox obox3 Raw\n")
	if err := os.Mkdir(filepath.Join(data, "obox3"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte("{\n \"FromBoard\":\"Bravo BBS\", \"ToBoard\":\"Charlie BBS\", \"FromNode\":2, \"ToNode\":3, \"Seq\":21, \"League\":100\n}\n")
	var packet game.Packet
	if err := json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	body, _, err := makeBundle(2, "direct", []transportEntry{{Name: "transit.brp", Raw: raw, Packet: packet, Route: []int{2}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "transport-in", "10000021.BRP"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := RunIn(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Queued) != 1 || result.Queued[0].NextHop != "Charlie BBS" {
		t.Fatalf("forward result = %+v", result)
	}
	forwarded, err := os.ReadFile(result.Queued[0].PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(forwarded) != string(raw) {
		t.Fatalf("forwarded a %d-byte file beginning %q; want the packet verbatim",
			len(forwarded), string(forwarded[:min(2, len(forwarded))]))
	}
}

func TestOboxMeshFanoutCanBeDisabled(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "OboxMeshFanout No\nLink 3 Obox obox3 Bundled\n")
	if err := os.Mkdir(filepath.Join(data, "obox3"), 0o755); err != nil {
		t.Fatal(err)
	}
	meshRoster := "1\nAlpha BBS\n1:229/100\nDetroit\nMI\nUSA\n\n" +
		"2\nBravo BBS\n1:229/200\nLansing\nMI\nUSA\n\n" +
		"3\nCharlie BBS\n1:229/300\nFlint\nMI\nUSA\n"
	if err := os.WriteFile(filepath.Join(data, store.NodeListFile), []byte(meshRoster), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(game.Packet{FromBoard: "Alpha BBS", FromNode: 1, Seq: 14, League: 100})
	var packet game.Packet
	json.Unmarshal(raw, &packet)
	body, _, err := makeBundle(1, "direct", []transportEntry{{Name: "broadcast.brp", Raw: raw, Packet: packet, Route: []int{1}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "transport-in", "10000003.BRP"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := RunIn(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Delivered != 1 || len(result.Queued) != 0 {
		t.Fatalf("local-only result = %+v", result)
	}
}

func TestInboundBSOBusyPeerDoesNotBlockOtherFanout(t *testing.T) {
	data := newBundledSetup(t, "Alpha BBS", "Link 3 BSO bso Normal Bundled\nLink 4 Obox obox4 Bundled\n")
	if err := os.Mkdir(filepath.Join(data, "obox4"), 0o755); err != nil {
		t.Fatal(err)
	}
	roster := "1 HOST 2 3 4\nAlpha BBS\n1:229/100\nDetroit\nMI\nUSA\n\n" +
		"2\nBravo BBS\n1:229/200\nLansing\nMI\nUSA\n\n" +
		"3\nCharlie BBS\n1:229/300\nFlint\nMI\nUSA\n\n" +
		"4\nDelta BBS\n1:229/400\nAnn Arbor\nMI\nUSA\n"
	if err := os.WriteFile(filepath.Join(data, store.NodeListFile), []byte(roster), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(game.Packet{FromBoard: "Bravo BBS", FromNode: 2, Seq: 15, League: 100})
	var packet game.Packet
	if err := json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	body, _, err := makeBundle(2, "direct", []transportEntry{{Name: "broadcast.brp", Raw: raw, Packet: packet, Route: []int{2}}})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(data, "transport-in", "10000004.BRP")
	if err := os.WriteFile(source, body, 0o644); err != nil {
		t.Fatal(err)
	}
	busy := filepath.Join(data, "bso", "00e5012c.bsy")
	if err := os.WriteFile(busy, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := RunIn(data)
	if err != nil {
		t.Fatal(err)
	}
	if first.Delivered != 1 || len(first.Queued) != 1 || first.Queued[0].NextHop != "Delta BBS" || len(first.Warnings) == 0 {
		t.Fatalf("first fanout = %+v", first)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("source removed before busy target completed: %v", err)
	}
	if err := os.Remove(busy); err != nil {
		t.Fatal(err)
	}
	second, err := RunIn(data)
	if err != nil {
		t.Fatal(err)
	}
	if second.Delivered != 0 || len(second.Queued) != 1 || second.Queued[0].NextHop != "Charlie BBS" {
		t.Fatalf("resumed fanout = %+v", second)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("completed source remains: %v", err)
	}
}

func newBundledSetup(t *testing.T, boardID, extraFTN string) string {
	t.Helper()
	data := t.TempDir()
	for _, dir := range []string{"door-in", "door-out", "transport-in", "netmail", "attach", "bso"} {
		if err := os.Mkdir(filepath.Join(data, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"config.json":         "{}\n",
		store.BoardConfigFile: "BoardID " + boardID + "\nLeagueNumber 100\nInbound door-in\nOutbound door-out\n",
		// Raw is the shipped default (#230), so this helper states the bundled
		// posture its name promises. As a posture rather than Link lines,
		// deliberately: a Link line also marks a peer as a broadcast fanout
		// target, so declaring links here would quietly change which peers the
		// broadcast tests expect.
		ConfigFile: "NetmailDir netmail\nAttachDir attach\nSubjectPath Basename\nInboundDir transport-in\nInboundNetmailDir transport-in\n" +
			"Bundled Yes\n" + extraFTN,
		store.NodeListFile: "1 HOST 2 3\nAlpha BBS\n1:229/100\nDetroit\nMI\nUSA\n\n" +
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

func writeNamedPacket(t *testing.T, data, name string, packet game.Packet) {
	t.Helper()
	body, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "door-out", name), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// #227: a board with no league number takes every league's packets as its own
// and has its own taken everywhere, so the transport refuses to move mail for
// it at all rather than moving it into a league it has not joined.
func TestTransportRefusesABoardWithNoLeagueNumber(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "")
	cfg := "BoardID Bravo BBS\nInbound door-in\nOutbound door-out\n" // no LeagueNumber line
	if err := os.WriteFile(filepath.Join(data, store.BoardConfigFile), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	writeNamedPacket(t, data, "a.brp", game.Packet{
		FromBoard: "Bravo BBS", ToBoard: "Alpha BBS", FromNode: 2, ToNode: 1, Seq: 1})

	for _, run := range []struct {
		name string
		fn   func(string) (Result, error)
	}{{"-out", RunOut}, {"-in", RunIn}} {
		result, err := run.fn(data)
		if err == nil {
			t.Fatalf("%s ran for a board with no league number, queuing %d bundle(s)", run.name, len(result.Queued))
		}
		if !strings.Contains(err.Error(), store.BoardConfigFile) {
			t.Errorf("%s did not name the file to fix: %v", run.name, err)
		}
	}
	// The packet is still there to send once the number is set: refusing must
	// not consume it.
	if _, err := os.Stat(filepath.Join(data, "door-out", "a.brp")); err != nil {
		t.Errorf("the refused packet did not survive in the outbound: %v", err)
	}
}

// #228: a run that queues nothing because its only peer is busy must not look
// like a run that had nothing to send. A scheduled event's log is the only
// place a sysop would ever notice the difference.
func TestRunOutReportsWhatIsStillWaiting(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "Link 1 BSO bso Normal Bundled\n")
	// A semaphore that is not ours: recovery leaves a mailer's own alone, so
	// the peer stays busy for the whole run.
	busy := filepath.Join(data, "bso", "00e50064.bsy")
	if err := os.WriteFile(busy, []byte("binkd pid=1234\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeNamedPacket(t, data, "waiting.brp", game.Packet{FromNode: 2, ToNode: 1, Seq: 1, League: 100})

	result, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Queued) != 0 {
		t.Fatalf("queued = %+v, want nothing while the peer holds its .bsy", result.Queued)
	}
	if result.Snapshots != 1 {
		t.Errorf("snapshots still waiting = %d, want 1", result.Snapshots)
	}
	if len(result.Waiting) != 1 || result.Waiting[0] != "Alpha BBS" {
		t.Errorf("waiting on = %+v, want the one busy peer", result.Waiting)
	}

	// Once the peer frees its queue the same snapshot completes, and the count
	// goes back to zero rather than sticking.
	if err := os.Remove(busy); err != nil {
		t.Fatal(err)
	}
	done, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(done.Queued) != 1 {
		t.Fatalf("queued after the peer freed up = %+v, want the held bundle", done.Queued)
	}
	if done.Snapshots != 0 || len(done.Waiting) != 0 {
		t.Errorf("still reported as waiting after publishing: %d snapshot(s), %+v", done.Snapshots, done.Waiting)
	}
}

// #228: a count says how much is waiting but not for how long or why, and the
// run that met the failure is usually long gone by the time anyone looks. The
// journal carries both, and keeps them across runs.
func TestRunOutJournalsWhyAPeerIsBehind(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "Link 1 BSO bso Normal Bundled\n")
	busy := filepath.Join(data, "bso", "00e50064.bsy")
	if err := os.WriteFile(busy, []byte("binkd pid=1234\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeNamedPacket(t, data, "held.brp", game.Packet{FromNode: 2, ToNode: 1, Seq: 1, League: 100})

	first, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Stalled) != 1 || !strings.Contains(first.Stalled[0], "Alpha BBS") {
		t.Fatalf("stalled = %+v, want the busy peer and its reason", first.Stalled)
	}
	if first.OldestWait <= 0 {
		t.Errorf("oldest wait = %v, want the age of the snapshot", first.OldestWait)
	}

	// The reason survives into the next run: nothing carries it in memory, and
	// a sysop reading a scheduled log has only what the journal kept.
	second, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Stalled) != 1 || second.Stalled[0] != first.Stalled[0] {
		t.Fatalf("second run stalled = %+v, want the same recorded reason", second.Stalled)
	}

	// And it is cleared when the peer finally takes the bundle, rather than
	// haunting a snapshot that has since succeeded.
	if err := os.Remove(busy); err != nil {
		t.Fatal(err)
	}
	done, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(done.Stalled) != 0 || done.OldestWait != 0 {
		t.Errorf("after publishing: stalled=%+v oldest=%v, want neither", done.Stalled, done.OldestWait)
	}
}

// An old journal predates created/progress/last_error. It must still load, and
// still date the snapshot, or the report silently stops working on exactly the
// boards that have been running longest.
func TestPendingReportReadsAJournalWithoutTheNewFields(t *testing.T) {
	dir := t.TempDir()
	batch := filepath.Join(dir, spoolDir, outSpoolDir, "0001")
	if err := os.MkdirAll(batch, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"id":"0001","targets":[{"node":1,"name":"Alpha BBS","alias":"00640001.BRP","bundle_file":"target-001.bundle","done":false}]}`
	if err := os.WriteFile(filepath.Join(batch, batchPlanFile), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-90 * time.Minute)
	if err := os.Chtimes(filepath.Join(batch, batchPlanFile), old, old); err != nil {
		t.Fatal(err)
	}
	var result Result
	countPendingOutbound(dir, &result)
	if result.Snapshots != 1 || len(result.Waiting) != 1 {
		t.Fatalf("legacy journal not counted: %+v", result)
	}
	if result.OldestWait < time.Hour {
		t.Errorf("oldest wait = %v, want the file's own mtime to stand in", result.OldestWait)
	}
}

// #228: the status report has to name what is unfinished, for whom, for how
// long and why — including the inbound side and a journal nothing can read,
// which is the case that otherwise goes unmentioned by everything.
func TestStatusReportsBothSpoolsAndTheirReasons(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "Link 1 BSO bso Normal Bundled\n")
	busy := filepath.Join(data, "bso", "00e50064.bsy")
	if err := os.WriteFile(busy, []byte("binkd pid=1234\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeNamedPacket(t, data, "held.brp", game.Packet{FromNode: 2, ToNode: 1, Seq: 1, League: 100})
	if _, err := RunOut(data); err != nil {
		t.Fatal(err)
	}

	// An inbound spool directory whose journal will not parse.
	broken := filepath.Join(data, spoolDir, inSpoolDir, "deadbeef")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, inboundReceiptFile), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	status, err := Status(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Peers) != 1 || status.Peers[0].Name != "Alpha BBS" || status.Peers[0].Snapshots != 1 {
		t.Fatalf("peers = %+v, want one snapshot waiting on Alpha BBS", status.Peers)
	}
	if status.Peers[0].LastError == "" {
		t.Error("the waiting peer carries no recorded reason")
	}
	if status.Peers[0].Oldest <= 0 {
		t.Errorf("oldest wait = %v, want an age", status.Peers[0].Oldest)
	}
	if len(status.Unreadable) != 1 || !strings.Contains(status.Unreadable[0], "deadbeef") {
		t.Errorf("unreadable = %+v, want the broken receipt directory", status.Unreadable)
	}

	// Status must not move anything: a sysop reads it while deciding.
	before, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if before.Snapshots != 1 {
		t.Errorf("a status read changed what is pending: %d snapshot(s)", before.Snapshots)
	}
}

// The peer with the longest wait is the one to act on, so it is named first.
func TestStatusOrdersPeersByHowLongTheyHaveWaited(t *testing.T) {
	data := t.TempDir()
	out := filepath.Join(data, spoolDir, outSpoolDir)
	write := func(id, peer string, age time.Duration) {
		dir := filepath.Join(out, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		plan := batchPlan{ID: id, Created: time.Now().Add(-age),
			Targets: []batchTarget{{Node: 1, Name: peer, Done: false}}}
		body, err := json.Marshal(plan)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, batchPlanFile), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("0001", "Recent BBS", time.Minute)
	write("0002", "Ancient BBS", 72*time.Hour)

	status, err := Status(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Peers) != 2 || status.Peers[0].Name != "Ancient BBS" {
		t.Fatalf("peers = %+v, want the longest wait first", status.Peers)
	}
}

// #230: a peer still on the released version cannot parse a ZIP bundle at all,
// and its ReadInbound aborts the whole run rather than skipping the file. A raw
// link sends it what it has always understood: one game packet per file, the
// bytes unchanged.
func TestRawLinkSendsOnePlainPacketPerFile(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "Link 1 Obox obox1 Raw\n")
	if err := os.MkdirAll(filepath.Join(data, "obox1"), 0o755); err != nil {
		t.Fatal(err)
	}
	first := game.Packet{FromBoard: "Bravo BBS", ToBoard: "Alpha BBS", FromNode: 2, ToNode: 1, Seq: 1, League: 100}
	second := game.Packet{FromBoard: "Bravo BBS", ToBoard: "Alpha BBS", FromNode: 2, ToNode: 1, Seq: 2, League: 100}
	writeNamedPacket(t, data, "one.brp", first)
	writeNamedPacket(t, data, "two.brp", second)

	result, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	// One file per packet: a raw file IS a packet, so there is no envelope to
	// hold the second.
	if len(result.Queued) != 2 {
		t.Fatalf("queued = %+v, want one handoff per packet", result.Queued)
	}
	seqs := map[uint64]bool{}
	for _, q := range result.Queued {
		body, err := os.ReadFile(q.PacketPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(body) > 0 && body[0] == 'P' {
			t.Fatalf("%s is a ZIP bundle, which the peer this link exists for cannot read", q.PacketPath)
		}
		// The test that matters: a board with only encoding/json must be able
		// to read it, which is all the released version does.
		var p game.Packet
		if err := json.Unmarshal(body, &p); err != nil {
			t.Fatalf("an old board could not parse %s: %v", q.PacketPath, err)
		}
		seqs[p.Seq] = true
	}
	if !seqs[1] || !seqs[2] {
		t.Errorf("packets delivered = %v, want both Seq 1 and Seq 2", seqs)
	}
}

// The same peer read back through the current reader: raw is not a dead end,
// because readTransport already accepts a plain packet as a one-entry legacy
// bundle. That is what makes the transition work in both directions.
func TestARawPacketIsStillReadableByThisBuild(t *testing.T) {
	raw := []byte(`{"FromBoard":"Bravo BBS","FromNode":2,"ToNode":1,"Seq":1,"League":100}`)
	manifest, entries, err := readTransport(raw, "00640001.BRP")
	if err != nil {
		t.Fatalf("this build cannot read what a raw link emits: %v", err)
	}
	if manifest.Delivery != "legacy" || len(entries) != 1 || entries[0].Packet.Seq != 1 {
		t.Fatalf("manifest=%+v entries=%d", manifest, len(entries))
	}
}
