package ftn

import (
	"archive/zip"
	"bytes"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

const (
	bundleFormat         = 1
	bundleManifestName   = "manifest.json"
	bundleMaxEntries     = 10000
	bundleMaxDecodedSize = 256 << 20
)

var errBundleTooLarge = errors.New("transport bundle is full")

// transportEntry carries one immutable game packet plus routing information
// which may change at an FTN hop. Raw is never re-marshalled.
type transportEntry struct {
	Name    string
	Raw     []byte
	Packet  game.Packet
	Route   []int
	Covered []int
}

type bundleManifest struct {
	Format   int                   `json:"format"`
	Delivery string                `json:"delivery"`
	Entries  []bundleManifestEntry `json:"entries"`
}

type bundleManifestEntry struct {
	Route   []int `json:"route"`
	Covered []int `json:"covered,omitempty"`
}

func makeBundle(transmitter int, delivery string, entries []transportEntry) ([]byte, bundleManifest, error) {
	if len(entries) == 0 {
		return nil, bundleManifest{}, fmt.Errorf("cannot make an empty transport bundle")
	}
	entries = entriesFromTransmitter(entries, transmitter)
	if err := validateTransportEntries(entries); err != nil {
		return nil, bundleManifest{}, err
	}
	manifest := bundleManifest{Format: bundleFormat, Delivery: delivery}
	manifest.Entries = manifestEntries(entries)
	body, err := encodeBundle(manifest, entries)
	return body, manifest, err
}

// encodeBundle rebuilds only the transport wrapper. The Raw bytes in entries
// are copied into ZIP members verbatim; in particular, signed game packets are
// never marshalled again.
func encodeBundle(manifest bundleManifest, entries []transportEntry) ([]byte, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("cannot make an empty transport bundle")
	}
	if len(entries) > bundleMaxEntries {
		return nil, fmt.Errorf("bundle has %d packet entries; maximum is %d", len(entries), bundleMaxEntries)
	}
	if err := validateTransportEntries(entries); err != nil {
		return nil, err
	}
	manifest.Entries = manifestEntries(entries)
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	expanded := len(manifestBytes)
	for _, entry := range entries {
		expanded += len(entry.Raw)
		if expanded > bundleMaxDecodedSize {
			return nil, fmt.Errorf("%w: expanded size exceeds %d bytes", errBundleTooLarge, bundleMaxDecodedSize)
		}
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	if err := writeZipMember(zw, bundleManifestName, manifestBytes); err != nil {
		return nil, err
	}
	for i, entry := range entries {
		name := store.PacketFilename(entry.Packet, entry.Raw)
		if err := writeZipMember(zw, fmt.Sprintf("packets/%06d/%s", i, name), entry.Raw); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func manifestEntries(entries []transportEntry) []bundleManifestEntry {
	out := make([]bundleManifestEntry, len(entries))
	for i, entry := range entries {
		out[i] = bundleManifestEntry{
			Route: append([]int(nil), entry.Route...), Covered: append([]int(nil), entry.Covered...),
		}
	}
	return out
}

func writeZipMember(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func randomBundleID() (string, error) {
	var id [16]byte
	if _, err := io.ReadFull(cryptorand.Reader, id[:]); err != nil {
		return "", fmt.Errorf("bundle id: %w", err)
	}
	return hex.EncodeToString(id[:]), nil
}

// readTransport decodes a ZIP transport bundle. A JSON game packet is accepted
// as a one-entry legacy bundle so receivers can be upgraded before senders.
func readTransport(data []byte, _ string) (bundleManifest, []transportEntry, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var packet game.Packet
		if err := json.Unmarshal(data, &packet); err != nil {
			return bundleManifest{}, nil, err
		}
		entry := transportEntry{Name: store.PacketFilename(packet, data), Raw: append([]byte(nil), data...), Packet: packet}
		if packet.FromNode > 0 {
			entry.Route = []int{packet.FromNode}
		}
		return bundleManifest{Format: 0, Delivery: "legacy"}, []transportEntry{entry}, nil
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return bundleManifest{}, nil, err
	}
	if len(zr.File) == 0 || len(zr.File) > bundleMaxEntries+1 {
		return bundleManifest{}, nil, fmt.Errorf("bundle has %d members; maximum is %d packet entries", len(zr.File), bundleMaxEntries)
	}
	files := make(map[string]*zip.File, len(zr.File))
	var packetFiles []*zip.File
	var total uint64
	for _, file := range zr.File {
		if file.FileInfo().IsDir() || filepath.ToSlash(file.Name) != file.Name || strings.Contains(file.Name, "../") || strings.HasPrefix(file.Name, "/") {
			return bundleManifest{}, nil, fmt.Errorf("unsafe bundle member %q", file.Name)
		}
		if _, duplicate := files[file.Name]; duplicate {
			return bundleManifest{}, nil, fmt.Errorf("duplicate bundle member %q", file.Name)
		}
		files[file.Name] = file
		if file.Name != bundleManifestName {
			packetFiles = append(packetFiles, file)
		}
		total += file.UncompressedSize64
		if total > bundleMaxDecodedSize {
			return bundleManifest{}, nil, fmt.Errorf("bundle expands beyond %d bytes", bundleMaxDecodedSize)
		}
	}
	mf := files[bundleManifestName]
	if mf == nil {
		return bundleManifest{}, nil, fmt.Errorf("bundle has no %s", bundleManifestName)
	}
	manifestBytes, err := readZipMember(mf)
	if err != nil {
		return bundleManifest{}, nil, fmt.Errorf("read manifest: %w", err)
	}
	var manifest bundleManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return bundleManifest{}, nil, fmt.Errorf("decode manifest: %w", err)
	}
	if manifest.Format != bundleFormat {
		return bundleManifest{}, nil, fmt.Errorf("transport bundle format %d is not supported", manifest.Format)
	}
	if (manifest.Delivery != "attach" && manifest.Delivery != "direct") ||
		len(manifest.Entries) == 0 || len(manifest.Entries) > bundleMaxEntries {
		return bundleManifest{}, nil, fmt.Errorf("invalid bundle manifest")
	}
	if len(packetFiles) != len(manifest.Entries) {
		return bundleManifest{}, nil, fmt.Errorf("bundle has %d packet members but manifest has %d routing entries", len(packetFiles), len(manifest.Entries))
	}
	entries := make([]transportEntry, 0, len(manifest.Entries))
	for i, item := range manifest.Entries {
		if !validNodeList(item.Route) || !validNodeList(item.Covered) {
			return bundleManifest{}, nil, fmt.Errorf("invalid transport route for packet member %d", i)
		}
		file := packetFiles[i]
		if !strings.HasPrefix(file.Name, "packets/") || !store.IsPacketFile(filepath.Base(file.Name)) {
			return bundleManifest{}, nil, fmt.Errorf("unexpected packet member %q", file.Name)
		}
		raw, err := readZipMember(file)
		if err != nil {
			return bundleManifest{}, nil, fmt.Errorf("read %s: %w", file.Name, err)
		}
		var packet game.Packet
		if err := json.Unmarshal(raw, &packet); err != nil {
			return bundleManifest{}, nil, fmt.Errorf("packet %s: %w", file.Name, err)
		}
		entries = append(entries, transportEntry{
			Name: store.PacketFilename(packet, raw), Raw: raw, Packet: packet,
			Route: append([]int(nil), item.Route...), Covered: append([]int(nil), item.Covered...),
		})
	}
	if _, ok := bundleTransmitter(entries); !ok {
		return bundleManifest{}, nil, fmt.Errorf("bundle packet routes do not name one transmitting node")
	}
	return manifest, entries, nil
}

func validNodeList(nodes []int) bool {
	seen := map[int]bool{}
	for _, node := range nodes {
		if node < 1 || node > 999 || seen[node] {
			return false
		}
		seen[node] = true
	}
	return true
}

func validateTransportEntries(entries []transportEntry) error {
	for i, entry := range entries {
		if !validNodeList(entry.Route) || !validNodeList(entry.Covered) {
			return fmt.Errorf("invalid transport route for packet entry %d", i)
		}
	}
	if _, ok := bundleTransmitter(entries); !ok {
		return fmt.Errorf("bundle packet routes do not name one transmitting node")
	}
	return nil
}

func entriesFromTransmitter(entries []transportEntry, transmitter int) []transportEntry {
	out := make([]transportEntry, len(entries))
	for i, entry := range entries {
		out[i] = entry
		out[i].Route = append([]int(nil), entry.Route...)
		if len(out[i].Route) == 0 || out[i].Route[len(out[i].Route)-1] != transmitter {
			out[i].Route = append(out[i].Route, transmitter)
		}
		out[i].Covered = append([]int(nil), entry.Covered...)
	}
	return out
}

func bundleTransmitter(entries []transportEntry) (int, bool) {
	transmitter := 0
	for _, entry := range entries {
		if len(entry.Route) == 0 {
			return 0, false
		}
		last := entry.Route[len(entry.Route)-1]
		if last < 1 || last > 999 || transmitter != 0 && transmitter != last {
			return 0, false
		}
		transmitter = last
	}
	return transmitter, transmitter != 0
}

func transportHops(entry transportEntry) int {
	hops := entry.Packet.Hops
	if len(entry.Route) > 1 {
		hops += len(entry.Route) - 1
	}
	return hops
}

func readZipMember(file *zip.File) ([]byte, error) {
	r, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
