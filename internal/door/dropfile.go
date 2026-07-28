// Package door reads BBS dropfiles so Immortal Barons can run as a native
// door. A dropfile is written by the BBS when a caller launches the door;
// it tells the door who the caller is and how to talk to them.
//
// DOOR32.SYS is the primary format (both Synchronet and Mystic write it,
// and it cleanly encodes the I/O method). DOOR.SYS, PCBOARD.SYS, and
// DORINFO1.DEF are supported as widely-used alternatives. Field positions were
// cross-checked against the Synchronet source (src/xpdoor/dropfiles.c), the
// Synchronet wiki PCBOARD.SYS and DORINFO1.DEF references, and BRE's own
// INSTALL.CFG.
//
// The sysop declares which format their BBS writes once (stored in door.json,
// set with -set-dropfile); ParseDropfileAs then reads that format regardless of
// the file's name. ParseDropfile still auto-dispatches on the filename for a
// bare path.
package door

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Format identifies a dropfile format the door can read. ID is what the sysop's
// config stores (door.json); Name is for display; File is the canonical filename
// for working-directory auto-detection.
type Format struct {
	ID   string
	Name string
	File string
}

// Formats lists the supported dropfile formats in menu order.
var Formats = []Format{
	{ID: "door32", Name: "DOOR32.SYS", File: "DOOR32.SYS"},
	{ID: "doorsys", Name: "DOOR.SYS", File: "DOOR.SYS"},
	{ID: "pcboard", Name: "PCBOARD.SYS", File: "PCBOARD.SYS"},
	{ID: "dorinfo", Name: "DORINFO1.DEF", File: "DORINFO1.DEF"},
}

// FormatByID returns the Format with the given ID.
func FormatByID(id string) (Format, bool) {
	for _, f := range Formats {
		if f.ID == id {
			return f, true
		}
	}
	return Format{}, false
}

// IOMode is how the BBS expects the door to talk to the caller.
type IOMode int

const (
	IOLocal IOMode = iota
	IOSerial
	IOSocket
	IOStdio
)

// Caller is the subset of dropfile information the game needs.
type Caller struct {
	Handle      string // alias/handle (falls back to real name)
	RealName    string
	Node        int
	SecondsLeft int
	ANSI        bool
	Rows        int
	IO          IOMode
	Socket      int    // socket/comm handle when IO is IOSocket
	BBSID       string // BBS software name/version, if provided
}

// ParseDropfileAs reads the dropfile at path as the named format (a Format ID).
// An empty format falls back to dispatching on the filename, preserving the old
// auto-detect behavior for a bare -dropfile path.
func ParseDropfileAs(path, format string) (*Caller, error) {
	switch format {
	case "":
		return ParseDropfile(path)
	case "door32":
		return parseLineFile(path, parseDoor32)
	case "doorsys":
		return parseLineFile(path, parseDoorSys)
	case "dorinfo":
		return parseLineFile(path, parseDorInfo)
	case "pcboard":
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return parsePCBoard(b)
	default:
		return nil, fmt.Errorf("unknown dropfile format %q", format)
	}
}

// ParseDropfile reads the dropfile at path, dispatching on its filename. Used
// when the format isn't configured (a bare path).
func ParseDropfile(path string) (*Caller, error) {
	name := strings.ToLower(filepath.Base(path))
	if name == "pcboard.sys" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return parsePCBoard(b)
	}
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	switch {
	case name == "door32.sys":
		return parseDoor32(lines)
	case name == "door.sys":
		return parseDoorSys(lines)
	case strings.HasPrefix(name, "dorinfo") && strings.HasSuffix(name, ".def"):
		return parseDorInfo(lines) // BBBS names it dorinfo<node>.def
	default:
		return nil, fmt.Errorf("unsupported dropfile %q (want DOOR32.SYS, DOOR.SYS, PCBOARD.SYS, or DORINFO1.DEF)", filepath.Base(path))
	}
}

// parseLineFile reads a line-based dropfile and hands its lines to parse.
func parseLineFile(path string, parse func([]string) (*Caller, error)) (*Caller, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	return parse(lines)
}

// DOOR32.SYS: 11 lines.
//
//	1 comm type (0=local, 1=serial, 2=telnet socket)
//	2 comm/socket handle    3 baud             4 BBS software id
//	5 user record number    6 real name        7 handle/alias
//	8 security level        9 time left (min)  10 emulation (0=ascii,1+=ansi)
//	11 node number
func parseDoor32(l []string) (*Caller, error) {
	if len(l) < 11 {
		return nil, fmt.Errorf("DOOR32.SYS too short: %d lines, want 11", len(l))
	}
	c := &Caller{
		BBSID:       field(l, 4),
		RealName:    field(l, 6),
		Handle:      field(l, 7),
		SecondsLeft: atoi(field(l, 9)) * 60,
		ANSI:        atoi(field(l, 10)) >= 1,
		Node:        atoi(field(l, 11)),
		Socket:      atoi(field(l, 2)),
		Rows:        25,
	}
	switch atoi(field(l, 1)) {
	case 1:
		c.IO = IOSerial
	case 2:
		c.IO = IOSocket
	default:
		c.IO = IOLocal
	}
	if c.Handle == "" {
		c.Handle = c.RealName
	}
	return c, nil
}

