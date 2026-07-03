package menu

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
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
	if _, _, err := run(t, "Q\r", menus.Game); err != nil {
		t.Fatalf("Quit should return nil, got %v", err)
	}
}

func TestQuitIsCaseInsensitive(t *testing.T) {
	menus := BuildMenus()
	if _, _, err := run(t, "q\r", menus.Game); err != nil {
		t.Fatalf("lowercase quit should work, got %v", err)
	}
}

func TestEnterAndLeaveSpendingMenu(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "R\r", menus.Spending) // Return immediately
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(f.out.String(), "Spending Menu") {
		t.Error("expected Spending menu title in output")
	}
}

func TestUnknownKeyIgnored(t *testing.T) {
	menus := BuildMenus()
	if _, _, err := run(t, "z\rQ\r", menus.Game); err != nil {
		t.Fatalf("unknown key should be ignored, got %v", err)
	}
}

func TestHiddenCoordinatorNotSelectable(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "Y\rR\r", menus.System)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(f.out.String(), "Configuration Editor") {
		t.Error("hidden Coordinator menu should not be reachable")
	}
}

func TestCoordinatorReachableWhenFlagged(t *testing.T) {
	menus := BuildMenus()
	f := &fakeSession{keys: []rune("Y\rR\rR\r")}
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
	f := &fakeSession{keys: []rune("L\r1\r5\r ")} // Buy Land -> type 1 (Coastal) -> qty 5
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
	f, _, err := run(t, "*\rR\rR\r", menus.Spending) // '*' -> System Menu -> Return -> Return
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(f.out.String(), "System Menu") {
		t.Error("expected System Menu title in output")
	}
}

func TestPreferenceToggleViaSystemMenu(t *testing.T) {
	menus := BuildMenus()
	f := &fakeSession{keys: []rune("P\rF\r")}
	w := newWorld()
	if err := Run(f, w, menus.System); err != io.EOF {
		t.Fatalf("expected EOF after script, got %v", err)
	}
	if !w.AutoFeed {
		t.Error("Auto-feed should be ON after toggling")
	}
}

// testResolveMenu builds a small menu exercising hotkey match, unique and
// ambiguous label-prefix match, a heading (unselectable), and a hidden item.
func testResolveMenu() *Menu {
	noop := func(session.Session, *game.World) Result { return Back }
	return &Menu{Items: []Item{
		{Key: 'C', Label: "Carriers", Do: noop},
		{Key: 'X', Label: "Card readers", Do: noop},
		{Label: "-- heading --"}, // Do == nil, not selectable
		{Key: 'H', Label: "Hidden Item", Do: noop, Hidden: func(*game.World) bool { return true }},
	}}
}

func TestResolveHotkeyMatch(t *testing.T) {
	m := testResolveMenu()
	g := newWorld()
	it := m.resolve("c", g)
	if it == nil || it.Label != "Carriers" {
		t.Fatalf("expected hotkey match on Carriers, got %v", it)
	}
}

func TestResolveUniqueLabelPrefixMatch(t *testing.T) {
	m := testResolveMenu()
	g := newWorld()
	it := m.resolve("Carr", g)
	if it == nil || it.Label != "Carriers" {
		t.Fatalf("expected unique prefix match on Carriers, got %v", it)
	}
}

func TestResolveAmbiguousPrefixReturnsNil(t *testing.T) {
	m := testResolveMenu()
	g := newWorld()
	if it := m.resolve("Car", g); it != nil {
		t.Fatalf("expected ambiguous prefix to resolve to nil, got %v", it)
	}
}

func TestResolveHiddenItemNotMatched(t *testing.T) {
	m := testResolveMenu()
	g := newWorld()
	if it := m.resolve("H", g); it != nil {
		t.Fatalf("expected hidden item to not match, got %v", it)
	}
	if it := m.resolve("Hidden", g); it != nil {
		t.Fatalf("expected hidden item label prefix to not match, got %v", it)
	}
}

func TestResolveHeadingNotMatched(t *testing.T) {
	m := testResolveMenu()
	g := newWorld()
	if it := m.resolve("--", g); it != nil {
		t.Fatalf("expected heading to not match, got %v", it)
	}
}

func TestReadChoiceBackspaceEditsBuffer(t *testing.T) {
	m := testResolveMenu()
	g := newWorld()
	// Type "X" (would match Card readers via hotkey), backspace it out,
	// then type "C" and Enter -> should select Carriers, not Card readers.
	f := &fakeSession{keys: []rune("X\x7fC\r")}
	it, err := m.readChoice(f, g)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if it == nil || it.Label != "Carriers" {
		t.Fatalf("expected Carriers after backspace-edit, got %v", it)
	}
}

func TestSetTaxRateViaSystemMenu(t *testing.T) {
	menus := BuildMenus()
	f := &fakeSession{keys: []rune("X\r50\r ")}
	w := newWorld()
	Run(f, w, menus.System)
	if w.Active.Tax != 50 {
		t.Errorf("expected tax rate 50, got %d", w.Active.Tax)
	}
}
