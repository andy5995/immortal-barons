package ftn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/store"
)

// An ftn.cfg left from the separate program is refused with its settings as
// bbs.cfg lines, values byte for byte so a Windows path survives the paste.
func TestLegacyConfigIsRefusedWithBBSCfgLines(t *testing.T) {
	dir := t.TempDir()
	if LegacyRefusal(dir) != nil {
		t.Fatal("a board with no ftn.cfg was refused")
	}
	body := "; comment\n" +
		`InboundDir C:\SBBS\fido\inbound` + "\n" +
		`InboundDir C:\SBBS\fido\inbound\unsecure` + "\n" +
		`InboundNetmailDir C:\SBBS\fido\netmail in` + "\n" +
		`NetmailDir C:\SBBS\fido\netmail` + "\n" +
		"Binkley Yes\n" +
		`Link 2 BSO C:\SBBS\fido\outbound.309 Crash` + "\n" +
		"Bundled Yes\nOboxMeshFanout No\nSubjectPath Basename\nAttachDir att\nSomethingOld 1\n"
	if err := os.WriteFile(filepath.Join(dir, LegacyConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	err := LegacyRefusal(dir)
	if err == nil {
		t.Fatal("ftn.cfg was accepted")
	}
	msg := err.Error()
	for _, want := range []string{
		`  IncomingFileDir C:\SBBS\fido\inbound` + "\n",
		`  IncomingFileDir C:\SBBS\fido\inbound\unsecure` + "\n",
		`  IncomingNetmailDir C:\SBBS\fido\netmail in` + "\n",
		`  OutgoingNetmailDir C:\SBBS\fido\netmail` + "\n",
		"  Mailer Binkley\n",
		`  Link 2 BSO C:\SBBS\fido\outbound.309 Crash` + "\n",
		"  Bundled Yes\n", "  OboxMeshFanout No\n", "  SubjectPath Basename\n", "  AttachDir att\n",
		"barons-ftn",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal has no %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "SomethingOld") || strings.Contains(msg, "Binkley Yes") {
		t.Errorf("the refusal carried a line bbs.cfg has no use for:\n%s", msg)
	}
	// The pasted lines load.
	var lines []string
	for _, line := range strings.Split(msg, "\n") {
		if strings.HasPrefix(line, "  ") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	if len(lines) != 10 {
		t.Errorf("%d lines to paste, want 10:\n%s", len(lines), msg)
	}
}

// The names an older version wrote are refused with their replacements, byte
// for byte: a Windows path keeps its backslashes and its spaces (#241). An FTN
// Link line is the transport's and is left alone, including one with a Raw
// modifier, which the store's own copy of the mode words once missed.
func TestLegacyBoardKeysAreRefusedWithTheirReplacements(t *testing.T) {
	dir := t.TempDir()
	body := "BoardID Alpha BBS\n" +
		`Inbound C:\BBS\IB Data\in` + "\n" +
		`outbound C:\BBS\out` + "\n" +
		`Link 3 D:\fbox\three` + "\n" +
		"Link 4 BSO bso Crash\n" +
		"Link 5 obox\n" +
		"Link 6 Attach\n" +
		"Link 7 Attach Raw\n"
	if err := os.WriteFile(filepath.Join(dir, store.BoardConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	err := store.LegacyBoardRefusal(dir, IsLink)
	if err == nil {
		t.Fatal("the old names were accepted")
	}
	for _, want := range []string{
		`  GameInbound C:\BBS\IB Data\in` + "\n",
		`  GameOutbound C:\BBS\out` + "\n",
		`  GameOutbound 3 D:\fbox\three`,
		// An old per-neighbor directory that happens to be named like a mode:
		// an Obox link always names a directory after the mode, so this is not one.
		`  GameOutbound 5 obox`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal has no line %q:\n%v", want, err)
		}
	}
	if strings.Contains(err.Error(), "Link 4") || strings.Contains(err.Error(), "Link 6") ||
		strings.Contains(err.Error(), "Link 7") {
		t.Errorf("the refusal named an FTN Link line:\n%v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, store.BoardConfigFile), []byte("GameInbound in\nLink 4 Obox box\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.LegacyBoardRefusal(dir, IsLink); err != nil {
		t.Errorf("a current bbs.cfg was refused: %v", err)
	}
}
