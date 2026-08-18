package play

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"regexp"
	"strconv"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/session"
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

// A planet holds 25 realms, and the 26th caller is told so instead of being
// handed a realm nobody could ever address (#144). The refusal has to reach the
// player — before slots the surplus realm was created in silence.
func TestFullPlanetRefusesTheTwentySixthCaller(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 25; i++ {
		if e := w.AddHuman(string(rune('a'+i)), "Realm"+string(rune('A'+i))); e == nil {
			t.Fatalf("realm %d of 25 was refused a slot", i+1)
		}
	}
	if err := store.Save(w, cfg); err != nil {
		t.Fatal(err)
	}

	f := &fakeSession{keys: []rune(" \rSurplusia\r0")}
	reason, err := Run(f, Identity{Handle: "Surplus"}, cfg, "2026-07-03")
	if err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if !strings.Contains(out, "All 25 realms of this planet are held") {
		t.Errorf("the caller was not told the planet is full, got: %q", out)
	}
	if strings.Contains(out, "Name your Realm") {
		t.Error("a refused caller should not reach the realm-name prompt")
	}
	if reason != "closed" {
		t.Errorf("session reason = %q, want %q", reason, "closed")
	}
	after, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Empires) != 25 {
		t.Errorf("planet holds %d realms after the refused call, want 25", len(after.Empires))
	}
	if after.FindByOwner("surplus") != nil {
		t.Error("the refused caller was given a realm anyway")
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

// An idle boot at the realm-name prompt ends the session and creates NO realm.
// It used to name the realm after the caller's BBS handle and carry on into the
// opening menu, so a caller who never answered the prompt was given a realm
// they had not named, moments after being told they were disconnected.
func TestIdleBootAtRealmNamePromptCreatesNoRealm(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	// The deadline decorator returns ErrSessionEnded once it boots; the reader
	// stands in for it, running dry at the realm-name prompt.
	f := &errAfterSession{
		keys: []rune(" \r"), // splash dismiss, Enter for English
		err:  session.ErrSessionEnded,
	}
	reason, err := Run(f, Identity{Handle: "Bobby"}, cfg, "2026-07-03")
	w, _ := store.Load(cfg)
	if e := w.FindByOwner("bobby"); e != nil {
		t.Fatalf("a realm was created (%q) for a caller who never named one", e.Name)
	}
	if err != nil {
		t.Fatalf("the session should end cleanly: %v", err)
	}
	// The opening menu must never be reached — that was the visible symptom.
	if strings.Contains(f.out.String(), "[Entry]") {
		t.Errorf("the session ran on into the opening menu after the boot:\n%s", f.out.String())
	}
	if reason == "" {
		t.Error("the door log needs a reason for the session ending")
	}
}

// Every line the onboarding path prints must fit the 80-column terminal. A
// translated string runs longer than the English it came from — the German
// invalid-name message is 84 columns, and unwrapped the terminal split "Reich"
// into "Re" and "ich" at the margin.
func TestOnboardingOutputFitsTheScreen(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	// Splash dismiss, German at the language picker, then a name too short to
	// pass, so the invalid-name message is printed before naming the realm.
	lang := 0
	for i, l := range i18n.Languages {
		if l.Code == "de" {
			lang = i + 2 // English is option 1
		}
	}
	if lang == 0 {
		t.Skip("no German catalog")
	}
	f := &fakeSession{keys: []rune(" " + strconv.Itoa(lang) + "\rx\rDrachenfels\ry0")}
	if _, err := Run(f, Identity{Handle: "Johnny"}, cfg, "2026-07-03"); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := f.out.String()
	if !strings.Contains(out, "Ungültig") {
		t.Fatalf("the German invalid-name message was never shown:\n%s", out)
	}
	for _, line := range strings.Split(stripANSI(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if n := utf8.RuneCountInString(line); n > ansi.ScreenCols {
			t.Errorf("line of %d columns (max %d): %q", n, ansi.ScreenCols, line)
		}
	}
}

var ansiEsc = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

func stripANSI(s string) string { return ansiEsc.ReplaceAllString(s, "") }
