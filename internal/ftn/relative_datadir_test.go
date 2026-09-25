package ftn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// A board run with the default, relative data directory (./data, as a door
// started from its install directory has) must still spell the attachment's
// full path in the netmail subject. The mailer resolves that path from its own
// working directory, so a relative subject names a file that is not there, and
// the mailer sends nothing: a Windows board with Irex queued thousands of
// messages this way (subject "data\att\GNJD1QEO.BRP").
func TestAttachSubjectIsAbsoluteWithARelativeDataDir(t *testing.T) {
	data := newBundledSetup(t, "Bravo BBS", "")
	cfg := "BoardID Bravo BBS\nLeagueNumber 100\nGameInbound door-in\nGameOutbound door-out\n" +
		"OutgoingNetmailDir netmail\nIncomingFileDir transport-in\nIncomingNetmailDir transport-in\n"
	if err := os.WriteFile(filepath.Join(data, store.BoardConfigFile), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	writeNamedPacket(t, data, "a.brp", game.Packet{
		FromBoard: "Bravo BBS", ToBoard: "Alpha BBS", FromNode: 2, ToNode: 1, Seq: 1, League: 100})

	// Moved somewhere short: an absolute subject has to fit FTN's 71 bytes,
	// and Go's per-test temp paths alone come close to that.
	short, err := os.MkdirTemp("", "ib")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(short) })
	if err := os.Rename(data, filepath.Join(short, "data")); err != nil {
		t.Fatal(err)
	}
	data = filepath.Join(short, "data")
	// macOS and Windows runners keep temp files deep enough that the full
	// subject passes FTN's 71 bytes; the fault this checks is the same there.
	if len(filepath.Join(data, "att", "255U0000.BRP")) > type2SubjectSize-1 {
		t.Skipf("temp path %q leaves no room for a %d-byte subject", data, type2SubjectSize-1)
	}

	t.Chdir(filepath.Dir(data))
	result, err := RunOut(filepath.Base(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Queued) == 0 {
		t.Fatalf("nothing was queued: %+v", result)
	}
	msgs, err := filepath.Glob(filepath.Join(data, "netmail", "*.msg"))
	if err != nil || len(msgs) == 0 {
		t.Fatalf("no netmail written (%v)", err)
	}
	for _, m := range msgs {
		raw, err := os.ReadFile(m)
		if err != nil {
			t.Fatal(err)
		}
		subject := strings.TrimPrefix(cString(raw[72:144]), "^")
		if !filepath.IsAbs(subject) {
			t.Errorf("%s: subject %q is relative; the mailer cannot find it", filepath.Base(m), subject)
			continue
		}
		if _, err := os.Stat(subject); err != nil {
			t.Errorf("%s: subject %q names no file: %v", filepath.Base(m), subject, err)
		}
	}
}

// A message an older build queued with a relative subject is not reused for the
// same attachment under absolute subjects, so a resumed batch queues a fresh,
// findable one. The basename modes still match by basename, as they must.
func TestMessageForAttachmentSkipsAnOldRelativeSubject(t *testing.T) {
	netmail := t.TempDir()
	origin, dest := Address{Zone: 1, Net: 229, Node: 200}, Address{Zone: 1, Net: 229, Node: 100}
	old := Config{OutgoingNetmailDir: netmail, SubjectMode: SubjectAbsolute}
	if _, err := createFileAttach(old, filepath.Join("data", "att", "255U0000.BRP"), origin, dest); err != nil {
		t.Fatal(err)
	}
	attachment := filepath.Join(t.TempDir(), "att", "255U0000.BRP")

	if got := messageForAttachment(old, attachment); got != "" {
		t.Errorf("absolute mode reused the relative-subject message %s", got)
	}
	basename := Config{OutgoingNetmailDir: netmail, SubjectMode: SubjectBasename}
	if got := messageForAttachment(basename, attachment); got == "" {
		t.Error("basename mode should still match by basename")
	}
}
