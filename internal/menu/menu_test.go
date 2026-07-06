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
	c := &ctx{World: w, active: p}

	bar := topBar(c)
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

// newWorld builds a fresh test ctx with an active human empire.
func newWorld() *ctx {
	cfg := game.DefaultConfig()
	cfg.AICount = 1
	w := game.NewWorldSeed(cfg, 1)
	c := &ctx{World: w, active: w.AddHuman("tester", "Testland")}
	c.Today = "2026-07-03"
	return c
}

func run(t *testing.T, keys string, root *Menu) (*fakeSession, *ctx, error) {
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
	f, _, err := run(t, "0", menus.Spending) // Quit immediately
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
	f := &fakeSession{keys: []rune("6C5\r ")} // Buy Land -> C (Coastal, single key) -> qty 5
	w := newWorld()
	before := w.Player().Land
	beforeCoastal := w.Player().Regions.Coastal
	Run(f, w, menus.Spending)
	if w.Player().Land != before+5 {
		t.Errorf("expected land %d, got %d", before+5, w.Player().Land)
	}
	if w.Player().Regions.Coastal != beforeCoastal+5 {
		t.Errorf("expected Coastal regions %d, got %d", beforeCoastal+5, w.Player().Regions.Coastal)
	}
}

func TestBuyLandThroughSpendingMenuCannotExceedPerTurnCap(t *testing.T) {
	menus := BuildMenus()
	w := newWorld()
	w.Config.MaxRegions = 5
	w.Player().Gold = 1_000_000
	before := w.Player().Land

	// First visit: buy the max affordable, which the rising price and the
	// per-turn cap both bound; expect it clamped to the 5-region cap.
	f := &fakeSession{keys: []rune("6C>\r")}
	Run(f, w, menus.Spending)
	if got := w.Player().Land - before; got != 5 {
		t.Fatalf("first purchase: want 5 regions bought, got %d", got)
	}

	// Second visit in the same turn (re-entering the Spending menu, the bug
	// scenario): the cap must already be exhausted, so the offered max is 0
	// and nothing more can be bought — the cap does not reset per action.
	f2 := &fakeSession{keys: []rune("6C>\r")}
	Run(f2, w, menus.Spending)
	if got := w.Player().Land - before; got != 5 {
		t.Errorf("second purchase in the same turn: want land still +5, got +%d", got)
	}
}

func TestBuyLandLoopsForMultiplePurchases(t *testing.T) {
	menus := BuildMenus()
	w := newWorld()
	before := w.Player().Land
	beforeCoastal := w.Player().Regions.Coastal
	beforeMountain := w.Player().Regions.Mountain
	// Buy Land -> C (Coastal) -> qty 5 -> pause -> M (Mountain) -> qty 3 ->
	// pause -> 0 (quit the buy loop) -> 0 (quit Spending menu).
	f := &fakeSession{keys: []rune("6C5\r M3\r 00")}
	Run(f, w, menus.Spending)
	if w.Player().Land != before+8 {
		t.Errorf("expected land %d, got %d", before+8, w.Player().Land)
	}
	if got := w.Player().Regions.Coastal; got != beforeCoastal+5 {
		t.Errorf("expected Coastal regions %d, got %d", beforeCoastal+5, got)
	}
	if got := w.Player().Regions.Mountain; got != beforeMountain+3 {
		t.Errorf("expected Mountain regions %d, got %d", beforeMountain+3, got)
	}
	out := f.out.String()
	if n := strings.Count(out, "Buy Regions"); n != 1 {
		t.Errorf("region list should be drawn once (not reprinted per purchase), got %d:\n%s", n, out)
	}
	if !strings.Contains(out, "Coastal regions purchased") || !strings.Contains(out, "Mountain regions purchased") {
		t.Errorf("expected both purchase confirmations in one visit:\n%s", out)
	}
}

