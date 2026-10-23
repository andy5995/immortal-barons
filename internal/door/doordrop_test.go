package door

import (
	"strings"
	"testing"
)

// doorDropSample is the spec's socket example cut to the keys a door reads plus
// a few it must ignore, CRLF and all. Callers replace or delete one key to
// exercise a rule; sharing the base keeps a test from passing because some
// unrelated key was also wrong.
func doorDropSample(edits map[string]string) string {
	lines := []string{
		"; Written by Synchronet 3.22a",
		"[file]", "FILE_UTF8=1", "FILE_TIME=2026-09-23T15:15:00-07:00",
		"[system]", "SYS_SOFTWARE=Synchronet 3.22a", "SYS_NAME=Example host",
		"SYS_OP=Example Sysop", "SYS_NODE_NUM=3",
		"[comm]", "COMM_TYPE=socket", "COMM_HANDLE=1234", "COMM_CHARSET=UTF-8",
		"[user]", "USER_ALIAS=Jörg the Red", "USER_NUMBER=42",
		"USER_KEY=42-1693526400", "USER_LANG=de",
		"[terminal]", "TERM_COLS=132", "TERM_ROWS=37", "TERM_TYPE=ansi",
		"[session]", "TIME_LEFT=2700",
		"[x-sbbs]", "X_SBBS_LEVEL=50",
	}
	var out []string
	for _, l := range lines {
		k, _, _ := strings.Cut(l, "=")
		if v, ok := edits[k]; ok {
			if v == "" {
				continue // delete the key
			}
			l = k + "=" + v
		}
		out = append(out, l)
	}
	return strings.Join(out, "\r\n") + "\r\n"
}

func TestParseDoorDrop(t *testing.T) {
	p := write(t, "DOORDROP.INI", doorDropSample(nil))
	c, err := ParseDropfile(p)
	if err != nil {
		t.Fatal(err)
	}
	want := Caller{
		Handle: "Jörg the Red", RealName: "Jörg the Red", Node: 3,
		SecondsLeft: 2700, ANSI: true, Rows: 37, IO: IOSocket, Socket: 1234,
		BBSID: "Synchronet 3.22a", Charset: "UTF-8", Language: "de",
	}
	if *c != want {
		t.Errorf("got  %+v\nwant %+v", *c, want)
	}
}

// Text is CP437 unless FILE_UTF8 says otherwise, so the same alias written by a
// CP437 host must come out as the same realm name.
func TestDoorDropCP437Text(t *testing.T) {
	body := strings.Replace(doorDropSample(map[string]string{"FILE_UTF8": ""}),
		"Jörg", "J\x94rg", 1) // 0x94 is ö in CP437
	c, err := ParseDropfileAs(write(t, "node3.ini", body), "doordrop")
	if err != nil {
		t.Fatal(err)
	}
	if c.Handle != "Jörg the Red" {
		t.Errorf("Handle = %q, want Jörg the Red", c.Handle)
	}
}

func TestDoorDropRejectsBadUTF8(t *testing.T) {
	body := strings.Replace(doorDropSample(nil), "Jörg", "J\x94rg", 1)
	if _, err := ParseDropfileAs(write(t, "DOORDROP.INI", body), "doordrop"); err == nil {
		t.Fatal("CP437 bytes under FILE_UTF8=1 should fail")
	}
}

