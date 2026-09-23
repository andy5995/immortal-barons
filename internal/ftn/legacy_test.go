package ftn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		`  NetmailDir C:\SBBS\fido\netmail` + "\n",
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
