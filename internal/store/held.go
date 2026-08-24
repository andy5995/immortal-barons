package store

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/andy5995/immortal-barons/internal/game"
)

// HeldDir is where a packet waits when it speaks a protocol this build cannot
// apply. It is NOT refused: refusing destroys a roster update, mail, or a
// returning strike that would have been perfectly good once the two boards are
// level again, and the sender has no way to know it should send it twice.
//
// Held packets are re-read at the start of every planetary run, so a board that
// upgrades applies its backlog on the next run with nobody doing anything.
const HeldDir = "held"

// heldPath is the data directory's held-packet folder.
func heldPath(dataDir string) string { return filepath.Join(dataDir, HeldDir) }

// moveFile moves a file, falling back to copy-and-delete when the two paths are
// on different filesystems. os.Rename alone is not enough here: an inbound
// directory is usually the MAILER's, which is routinely a different mount from
// the game's data directory, and rename across one fails with EXDEV. That would
// fail the whole planetary run rather than hold one packet.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	// Written under a temporary name and renamed into place, so a run
	// interrupted mid-copy cannot leave a half-packet that later reads as a
	// corrupt one. The rename is within one directory, so it cannot hit EXDEV.
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Remove(src)
}

// holdPacket moves an inbound packet file aside to wait for a build that can
// read it. A failure to move it is reported to the caller: silently leaving the
// file in the inbound directory would have it re-read, re-held and re-announced
// on every run for as long as the mismatch lasts.
func holdPacket(dataDir, path string) error {
	dir := heldPath(dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return moveFile(path, filepath.Join(dir, filepath.Base(path)))
}

// releaseHeld moves every held packet this build can now read back into the
// inbound directory, and reports how many moved. The ones it still cannot read
// stay put.
//
// Moving them back rather than applying them here is deliberate: they then go
// through the ordinary inbound path, so the league check, the duplicate check,
// the addressing check and both signature checks all apply exactly as they would
// have on the day the packet arrived. A second, shorter path into the world is
// how a check gets skipped by accident.
func releaseHeld(dataDir, inboundDir string) (int, error) {
	entries, err := os.ReadDir(heldPath(dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var moved int
	for _, e := range entries {
		if e.IsDir() || !IsPacketFile(e.Name()) {
			continue
		}
		path := filepath.Join(heldPath(dataDir), e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var p game.Packet
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		if !game.SpeaksOurProtocol(p.Protocol) {
			continue
		}
		if err := moveFile(path, filepath.Join(inboundDir, e.Name())); err != nil {
			continue
		}
		moved++
	}
	return moved, nil
}
