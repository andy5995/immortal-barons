package door

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// BBSDEV.DRP is a modern drop file that pins down what the older formats leave
// to convention: UTF-8, a named communications mechanism, an absolute UTC
// logoff deadline, an IANA charset, and a BCP 47 language. The spec, its ABNF
// and its examples are at https://github.com/RealDeuce/bbsdev.drp — this parser
// follows version 1 of that document.
//
// Version 1.0 is exactly 19 lines. A later minor version may only append lines
// and relax validation, so a newer minor is parsed for the 19 lines below and
// its extra lines are ignored; a newer MAJOR is refused, as the spec requires.
const (
	bbsDevLines    = 19
	bbsDevMajor    = 1
	bbsDevTimeForm = "2006-01-02T15:04:05Z"
)

// nowFunc is time.Now, replaced in tests. The logoff deadline on line 11 is
// absolute, so turning it into the seconds-left the rest of the door works in
// needs a clock.
var nowFunc = time.Now

// parseBBSDev reads a BBSDEV.DRP. It takes the raw bytes because three of the
// spec's rejection rules — a byte-order mark, invalid UTF-8, and bare CR line
// endings — are invisible once a line scanner has been over the file.
func parseBBSDev(b []byte) (*Caller, error) {
	lines, err := bbsDevLinesOf(b)
	if err != nil {
		return nil, err
	}
	if err := bbsDevVersion(field(lines, 1)); err != nil {
		return nil, err
	}
	if len(lines) < bbsDevLines {
		return nil, fmt.Errorf("BBSDEV.DRP too short: %d lines, want %d", len(lines), bbsDevLines)
	}

	c := &Caller{Rows: 25}
	if err := bbsDevComms(c, field(lines, 2), field(lines, 3)); err != nil {
		return nil, err
	}

	c.Handle = field(lines, 4)
	c.RealName = c.Handle // the format carries an alias only, by design
	if c.Handle == "" {
		return nil, fmt.Errorf("line 4 (user alias): empty")
	}
	if field(lines, 5) == "" {
		return nil, fmt.Errorf("line 5 (user key): empty")
	}

	if _, err := bbsDevScreen(field(lines, 6), "width"); err != nil {
		return nil, err
	}
	rows, err := bbsDevScreen(field(lines, 7), "height")
	if err != nil {
		return nil, err
	}
	c.Rows = rows

	if c.ANSI, err = bbsDevYesNo(field(lines, 8), 8, "ANSI"); err != nil {
		return nil, err
	}
	if _, err = bbsDevYesNo(field(lines, 9), 9, "RIP"); err != nil {
		return nil, err
	}

	if c.SecondsLeft, err = bbsDevLogoff(field(lines, 11)); err != nil {
		return nil, err
	}

	for _, f := range []struct {
		n    int
		what string
	}{
		{12, "encoding"}, {13, "language"},
		{14, "BBS software"}, {15, "board name"}, {16, "sysop alias"},
	} {
		if field(lines, f.n) == "" {
			return nil, fmt.Errorf("line %d (%s): empty", f.n, f.what)
		}
	}
	c.BBSID = field(lines, 14)

	if err := bbsDevAccess(field(lines, 17)); err != nil {
		return nil, err
	}
	if c.Node, err = bbsDevUint(field(lines, 18), 18, "node number"); err != nil {
		return nil, err
	}
	if _, err = bbsDevYesNo(field(lines, 19), 19, "show local display"); err != nil {
		return nil, err
	}
	return c, nil
}

// bbsDevLinesOf splits the file the way the spec defines it: UTF-8 with no
// byte-order mark, lines ended by CRLF (LF also accepted), and no bare CR.
func bbsDevLinesOf(b []byte) ([]string, error) {
	if bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		return nil, fmt.Errorf("BBSDEV.DRP starts with a byte-order mark")
	}
	if !utf8.Valid(b) {
		return nil, fmt.Errorf("BBSDEV.DRP is not valid UTF-8")
	}
	s := strings.TrimSuffix(string(b), "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		l = strings.TrimSuffix(l, "\r")
		if strings.ContainsRune(l, '\r') {
			return nil, fmt.Errorf("line %d: bare carriage return", i+1)
		}
		lines[i] = strings.TrimSpace(l)
	}
	return lines, nil
}

