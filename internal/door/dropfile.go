// Package door reads BBS dropfiles so Immortal Barons can run as a native
// door. A dropfile is written by the BBS when a caller launches the door;
// it tells the door who the caller is and how to talk to them.
//
// DOOR32.SYS is the primary format (both Synchronet and Mystic write it,
// and it cleanly encodes the I/O method). DOOR.SYS is supported as a
// widely-used fallback. Field positions were cross-checked against the
// Synchronet source (src/xpdoor/dropfiles.c) and BRE's own INSTALL.CFG.
package door

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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

// ParseDropfile reads the dropfile at path, dispatching on its filename.
func ParseDropfile(path string) (*Caller, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(filepath.Base(path)) {
	case "door32.sys":
		return parseDoor32(lines)
	case "door.sys":
		return parseDoorSys(lines)
	default:
		return nil, fmt.Errorf("unsupported dropfile %q (want DOOR32.SYS or DOOR.SYS)", filepath.Base(path))
	}
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