// DOOR.SYS: line 1 encodes the I/O mode (COM0:STDIO / COM0:SOCKETn / COMx:);
// name is line 10, seconds/minutes left lines 18/19, ANSI flag line 20
// ("GR" = graphics), screen rows line 21.
func parseDoorSys(l []string) (*Caller, error) {
	if len(l) < 21 {
		return nil, fmt.Errorf("DOOR.SYS too short: %d lines, want >=21", len(l))
	}
	c := &Caller{
		Node:   atoi(field(l, 4)),
		Handle: field(l, 10),
		Rows:   atoi(field(l, 21)),
	}
	switch m := strings.ToUpper(field(l, 1)); {
	case m == "COM0:STDIO":
		c.IO = IOStdio
	case strings.HasPrefix(m, "COM0:SOCKET"):
		c.IO = IOSocket
		c.Socket = atoi(m[len("COM0:SOCKET"):])
	case strings.HasPrefix(m, "COM0"):
		c.IO = IOLocal
	default:
		c.IO = IOSerial
	}
	sec := atoi(field(l, 18))
	if m := atoi(field(l, 19)) * 60; m > sec {
		sec = m
	}
	c.SecondsLeft = sec
	c.ANSI = strings.ToUpper(field(l, 20)) == "GR"
	if c.Rows == 0 {
		c.Rows = 25
	}
	return c, nil
}

// DORINFO1.DEF (RBBS/QuickBBS/RemoteAccess): 12+ CRLF-delimited lines.
//
//	1 BBS name          2 sysop first     3 sysop last
//	4 COM port          5 baud string     6 reserved
//	7 caller first      8 caller last     9 caller city
//	10 graphics (1=ANSI)  11 access level  12 minutes left
//
// No node number or screen-length field. Offsets match the Synchronet wiki
// DORINFO reference, BRE's INSTALL.CFG map, and BBBS's opendoor.bz writer. Like
// PCBOARD.SYS it carries no socket/stdio indicator, so I/O defaults to stdio
// (COM0 => local; both are stdio on a *nix door).
func parseDorInfo(l []string) (*Caller, error) {
	if len(l) < 12 {
		return nil, fmt.Errorf("DORINFO1.DEF too short: %d lines, want >=12", len(l))
	}
	handle := strings.TrimSpace(field(l, 7) + " " + field(l, 8))
	c := &Caller{
		Handle:      handle,
		RealName:    handle,
		SecondsLeft: atoi(field(l, 12)) * 60,
		ANSI:        atoi(field(l, 10)) >= 1,
		Rows:        25,
		IO:          IOStdio,
	}
	if strings.HasPrefix(strings.ToUpper(field(l, 4)), "COM0") {
		c.IO = IOLocal
	}
	return c, nil
}

// parsePCBoard reads a PCBOARD.SYS drop file. Unlike the line-based formats it
// is a packed binary record. The fields the game needs all sit in the stable
// 128-byte core block (plus the Use-ANSI byte at offset 128), so this is robust
// across PCBoard versions 12/14/15 — later versions only append fields. Byte
// offsets follow the Synchronet wiki PCBOARD.SYS reference. PCBOARD.SYS carries
// no socket/stdio indicator, so I/O defaults to stdio: a *nix BBS pipes the
// caller to stdin/stdout. A Windows winsock socket door needs DOOR32.SYS, which
// carries the handle.
//
// Not yet validated against a real BBBS-generated PCBOARD.SYS; the layout is the
// documented PCBoard standard, which any conformant writer follows for the core
// block, but a smoke test against BBBS output is still worthwhile.
func parsePCBoard(b []byte) (*Caller, error) {
	if len(b) < 129 {
		return nil, fmt.Errorf("PCBOARD.SYS too short: %d bytes, want >=129", len(b))
	}
	name := strings.TrimSpace(string(b[84:109]))                          // User's Full Name (offset 84, 25 bytes)
	minutes := max(int(int16(binary.LittleEndian.Uint16(b[109:111]))), 0) // Minutes remaining (offset 109)
	node := int(b[111])                                                   // Node number (offset 111; ' ' when no network)
	if b[111] == ' ' {
		node = 0
	}
	ansi := b[128] == 1 || b[128] == '1' // Use ANSI (offset 128)
	if !ansi && (b[11] == 'Y' || b[11] == 'y') {
		ansi = true // Graphics Mode (offset 11) as a backup
	}
	return &Caller{
		Handle:      name,
		RealName:    name,
		Node:        node,
		SecondsLeft: minutes * 60,
		ANSI:        ansi,
		Rows:        25, // no screen-length field in PCBOARD.SYS (it lives in USERS.SYS)
		IO:          IOStdio,
	}, nil
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

// field returns 1-based line n, trimmed, or "" if absent.
func field(lines []string, n int) string {
	if n-1 < len(lines) {
		return strings.TrimSpace(lines[n-1])
	}
	return ""
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
