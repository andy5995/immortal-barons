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
	type2HeaderSize  = 190
	type2SubjectSize = 72
	programName      = "Immortal Barons"

	// subjectMarginBytes is how little headroom is worth reporting. The packet
	// name is fixed-width for a given pair of nodes, so a path that fits today
	// does not grow on its own; what consumes a thin margin is a board joining
	// the league on a longer node number, or a change to the directory.
	subjectMarginBytes = 8

	attributePrivate    = 0x0001
	attributeFileAttach = 0x0010
	attributeKillSent   = 0x0080
	attributeLocal      = 0x0100
)

// createFileAttach writes an FTS-0001 Type-2 .msg file. The layout and control
// lines follow the OpenDoors implementation used by The Clans: a full attached
// pathname in the subject, file-attach/local/private/kill-sent attributes, and
// TOPT, FMPT, INTL, MSGID, PID and FLAGS KFS control lines as applicable.
func createFileAttach(transport Config, attached string, origin, destination Address) (string, error) {
	subject, _, err := fileAttachSubject(transport, attached)
	if err != nil {
		return "", err
	}
	netmailDir, binkley := transport.NetmailDir, transport.Binkley
	now := time.Now()
	id := messageID(now)
	header := type2Header(subject, origin, destination, now)
	body := messageText(origin, destination, id, binkley)
	data := make([]byte, 0, len(header)+len(body)+1)
	data = append(data, header[:]...)
	data = append(data, body...)
	data = append(data, 0)
	first, err := firstUnusedMessageNumber(netmailDir)
	if err != nil {
		return "", err
	}
	for number := first; ; number++ {
		path := filepath.Join(netmailDir, strconv.FormatUint(uint64(number), 10)+".msg")
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			continue // another handler won this message number
		}
		if err != nil {
			return "", err
		}
		if _, err := f.Write(data); err != nil {
			f.Close()
			os.Remove(path)
			return "", err
		}
		if err := f.Close(); err != nil {
			os.Remove(path)
			return "", err
		}
		return path, nil
	}
}

// fileAttachSubject spells the claimed file the way the configured mailer
// resolves it, and validates that before anything is handed over. The Type-2
// field is 72 bytes including its terminating NUL; Binkley consumes one more
// byte for its ^ convention. It also returns the bytes left unused, so a run
// can report a margin thin enough to fail on the next sequence digit.
func fileAttachSubject(transport Config, attached string) (string, int, error) {
	limit := type2SubjectSize - 1
	prefix := ""
	mode := ""
	if transport.Binkley {
		limit--
		prefix = "^"
		mode = " in Binkley mode"
	}
	spelled := transport.subjectPath(attached)
	if strings.IndexByte(spelled, 0) >= 0 {
		return "", 0, fmt.Errorf("attachment path contains a NUL byte")
	}
	if len(spelled) > limit {
		return "", 0, fmt.Errorf("attachment subject %q is %d bytes; FTN Type-2 permits at most %d%s. %s",
			spelled, len(spelled), limit, mode, subjectAdvice(transport.SubjectMode))
	}
	return prefix + spelled, limit - len(spelled), nil
}

func subjectAdvice(mode SubjectMode) string {
	switch mode {
	case SubjectBasename:
		return "The filename alone does not fit, so no SubjectPath setting can shorten it"
	case SubjectPrefixed:
		return `Shorten the SubjectPath prefix in ftn.cfg, or set "SubjectPath Basename" to write the filename alone, ` +
			"with AttachDir naming the directory the mailer searches"
	}
	return `Set SubjectPath in ftn.cfg: "Basename" writes the filename alone, with AttachDir naming the directory ` +
		`the mailer searches; a prefix such as "SubjectPath fido" is resolved against the mailer's working directory`
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
