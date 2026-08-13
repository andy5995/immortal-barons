package play

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/store"
)

// peekSession replays keys, running peek before each one. peek sees the world
// exactly as another BBS node would: read back off disk, mid-session.
type peekSession struct {
	keys []rune
	pos  int
	out  bytes.Buffer
	peek func(step int)
}

func (f *peekSession) ReadKey() (rune, error) {
	f.peek(f.pos)
	if f.pos >= len(f.keys) {
		return 0, io.EOF
	}
	r := f.keys[f.pos]
	f.pos++
	return r, nil
}

func (f *peekSession) Write(p []byte) (int, error) { return f.out.Write(p) }

// TestCallerIsOnlineAtTheOpeningMenu is the case the feature exists for: a
// baron who has used their turns sits at the opening menu without acting, and
// another node has to see them. Stamping only on a menu action left them
// invisible for the whole visit, which is how this shipped broken the first
// time.
//
// The check reads world.json rather than the in-process World, because that is
// the only channel between nodes — a stamp that never reaches the file is a
// stamp nobody else can see.
func TestCallerIsOnlineAtTheOpeningMenu(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	// splash dismiss, Enter (English), realm name, then Quit at the opening menu.
	keys := " \rKhanate\r0"
	online := map[int]bool{}
	f := &peekSession{
		keys: []rune(keys),
		// Sample every read rather than one hardcoded index: the number of reads
		// onboarding costs is not this test's business, and pinning it would rot
		// the moment a prompt is added.
		peek: func(step int) {
			w, err := store.Load(cfg)
			if err != nil {
				t.Errorf("another node could not load the world: %v", err)
				return
			}
			if e := w.FindByOwner("khan"); e != nil {
				online[step] = e.Online()
			}
		},
	}
	if _, err := Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	// Anchor on the screen, not on a read count: onboarding's key cost is not
	// this test's business, and the session ends on the opening menu's own read.
	if !strings.Contains(f.out.String(), "Game Bulletins") {
		t.Fatalf("script never reached the opening menu; output was:\n%s", f.out.String())
	}
	if !online[f.pos] {
		t.Errorf("a caller sitting at the opening menu must read as online to other nodes "+
			"(observed %v)", online)
	}
}

// TestSessionEndClearsPresence is the other half: quitting drops the baron off
// the roster at once instead of leaving them there for the window.
func TestSessionEndClearsPresence(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	f := &fakeSession{keys: []rune(" \rKhanate\r0")}
	if _, err := Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	e := w.FindByOwner("khan")
	if e == nil {
		t.Fatal("empire should have been created and saved")
	}
	if e.Online() {
		t.Error("a baron who quit must not still read as online")
	}
	if e.LastActive != 0 {
		t.Errorf("LastActive = %d after a clean quit, want 0", e.LastActive)
	}
}
