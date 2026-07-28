package door

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseDoor32(t *testing.T) {
	// comm=2(socket) handle=7 baud bbsid rec real=John Q handle=Khan
	// level time=30min emu=1(ansi) node=1
	p := write(t, "door32.sys",
		"2\n7\n57600\nImmortal BBS\n42\nJohn Q Sysop\nKhan\n80\n30\n1\n1\n")
	c, err := ParseDropfile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Handle != "Khan" {
		t.Errorf("handle: want Khan, got %q", c.Handle)
	}
	if c.RealName != "John Q Sysop" {
		t.Errorf("real name: got %q", c.RealName)
	}
	if c.SecondsLeft != 1800 {
		t.Errorf("seconds left: want 1800, got %d", c.SecondsLeft)
	}
	if !c.ANSI {
		t.Error("ANSI should be true (emulation 1)")
	}
	if c.Node != 1 {
		t.Errorf("node: want 1, got %d", c.Node)
	}
	if c.IO != IOSocket || c.Socket != 7 {
		t.Errorf("io: want socket/7, got %v/%d", c.IO, c.Socket)
	}
}

func TestParseDoor32FallsBackToRealName(t *testing.T) {
	p := write(t, "door32.sys",
		"0\n0\n0\nBBS\n1\nJane Doe\n\n10\n60\n0\n2\n")
	c, err := ParseDropfile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Handle != "Jane Doe" {
		t.Errorf("empty handle should fall back to real name, got %q", c.Handle)
	}
	if c.ANSI {
		t.Error("emulation 0 should be non-ANSI")
	}
	if c.IO != IOLocal {
		t.Errorf("comm type 0 should be local, got %v", c.IO)
	}
}

func TestParseDoorSysStdio(t *testing.T) {
	lines := "COM0:STDIO\n38400\n8\n3\n5\n6\n7\n8\n9\n" +
		"Sir Test\nCity\nphone\nphone2\npass\n50\n10\n01/01/99\n0\n45\nGR\n24\n"
	p := write(t, "door.sys", lines)
	c, err := ParseDropfile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.IO != IOStdio {
		t.Errorf("io: want stdio, got %v", c.IO)
	}
	if c.Handle != "Sir Test" {
		t.Errorf("handle: got %q", c.Handle)
	}
	if c.Node != 3 {
		t.Errorf("node: want 3, got %d", c.Node)
	}
	if c.SecondsLeft != 45*60 {
		t.Errorf("seconds: want 2700 (45 min), got %d", c.SecondsLeft)
	}
	if !c.ANSI {
		t.Error("GR should mean ANSI on")
	}
	if c.Rows != 24 {
		t.Errorf("rows: want 24, got %d", c.Rows)
	}
}

