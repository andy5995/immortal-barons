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
	err := Run(f, game.NewSeed(1), Build())
	return f, err
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
	w := game.NewSeed(1)
	w.Coordinator = true
	if err := Run(f, w, Build()); err != nil {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(f.out.String(), "Configuration Editor") {
		t.Error("coordinator menu should be reachable when flagged")
	}
}

func TestBuyLandThroughMenu(t *testing.T) {
	// Main -> Buy -> Buy Land -> "5" Enter -> ... then Quit.
	f := &fakeSession{keys: []rune("BL5\r ")}
	w := game.NewSeed(1)
	before := w.Player().Land
	Run(f, w, Build()) // ends on EOF after the scripted keys
	if got := w.Player().Land; got != before+5 {
		t.Errorf("expected land %d, got %d", before+5, got)
	}
}

func TestNextTurnAdvances(t *testing.T) {
	f := &fakeSession{keys: []rune("N ")}
	w := game.NewSeed(1)
	Run(f, w, Build())
	if w.Turn != 1 {
		t.Errorf("expected turn 1 after one Next Turn, got %d", w.Turn)
	}
}
