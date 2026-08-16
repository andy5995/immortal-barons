package ftn

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestType2FileAttachLayout(t *testing.T) {
	origin := Address{Zone: 1, Net: 229, Node: 100, Point: 2}
	destination := Address{Zone: 2, Net: 500, Node: 10, Point: 4}
	now := time.Date(2026, time.August, 16, 12, 34, 56, 0, time.Local)
	header := type2Header("/bbs/out/fido/test.brp", origin, destination, now)

	if got := cString(header[0:36]); got != programName {
		t.Errorf("from = %q", got)
	}
	if got := cString(header[36:72]); got != programName {
		t.Errorf("to = %q", got)
	}
	if got := cString(header[72:144]); got != "/bbs/out/fido/test.brp" {
		t.Errorf("subject = %q", got)
	}
	if got := cString(header[144:164]); got != "16 Aug 26  12:34:56" {
		t.Errorf("date = %q", got)
	}
	words := header[164:]
	word := func(n int) uint16 { return binary.LittleEndian.Uint16(words[n*2:]) }
	if word(1) != 10 || word(2) != 100 || word(4) != 229 || word(5) != 500 {
		t.Errorf("node/net fields are wrong: %v", words)
	}
	if word(6) != 2 || word(7) != 1 || word(8) != 4 || word(9) != 2 {
		t.Errorf("zone/point fields are wrong: %v", words)
	}
	if want := uint16(attributePrivate | attributeFileAttach | attributeKillSent | attributeLocal); word(11) != want {
		t.Errorf("attributes = %#x, want %#x", word(11), want)
	}

	text := messageText(origin, destination, 0x1234abcd, false)
	for _, want := range []string{
		"\x01TOPT 4\r", "\x01FMPT 2\r",
		"\x01INTL 2:500/10 1:229/100\r",
		"\x01MSGID: 1:229/100.2 1234abcd\r",
		"\x01PID: Immortal Barons\r", "\x01FLAGS KFS",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("message text lacks %q: %q", want, text)
		}
	}
	binkley := messageText(origin, destination, 0x1234abcd, true)
	if strings.Contains(binkley, "FLAGS KFS") {
		t.Errorf("Binkley message contains FLAGS KFS: %q", binkley)
	}
}

func TestCreateFileAttachClaimsMessageNumber(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "1.msg"), []byte("occupied"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := createFileAttach(dir, "/bbs/out/fido/test.brp",
		Address{Zone: 1, Net: 229, Node: 100}, Address{Zone: 1, Net: 229, Node: 200}, false)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "2.msg" {
		t.Fatalf("message = %s, want 2.msg", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) <= type2HeaderSize || data[len(data)-1] != 0 {
		t.Fatalf("message size/terminator is wrong: %d bytes", len(data))
	}
	if !bytes.Contains(data[type2HeaderSize:], []byte("\x01FLAGS KFS")) {
		t.Fatal("message has no KFS flag")
	}

	binkleyPath, err := createFileAttach(dir, "/bbs/out/fido/next.brp",
		Address{Zone: 1, Net: 229, Node: 100}, Address{Zone: 1, Net: 229, Node: 200}, true)
	if err != nil {
		t.Fatal(err)
	}
	binkley, err := os.ReadFile(binkleyPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := cString(binkley[72:144]); got != "^/bbs/out/fido/next.brp" {
		t.Errorf("Binkley subject = %q", got)
	}
	if bytes.Contains(binkley[type2HeaderSize:], []byte("FLAGS KFS")) {
		t.Fatal("Binkley message has a KFS control line")
	}
}

func cString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}