func TestBuyLandAdvisorsThenContinuesLoop(t *testing.T) {
	menus := BuildMenus()
	w := newWorld()
	before := w.Player().Land
	// Buy Land -> * (Advisors) -> pause -> C (Coastal) -> qty 4 -> pause ->
	// 0 (quit the buy loop) -> 0 (quit Spending menu).
	f := &fakeSession{keys: []rune("6* C4\r 00")}
	Run(f, w, menus.Spending)
	if w.Player().Land != before+4 {
		t.Errorf("expected land %d, got %d", before+4, w.Player().Land)
	}
	out := f.out.String()
	if !strings.Contains(out, "Visit Advisors") {
		t.Errorf("expected Advisors output, got:\n%s", out)
	}
	if !strings.Contains(out, "Coastal") {
		t.Errorf("expected region list after Advisors, got:\n%s", out)
	}
}

func TestBuyLandCapBlocksButLoopContinues(t *testing.T) {
	menus := BuildMenus()
	w := newWorld()
	w.Config.MaxRegions = 5
	w.Player().Gold = 1_000_000
	before := w.Player().Land

	// Buy Land -> C -> 5 (hits the cap) -> pause -> C again -> > (offer is
	// now clamped to 0, so nothing more is bought) -> * (Advisors) -> pause
	// -> 0 (quit the buy loop) -> 0 (quit Spending menu).
	f := &fakeSession{keys: []rune("6C5\r C>\r * 00")}
	Run(f, w, menus.Spending)
	if got := w.Player().Land - before; got != 5 {
		t.Fatalf("want 5 regions bought before hitting the cap, got %d", got)
	}
	out := f.out.String()
	if !strings.Contains(out, "Visit Advisors") {
		t.Errorf("expected Advisors to still be reachable after the cap blocked further buys, got:\n%s", out)
	}
}

func TestReachSystemMenuFromSpending(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "*00", menus.Spending) // '*' -> System Menu -> Quit -> Quit
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

func TestAboutFromGameMenu(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "I 0", menus.Game) // About -> pause -> Quit
	if err != nil {
		t.Fatalf("got %v", err)
	}
	out := f.out.String()
	for _, want := range []string{"Immortal Barons", "v" + game.Version, "andy5995.github.io/immortal-barons", "Barren Realms Elite"} {
		if !strings.Contains(out, want) {
			t.Errorf("About output missing %q: %q", want, out)
		}
	}
}

func TestAboutFromSystemMenu(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "I 0", menus.System) // About -> pause -> Quit (Back)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	out := f.out.String()
	for _, want := range []string{"Immortal Barons", "v" + game.Version, "andy5995.github.io/immortal-barons", "Barren Realms Elite"} {
		if !strings.Contains(out, want) {
			t.Errorf("About output missing %q: %q", want, out)
		}
	}
}

