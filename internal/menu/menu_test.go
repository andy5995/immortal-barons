package menu

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// fakeSession feeds a scripted key sequence and captures written output.
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

func run(t *testing.T, keys string) (*fakeSession, error) {
	t.Helper()
	f := &fakeSession{keys: []rune(keys)}
	cfg := game.DefaultConfig()
	cfg.AICount = 1
	w := game.NewWorldSeed(cfg, 1)
	w.Active = w.AddHuman("tester", "Testland")
	w.Today = "2026-07-03"
	return f, Run(f, w, Build())
}

func TestQuitFromMain(t *testing.T) {
	if _, err := run(t, "Q"); err != nil {
		t.Fatalf("Quit should return nil, got %v", err)
	}
}

func TestQuitIsCaseInsensitive(t *testing.T) {
	if _, err := run(t, "q"); err != nil {
		t.Fatalf("lowercase quit should work, got %v", err)
	}
}

func TestEnterAndLeaveSubmenu(t *testing.T) {
	f, err := run(t, "BRQ") // Buy -> Return -> Quit
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(f.out.String(), "Buy / Sell") {
		t.Error("expected Buy submenu title in output")
	}
}

func TestUnknownKeyIgnored(t *testing.T) {
	if _, err := run(t, "zQ"); err != nil {
		t.Fatalf("unknown key should be ignored, got %v", err)
	}
}

func TestHiddenCoordinatorNotSelectable(t *testing.T) {
	f, err := run(t, "YQ")
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(f.out.String(), "Configuration Editor") {
		t.Error("hidden Coordinator menu should not be reachable")
	}
}

func TestCoordinatorReachableWhenFlagged(t *testing.T) {
	f := &fakeSession{keys: []rune("YRQ")}
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	w.Active = w.AddHuman("tester", "Testland")
	w.Today = "2026-07-03"
	w.Coordinator = true
	if err := Run(f, w, Build()); err != nil {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(f.out.String(), "Configuration Editor") {
		t.Error("coordinator menu should be reachable when flagged")
	}
}

func TestBuyLandThroughMenu(t *testing.T) {
	f := &fakeSession{keys: []rune("BL5\r ")}
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	w.Active = w.AddHuman("tester", "Testland")
	w.Today = "2026-07-03"
	before := w.Active.Land
	Run(f, w, Build())
	if w.Active.Land != before+5 {
		t.Errorf("expected land %d, got %d", before+5, w.Active.Land)
	}
}

func TestPreferenceToggle(t *testing.T) {
	f := &fakeSession{keys: []rune("PF")}
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	w.Active = w.AddHuman("tester", "Testland")
	w.Today = "2026-07-03"
	if err := Run(f, w, Build()); err != io.EOF {
		t.Fatalf("expected EOF after script, got %v", err)
	}
	if !w.AutoFeed {
		t.Error("Auto-feed should be ON after toggling")
	}
}

func TestNextTurnConsumesATurn(t *testing.T) {
	f := &fakeSession{keys: []rune("N ")}
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	w.Active = w.AddHuman("tester", "Testland")
	w.Today = "2026-07-03"
	left := w.Active.TurnsLeft
	Run(f, w, Build())
	if w.Active.TurnsLeft != left-1 {
		t.Errorf("Next Turn should consume a turn: want %d, got %d", left-1, w.Active.TurnsLeft)
	}
}
