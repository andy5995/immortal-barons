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
	path, err := createFileAttach(Config{NetmailDir: dir}, "/bbs/out/fido/test.brp",
		Address{Zone: 1, Net: 229, Node: 100}, Address{Zone: 1, Net: 229, Node: 200})
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

	binkleyPath, err := createFileAttach(Config{NetmailDir: dir, Binkley: true}, "/bbs/out/fido/next.brp",
		Address{Zone: 1, Net: 229, Node: 100}, Address{Zone: 1, Net: 229, Node: 200})
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

func TestFileAttachSubjectLimits(t *testing.T) {
	plain, binkley := Config{}, Config{Binkley: true}
	if _, spare, err := fileAttachSubject(plain, strings.Repeat("x", 71)); err != nil {
		t.Fatalf("71-byte non-Binkley subject: %v", err)
	} else if spare != 0 {
		t.Errorf("spare = %d, want 0", spare)
	}
	if _, _, err := fileAttachSubject(plain, strings.Repeat("x", 72)); err == nil {
		t.Fatal("accepted 72-byte non-Binkley attachment path")
	}
	if got, spare, err := fileAttachSubject(binkley, strings.Repeat("x", 70)); err != nil {
		t.Fatalf("70-byte Binkley subject: %v", err)
	} else if len(got) != 71 || got[0] != '^' || spare != 0 {
		t.Fatalf("Binkley subject = %q, spare = %d", got, spare)
	}
	if _, _, err := fileAttachSubject(binkley, strings.Repeat("x", 71)); err == nil {
		t.Fatal("accepted 71-byte Binkley attachment path")
	}
}

func TestSubjectSpellingModes(t *testing.T) {
	const attached = "/sbbs/xtrn/ib/data/outbound/fido/L100-2-000000000064z-3.brp"
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"default is the unchanged absolute path", Config{}, attached},
		{"explicit absolute", Config{SubjectMode: SubjectAbsolute}, attached},
		{"basename", Config{SubjectMode: SubjectBasename}, "L100-2-000000000064z-3.brp"},
		{"prefix gains a separator", Config{SubjectMode: SubjectPrefixed, SubjectPrefix: "ibout"},
			"ibout/L100-2-000000000064z-3.brp"},
		{"prefix keeps its own separator", Config{SubjectMode: SubjectPrefixed, SubjectPrefix: "ibout/"},
			"ibout/L100-2-000000000064z-3.brp"},
		{"backslash prefix stays a mailer path", Config{SubjectMode: SubjectPrefixed, SubjectPrefix: `C:\sbbs\ibout`},
			`C:\sbbs\ibout\L100-2-000000000064z-3.brp`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _, err := fileAttachSubject(c.cfg, attached)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("subject = %q, want %q", got, c.want)
			}
		})
	}
}

// The absolute path below is 70 bytes at sequence 99 and 71 at 100, which is
// one over the Binkley limit: the failure that took a live board down.
// Basename-only spends nothing on directories, so the extra digit stops
// mattering.
func TestSequenceDigitBreaksAbsoluteButNotBasename(t *testing.T) {
	const dir = "/sbbs/xtrn/immortal-barons/data/fido/"
	at99 := dir + strings.Repeat("p", 70-len(dir))
	at100 := at99 + "0"

	binkley := Config{Binkley: true}
	if _, _, err := fileAttachSubject(binkley, at99); err != nil {
		t.Fatalf("sequence 99 already failed: %v", err)
	}
	_, _, err := fileAttachSubject(binkley, at100)
	if err == nil {
		t.Fatal("the extra sequence digit was accepted; this test no longer covers the incident")
	}
	if !strings.Contains(err.Error(), "SubjectPath") || !strings.Contains(err.Error(), "AttachDir") {
		t.Errorf("error does not name the settings that fix it: %v", err)
	}

	short := Config{Binkley: true, SubjectMode: SubjectBasename}
	for _, path := range []string{at99, at100} {
		if _, spare, err := fileAttachSubject(short, path); err != nil {
			t.Errorf("basename subject for %s: %v", path, err)
		} else if spare < subjectMarginBytes {
			t.Errorf("basename subject for %s left only %d bytes", path, spare)
		}
	}
}

// #232 review: an earlier version of subjectAdvice told a SubjectPrefixed
// sysop to shorten AttachDir, which does nothing -- the subject in this
// mode is SubjectPrefix plus the filename, and AttachDir's own value never
// appears in it. Proves that directly: two attached paths differing only
// in their directory (standing in for two different AttachDir values)
// produce the identical subject once SubjectPrefixed spells it, and the
// advice for this mode names SubjectPath, not AttachDir.
func TestSubjectPrefixedIgnoresAttachDirsOwnLength(t *testing.T) {
	cfg := Config{Binkley: true, SubjectMode: SubjectPrefixed, SubjectPrefix: "fido"}
	short, _, err := fileAttachSubject(cfg, "/short/p.brp")
	if err != nil {
		t.Fatal(err)
	}
	long, _, err := fileAttachSubject(cfg, "/a/very/considerably/longer/attach/directory/p.brp")
	if err != nil {
		t.Fatal(err)
	}
	if short != long {
		t.Fatalf("subject changed with the directory portion of the attached path: %q vs %q -- "+
			"SubjectPrefixed is supposed to depend only on SubjectPrefix and the filename", short, long)
	}

	// The advice may still mention AttachDir to explain why it does not
	// help here (accurate, and worth saying) -- what it must not do is
	// recommend changing it as the fix, the mistake this test exists to
	// catch.
	advice := subjectAdvice(SubjectPrefixed)
	if !strings.Contains(advice, "SubjectPath") {
		t.Errorf("SubjectPrefixed advice does not mention SubjectPath: %v", advice)
	}
	if strings.Contains(advice, "Set AttachDir") {
		t.Errorf("SubjectPrefixed advice tells the sysop to set AttachDir, which does not affect this mode's subject length: %v", advice)
	}
}

func cString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}
