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

// newWorld builds a fresh test world with an active human empire.
func newWorld() *game.World {
	cfg := game.DefaultConfig()
	cfg.AICount = 1
	w := game.NewWorldSeed(cfg, 1)
	w.Active = w.AddHuman("tester", "Testland")
	w.Today = "2026-07-03"
	return w
}

func run(t *testing.T, keys string, root *Menu) (*fakeSession, *game.World, error) {
	t.Helper()
	f := &fakeSession{keys: []rune(keys)}
	w := newWorld()
	return f, w, Run(f, w, root)
}

func TestQuitFromGameMenu(t *testing.T) {
	menus := BuildMenus()
	if _, _, err := run(t, "Q", menus.Game); err != nil {
		t.Fatalf("Quit should return nil, got %v", err)
	}
}

func TestQuitIsCaseInsensitive(t *testing.T) {
	menus := BuildMenus()
	if _, _, err := run(t, "q", menus.Game); err != nil {
		t.Fatalf("lowercase quit should work, got %v", err)
	}
}

func TestEnterAndLeaveSpendingMenu(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "R", menus.Spending) // Return immediately
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(f.out.String(), "Spending Menu") {
		t.Error("expected Spending menu title in output")
	}
}

func TestUnknownKeyIgnored(t *testing.T) {
	menus := BuildMenus()
	if _, _, err := run(t, "zQ", menus.Game); err != nil {
		t.Fatalf("unknown key should be ignored, got %v", err)
	}
}

func TestHiddenCoordinatorNotSelectable(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "YR", menus.System)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(f.out.String(), "Configuration Editor") {
		t.Error("hidden Coordinator menu should not be reachable")
	}
}

func TestCoordinatorReachableWhenFlagged(t *testing.T) {
	menus := BuildMenus()
	f := &fakeSession{keys: []rune("YRR")}
	w := newWorld()
	w.Coordinator = true
	if err := Run(f, w, menus.System); err != nil {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(f.out.String(), "Configuration Editor") {
		t.Error("coordinator menu should be reachable when flagged")
	}
}

func TestBuyLandThroughSpendingMenu(t *testing.T) {
	menus := BuildMenus()
	f := &fakeSession{keys: []rune("L1\r5\r ")} // Buy Land -> type 1 (Coastal) -> qty 5
	w := newWorld()
	before := w.Active.Land
	beforeCoastal := w.Active.Regions.Coastal
	Run(f, w, menus.Spending)
	if w.Active.Land != before+5 {
		t.Errorf("expected land %d, got %d", before+5, w.Active.Land)
	}
	if w.Active.Regions.Coastal != beforeCoastal+5 {
		t.Errorf("expected Coastal regions %d, got %d", beforeCoastal+5, w.Active.Regions.Coastal)
	}
}

func TestReachSystemMenuFromSpending(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "*RR", menus.Spending) // '*' -> System Menu -> Return -> Return
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(f.out.String(), "System Menu") {
		t.Error("expected System Menu title in output")
	}
}

func TestPreferenceToggleViaSystemMenu(t *testing.T) {
	menus := BuildMenus()
	f := &fakeSession{keys: []rune("PF")}
	w := newWorld()
	if err := Run(f, w, menus.System); err != io.EOF {
		t.Fatalf("expected EOF after script, got %v", err)
	}
	if !w.AutoFeed {
		t.Error("Auto-feed should be ON after toggling")
	}
}

func TestSetTaxRateViaSystemMenu(t *testing.T) {
	menus := BuildMenus()
	f := &fakeSession{keys: []rune("X50\r ")}
	w := newWorld()
	Run(f, w, menus.System)
	if w.Active.Tax != 50 {
		t.Errorf("expected tax rate 50, got %d", w.Active.Tax)
	}
}
