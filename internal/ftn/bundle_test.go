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

func TestRunOutSupportsUnsetLeagueNumber(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "")
	configPath := filepath.Join(data, store.BoardConfigFile)
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	config = bytes.Replace(config, []byte("LeagueNumber 100\n"), nil, 1)
	if err := os.WriteFile(configPath, config, 0o644); err != nil {
		t.Fatal(err)
	}
	writeNamedPacket(t, data, "unset.brp", game.Packet{FromNode: 2, ToNode: 1, Seq: 1})

	result, err := RunOut(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Queued) != 1 {
		t.Fatalf("queued = %+v, want one bundle", result.Queued)
	}
	if got := filepath.Base(result.Queued[0].PacketPath); !strings.HasPrefix(got, "0002") {
		t.Fatalf("unset-league alias = %q, want league-0/node-2 namespace 0002", got)
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
	data := newBundledSetup(t, "Bravo BBS", "Link 1 BSO bso Normal\n")
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
	data := newBundledSetup(t, "Bravo BBS", "Link 1 BSO bso Normal\n")
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
	data := newBundledSetup(t, "Bravo BBS", "Link 1 BSO bso Normal\n")
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
	data := newBundledSetup(t, "Alpha BBS", "Link 3 Obox obox3\n")
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

func TestOboxMeshFanoutCanBeDisabled(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "OboxMeshFanout No\nLink 3 Obox obox3\n")
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
	data := newBundledSetup(t, "Alpha BBS", "Link 3 BSO bso Normal\nLink 4 Obox obox4\n")
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
		ConfigFile:            "NetmailDir netmail\nAttachDir attach\nSubjectPath Basename\nInboundDir transport-in\nInboundNetmailDir transport-in\n" + extraFTN,
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
