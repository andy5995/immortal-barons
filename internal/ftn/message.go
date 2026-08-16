package ftn

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	type2HeaderSize = 190
	programName     = "Immortal Barons"

	attributePrivate    = 0x0001
	attributeFileAttach = 0x0010
	attributeKillSent   = 0x0080
	attributeLocal      = 0x0100
)

// createFileAttach writes an FTS-0001 Type-2 .msg file. The layout and control
// lines follow the OpenDoors implementation used by The Clans: a full attached
// pathname in the subject, file-attach/local/private/kill-sent attributes, and
// TOPT, FMPT, INTL, MSGID, PID and FLAGS KFS control lines as applicable.
func createFileAttach(netmailDir, attached string, origin, destination Address, binkley bool) (string, error) {
	subject := attached
	if binkley {
		subject = "^" + subject
	}
	if strings.IndexByte(subject, 0) >= 0 || len(subject) > 71 {
		return "", fmt.Errorf("attachment subject is %d bytes; an FTN .msg subject holds at most 71", len(subject))
	}
	now := time.Now()
	id := messageID(now)
	header := type2Header(subject, origin, destination, now)
	body := messageText(origin, destination, id, binkley)
	data := make([]byte, 0, len(header)+len(body)+1)
	data = append(data, header[:]...)
	data = append(data, body...)
	data = append(data, 0)
	// Publish only a complete message: the mailer cannot see the temporary name,
	// and linking it to N.msg is both atomic and no-replace across competing
	// handler processes.
	tmp, err := os.CreateTemp(netmailDir, ".barons-ftn-*.tmp")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return "", err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	first, err := firstUnusedMessageNumber(netmailDir)
	if err != nil {
		return "", err
	}
	for number := first; ; number++ {
		path := filepath.Join(netmailDir, strconv.FormatUint(uint64(number), 10)+".msg")
		err := os.Link(tmpPath, path)
		if os.IsExist(err) {
			continue // another handler won this message number
		}
		if err != nil {
			return "", err
		}
		return path, nil
	}
}

func type2Header(attached string, origin, destination Address, now time.Time) [type2HeaderSize]byte {
	var header [type2HeaderSize]byte
	putString(header[0:36], programName)
	putString(header[36:72], programName)
	putString(header[72:144], attached)
	putString(header[144:164], now.Format("02 Jan 06  15:04:05"))
	words := []uint16{
		0,
		destination.Node,
		origin.Node,
		0,
		origin.Net,
		destination.Net,
		destination.Zone,
		origin.Zone,
		destination.Point,
		origin.Point,
		0,
		attributePrivate | attributeFileAttach | attributeKillSent | attributeLocal,
		0,
	}
	for i, word := range words {
		binary.LittleEndian.PutUint16(header[164+i*2:], word)
	}
	return header
}

func putString(dst []byte, value string) {
	copy(dst, value)
}

func messageText(origin, destination Address, id uint32, binkley bool) string {
	var b strings.Builder
	if destination.Point != 0 {
		fmt.Fprintf(&b, "\x01TOPT %d\r", destination.Point)
	}
	if origin.Point != 0 {
		fmt.Fprintf(&b, "\x01FMPT %d\r", origin.Point)
	}
	fmt.Fprintf(&b, "\x01INTL %d:%d/%d %d:%d/%d\r",
		destination.Zone, destination.Net, destination.Node,
		origin.Zone, origin.Net, origin.Node)
	fmt.Fprintf(&b, "\x01MSGID: %s %x\r", origin, id)
	b.WriteString("\x01PID: Immortal Barons\r")
	if !binkley {
		b.WriteString("\x01FLAGS KFS")
	}
	return b.String()
}

func messageID(now time.Time) uint32 {
	var random [4]byte
	if _, err := io.ReadFull(cryptorand.Reader, random[:]); err == nil {
		return uint32(now.Unix()) + binary.LittleEndian.Uint32(random[:])
	}
	return uint32(now.UnixNano())
}

func firstUnusedMessageNumber(dir string) (uint32, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	used := make(map[uint32]bool)
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".msg") {
			continue
		}
		base := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		n, err := strconv.ParseUint(base, 10, 32)
		if err == nil && n != 0 {
			used[uint32(n)] = true
		}
	}
	for n := uint32(1); n != 0; n++ {
		if !used[n] {
			return n, nil
		}
	}
	return 0, fmt.Errorf("netmail directory has no unused .msg number")
}
