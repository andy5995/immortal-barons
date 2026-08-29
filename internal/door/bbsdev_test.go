package door

import (
	"strings"
	"testing"
	"time"
)

// bbsDevAt fixes the clock so the deadline in the sample files below resolves to
// a known number of seconds instead of shrinking as the test suite ages.
func bbsDevAt(t *testing.T, when string) {
	t.Helper()
	at, err := time.Parse(bbsDevTimeForm, when)
	if err != nil {
		t.Fatal(err)
	}
	nowFunc = func() time.Time { return at }
	t.Cleanup(func() { nowFunc = time.Now })
}

// bbsDevSample is the spec's canonical socket example, CRLF and all. Callers
// edit one line to exercise a rule; sharing the base keeps a test from passing
// because some unrelated field was also wrong.
func bbsDevSample(edits map[int]string) string {
	lines := []string{
		"1.0", "socket", "5", "Example User",
		"550e8400-e29b-41d4-a716-446655440000",
		"80", "25", "Y", "N", "1.332",
		"2026-08-29T20:00:00Z", "IBM437", "en-US",
		"Synchronet 3.21", "Example Board", "Example Sysop",
		"90", "1", "Y",
	}
	for n, v := range edits {
		lines[n-1] = v
	}
	return strings.Join(lines, "\r\n") + "\r\n"
}

func TestParseBBSDev(t *testing.T) {
	bbsDevAt(t, "2026-08-29T19:30:00Z")
	p := write(t, "BBSDEV.DRP", bbsDevSample(nil))
	c, err := ParseDropfile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Handle != "Example User" || c.RealName != "Example User" {
		t.Errorf("handle/real name: got %q/%q", c.Handle, c.RealName)
	}
	if c.IO != IOSocket || c.Socket != 5 {
		t.Errorf("io: want socket 5, got %v %d", c.IO, c.Socket)
	}
	if c.Rows != 25 {
		t.Errorf("rows: want 25, got %d", c.Rows)
	}
	if !c.ANSI {
		t.Error("ANSI should be true")
	}
	if c.Node != 1 {
		t.Errorf("node: want 1, got %d", c.Node)
	}
	if c.BBSID != "Synchronet 3.21" {
		t.Errorf("BBS id: got %q", c.BBSID)
	}
	// The deadline is absolute, so the door's own seconds-left is a subtraction
	// against the clock, not a field.
	if c.SecondsLeft != 1800 {
		t.Errorf("seconds left: want 1800, got %d", c.SecondsLeft)
	}
}

func TestParseBBSDevCommunicationsTypes(t *testing.T) {
	bbsDevAt(t, "2026-08-29T19:30:00Z")
	for _, tc := range []struct {
		kind, param string
		want        IOMode
	}{
		{"local", "", IOLocal},
		{"stdio", "", IOStdio},
		{"socket", "0", IOSocket}, // a real descriptor, not an unset field
	} {
		p := write(t, "BBSDEV.DRP", bbsDevSample(map[int]string{2: tc.kind, 3: tc.param}))
		c, err := ParseDropfileAs(p, "bbsdev")
		if err != nil {
			t.Fatalf("%s: %v", tc.kind, err)
		}
		if c.IO != tc.want {
			t.Errorf("%s: io = %v, want %v", tc.kind, c.IO, tc.want)
		}
	}
}

// The hardware modes are refused by name. The spec forbids falling back to
// another mechanism, and this door has no serial or FOSSIL path to fall back to.
func TestParseBBSDevRefusesHardwareModes(t *testing.T) {
	bbsDevAt(t, "2026-08-29T19:30:00Z")
	for _, kind := range []string{"serial", "winserial", "uart", "fossil"} {
		p := write(t, "BBSDEV.DRP", bbsDevSample(map[int]string{2: kind, 3: "6"}))
		_, err := ParseDropfileAs(p, "bbsdev")
		if err == nil {
			t.Fatalf("%s: want an error", kind)
		}
		if !strings.Contains(err.Error(), kind) {
			t.Errorf("%s: error should name the mode, got %v", kind, err)
		}
	}
}

