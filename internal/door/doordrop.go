package door

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// DOORDROP.INI is a named-value drop file: KEY=value lines under INI section
// headers, read by key rather than by line number. The spec is Rob Swindell's
// draft in the Synchronet tree (docs/doordrop_ini.md); this parser follows
// draft 0.8 (2026-10-02). The draft says not to implement it except to
// experiment until 1.0, and keys may still be renamed, so treat this as
// experimental and re-read the spec before relying on it.
//
// The spec asks a consumer to check only the values it uses, so this reads the
// handful of keys the game needs and ignores the rest, including keys it
// requires of a producer that the game never looks at (FILE_TIME, SYS_NAME, …).

// parseDoorDrop reads a DOORDROP.INI. It takes the raw bytes because text
// values are CP437 unless the file says FILE_UTF8, and that has to be decided
// before any of them is turned into a Go string.
func parseDoorDrop(b []byte) (*Caller, error) {
	keys := doorDropKeys(b)

	utf8Text, err := doorDropBool(keys, "FILE_UTF8", false)
	if err != nil {
		return nil, err
	}
	text := func(k string) (string, error) {
		v := keys[k]
		if !utf8Text {
			return charmap.CodePage437.NewDecoder().String(v)
		}
		if !utf8.ValidString(v) {
			return "", fmt.Errorf("%s: not valid UTF-8, though FILE_UTF8 says it is", k)
		}
		return v, nil
	}

	c := &Caller{Rows: 24}
	if err := doorDropComm(c, keys); err != nil {
		return nil, err
	}
	local := c.IO == IOLocal

	if c.Node, err = doorDropInt(keys, "SYS_NODE_NUM", -1); err != nil {
		return nil, err
	}
	if c.Node < 0 {
		return nil, fmt.Errorf("SYS_NODE_NUM: missing")
	}

	if c.Handle, err = text("USER_ALIAS"); err != nil {
		return nil, err
	}
	if c.Handle == "" && !local {
		return nil, fmt.Errorf("USER_ALIAS: missing")
	}
	if c.RealName, err = text("USER_REALNAME"); err != nil {
		return nil, err
	}
	if c.RealName == "" {
		c.RealName = c.Handle
	}
	if c.BBSID, err = text("SYS_SOFTWARE"); err != nil {
		return nil, err
	}

	if keys["COMM_CHARSET"] == "" && !local {
		return nil, fmt.Errorf("COMM_CHARSET: missing")
	}
	c.Charset = keys["COMM_CHARSET"]
	c.Language = keys["USER_LANG"]

	if c.SecondsLeft, err = doorDropInt(keys, "TIME_LEFT", 0); err != nil {
		return nil, err
	}
	if c.Rows, err = doorDropInt(keys, "TERM_ROWS", 24); err != nil {
		return nil, err
	}
	if c.Rows == 0 {
		c.Rows = 24
	}
	// An unknown TERM_TYPE reads as dumb, per the spec's rule for tokens. AVATAR
	// includes ANSI, and a door without AVATAR output treats it as ANSI.
	switch keys["TERM_TYPE"] {
	case "ansi", "avatar":
		c.ANSI = true
	}
	return c, nil
}

// doorDropKeys splits the file into its KEY=value pairs. Section headers and
// comments contain no key the spec defines, so they are skipped along with any
// other line that has no '='. A Ctrl-Z ends the file, as a DOS text read does,
// and the first occurrence of a repeated key wins.
func doorDropKeys(b []byte) map[string]string {
	if i := bytes.IndexByte(b, 0x1A); i >= 0 {
		b = b[:i]
	}
	keys := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if _, seen := keys[k]; !seen {
			keys[k] = strings.TrimSpace(v)
		}
	}
	return keys
}

// doorDropComm maps COMM_TYPE onto the door's I/O modes. The spec forbids
// falling back to another type, so everything the door cannot drive is refused
// by name: telnet needs the door to speak the Telnet protocol itself, which the
// socket backend does not, and the serial family has no path here at all.
func doorDropComm(c *Caller, keys map[string]string) error {
	switch t := keys["COMM_TYPE"]; t {
	case "local":
		c.IO = IOLocal
	case "stdio":
		c.IO = IOStdio
	case "socket":
		// The spec requires a real, open handle for a socket and rules out a
		// placeholder such as -1, so unlike DOOR32.SYS there is no redirect to
		// read into a missing one.
		if keys["COMM_HANDLE"] == "" {
			return fmt.Errorf("COMM_HANDLE: missing for COMM_TYPE=socket")
		}
		sock, err := doorDropInt(keys, "COMM_HANDLE", 0)
		if err != nil {
			return err
		}
		c.IO, c.Socket = IOSocket, sock
	case "telnet", "serial", "fossil", "uart":
		return fmt.Errorf("COMM_TYPE: %q is not supported; configure your BBS for a local, stdio, or socket door", t)
	case "":
		return fmt.Errorf("COMM_TYPE: missing")
	default:
		return fmt.Errorf("COMM_TYPE: %q is not a type the spec defines", t)
	}
	return nil
}

// doorDropInt reads an unsigned decimal key, returning def when it is absent.
// The spec's int stops at 2^31-1; the parse allows the wider handle type too,
// bounded by what the platform's int can hold.
func doorDropInt(keys map[string]string, k string, def int) (int, error) {
	s, ok := keys[k]
	if !ok || s == "" {
		return def, nil
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not an unsigned number", k, s)
	}
	if v > math.MaxInt {
		return 0, fmt.Errorf("%s: %d is too large for this platform", k, v)
	}
	return int(v), nil
}

// doorDropBool reads a bool key: 0 is false and any other number is true. The
// spec forbids comparing against 1, since a key may define larger values that
// refine true.
func doorDropBool(keys map[string]string, k string, def bool) (bool, error) {
	if s, ok := keys[k]; !ok || s == "" {
		return def, nil
	}
	v, err := doorDropInt(keys, k, 0)
	return v != 0, err
}
