package ftn

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/store"
)

var errPeerBusy = errors.New("peer BSO queue is busy")

const (
	bsyOwnerPrefix = "barons-ftn pid="
	bsyStaleAge    = 5 * time.Minute
)

type bsyLock struct {
	lock *store.FileLock
}

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

func acquireBSY(path string) (*bsyLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	for attempts := 0; attempts < 3; attempts++ {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			lock, err := store.LockFile(f, false)
			if err != nil {
				f.Close()
				_ = os.Remove(path)
				return nil, err
			}
			if _, err := fmt.Fprintf(f, "%s%d\n", bsyOwnerPrefix, os.Getpid()); err != nil {
				lock.Release()
				_ = os.Remove(path)
				return nil, err
			}
			if err := f.Sync(); err != nil {
				lock.Release()
				_ = os.Remove(path)
				return nil, err
			}
			return &bsyLock{lock: lock}, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		recovered, err := recoverOwnBSY(path)
		if err != nil {
			return nil, err
		}
		if !recovered {
			return nil, errPeerBusy
		}
	}
	return nil, errPeerBusy
}

// recoverOwnBSY removes only an old semaphore which identifies barons-ftn and
// whose ownership lock is no longer held by its creating process. Foreign and
// legacy empty semaphores remain the mailer's recovery responsibility.
func recoverOwnBSY(path string) (bool, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	lock, err := store.LockFile(f, false)
	if errors.Is(err, store.ErrBusy) {
		f.Close()
		return false, nil
	}
	if err != nil {
		f.Close()
		return false, err
	}

	info, err := f.Stat()
	if err != nil {
		lock.Release()
		return false, err
	}
	data, err := io.ReadAll(io.LimitReader(f, 70))
	if err != nil {
		lock.Release()
		return false, err
	}
	pidText, ours := strings.CutPrefix(strings.TrimSpace(string(data)), bsyOwnerPrefix)
	pid, pidErr := strconv.Atoi(pidText)
	if !ours || pidErr != nil || pid < 1 || time.Since(info.ModTime()) < bsyStaleAge {
		lock.Release()
		return false, nil
	}
	current, err := os.Stat(path)
	if os.IsNotExist(err) {
		lock.Release()
		return true, nil
	}
	if err != nil {
		lock.Release()
		return false, err
	}
	if !os.SameFile(info, current) {
		lock.Release()
		return true, nil
	}
	if err := removeBSYFile(path, lock); err != nil {
		return false, err
	}
	return true, nil
}

func releaseBSY(path string, held *bsyLock) error {
	return removeBSYFile(path, held.lock)
}

// Unix permits unlinking an open locked file, which closes the recovery race.
// Windows refuses that unlink; there we close first, and a competing open
// handle makes Remove fail rather than deleting another process's semaphore.
func removeBSYFile(path string, lock *store.FileLock) error {
	if runtime.GOOS == "windows" {
		if err := lock.Release(); err != nil {
			return err
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	removeErr := os.Remove(path)
	releaseErr := lock.Release()
	if removeErr != nil && !os.IsNotExist(removeErr) {
		return removeErr
	}
	return releaseErr
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
		if err != nil {
			continue
		}
		existingTransmitter, sameTransmitter := bundleTransmitter(entries)
		if manifest.Format != bundleFormat || manifest.Delivery != "direct" ||
			!sameTransmitter || existingTransmitter != incomingTransmitter || !sameBundleLeague(entries, incomingEntries) {
			continue
		}
		if len(entries)+len(incomingEntries) > bundleMaxEntries {
			continue
		}
		// Index once rather than comparing every incoming body with every
		// existing body while the peer's .bsy is held. The byte comparison
		// inside a digest bucket keeps "duplicate" defined as exact bytes,
		// even in the theoretical event of a SHA-256 collision.
		existingByDigest := make(map[[sha256.Size]byte][][]byte, len(entries)+len(incomingEntries))
		for _, entry := range entries {
			digest := sha256.Sum256(entry.Raw)
			existingByDigest[digest] = append(existingByDigest[digest], entry.Raw)
		}
		added := 0
		for _, incomingEntry := range incomingEntries {
			duplicate := false
			digest := sha256.Sum256(incomingEntry.Raw)
			for _, existingRaw := range existingByDigest[digest] {
				if bytes.Equal(existingRaw, incomingEntry.Raw) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				entries = append(entries, incomingEntry)
				existingByDigest[digest] = append(existingByDigest[digest], incomingEntry.Raw)
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