// A newer MINOR may only append lines, so the 19 defined ones still parse and
// the extra is ignored. A newer MAJOR is refused.
func TestParseBBSDevVersionRules(t *testing.T) {
	bbsDevAt(t, "2026-08-29T19:30:00Z")
	p := write(t, "BBSDEV.DRP", bbsDevSample(map[int]string{1: "1.7"})+"a future field\r\n")
	c, err := ParseDropfileAs(p, "bbsdev")
	if err != nil {
		t.Fatalf("1.7 with an appended line should parse: %v", err)
	}
	if c.Handle != "Example User" {
		t.Errorf("handle: got %q", c.Handle)
	}
	p = write(t, "BBSDEV.DRP", bbsDevSample(map[int]string{1: "2.0"}))
	if _, err := ParseDropfileAs(p, "bbsdev"); err == nil {
		t.Error("major version 2 should be refused")
	}
}

// An empty deadline is the format's "no forced logoff", which the game already
// spells as zero. A deadline in the past must NOT become that same zero.
func TestParseBBSDevLogoffDeadline(t *testing.T) {
	bbsDevAt(t, "2026-08-29T19:30:00Z")
	p := write(t, "BBSDEV.DRP", bbsDevSample(map[int]string{11: ""}))
	c, err := ParseDropfileAs(p, "bbsdev")
	if err != nil {
		t.Fatal(err)
	}
	if c.SecondsLeft != 0 {
		t.Errorf("no deadline: want 0 seconds left, got %d", c.SecondsLeft)
	}
	p = write(t, "BBSDEV.DRP", bbsDevSample(map[int]string{11: "2026-08-29T19:00:00Z"}))
	if _, err := ParseDropfileAs(p, "bbsdev"); err == nil {
		t.Error("a deadline already passed should be refused, not read as unlimited")
	}
}

func TestParseBBSDevRejectsMalformed(t *testing.T) {
	bbsDevAt(t, "2026-08-29T19:30:00Z")
	for _, tc := range []struct {
		name, body string
	}{
		{"byte-order mark", "\xef\xbb\xbf" + bbsDevSample(nil)},
		{"invalid UTF-8", strings.Replace(bbsDevSample(nil), "Example User", "Example \xff User", 1)},
		{"bare CR", strings.ReplaceAll(bbsDevSample(nil), "\r\n", "\r")},
		{"too short", strings.Join(strings.Split(bbsDevSample(nil), "\r\n")[:12], "\r\n") + "\r\n"},
		{"empty alias", bbsDevSample(map[int]string{4: ""})},
		{"empty user key", bbsDevSample(map[int]string{5: ""})},
		{"zero width", bbsDevSample(map[int]string{6: "0"})},
		{"height out of range", bbsDevSample(map[int]string{7: "65536"})},
		{"lowercase yes-no", bbsDevSample(map[int]string{8: "y"})},
		{"malformed deadline", bbsDevSample(map[int]string{11: "2026-08-29 20:00:00"})},
		{"deadline with an offset", bbsDevSample(map[int]string{11: "2026-08-29T20:00:00+01:00"})},
		{"empty encoding", bbsDevSample(map[int]string{12: ""})},
		{"empty board name", bbsDevSample(map[int]string{15: ""})},
		{"bad access level", bbsDevSample(map[int]string{17: "operator"})},
		{"negative node", bbsDevSample(map[int]string{18: "-1"})},
		{"unknown comms type", bbsDevSample(map[int]string{2: "websocket", 3: ""})},
		{"parameter on a mode that takes none", bbsDevSample(map[int]string{2: "stdio", 3: "5"})},
	} {
		p := write(t, "BBSDEV.DRP", tc.body)
		if _, err := ParseDropfileAs(p, "bbsdev"); err == nil {
			t.Errorf("%s: want an error", tc.name)
		}
	}
}

// LF-only files are what a Unix tool leaves behind, and the spec requires a
// consumer to take them.
func TestParseBBSDevAcceptsLF(t *testing.T) {
	bbsDevAt(t, "2026-08-29T19:30:00Z")
	p := write(t, "BBSDEV.DRP", strings.ReplaceAll(bbsDevSample(nil), "\r\n", "\n"))
	if _, err := ParseDropfileAs(p, "bbsdev"); err != nil {
		t.Fatal(err)
	}
}

// The sysop's chosen format is read whatever the file is called, and the
// filename dispatch still finds it by its canonical name.
func TestBBSDevIsRegistered(t *testing.T) {
	f, ok := FormatByID("bbsdev")
	if !ok {
		t.Fatal("bbsdev is not in Formats")
	}
	if f.Env != "BBSDEV_DRP" {
		t.Errorf("Env: want BBSDEV_DRP, got %q", f.Env)
	}
	if !f.match("bbsdev.drp") {
		t.Error("bbsdev.drp should match by name")
	}
}
