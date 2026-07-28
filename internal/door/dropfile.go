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

// Format describes a dropfile format the door can read. ID is what the sysop's
// config stores (door.json); Name is for display; File is the canonical filename
// for working-directory auto-detection.
//
// read and matches keep this the single source of truth: adding a format means
// adding one entry here, not touching a parse switch, a filename switch, and a
// hardcoded list in an error message.
type Format struct {
	ID   string
	Name string
	File string

	read    func(path string) (*Caller, error) // parses this format
	matches func(lowerName string) bool        // nil = the file name equals File
}

// Formats lists the supported dropfile formats in menu order.
var Formats = []Format{
	{ID: "door32", Name: "DOOR32.SYS", File: "DOOR32.SYS", read: fromLines(parseDoor32)},
	{ID: "doorsys", Name: "DOOR.SYS", File: "DOOR.SYS", read: fromLines(parseDoorSys)},
	{ID: "pcboard", Name: "PCBOARD.SYS", File: "PCBOARD.SYS", read: fromBytes(parsePCBoard)},
	{ID: "dorinfo", Name: "DORINFO1.DEF", File: "DORINFO1.DEF", read: fromLines(parseDorInfo),
		matches: func(n string) bool { // BBBS names it dorinfo<node>.def
			return strings.HasPrefix(n, "dorinfo") && strings.HasSuffix(n, ".def")
		}},
}

// fromLines adapts a line-based parser to Format.read.
func fromLines(parse func([]string) (*Caller, error)) func(string) (*Caller, error) {
	return func(path string) (*Caller, error) {
		lines, err := readLines(path)
		if err != nil {
			return nil, err
		}
		return parse(lines)
	}
}

// fromBytes adapts a binary parser to Format.read.
func fromBytes(parse func([]byte) (*Caller, error)) func(string) (*Caller, error) {
	return func(path string) (*Caller, error) {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return parse(b)
	}
}

