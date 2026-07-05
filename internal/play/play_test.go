package play

import (
	"bytes"
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

func cfgIn(dir string) game.Config {
	c := game.DefaultConfig()
	c.DataDir = dir
	return c
}

func TestOnboardsThenPersists(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	// splash dismiss, then realm name "Khanate", then Quit
	f := &fakeSession{keys: []rune(" Khanate\r0")}
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
}

func TestReturningPlayerResumes(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	f1 := &fakeSession{keys: []rune(" Khanate\r0")}
	Run(f1, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	f2 := &fakeSession{keys: []rune(" 0")} // no naming prompt second time
	Run(f2, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	if strings.Contains(f2.out.String(), "Name your Realm") {
		t.Error("returning player should not be asked to name a realm")
	}
}

func TestBusyLockIsReported(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	held, err := store.Lock(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	f := &fakeSession{}
	if _, err := Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatalf("busy should be handled gracefully, got %v", err)
	}
	if !strings.Contains(f.out.String(), "busy") {
		t.Error("should tell the caller the game is busy")
	}
}

func TestEventsShownThenCleared(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	// create + save an empire with a pending event
	w := game.NewWorld(cfg)
	e := w.AddHuman("khan", "Khanate")
	e.Events = []string{"Enemy raided you!"}
	store.Save(w, cfg)

	f := &fakeSession{keys: []rune(" 0")}
	Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	if !strings.Contains(f.out.String(), "raided") {
		t.Error("pending event should be shown on login")
	}
	w2, _ := store.Load(cfg)
	if len(w2.FindByOwner("khan").Events) != 0 {
		t.Error("events should be cleared after being shown")
	}
}
