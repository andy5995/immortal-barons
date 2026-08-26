package play

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// TestRunReportsASessionWhoseSavesFailed is the end-to-end half of the fix: a
// session whose transactions cannot reach the disk must END WITH AN ERROR, not
// exit cleanly having quietly discarded the caller's turn.
//
// That silence is what made the Windows CI failure unreadable — the door exited
// 0, the test harness therefore printed no process output, and all that was left
// was an empire missing from world.json with nothing anywhere saying why.
//
// The failure is induced with a read-only data directory, the most portable way
// to make a real Save fail without reaching into the store: Save writes
// world.json.tmp before renaming it into place, and that write is refused.
func TestRunReportsASessionWhoseSavesFailed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a read-only directory does not stop a write on Windows; the mechanism under test is platform-independent")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root ignores directory permissions")
	}

	dir := t.TempDir()
	cfg := cfgIn(dir)
	if err := store.Save(game.NewWorld(cfg), cfg); err != nil {
		t.Fatalf("seed world: %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	// Restore write permission so t.TempDir's cleanup can remove the directory.
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	// Splash dismiss, Enter (English), realm name, confirm — enough to reach a
	// transaction that must be written.
	f := &fakeSession{keys: []rune(" \rTestrealm\ry")}
	_, err := Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	if err == nil {
		t.Fatal("Run returned no error for a session whose saves all failed; a lost turn must not look like a completed one")
	}
	if !strings.Contains(err.Error(), "could not be saved") {
		t.Errorf("the error should say the session was not recorded, got %q", err)
	}
}