// match reports whether a lower-cased file name looks like this format.
func (f Format) match(lowerName string) bool {
	if f.matches != nil {
		return f.matches(lowerName)
	}
	return lowerName == strings.ToLower(f.File)
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

// formatNames lists the supported format names, derived from Formats so an error
// message can't drift from the registry.
func formatNames() string {
	n := make([]string, len(Formats))
	for i, f := range Formats {
		n[i] = f.Name
	}
	return strings.Join(n, ", ")
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
	if format == "" {
		return ParseDropfile(path)
	}
	f, ok := FormatByID(format)
	if !ok {
		return nil, fmt.Errorf("unknown dropfile format %q (want %s)", format, formatNames())
	}
	return checkCaller(f.read(path))
}

// ParseDropfile reads the dropfile at path, dispatching on its filename. Used
// when the format isn't configured (a bare path).
func ParseDropfile(path string) (*Caller, error) {
	base := filepath.Base(path)
	lower := strings.ToLower(base)
	for _, f := range Formats {
		if f.match(lower) {
			return checkCaller(f.read(path))
		}
	}
	return nil, fmt.Errorf("unsupported dropfile %q (want %s)", base, formatNames())
}

// fieldReader reads numeric drop file fields, remembering the first failure so a
// parser can read a run of fields and check once (the errWriter pattern from
// Effective Go).
//
// An empty field is 0 — BBS software routinely leaves optional fields blank — but
// a non-empty field that is not a number means the file is corrupt or is not the
// format we were told to read. That second case is the common real-world
// misconfiguration: a sysop selects DOOR32.SYS while the BBS writes DOOR.SYS, and
// without this the text in a numeric slot silently reads as 0.
type fieldReader struct {
	lines []string
	err   error
}

func (r *fieldReader) num(n int, what string) int {
	if r.err != nil {
		return 0
	}
	s := field(r.lines, n)
	if s == "" {
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		r.err = fmt.Errorf("line %d (%s): %q is not a number", n, what, s)
		return 0
	}
	return v
}

// checkCaller rejects a drop file that parsed structurally but carries no usable
// caller identity. A drop file is written by another program, so it is a real
// boundary: a corrupt or wrong-format file can satisfy the length checks and
// still yield an empty name. Without this the empty name falls back to a
// node-numbered handle, so every caller with a malformed drop file would share
// one empire. Fail the launch instead — the sysop sees it in ib-door.log.
func checkCaller(c *Caller, err error) (*Caller, error) {
	if err != nil {
		return nil, err
	}
	if c.Handle == "" {
		return nil, fmt.Errorf("drop file has no caller name (corrupt, or not the configured format)")
	}
	return c, nil
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
	r := &fieldReader{lines: l}
	comm := r.num(1, "comm type")
	sock := r.num(2, "comm/socket handle")
	mins := r.num(9, "time left")
	emu := r.num(10, "emulation")
	node := r.num(11, "node number")
	if r.err != nil {
		return nil, r.err
	}
	if comm < 0 || comm > 2 {
		return nil, fmt.Errorf("line 1 (comm type): %d is not 0 (local), 1 (serial), or 2 (socket)", comm)
	}
	c := &Caller{
		BBSID:       field(l, 4),
		RealName:    field(l, 6),
		Handle:      field(l, 7),
		SecondsLeft: max(mins, 0) * 60,
		ANSI:        emu >= 1,
		Node:        node,
		Socket:      sock,
		Rows:        25,
	}
	switch comm {
	case 1:
		c.IO = IOSerial
	case 2:
		c.IO = IOSocket
	default:
		c.IO = IOLocal
	}
	// Socket mode with no handle would attach to fd 0 (stdin) instead of the
	// caller — fail rather than talk to the wrong stream.
	if c.IO == IOSocket && sock <= 0 {
		return nil, fmt.Errorf("line 2 (socket handle): socket mode needs a handle, got %d", sock)
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
	r := &fieldReader{lines: l}
	node := r.num(4, "node number")
	secs := r.num(18, "seconds left")
	mins := r.num(19, "minutes left")
	rows := r.num(21, "screen rows")
	if r.err != nil {
		return nil, r.err
	}
	c := &Caller{
		Node:   node,
		Handle: field(l, 10),
		Rows:   rows,
	}
	switch m := strings.ToUpper(field(l, 1)); {
	case m == "COM0:STDIO":
		c.IO = IOStdio
	case strings.HasPrefix(m, "COM0:SOCKET"):
		c.IO = IOSocket
		c.Socket = atoi(m[len("COM0:SOCKET"):])
		if c.Socket <= 0 {
			return nil, fmt.Errorf("line 1 (%s): socket mode needs a handle", m)
		}
	case strings.HasPrefix(m, "COM0"):
		c.IO = IOLocal
	default:
		c.IO = IOSerial
	}
	c.SecondsLeft = max(secs, mins*60, 0)
	c.ANSI = strings.ToUpper(field(l, 20)) == "GR"
	if c.Rows <= 0 {
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
// DORINFO reference (https://wiki.synchro.net/ref:dorinfo1.def), BRE's
// INSTALL.CFG map, and BBBS's opendoor.bz writer. Like
// PCBOARD.SYS it carries no socket/stdio indicator, so I/O defaults to stdio
// (COM0 => local; both are stdio on a *nix door).
func parseDorInfo(l []string) (*Caller, error) {
	if len(l) < 12 {
		return nil, fmt.Errorf("DORINFO1.DEF too short: %d lines, want >=12", len(l))
	}
	r := &fieldReader{lines: l}
	graphics := r.num(10, "graphics")
	mins := r.num(12, "minutes left")
	if r.err != nil {
		return nil, r.err
	}
	handle := strings.TrimSpace(field(l, 7) + " " + field(l, 8))
	c := &Caller{
		Handle:      handle,
		RealName:    handle,
		SecondsLeft: max(mins, 0) * 60,
		ANSI:        graphics >= 1,
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
// offsets follow the Synchronet wiki PCBOARD.SYS reference
// (https://wiki.synchro.net/ref:pcboard.sys). PCBOARD.SYS carries
// no socket/stdio indicator, so I/O defaults to stdio: a *nix BBS pipes the
// caller to stdin/stdout. A Windows winsock socket door needs DOOR32.SYS, which
// carries the handle.
//
// Not yet validated against a real BBBS-generated PCBOARD.SYS (#76): the layout
// is the documented PCBoard standard, and BBBS writes the v12/v14 shape, so the
// "later versions only append" assumption above is the part still unverified.
func parsePCBoard(b []byte) (*Caller, error) {
	// The spec allows a writer to truncate the file to the 128-byte core block,
	// so require only through the last core field we read (node, offset 111) and
	// treat anything past the block as optional.
	if len(b) < 112 {
		return nil, fmt.Errorf("PCBOARD.SYS too short: %d bytes, want >=112", len(b))
	}
	name := strings.TrimSpace(string(b[84:109]))                          // User's Full Name (offset 84, 25 bytes)
	minutes := max(int(int16(binary.LittleEndian.Uint16(b[109:111]))), 0) // Minutes remaining (offset 109)
	node := int(b[111])                                                   // Node number (offset 111; ' ' when no network)
	if b[111] == ' ' {
		node = 0
	}
	ansi := b[11] == 'Y' || b[11] == 'y' // Graphics Mode (offset 11), always in the core block
	if len(b) > 128 && (b[128] == 1 || b[128] == '1') {
		ansi = true // Use ANSI (offset 128), only when the file extends past the core block
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