// testByKeyMenu builds a small menu exercising hotkey match, a heading
// (unselectable), and a hidden item.
func testByKeyMenu() *Menu {
	noop := func(session.Session, *ctx) Result { return Back }
	return &Menu{Items: []Item{
		{Key: 'C', Label: "Carriers", Do: noop},
		{Key: 'X', Label: "Card readers", Do: noop},
		{Label: "-- heading --"}, // Do == nil, not selectable
		{Key: 'H', Label: "Hidden Item", Do: noop, Hidden: func(*ctx) bool { return true }},
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
	if w.Player().Tax != 50 {
		t.Errorf("expected tax rate 50, got %d", w.Player().Tax)
	}
}

func TestInterBBSItemsHiddenUnlessEnabled(t *testing.T) {
	// The interplanetary actions live behind a single gated "InterPlanetary
	// Ops" entry on the War menu; it is hidden unless the game is InterBBS.
	f := &fakeSession{keys: []rune("0")}
	w := newWorld() // default: no IBBS, no league
	Run(f, w, BuildMenus().Attack)
	if strings.Contains(f.out.String(), "InterPlanetary Ops") {
		t.Error("InterPlanetary Ops should be hidden when IBBS/league is off")
	}

	f2 := &fakeSession{keys: []rune("0")}
	w2 := newWorld()
	w2.Config.IBBS = true
	Run(f2, w2, BuildMenus().Attack)
	if !strings.Contains(f2.out.String(), "InterPlanetary Ops") {
		t.Error("InterPlanetary Ops should appear when IBBS is enabled")
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
		{Key: '1', Label: "Buy", Do: func(session.Session, *ctx) Result { return Stay }},
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

func TestDefaultOnEnterPlaysWhenTurnsRemain(t *testing.T) {
	w := newWorld()
	w.Player().TurnsLeft = 5
	m := &Menu{Items: []Item{
		{Key: '1', Label: "Play", Do: func(session.Session, *ctx) Result { return Stay }},
		{Key: '0', Label: "Quit", Do: back},
	}}
	m.DefaultOnEnter = func(g *ctx) *Item {
		if g.Player().TurnsLeft > 0 {
			return m.byKey('1', g)
		}
		return m.byKey('0', g)
	}
	f := &fakeSession{keys: []rune{'\r'}}
	it, err := m.readChoice(f, w)
	if err != nil {
		t.Fatal(err)
	}
	if it == nil || it.Key != '1' {
		t.Errorf("Enter with turns remaining should select Play, got %v", it)
	}
	if !strings.Contains(f.out.String(), "Play") {
		t.Errorf("prompt should show Play, got %q", f.out.String())
	}
}

func TestDefaultOnEnterQuitsWhenNoTurns(t *testing.T) {
	w := newWorld()
	w.Player().TurnsLeft = 0
	m := &Menu{Items: []Item{
		{Key: '1', Label: "Play", Do: func(session.Session, *ctx) Result { return Stay }},
		{Key: '0', Label: "Quit", Do: back},
	}}
	m.DefaultOnEnter = func(g *ctx) *Item {
		if g.Player().TurnsLeft > 0 {
			return m.byKey('1', g)
		}
		return m.byKey('0', g)
	}
	f := &fakeSession{keys: []rune{'\r'}}
	it, err := m.readChoice(f, w)
	if err != nil {
		t.Fatal(err)
	}
	if it == nil || it.Key != '0' {
		t.Errorf("Enter with no turns left should select Quit, got %v", it)
	}
	if !strings.Contains(f.out.String(), "Quit") {
		t.Errorf("prompt should show Quit, got %q", f.out.String())
	}
}

func TestDefaultOnEnterNilHookIgnoresEnter(t *testing.T) {
	w := newWorld()
	m := &Menu{Items: []Item{{Key: '0', Label: "Quit", Do: back}}}
	f := &fakeSession{keys: []rune{'\r'}}
	if it, _ := m.readChoice(f, w); it != nil {
		t.Errorf("Enter with no DefaultOnEnter hook should select nothing, got %v", it)
	}
}

// TestSubmenusUseQuitNotReturn is the #62 regression check: every submenu
// that used to say "Return" now says "Quit" instead, with no leftover
// "Return" label anywhere in the tree.
func TestSubmenusUseQuitNotReturn(t *testing.T) {
	menus := BuildMenus()
	for name, m := range map[string]*Menu{
		"Sell":      menus.Sell,
		"Diplomacy": menus.Diplomacy,
		"Messages":  menus.Messages,
	} {
		f, _, err := run(t, "0", m)
		if err != nil {
			t.Fatalf("%s: got %v", name, err)
		}
		out := f.out.String()
		if strings.Contains(out, "Return") {
			t.Errorf("%s: expected no \"Return\" label, got:\n%s", name, out)
		}
		if !strings.Contains(out, "Quit") {
			t.Errorf("%s: expected a \"Quit\" label, got:\n%s", name, out)
		}
	}
}

// TestEnterActivatesSubmenuQuit confirms the DefaultOnEnter hook wired onto
// submenus (#62): pressing Enter alone shows and selects "Quit", the same
// way '0' does.
func TestEnterActivatesSubmenuQuit(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "\r", menus.Bank)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(f.out.String(), "Quit") {
		t.Errorf("expected Enter to show/select Quit, got:\n%s", f.out.String())
	}
}

// TestEnterExitsSpendingRespectsPreference confirms #62 didn't weaken the
// EnterExitsBuy preference: it still gates whether Enter exits the real
// Spending menu, independent of the DefaultOnEnter hook used elsewhere.
func TestEnterExitsSpendingRespectsPreference(t *testing.T) {
	menus := BuildMenus()

	w := newWorld()
	w.EnterExitsBuy = true
	if it, err := menus.Spending.readChoice(&fakeSession{keys: []rune{'\r'}}, w); err != nil {
		t.Fatal(err)
	} else if it == nil || it.Key != '0' {
		t.Errorf("pref on: expected Enter to select Spending's '0' Quit, got %v", it)
	}

	w2 := newWorld()
	w2.EnterExitsBuy = false
	if it, err := menus.Spending.readChoice(&fakeSession{keys: []rune{'\r'}}, w2); err != nil {
		t.Fatal(err)
	} else if it != nil {
		t.Errorf("pref off: expected Enter to select nothing, got %v", it)
	}
}

func TestComposeMessageSaves(t *testing.T) {
	f := &fakeSession{keys: []rune("hello\rworld\r/\rS")} // two lines, then /S
	text, send := composeMessage(f)
	if !send {
		t.Fatal("expected save")
	}
	if text != "hello\nworld" {
		t.Errorf("text = %q, want %q", text, "hello\nworld")
	}
}

func TestComposeMessageAborts(t *testing.T) {
	f := &fakeSession{keys: []rune("secret\r/\rA")} // one line, then /A
	_, send := composeMessage(f)
	if send {
		t.Error("expected abort (send=false)")
	}
}

func TestComposeMessageClearThenSave(t *testing.T) {
	f := &fakeSession{keys: []rune("oops\r/\rCkeep\r/\rS")} // clear, then one line, save
	text, send := composeMessage(f)
	if !send || text != "keep" {
		t.Errorf("after clear: text=%q send=%v, want %q true", text, send, "keep")
	}
}

func TestMenuChromeTranslated(t *testing.T) {
	menus := BuildMenus()
	f := &fakeSession{}
	w := newWorld()
	w.Player().Language = "de"
	draw(f, w, menus.Spending)
	out := f.out.String()
	for _, want := range []string{"Ausgabenmenü", "Soldaten", "Panzer"} {
		if !strings.Contains(out, want) {
			t.Errorf("German menu missing %q; output:\n%s", want, out)
		}
	}
}

func TestMenuChromeEnglishByDefault(t *testing.T) {
	menus := BuildMenus()
	f := &fakeSession{}
	w := newWorld() // Language "" => English
	draw(f, w, menus.Spending)
	if out := f.out.String(); !strings.Contains(out, "Troopers") {
		t.Errorf("default menu should be English; output:\n%s", out)
	}
}

func TestHelperTranslatesViaWrappedSession(t *testing.T) {
	w := newWorld()
	w.Player().Language = "de"
	f := &fakeSession{keys: []rune("\r")}
	ls := &langSession{Session: f, c: w}
	prompt(ls, "Troopers") // "Troopers" -> "Soldaten" in de.po
	if !strings.Contains(f.out.String(), "Soldaten") {
		t.Errorf("prompt through wrapped session not translated:\n%s", f.out.String())
	}
}

func TestPlainSessionStaysEnglish(t *testing.T) {
	f := &fakeSession{keys: []rune("\r")}
	prompt(f, "Troopers") // plain session -> no Lang() -> English
	if !strings.Contains(f.out.String(), "Troopers") {
		t.Errorf("plain session should be English:\n%s", f.out.String())
	}
}
