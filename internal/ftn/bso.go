package ftn

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var errPeerBusy = errors.New("peer BSO queue is busy")

func bsoPaths(root string, address Address, flavour string) (busy, flow string, err error) {
	base := fmt.Sprintf("%04x%04x", address.Net, address.Node)
	dir := root
	if address.Point != 0 {
		dir = filepath.Join(root, base+".pnt")
		base = fmt.Sprintf("%08x", address.Point)
	}
	ext := map[string]string{
		"Immediate": ".ilo", "Continuous": ".clo", "Direct": ".dlo",
		"Normal": ".flo", "Hold": ".hlo",
	}[normalFlavour(flavour)]
	if ext == "" {
		return "", "", fmt.Errorf("unknown BSO flavour %q", flavour)
	}
	return filepath.Join(dir, base+".bsy"), filepath.Join(dir, base+ext), nil
}

func acquireBSY(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		return nil, errPeerBusy
	}
	return f, err
}

func releaseBSY(path string, f *os.File) error {
	closeErr := f.Close()
	removeErr := os.Remove(path)
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}

func addFlowEntry(path, attachment string) error {
	line := "^" + attachment
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, existing := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if existing == line {
			return nil
		}
	}
	if len(data) > 0 && data[len(data)-1] != '\n' && data[len(data)-1] != '\r' {
		data = append(data, '\n')
	}
	data = append(data, line...)
	data = append(data, '\n')
	return replaceFileAtomic(path, data, 0o644)
}

// appendBSOBundle merges incoming into a compatible bundle which this helper
// already advertised with a delete-after-send flow entry. The caller must hold
// the destination's .bsy for the complete read/replace operation.
func appendBSOBundle(flow, attachDir string, incoming []byte) (string, bool, error) {
	incomingManifest, incomingEntries, err := readTransport(incoming, "incoming.BRP")
	if err != nil {
		return "", false, err
	}
	if incomingManifest.Format != bundleFormat || incomingManifest.Delivery != "direct" {
		return "", false, fmt.Errorf("outgoing BSO handoff is not a BSO transport bundle")
	}
	incomingTransmitter, ok := bundleTransmitter(incomingEntries)
	if !ok {
		return "", false, fmt.Errorf("outgoing BSO bundle has no common transmitting hop")
	}
	paths, err := deleteAfterSendEntries(flow)
	if err != nil {
		return "", false, err
	}
	for _, path := range paths {
		if !pathInDirectory(path, attachDir) {
			continue
		}
		existing, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", false, err
		}
		manifest, entries, err := readTransport(existing, filepath.Base(path))
		existingTransmitter, sameTransmitter := bundleTransmitter(entries)
		if err != nil || manifest.Format != bundleFormat || manifest.Delivery != "direct" ||
			!sameTransmitter || existingTransmitter != incomingTransmitter || !sameBundleLeague(entries, incomingEntries) {
			continue
		}
		if len(entries)+len(incomingEntries) > bundleMaxEntries {
			continue
		}
		added := 0
		for _, incomingEntry := range incomingEntries {
			duplicate := false
			for _, existingEntry := range entries {
				if bytes.Equal(existingEntry.Raw, incomingEntry.Raw) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				entries = append(entries, incomingEntry)
				added++
			}
		}
		if added == 0 {
			return path, true, nil
		}
		merged, err := encodeBundle(manifest, entries)
		if errors.Is(err, errBundleTooLarge) {
			continue
		}
		if err != nil {
			return "", false, err
		}
		if err := replaceFileAtomic(path, merged, 0o644); err != nil {
			return "", false, err
		}
		return path, true, nil
	}
	return "", false, nil
}

func sameBundleLeague(left, right []transportEntry) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	league := left[0].Packet.League
	for _, group := range [][]transportEntry{left, right} {
		for _, entry := range group {
			if entry.Packet.League != league {
				return false
			}
		}
	}
	return true
}

func deleteAfterSendEntries(flow string) ([]string, error) {
	data, err := os.ReadFile(flow)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if !strings.HasPrefix(line, "^") {
			continue
		}
		if path := strings.TrimSpace(strings.TrimPrefix(line, "^")); path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func pathInDirectory(path, dir string) bool {
	path, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
