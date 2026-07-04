package menu

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

func TestStatusBar(t *testing.T) {
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	p := w.AddHuman("me", "Waste Kings")
	w.Active = p

	bar := topBar(w)
	for _, want := range []string{"LOCAL", "Immortal Barons v" + game.Version, "Waste Kings"} {
		if !strings.Contains(bar, want) {
			t.Errorf("status bar missing %q: %q", want, bar)
		}
	}
	// Visible width (color codes stripped) must be exactly barWidth.
	visible := bar
	for _, code := range []string{ansi.BgBlue, ansi.FgBrightWhite, ansi.Reset} {
		visible = strings.ReplaceAll(visible, code, "")
	}
	if got := len([]rune(visible)); got != barWidth {
		t.Errorf("visible width = %d, want %d", got, barWidth)
	}
}

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
	if _, _, err := run(t, "0", menus.Game); err != nil {
		t.Fatalf("Quit should return nil, got %v", err)
	}
}

func TestQuitIsCaseInsensitive(t *testing.T) {
	menus := BuildMenus()
	if _, _, err := run(t, "0", menus.Game); err != nil {
		t.Fatalf("lowercase quit should work, got %v", err)
	}
}

func TestEnterAndLeaveSpendingMenu(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "0", menus.Spending) // Return immediately
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(f.out.String(), "Spending Menu") {
		t.Error("expected Spending menu title in output")
	}
}

func TestUnknownKeyIgnored(t *testing.T) {
	menus := BuildMenus()
	if _, _, err := run(t, "z0", menus.Game); err != nil {
		t.Fatalf("unknown key should be ignored, got %v", err)
	}
}

func TestHiddenCoordinatorNotSelectable(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "Y0", menus.System)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(f.out.String(), "Configuration Editor") {
		t.Error("hidden Coordinator menu should not be reachable")
	}
}

