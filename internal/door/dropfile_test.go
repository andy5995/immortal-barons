package door

import (
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

func TestUnsupportedDropfile(t *testing.T) {
	p := write(t, "dorinfo1.def", "whatever\n")
	if _, err := ParseDropfile(p); err == nil {
		t.Error("expected error for unsupported dropfile type")
	}
}

func TestMissingDropfile(t *testing.T) {
	if _, err := ParseDropfile("/no/such/door32.sys"); err == nil {
		t.Error("expected error for missing file")
	}
}