// LF alone, a Ctrl-Z end, '#' comments, spaces around '=', a repeated key and
// a value holding '=' and ';' are all things the spec tells a reader to cope
// with.
func TestDoorDropReaderTolerance(t *testing.T) {
	body := strings.Join([]string{
		"# a hash comment", "[comm]", "COMM_TYPE = stdio", "COMM_CHARSET=CP437",
		"[system]", "SYS_NODE_NUM=1", "SYS_NODE_NUM=9",
		"[user]", "USER_ALIAS=Joe;Bob=1", "USER_REALNAME=Joe Bob",
		"\x1aUSER_LANG=sv",
	}, "\n")
	c, err := ParseDropfileAs(write(t, "DOORDROP.INI", body), "doordrop")
	if err != nil {
		t.Fatal(err)
	}
	if c.IO != IOStdio || c.Node != 1 || c.Handle != "Joe;Bob=1" || c.RealName != "Joe Bob" {
		t.Errorf("got %+v", *c)
	}
	if c.Language != "" {
		t.Errorf("Language = %q: a key after Ctrl-Z was read", c.Language)
	}
	if c.ANSI || c.Rows != 24 || c.SecondsLeft != 0 {
		t.Errorf("defaults: ANSI=%v Rows=%d SecondsLeft=%d, want false 24 0", c.ANSI, c.Rows, c.SecondsLeft)
	}
}

func TestDoorDropCommTypes(t *testing.T) {
	for _, tc := range []struct {
		edits map[string]string
		ok    bool
		io    IOMode
	}{
		{map[string]string{"COMM_TYPE": "stdio", "COMM_HANDLE": ""}, true, IOStdio},
		{map[string]string{"COMM_TYPE": "local", "COMM_CHARSET": ""}, true, IOLocal},
		{map[string]string{"COMM_HANDLE": "0"}, true, IOSocket},
		{map[string]string{"COMM_HANDLE": ""}, false, 0},
		{map[string]string{"COMM_HANDLE": "-1"}, false, 0},
		{map[string]string{"COMM_TYPE": "telnet"}, false, 0},
		{map[string]string{"COMM_TYPE": "fossil"}, false, 0},
		{map[string]string{"COMM_TYPE": "carrier-pigeon"}, false, 0},
		{map[string]string{"COMM_TYPE": ""}, false, 0},
		{map[string]string{"COMM_CHARSET": ""}, false, 0},
	} {
		c, err := ParseDropfileAs(write(t, "DOORDROP.INI", doorDropSample(tc.edits)), "doordrop")
		if (err == nil) != tc.ok {
			t.Errorf("%v: err = %v, want ok=%v", tc.edits, err, tc.ok)
			continue
		}
		if tc.ok && c.IO != tc.io {
			t.Errorf("%v: IO = %v, want %v", tc.edits, c.IO, tc.io)
		}
	}
}

func TestDoorDropRequiredAndNumbers(t *testing.T) {
	for _, edits := range []map[string]string{
		{"USER_ALIAS": ""},
		{"SYS_NODE_NUM": ""},
		{"SYS_NODE_NUM": "three"},
		{"TIME_LEFT": "-5"},
		{"FILE_UTF8": "yes"},
	} {
		if _, err := ParseDropfileAs(write(t, "DOORDROP.INI", doorDropSample(edits)), "doordrop"); err == nil {
			t.Errorf("%v should fail", edits)
		}
	}
}

// A bool is any nonzero number, never a comparison against 1, and TERM_TYPE
// tokens map onto the one ANSI flag the game keeps.
func TestDoorDropBoolAndTermType(t *testing.T) {
	c, err := ParseDropfileAs(write(t, "DOORDROP.INI",
		doorDropSample(map[string]string{"FILE_UTF8": "2", "TERM_TYPE": "avatar"})), "doordrop")
	if err != nil {
		t.Fatal(err)
	}
	if c.Handle != "Jörg the Red" || !c.ANSI {
		t.Errorf("FILE_UTF8=2 or avatar misread: %+v", *c)
	}
	for _, tt := range []string{"dumb", "petscii", "vt52"} {
		c, err := ParseDropfileAs(write(t, "DOORDROP.INI",
			doorDropSample(map[string]string{"TERM_TYPE": tt})), "doordrop")
		if err != nil {
			t.Fatal(err)
		}
		if c.ANSI {
			t.Errorf("TERM_TYPE=%s read as ANSI", tt)
		}
	}
}
