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

// subjectAdvice names the setting that shortens a too-long subject, which is
// a different setting in each mode. Only SubjectAbsolute spells AttachDir's
// own path into the subject, so it is the only mode where shortening AttachDir
// changes the length; under SubjectPrefixed the subject is the prefix plus the
// filename and AttachDir's value never appears in it, yet the file is still
// written at AttachDir, which is why the shortened prefix has to name that
// same directory. Basename needs the mailer independently configured to search
// AttachDir itself, which SBBSecho cannot do -- it reads the directory out of
// the Subject and chdirs to ctrl at startup, so a bare filename is reported
// not found (sbbsecho.c:5944).
func subjectAdvice(mode SubjectMode) string {
	switch mode {
	case SubjectBasename:
		return "The filename alone does not fit, so no SubjectPath setting can shorten it"
	case SubjectPrefixed:
		return `Shorten the SubjectPath prefix -- that is what changes the subject's length, not AttachDir's ` +
			`own value -- and point AttachDir at the same directory the shortened prefix names, since the file ` +
			`is written at AttachDir and a subject naming somewhere else will not be found. Switching to ` +
			`"SubjectPath Basename" is the other way to shorten it, correct only if the mailer is ` +
			`independently configured to search AttachDir's own directory instead of reading it from the ` +
			"subject -- not every mailer supports that; SBBSecho, for one, has no such search path at all"
	}
	return `Set AttachDir in ftn.cfg to a shorter directory -- with SubjectPath left on its default ` +
		`("Absolute"), the subject is AttachDir's own path plus the filename, so this is what actually ` +
		"shortens it"
}

// subjectMarginPointer is the short form, for checkSubjectMargin's warning.
// The error fires once, on the run it blocks, so it can afford subjectAdvice
// in full; the warning repeats on every successful run for as long as a board
// stays within subjectMarginBytes, and that much prose each time trains a
// sysop to stop reading it. Basename's advice is already one sentence, so it
// is returned unchanged rather than padded with a pointer to the docs.
func subjectMarginPointer(mode SubjectMode) string {
	if mode == SubjectBasename {
		return subjectAdvice(mode)
	}
	const seeDocs = `see "Keeping attach subjects short" in docs/ftn-transport.md`
	if mode == SubjectPrefixed {
		return "shorten the SubjectPath prefix in ftn.cfg before it runs out; " + seeDocs
	}
	return "shorten AttachDir in ftn.cfg before it runs out; " + seeDocs
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