// bbsDevVersion accepts any minor of major 1 and refuses any other major, which
// is what makes an unread appended line safe to ignore.
func bbsDevVersion(s string) error {
	major, minor, ok := strings.Cut(s, ".")
	if !ok {
		return fmt.Errorf("line 1 (format version): %q is not major.minor", s)
	}
	if _, err := strconv.ParseUint(minor, 10, 64); err != nil || (len(minor) > 1 && minor[0] == '0') {
		return fmt.Errorf("line 1 (format version): %q is not major.minor", s)
	}
	if major != strconv.Itoa(bbsDevMajor) {
		return fmt.Errorf("line 1 (format version): major version %s is not supported (this door reads %d.x)", major, bbsDevMajor)
	}
	return nil
}

// bbsDevComms maps the communications registry onto the door's I/O modes. The
// four hardware modes are refused here rather than carried as IOSerial: the
// spec requires a consumer to reject a mechanism it does not support instead of
// falling back to another, and this door has no serial or FOSSIL path at all.
func bbsDevComms(c *Caller, kind, param string) error {
	switch kind {
	case "local":
		c.IO = IOLocal
	case "stdio":
		c.IO = IOStdio
	case "socket":
		sock, err := bbsDevUint(param, 3, "socket value")
		if err != nil {
			return err
		}
		// Zero is a real descriptor here, unlike in DOOR32.SYS where an empty
		// numeric field also reads as 0: this format has no empty fields to
		// confuse it with, so a board that says socket 0 means it.
		c.IO, c.Socket = IOSocket, sock
		return nil
	case "serial", "winserial", "uart", "fossil":
		return fmt.Errorf("line 2 (communications type): %q is not supported; configure your BBS for a local, stdio, or socket door", kind)
	default:
		return fmt.Errorf("line 2 (communications type): %q is not a registered type", kind)
	}
	if param != "" {
		return fmt.Errorf("line 3 (communications parameters): must be empty for %q, got %q", kind, param)
	}
	return nil
}

// bbsDevScreen reads a screen dimension, which the spec bounds at 1 through
// 65535.
func bbsDevScreen(s, what string) (int, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil || v < 1 || v > 65535 {
		return 0, fmt.Errorf("screen %s: %q is not 1 through 65535", what, s)
	}
	return int(v), nil
}

func bbsDevYesNo(s string, n int, what string) (bool, error) {
	switch s {
	case "Y":
		return true, nil
	case "N":
		return false, nil
	}
	return false, fmt.Errorf("line %d (%s): %q is not Y or N", n, what, s)
}

// bbsDevUint reads one of the format's unsigned integers. The spec's range runs
// to 2^64-1, which no field the game keeps can hold, so a value past the
// platform's int is an error rather than a silent wrap.
func bbsDevUint(s string, n int, what string) (int, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("line %d (%s): %q is not an unsigned number", n, what, s)
	}
	if v > math.MaxInt {
		return 0, fmt.Errorf("line %d (%s): %d is too large for this platform", n, what, v)
	}
	return int(v), nil
}

// bbsDevAccess validates the access level. The game has no security model, so
// the value is checked and discarded — a file that fails here is the wrong
// format or a broken writer, which is worth saying before a caller is dropped
// for a reason nobody can see.
func bbsDevAccess(s string) error {
	if s == "sysop" || s == "cosysop" {
		return nil
	}
	_, err := bbsDevUint(s, 17, "access level")
	return err
}

// bbsDevLogoff turns the absolute UTC deadline on line 11 into the seconds-left
// the rest of the door works in. An empty line is no forced logoff, which the
// game already spells as zero.
//
// A deadline that has already passed is an error: zero would read as "no limit"
// and hand the caller an unbounded session, which is the opposite of what the
// board asked for.
func bbsDevLogoff(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	t, err := time.Parse(bbsDevTimeForm, s)
	if err != nil {
		return 0, fmt.Errorf("line 11 (logoff time): %q is not YYYY-MM-DDTHH:MM:SSZ", s)
	}
	left := t.Sub(nowFunc())
	if left <= 0 {
		return 0, fmt.Errorf("line 11 (logoff time): the deadline %s has already passed", s)
	}
	return int(left / time.Second), nil
}
