package store

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

const twoBoardRoster = "1\nAlpha BBS\n1:1/1\nLocal\nXX\nUSA\n\n2\nBravo BBS\n1:1/2\nLocal\nXX\nUSA\n"

func rosterDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, NodeListFile), []byte(twoBoardRoster), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBoardInRosterAcceptsAnExactName(t *testing.T) {
	if err := CheckBoardInRoster(rosterDir(t), "Bravo BBS"); err != nil {
		t.Errorf("exact name rejected: %v", err)
	}
}

// The reported failure was a BoardID left at its default while the roster was
// right, three steps from where it surfaced (#154).
func TestBoardInRosterNamesTheRosterWhenNothingIsClose(t *testing.T) {
	err := CheckBoardInRoster(rosterDir(t), "local")
	if err == nil {
		t.Fatal("an unlisted board name was accepted")
	}
	for _, want := range []string{"local", "Alpha BBS", "Bravo BBS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q: %v", want, err)
		}
	}
}

func TestBoardInRosterSuggestsTheNearMiss(t *testing.T) {
	for _, typo := range []string{"bravo bbs", "Bravo  BBS", "Bravo BBs", "Brave BBS"} {
		err := CheckBoardInRoster(rosterDir(t), typo)
		if err == nil {
			t.Errorf("%q was accepted as a board name", typo)
			continue
		}
		if !strings.Contains(err.Error(), `closest entry is "Bravo BBS"`) {
			t.Errorf("%q: no near miss offered: %v", typo, err)
		}
	}
}

// A member is told to reset before the Coordinator's first packet arrives, so
// having no roster yet must not block the setup that creates the board.
func TestBoardInRosterPassesWithNoRoster(t *testing.T) {
	if err := CheckBoardInRoster(t.TempDir(), "anything"); err != nil {
		t.Errorf("a missing roster was treated as an error: %v", err)
	}
}

func TestCheckupReportsEveryProblemAtOnce(t *testing.T) {
	dir := rosterDir(t)
	cfg := game.DefaultConfig()
	cfg.DataDir = dir
	cfg.IBBS = true
	cfg.BoardID = "bravo bbs"
	cfg.InboundDir = filepath.Join(dir, "missing-in")
	cfg.OutboundDir = filepath.Join(dir, "missing-out")

	failed := map[string]string{}
	for _, c := range Checkup(cfg) {
		if !c.OK {
			failed[c.Name] = c.Detail
		}
	}
	for _, want := range []string{"Board name", "Inbound directory", "Outbound directory", "Coordinator key"} {
		if _, ok := failed[want]; !ok {
			t.Errorf("%s was not reported as a problem", want)
		}
	}
	if !strings.Contains(failed["Board name"], "Bravo BBS") {
		t.Errorf("the board-name failure does not offer the near miss: %q", failed["Board name"])
	}
}

func TestCheckupOnABoardThatPlaysAlone(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.IBBS = false
	checks := Checkup(cfg)
	if len(checks) != 1 || checks[0].OK {
		t.Fatalf("a single-board game should report one problem, got %+v", checks)
	}
}

// A roster that is present but unusable is a different answer from one that
// has not arrived yet: the first is a setup error, the second is normal.
func TestBoardInRosterReportsAnUnusableRoster(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, NodeListFile), []byte("not a roster\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := CheckBoardInRoster(dir, "Bravo BBS")
	if err == nil {
		t.Fatal("an unparseable roster was treated as no roster at all")
	}
	if !strings.Contains(err.Error(), "lists no boards") {
		t.Errorf("unexpected message: %v", err)
	}
}

// Reported once, on the roster line — not again as a board-name failure.
func TestCheckupBlamesTheRosterOnceWhenItWillNotLoad(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, NodeListFile), []byte("not a roster\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := game.DefaultConfig()
	cfg.DataDir, cfg.IBBS, cfg.BoardID = dir, true, "Bravo BBS"

	checks := Checkup(cfg)
	names := map[string]Check{}
	for _, c := range checks {
		names[c.Name] = c
	}
	if c, ok := names["League roster"]; !ok || c.OK {
		t.Error("the roster was not reported as the problem")
	}
	if c, ok := names["Board name"]; !ok || c.OK || strings.Contains(c.Detail, "lists no boards") {
		t.Errorf("the board name repeated the roster's failure: %+v", c)
	}
	if _, ok := names["Coordinator key"]; !ok {
		t.Error("a bad roster stopped the later checks from running")
	}
}

// The probe is what makes this answer true on Windows, where a directory's
// write access lives in an ACL that never reaches os.Stat's FileMode — that is
// derived from FILE_ATTRIBUTE_READONLY alone, which Windows ignores for
// directories. A stat-only check would pass a directory the door cannot write.
func TestDirUsableWritesToProveIt(t *testing.T) {
	dir := t.TempDir()
	if !dirUsable(dir, true) {
		t.Error("a writable directory was reported unusable")
	}
	left, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Errorf("the probe left files behind: %v", left)
	}
	if dirUsable(filepath.Join(dir, "nope"), false) {
		t.Error("a missing directory was reported usable")
	}
}

func TestDirUsableRejectsAnUnwritableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Chmod cannot make a directory unwritable on Windows; access lives in the ACL")
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes regardless of the mode bits")
	}
	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	if dirUsable(dir, true) {
		t.Error("an unwritable directory was reported writable")
	}
	if !dirUsable(dir, false) {
		t.Error("a readable directory was reported unreadable")
	}
}
