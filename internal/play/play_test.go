package play

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

type fakeSession struct {
	keys []rune
	pos  int
	out  bytes.Buffer
}

func (f *fakeSession) ReadKey() (rune, error) {
	if f.pos >= len(f.keys) {
		return 0, io.EOF
	}
	r := f.keys[f.pos]
	f.pos++
	return r, nil
}
func (f *fakeSession) Write(p []byte) (int, error) { return f.out.Write(p) }

// errAfterSession replays keys, then returns a fixed non-EOF error (a stand-in
// for a winsock read error) once they run out — to exercise the splash/
// onboarding session.End path.
type errAfterSession struct {
	keys []rune
	pos  int
	out  bytes.Buffer
	err  error
}

func (f *errAfterSession) ReadKey() (rune, error) {
	if f.pos >= len(f.keys) {
		return 0, f.err
	}
	r := f.keys[f.pos]
	f.pos++
	return r, nil
}
func (f *errAfterSession) Write(p []byte) (int, error) { return f.out.Write(p) }

// A read error during onboarding (here at the realm-name Confirm? prompt)
// unwinds via session.End; play.Run must record WHY in the reason, not just
// "disconnect" (so the door log shows the underlying socket error).
func TestReadErrorDuringOnboardingSurfacesInReason(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	// splash dismiss, Enter (English), realm name, then the Confirm? read errors.
	f := &errAfterSession{
		keys: []rune(" \rTestrealm\r"),
		err:  errors.New("winsock-boom-10054"),
	}
	reason, _ := Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	if !strings.Contains(reason, "winsock-boom-10054") {
		t.Errorf("expected the underlying read error in the reason, got %q", reason)
	}
}

func cfgIn(dir string) game.Config {
	c := game.DefaultConfig()
	c.DataDir = dir
	// Realm reaping off for session tests. These drive sessions that onboard and
	// quit without ever playing a turn, alongside a writer goroutine rolling the
	// day over — so a realm is legitimately erased moments after it is created,
	// and any assertion counting onboarded realms fails intermittently. Reaping
	// itself is covered in internal/game (idle_test.go); nothing here is about it.
	c.IdleDaysRemove = 0
	// Load now errors on a missing world, so stand one up first — as a real
	// deployment does with -reset before the first caller plays.
	if err := store.Save(store.NewGame(c), c); err != nil {
		panic(err)
	}
	return c
}

func TestOnboardsThenPersists(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	// splash dismiss, Enter for the language prompt (English), realm name
	// "Khanate", then Quit
	f := &fakeSession{keys: []rune(" \rKhanate\r0")}
	if _, err := Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	w, _ := store.Load(cfg)
	e := w.FindByOwner("khan")
	if e == nil {
		t.Fatal("empire should have been created and saved")
	}
	if e.Name != "Khanate" {
		t.Errorf("realm name: got %q", e.Name)
	}
	if e.Language != "" {
		t.Errorf("Enter at the language prompt should select English, got %q", e.Language)
	}
}

func TestReturningPlayerResumes(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	f1 := &fakeSession{keys: []rune(" \rKhanate\r0")}
	Run(f1, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	f2 := &fakeSession{keys: []rune(" 0")} // no naming or language prompt second time
	Run(f2, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	if strings.Contains(f2.out.String(), "Name your Realm") {
		t.Error("returning player should not be asked to name a realm")
	}
	if strings.Contains(f2.out.String(), "Select your language") {
		t.Error("returning player should not be asked to select a language")
	}
}

func TestFirstRunLanguageSelection(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	// splash dismiss, "2" (Deutsch) at the language prompt, realm name, Quit
	f := &fakeSession{keys: []rune(" 2\rKhanate\r0")}
	if _, err := Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	w, _ := store.Load(cfg)
	e := w.FindByOwner("khan")
	if e == nil {
		t.Fatal("empire should have been created and saved")
	}
	if e.Language != "de" {
		t.Errorf("Empire.Language: got %q, want %q", e.Language, "de")
	}
}

func TestReturningPlayerNotPrompted(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	// create + save an empire that already has a language set
	w := game.NewWorld(cfg)
	e := w.AddHuman("khan", "Khanate")
	e.Language = "ru"
	store.Save(w, cfg)

	f := &fakeSession{keys: []rune(" 0")}
	Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	if strings.Contains(f.out.String(), "Select your language") {
		t.Error("returning player with a language already set should not be prompted")
	}
	w2, _ := store.Load(cfg)
	if w2.FindByOwner("khan").Language != "ru" {
		t.Error("returning player's language should not be changed")
	}
}

// Events are no longer consumed before the opening menu: a caller who logs in
// and quits without playing keeps them, so they surface at their first Play
// under "Since your last play" (BRE flow — see gameflow_test for the display).
func TestEventsPersistUntilPlayed(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	// create + save an empire with a pending event
	w := game.NewWorld(cfg)
	e := w.AddHuman("khan", "Khanate")
	e.Events = []game.Event{{Text: "Enemy raided you!"}}
	store.Save(w, cfg)

	f := &fakeSession{keys: []rune(" 0")} // log in, quit without playing
	Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	if strings.Contains(f.out.String(), "raided") {
		t.Error("events must not be shown before the opening menu; only after Play")
	}
	w2, _ := store.Load(cfg)
	if len(w2.FindByOwner("khan").Events) != 1 {
		t.Error("events must persist through a login without Play, not be consumed")
	}
}