func TestParseDoorSysSocket(t *testing.T) {
	lines := "COM0:SOCKET123\n0\n0\n1\n5\n6\n7\n8\n9\n" +
		"Caller\nx\nx\nx\nx\n0\n0\nx\n600\n0\nNG\n25\n"
	p := write(t, "door.sys", lines)
	c, err := ParseDropfile(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.IO != IOSocket || c.Socket != 123 {
		t.Errorf("io: want socket/123, got %v/%d", c.IO, c.Socket)
	}
	if c.SecondsLeft != 600 {
		t.Errorf("seconds should come from line 18 (600), got %d", c.SecondsLeft)
	}
	if c.ANSI {
		t.Error("NG should mean ANSI off")
	}
}

func TestParsePCBoard(t *testing.T) {
	// A 200-byte space-filled PCBOARD.SYS with the core fields the game reads:
	// Full Name @84, minutes-left @109, node @111, Use-ANSI @128.
	b := make([]byte, 200)
	for i := range b {
		b[i] = ' '
	}
	copy(b[84:], []byte("Baron Sahara"))
	binary.LittleEndian.PutUint16(b[109:], uint16(int16(45))) // 45 minutes left
	b[111] = 3                                                // node 3
	b[128] = 1                                                // ANSI on

	p := filepath.Join(t.TempDir(), "PCBOARD.SYS")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := ParseDropfile(p)
	if err != nil {
		t.Fatalf("ParseDropfile: %v", err)
	}
	if c.Handle != "Baron Sahara" {
		t.Errorf("Handle = %q, want %q", c.Handle, "Baron Sahara")
	}
	if c.SecondsLeft != 45*60 {
		t.Errorf("SecondsLeft = %d, want %d", c.SecondsLeft, 45*60)
	}
	if c.Node != 3 {
		t.Errorf("Node = %d, want 3", c.Node)
	}
	if !c.ANSI {
		t.Error("ANSI should be on")
	}
	if c.IO != IOStdio {
		t.Errorf("IO = %v, want IOStdio", c.IO)
	}
}

// The spec lets a writer truncate PCBOARD.SYS to the 128-byte core block, which
// omits the Use-ANSI byte at offset 128; ANSI then comes from Graphics Mode at
// offset 11, inside the block.
func TestParsePCBoardTruncatedToCoreBlock(t *testing.T) {
	b := make([]byte, 128)
	for i := range b {
		b[i] = ' '
	}
	b[11] = 'Y' // Graphics Mode = yes
	copy(b[84:], []byte("Baron Sahara"))
	binary.LittleEndian.PutUint16(b[109:], uint16(int16(45)))
	b[111] = 3

	p := filepath.Join(t.TempDir(), "PCBOARD.SYS")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := ParseDropfile(p)
	if err != nil {
		t.Fatalf("a 128-byte PCBOARD.SYS is spec-legal: %v", err)
	}
	if c.Handle != "Baron Sahara" || c.SecondsLeft != 45*60 || c.Node != 3 {
		t.Errorf("got %+v, want Baron Sahara/2700s/node 3", c)
	}
	if !c.ANSI {
		t.Error("Graphics Mode 'Y' should mean ANSI on when offset 128 is absent")
	}
}

func TestParsePCBoardTooShort(t *testing.T) {
	p := write(t, "PCBOARD.SYS", "short")
	if _, err := ParseDropfile(p); err == nil {
		t.Error("expected error for a truncated PCBOARD.SYS")
	}
}

func TestParseDorInfo(t *testing.T) {
	// 1 BBS  2 sysop-first  3 sysop-last  4 COM0  5 baud  6 reserved
	// 7 Khan  8 Noonien  9 city  10 graphics=1  11 level  12 minutes=30
	lines := "My BBS\nThe\nSysop\nCOM0\n0 BAUD,N,8,1\n0\nKhan\nNoonien\nSeti Alpha V\n1\n50\n30\n-1\n"
	p := write(t, "dorinfo1.def", lines)
	c, err := ParseDropfile(p)
	if err != nil {
		t.Fatalf("ParseDropfile: %v", err)
	}
	if c.Handle != "Khan Noonien" {
		t.Errorf("Handle = %q, want %q", c.Handle, "Khan Noonien")
	}
	if c.SecondsLeft != 30*60 {
		t.Errorf("SecondsLeft = %d, want %d", c.SecondsLeft, 30*60)
	}
	if !c.ANSI {
		t.Error("graphics=1 should mean ANSI on")
	}
	if c.IO != IOLocal {
		t.Errorf("IO = %v, want IOLocal (COM0)", c.IO)
	}
}

// A drop file can satisfy the length checks and still carry no caller name
// (corrupt, or the wrong format for its name). Reject it rather than inventing a
// node-numbered handle, which would put every such caller in one shared empire.
func TestParseRejectsMissingCallerName(t *testing.T) {
	cases := []struct{ name, file, content string }{
		{"door32", "door32.sys", "2\n7\n38400\nBBS\n1\n\n\n10\n30\n1\n1\n"},
		{"dorinfo", "dorinfo1.def", "B\ns\ns\nCOM1\nb\n0\n\n\ncity\n1\n10\n30\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := write(t, c.file, c.content)
			if _, err := ParseDropfile(p); err == nil {
				t.Error("expected an error for a drop file with no caller name")
			}
		})
	}
}

// Text in a numeric slot means the file is corrupt or is not the format we were
// told to read. The headline case: the sysop selects DOOR32.SYS but the BBS
// writes DOOR.SYS, whose line 1 is "COM0:STDIO" where a comm type belongs.
func TestParseRejectsNonNumericField(t *testing.T) {
	doorSysContent := "COM0:STDIO\n38400\n8\n1\n38400\nY\nY\nY\nY\nKhan Noonien\nCity\n" +
		"1\n1\n50\n1\n1\n1\n1800\n30\nGR\n24\n"
	cases := []struct{ name, format, content string }{
		{"door.sys read as door32", "door32", doorSysContent},
		{"garbage comm type", "door32", "xyz\n7\n38400\nBBS\n1\nR\nKhan\n10\n30\n1\n1\n"},
		{"garbage time left", "dorinfo", "B\ns\ns\nCOM1\nb\n0\nKhan\nNoonien\ncity\n1\n10\nsoon\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := write(t, "drop.tmp", c.content)
			if _, err := ParseDropfileAs(p, c.format); err == nil {
				t.Error("expected an error for a non-numeric value in a numeric field")
			}
		})
	}
}

// Socket mode with no handle would attach to fd 0 (stdin) rather than the caller.
func TestParseDoor32RejectsSocketModeWithoutHandle(t *testing.T) {
	p := write(t, "door32.sys", "2\n0\n38400\nBBS\n1\nReal Name\nKhan\n10\n30\n1\n1\n")
	if _, err := ParseDropfile(p); err == nil {
		t.Error("expected an error for socket mode with handle 0")
	}
}

func TestUnsupportedDropfile(t *testing.T) {
	p := write(t, "chain.txt", "whatever\n")
	if _, err := ParseDropfile(p); err == nil {
		t.Error("expected error for unsupported dropfile type")
	}
}

func TestMissingDropfile(t *testing.T) {
	if _, err := ParseDropfile("/no/such/door32.sys"); err == nil {
		t.Error("expected error for missing file")
	}
}
