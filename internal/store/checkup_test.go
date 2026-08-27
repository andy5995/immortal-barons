package store

import (
	"crypto/ed25519"
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

// checkNamed returns one check by name, so a test can assert on it without
// depending on where it sits in the report.
func checkNamed(t *testing.T, cfg game.Config, name string) Check {
	t.Helper()
	for _, c := range Checkup(cfg) {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %q check in the report", name)
	return Check{}
}

// A key file that is present but does not decode is the same to the game as no
// key at all — loadLeagueKeys reads it as nil and every league order is refused.
// Reporting "ok" for it answers the sysop's actual question wrongly and sends
// them hunting elsewhere, which is what happened on a live league.
func TestCheckupRejectsAKeyFileThatDoesNotDecode(t *testing.T) {
	for _, tc := range []struct {
		name, file, contents, want string
	}{
		{"Coordinator key", CoordPubFile, "not hex at all", "does not read as a key"},
		{"Coordinator key", CoordPubFile, "abcd", "does not read as a key"}, // valid hex, too short
		{"Board signing key", BoardKeyFile, "zz", "does not read as a key"},
	} {
		t.Run(tc.file+"/"+tc.contents, func(t *testing.T) {
			dir := rosterDir(t)
			cfg := game.DefaultConfig()
			cfg.DataDir = dir
			cfg.IBBS = true
			cfg.BoardID = "Bravo BBS"
			cfg.InboundDir, cfg.OutboundDir = dir, dir
			if err := os.WriteFile(filepath.Join(dir, tc.file), []byte(tc.contents+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			got := checkNamed(t, cfg, tc.name)
			if got.OK {
				t.Fatalf("%s reported ok for an undecodable file: %q", tc.name, got.Detail)
			}
			if !strings.Contains(got.Detail, tc.want) {
				t.Errorf("detail %q does not say the file is unreadable", got.Detail)
			}
		})
	}
}

// The good path must still pass, or the check above would be satisfied by a
// report that fails on everything.
func TestCheckupAcceptsAKeyThatDecodes(t *testing.T) {
	dir := rosterDir(t)
	cfg := game.DefaultConfig()
	cfg.DataDir = dir
	cfg.IBBS = true
	cfg.BoardID = "Bravo BBS"
	cfg.InboundDir, cfg.OutboundDir = dir, dir
	if err := InstallCoordPub(dir, strings.Repeat("ab", ed25519.PublicKeySize)); err != nil {
		t.Fatal(err)
	}
	if got := checkNamed(t, cfg, "Coordinator key"); !got.OK {
		t.Errorf("a well-formed coord.pub was rejected: %q", got.Detail)
	}
}

// #227: 0 is not "unset" anywhere it is read — a board left there accepts every
// league's packets and has its own accepted in turn — so an inter-BBS board is
// required to carry the number its league runs under.
func TestCheckLeagueNumberRequiredOnlyForALeagueBoard(t *testing.T) {
	// Whether a board is in a league at all is the caller's question, so the
	// stand-alone case is checked where it is actually asked: Checkup returns
	// before it ever reaches the league settings.
	alone := game.DefaultConfig()
	alone.DataDir = rosterDir(t)
	alone.IBBS = false
	for _, c := range Checkup(alone) {
		if c.Name == "League number" {
			t.Errorf("a board that plays alone was asked for a league number: %q", c.Detail)
		}
	}

	member := game.DefaultConfig()
	member.DataDir = rosterDir(t)
	member.IBBS = true
	member.BoardID = "Bravo BBS"
	err := CheckLeagueNumber(member)
	if err == nil {
		t.Fatal("a league board with no league number was accepted")
	}
	// The message has to send the sysop to the two things they need: who has the
	// number, and which file it goes in.
	for _, want := range []string{"League Coordinator", BoardConfigFile, "LeagueNumber"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v", want, err)
		}
	}

	member.LeagueNumber = 42
	if err := CheckLeagueNumber(member); err != nil {
		t.Errorf("a league board with a number was refused: %v", err)
	}
}

// Node 1 has nobody to ask — the number is that board's own to pick — so it is
// told to choose one rather than sent to a Coordinator that is itself.
func TestCheckLeagueNumberTellsTheCoordinatorToChoose(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = rosterDir(t)
	cfg.IBBS = true
	cfg.BoardID = "Alpha BBS" // node 1 in twoBoardRoster
	err := CheckLeagueNumber(cfg)
	if err == nil {
		t.Fatal("the Coordinator's board with no league number was accepted")
	}
	if !strings.Contains(err.Error(), "yours to choose") {
		t.Errorf("the Coordinator is not told the number is theirs to pick: %v", err)
	}
	if strings.Contains(err.Error(), "ask your League Coordinator") {
		t.Errorf("the Coordinator is told to ask itself: %v", err)
	}
}

// The setup report is where a sysop is meant to find this before a transport
// run does, so it has to FAIL rather than pass with a note.
func TestCheckupFailsOnAMissingLeagueNumber(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = rosterDir(t)
	cfg.IBBS = true
	cfg.BoardID = "Bravo BBS"
	cfg.LeagueNumber = 0
	for _, c := range Checkup(cfg) {
		if c.Name != "League number" {
			continue
		}
		if c.OK {
			t.Fatalf("a missing league number reported as ok: %q", c.Detail)
		}
		return
	}
	t.Fatal("Checkup reported no League number line at all")
}