func TestCoordinatorReachableWhenFlagged(t *testing.T) {
	menus := BuildMenus()
	f := &fakeSession{keys: []rune("Y00")}
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
	f := &fakeSession{keys: []rune("6C\r5\r ")} // Buy Land -> type C (Coastal) -> qty 5
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
	f, _, err := run(t, "*00", menus.Spending) // '*' -> System Menu -> Return -> Return
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

// testByKeyMenu builds a small menu exercising hotkey match, a heading
// (unselectable), and a hidden item.
func testByKeyMenu() *Menu {
	noop := func(session.Session, *game.World) Result { return Back }
	return &Menu{Items: []Item{
		{Key: 'C', Label: "Carriers", Do: noop},
		{Key: 'X', Label: "Card readers", Do: noop},
		{Label: "-- heading --"}, // Do == nil, not selectable
		{Key: 'H', Label: "Hidden Item", Do: noop, Hidden: func(*game.World) bool { return true }},
	}}
}

func TestByKeyHotkeyMatch(t *testing.T) {
	m := testByKeyMenu()
	g := newWorld()
	it := m.byKey('c', g)
	if it == nil || it.Label != "Carriers" {
		t.Fatalf("expected hotkey match on Carriers, got %v", it)
	}
}

func TestByKeyHiddenItemNotMatched(t *testing.T) {
	m := testByKeyMenu()
	g := newWorld()
	if it := m.byKey('H', g); it != nil {
		t.Fatalf("expected hidden item to not match, got %v", it)
	}
}

func TestByKeyUnknownKeyNotMatched(t *testing.T) {
	m := testByKeyMenu()
	g := newWorld()
	if it := m.byKey('Z', g); it != nil {
		t.Fatalf("expected unknown key to not match, got %v", it)
	}
}

func TestReadChoiceSelectsOnSingleKeypress(t *testing.T) {
	m := testByKeyMenu()
	g := newWorld()
	f := &fakeSession{keys: []rune("C")}
	it, err := m.readChoice(f, g)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if it == nil || it.Label != "Carriers" {
		t.Fatalf("expected Carriers on single keypress, got %v", it)
	}
}

func TestMenuColorRendersTitleAndHotkeys(t *testing.T) {
	m := &Menu{
		Title: "Colorful Menu",
		Color: ansi.FgBrightMagenta,
		Items: []Item{
			{Key: '0', Label: "Return", Do: back},
		},
	}
	f := &fakeSession{keys: []rune("0")}
	g := newWorld()
	if err := Run(f, g, m); err != nil {
		t.Fatalf("got %v", err)
	}
	out := f.out.String()
	// The title is centered within a colored rule line: "──[Colorful Menu]──".
	if !strings.Contains(out, "[Colorful Menu]") {
		t.Errorf("expected bracketed title in output, got:\n%s", out)
	}
	if !strings.Contains(out, ansi.FgBrightMagenta) {
		t.Error("expected menu color code in drawn output")
	}
}

func TestSetTaxRateViaSystemMenu(t *testing.T) {
	menus := BuildMenus()
	f := &fakeSession{keys: []rune("R50\r ")}
	w := newWorld()
	Run(f, w, menus.System)
	if w.Active.Tax != 50 {
		t.Errorf("expected tax rate 50, got %d", w.Active.Tax)
	}
}

func TestInterBBSItemsHiddenUnlessEnabled(t *testing.T) {
	f := &fakeSession{keys: []rune("0")}
	w := newWorld() // default: no IBBS, no league
	Run(f, w, BuildMenus().Trading)
	if strings.Contains(f.out.String(), "View IPScores") {
		t.Error("IPScores should be hidden when IBBS/league is off")
	}

	f2 := &fakeSession{keys: []rune("0")}
	w2 := newWorld()
	w2.Config.IBBS = true
	Run(f2, w2, BuildMenus().Trading)
	if !strings.Contains(f2.out.String(), "View IPScores") {
		t.Error("IPScores should appear when IBBS is enabled")
	}
}

func TestHelpBrowseShowsControls(t *testing.T) {
	// category 1 (controls) -> topic 1 -> pause -> back (0) -> leave (0)
	f := &fakeSession{keys: []rune("1\r1\r 0\r0\r")}
	w := newWorld()
	helpBrowse(f, w)
	out := f.out.String()
	if !strings.Contains(out, "Controls") {
		t.Error("help browser should list the Controls category")
	}
	if !strings.Contains(out, "Moving Through the Menus") && !strings.Contains(out, "Entering Numbers") {
		t.Error("help browser should render a controls topic")
	}
}

func TestPromptSuggestedExpandsKAndM(t *testing.T) {
	f := &fakeSession{keys: []rune("1k\r")}
	if got := promptSuggested(f, "How many?", 0, 1_000_000); got != 1000 {
		t.Errorf("1k should expand to 1000, got %d", got)
	}
	if !strings.Contains(f.out.String(), "1000") {
		t.Errorf("screen should show 1000, not a literal k: %q", f.out.String())
	}

	f2 := &fakeSession{keys: []rune("2m\r")}
	if got := promptSuggested(f2, "How many?", 0, 100_000_000); got != 2_000_000 {
		t.Errorf("2m should expand to 2000000, got %d", got)
	}
}

func TestEnterExitsWhenPrefOn(t *testing.T) {
	w := newWorld()
	w.EnterExitsBuy = true
	m := &Menu{ExitOnEnter: true, Items: []Item{
		{Key: '1', Label: "Buy", Do: func(session.Session, *game.World) Result { return Stay }},
		{Key: '0', Label: "Quit", Do: back},
	}}
	f := &fakeSession{keys: []rune{'\r'}}
	it, err := m.readChoice(f, w)
	if err != nil {
		t.Fatal(err)
	}
	if it == nil || it.Key != '0' {
		t.Errorf("Enter with pref on should select the '0' exit, got %v", it)
	}
}

func TestEnterIgnoredWhenPrefOff(t *testing.T) {
	w := newWorld()
	w.EnterExitsBuy = false
	m := &Menu{ExitOnEnter: true, Items: []Item{{Key: '0', Label: "Quit", Do: back}}}
	f := &fakeSession{keys: []rune{'\r'}}
	if it, _ := m.readChoice(f, w); it != nil {
		t.Errorf("Enter with pref off should select nothing, got %v", it)
	}
}
