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

// fakeSession feeds a scripted key sequence and captures written output.
type fakeSession struct {
	keys []rune
	pos  int
	boot bool // when keys run out, return an idle boot (ErrSessionEnded) not io.EOF
	out  bytes.Buffer
}

func (f *fakeSession) ReadKey() (rune, error) {
	if f.pos >= len(f.keys) {
		if f.boot {
			return 0, session.ErrSessionEnded
		}
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
	w.AddHuman("tester", "Testland")
	c := &ctx{World: w, handle: "tester", Term: Term{UTF8: true}}
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

// Hotkeys are letters on some menus, and dispatch is case-folded
// (unicode.ToUpper in readChoice) — prove it with a lowercase letter. (This
// test once fed "0", a keystroke with no case, and could not fail.)
func TestHotkeysAreCaseInsensitive(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "g  0", menus.Game) // lowercase 'g' -> (G) Game Setup, dismiss its pauses, quit
	if err != nil {
		t.Fatalf("lowercase hotkey run failed: %v", err)
	}
	if !strings.Contains(f.out.String(), "Turns per day") {
		t.Errorf("lowercase 'g' should open Game Setup, got:\n%s", f.out.String())
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

// TestMenusDoNotClearOnDraw checks that drawing a menu never wipes the screen,
// so terminal scrollback is preserved as the player moves between menus.
func TestMenusDoNotClearOnDraw(t *testing.T) {
	menus := BuildMenus()
	for name, m := range map[string]*Menu{"Spending": menus.Spending, "Attack": menus.Attack} {
		f := &fakeSession{}
		draw(f, newWorld(), m)
		if strings.Contains(f.out.String(), ansi.Clear) {
			t.Errorf("%s menu draw should not emit ansi.Clear", name)
		}
	}
}

// TestBuyTroopersShowsConfirmationWithoutPause buys Troopers through the
// Spending menu and checks the purchase confirmation prints without a pause
// (no extra keypress needed) and the menu redraws in place afterward with
// the updated Owned count.
func TestBuyTroopersShowsConfirmationWithoutPause(t *testing.T) {
	menus := BuildMenus()
	w := newWorld()
	w.Player().Gold = 1_000_000
	beforeTroopers := w.Player().Troopers
	// '1' (Troopers) -> qty 5 -> Enter -> '0' (Quit Spending), with no pause
	// keypress between the purchase and the next menu prompt.
	f := &fakeSession{keys: []rune("15\r0")}
	if err := Run(f, w, menus.Spending); err != nil {
		t.Fatalf("got %v", err)
	}
	if got := w.Player().Troopers; got != beforeTroopers+5 {
		t.Errorf("expected Troopers %d, got %d", beforeTroopers+5, got)
	}
	out := f.out.String()
	if !strings.Contains(out, "Troopers purchased") {
		t.Errorf("expected purchase confirmation, got:\n%s", out)
	}
	if n := strings.Count(out, "Spending Menu"); n < 2 {
		t.Errorf("expected the Spending menu to redraw after the purchase, got %d draws:\n%s", n, out)
	}
}

func TestUnknownKeyIgnored(t *testing.T) {
	menus := BuildMenus()
	if _, _, err := run(t, "z0", menus.Game); err != nil {
		t.Fatalf("unknown key should be ignored, got %v", err)
	}
}

func TestBuyLandThroughSpendingMenu(t *testing.T) {
	menus := BuildMenus()
	f := &fakeSession{keys: []rune("6C5\r ")} // Buy Land -> C (Coastal, single key) -> qty 5
	w := newWorld()
	w.Player().Gold = 1_000_000
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
	w.Player().Gold = 1_000_000
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
	w.Player().Gold = 1_000_000
	before := w.Player().Land
	// Buy Land -> * (Advisors) -> pause -> C (Coastal) -> qty 4 -> pause ->
	// 0 (quit the buy loop) -> 0 (quit Spending menu).
	f := &fakeSession{keys: []rune("6* C4\r 00")}
	Run(f, w, menus.Spending)
	if w.Player().Land != before+4 {
		t.Errorf("expected land %d, got %d", before+4, w.Player().Land)
	}
	out := f.out.String()
	if !strings.Contains(out, ansi.FgBrightWhite+"Advisors") {
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
	if !strings.Contains(out, ansi.FgBrightWhite+"Advisors") {
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
	f := &fakeSession{keys: []rune("P7")} // P = Preferences, 7 = Auto-Feed Empire
	w := newWorld()
	w.AutoFeed = false // default is on, so toggle it off first and expect it back on
	if err := Run(f, w, menus.System); err != io.EOF {
		t.Fatalf("expected EOF after script, got %v", err)
	}
	if !w.AutoFeed {
		t.Error("Auto-feed should be ON after toggling")
	}
}

// Preference labels show BRE's bare Yes/No (right-aligned), not IB's old
// bracketed [ON]/[OFF] (verified against a live BRE Preferences capture).
func TestPreferenceLabelUsesBareYesNo(t *testing.T) {
	w := newWorld()
	label := onOff("Auto-Feed Empire", func(g *ctx) *bool { return &g.AutoFeed })

	w.AutoFeed = true
	on := label(w)
	if !strings.Contains(on, "Yes") || strings.ContainsAny(on, "[]") || strings.Contains(on, "ON") {
		t.Errorf("enabled pref should read bare 'Yes', got %q", on)
	}
	w.AutoFeed = false
	off := label(w)
	if !strings.Contains(off, "No") || strings.ContainsAny(off, "[]") {
		t.Errorf("disabled pref should read bare 'No', got %q", off)
	}
}

func TestAboutFromGameMenu(t *testing.T) {
	menus := BuildMenus()
	// Help lightbar: ? -> type 'a' (jumps to About) -> Enter (select) -> dismiss
	// pause -> q (leave help) -> 0 (Quit game).
	f, _, err := run(t, "?a\rzq0", menus.Game)
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
	// Help lightbar: ? -> type 'a' (jumps to About) -> Enter (select) -> dismiss
	// pause -> q (leave help) -> 0 (Back on System menu).
	f, _, err := run(t, "?a\rzq0", menus.System)
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
	// The title is centered within a colored rule line; per BRE the brackets are
	// the bright accent and the title itself is bright white, so the parts are
	// separated by color codes rather than a contiguous "[Colorful Menu]".
	if !strings.Contains(out, "Colorful Menu") {
		t.Errorf("expected the title in output, got:\n%s", out)
	}
	if !strings.Contains(out, ansi.FgBrightMagenta) {
		t.Error("expected menu color code in drawn output")
	}
	// The hotkey half of this test's name: the item's key renders in the
	// dim-accent parenthesized form, "(" + bright key + ")".
	if !strings.Contains(sgr.ReplaceAllString(out, ""), "(0) Return") {
		t.Errorf("expected the (0) hotkey beside its label, got:\n%s", out)
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
	// Ops" entry (9) on the opening menu; it is hidden unless the game is InterBBS.
	f := &fakeSession{}
	w := newWorld() // default: no IBBS, no league
	draw(f, w, BuildMenus().Game)
	if strings.Contains(f.out.String(), "InterPlanetary Ops") {
		t.Error("InterPlanetary Ops should be hidden when IBBS/league is off")
	}

	f2 := &fakeSession{}
	w2 := newWorld()
	w2.Config.IBBS = true
	draw(f2, w2, BuildMenus().Game)
	if !strings.Contains(f2.out.String(), "InterPlanetary Ops") {
		t.Error("InterPlanetary Ops should appear when IBBS is enabled")
	}
}

// TestInterPlanetaryMenuMatchesBRE checks the InterPlanetary Operations menu
// carries BRE's full item set (#75), not just the subset that existed
// before cross-planet trade/message/special-ops actions were wired in.
func TestInterPlanetaryMenuMatchesBRE(t *testing.T) {
	// "9" enters InterPlanetary Ops from the opening menu, "0" quits it, "0"
	// quits the opening menu.
	f := &fakeSession{keys: []rune("900")}
	w := newWorld()
	w.Config.IBBS = true
	if err := Run(f, w, BuildMenus().Game); err != nil {
		t.Fatalf("got %v", err)
	}
	out := f.out.String()
	for _, want := range []string{
		"View IPScores", "Terrorist Ops", "Send Trade Deal", "Create Group Attack",
		"Join Group Attack", "Indiv. Attack Force", "Send Message", "Special Operations",
		"SDI Program", "Doomer Kaboomer Ops", "Diplomacy List", "Spy Database",
		"Travel Times", "Visit Bank", "Help", "Quit",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("InterPlanetary Ops menu missing %q, got:\n%s", want, out)
		}
	}
}

// TestInterPlanetarySpecialOpsMenu checks BRE's "Special Operations" IP item
// opens the separate interplanetary Special Operations menu (cross-planet
// covert set), not the local Covert menu.
func TestInterPlanetarySpecialOpsMenu(t *testing.T) {
	// "9" enters InterPlanetary Ops, "8" selects Special Operations, then
	// "0" "0" "0" quits Special Ops, InterPlanetary Ops, and the opening menu.
	f := &fakeSession{keys: []rune("98000")}
	w := newWorld()
	w.Config.IBBS = true
	if err := Run(f, w, BuildMenus().Game); err != nil {
		t.Fatalf("got %v", err)
	}
	out := f.out.String()
	for _, want := range []string{"Special Operations", "Send SpyGuy", "Bomb Food Market"} {
		if !strings.Contains(out, want) {
			t.Errorf("IP Special Operations menu missing %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Covert Operations") {
		t.Errorf("IP Special Operations should NOT open the local Covert menu:\n%s", out)
	}
}

// TestPlanetaryTreatiesMatchesBRE checks the IP-ops "Diplomacy List" renders
// BRE's Planetary Treaties screen: each known BBS with its treaty status.
func TestPlanetaryTreatiesMatchesBRE(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")}
	w := newWorld()
	w.RemoteBoards = []game.RemoteBoard{{BoardID: "ZZap BBS", Date: "2026-07-02"}}
	planetaryTreaties(f, w)
	out := f.out.String()
	for _, want := range []string{"Planetary Treaties", "ZZap BBS", "None"} {
		if !strings.Contains(out, want) {
			t.Errorf("planetaryTreaties missing %q:\n%s", want, out)
		}
	}
}

// TestIPScoresMatchesBRE checks View IPScores renders BRE's cross-planet board:
// the Id/Empire Name/Territory/Score/Net Worth header and lettered (A) id.
func TestIPScoresMatchesBRE(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")}
	w := newWorld()
	w.RemoteBoards = []game.RemoteBoard{{BoardID: "ZZap BBS", Date: "2026-07-02",
		Scores: []game.RemoteScore{{Empire: "Iron Dominion", Land: 120, NetWorth: 50000, Score: 61000}}}}
	interbbsScores(f, w)
	out := f.out.String()
	for _, want := range []string{"InterBBS Scores", "Id", "Empire Name", "Territory", "Score", "Net Worth", "(A)", "Iron Dominion"} {
		if !strings.Contains(out, want) {
			t.Errorf("interbbsScores missing %q:\n%s", want, out)
		}
	}
}

// TestTravelTimesMatchesBRE checks the Travel Times screen uses BRE's header
// and its two units — hours under two days, days above — with "No Data" for a
// board that has never answered a probe. The figures are golden literals: 3.6
// hours is 0.15 days and 2.50 days is 2.5, both as BRE quantizes them.
func TestTravelTimesMatchesBRE(t *testing.T) {
	f := &fakeSession{keys: []rune(" ")}
	w := newWorld()
	w.RemoteBoards = []game.RemoteBoard{{BoardID: "ZZap BBS"}, {BoardID: "Far Star BBS"}}
	w.LeagueNodes = []game.LeagueNode{{Number: 5, Name: "Sky Rocket BBS"}}
	w.TravelTimes = map[string]float64{"ZZap BBS": 0.15, "Far Star BBS": 2.5}
	travelTimes(f, w)
	out := f.out.String()
	for _, want := range []string{
		"Average Turn Around Times to All BBSes",
		"ZZap BBS", "3.60 hours", "Far Star BBS", "2.50 days",
		"Sky Rocket BBS", "No Data",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("travelTimes missing %q:\n%s", want, out)
		}
	}
}

// A round trip is reported in the largest unit that still shows a figure. The
// seconds and minutes tiers are IB's own: a link answering in under a minute
// printed 0.00 hours, which reads as "never measured" rather than "fast".
func TestTurnaroundLabelUnits(t *testing.T) {
	const day = 1.0
	cases := []struct {
		days float64
		want string
	}{
		{0, "No Data"},
		{0.5 / (24 * 60 * 60), "1 second"}, // half a second still reads as a measurement
		{36.0 / (24 * 60 * 60), "36 seconds"},
		{90.0 / (24 * 60 * 60), "2 minutes"},
		{60.0 / (24 * 60 * 60), "1 minute"},
		{30.0 / (24 * 60), "30 minutes"},
		{3.0 / 24, "3.00 hours"},
		{3 * day, "3.00 days"},
	}
	f := &fakeSession{}
	for _, c := range cases {
		if got, _ := turnaroundLabel(f, c.days); got != c.want {
			t.Errorf("turnaroundLabel(%v days) = %q, want %q", c.days, got, c.want)
		}
	}
}

func TestHelpBrowseShowsControls(t *testing.T) {
	// lightbar: type 'c' (jumps to Controls) -> Enter -> Enter (first topic) ->
	// dismiss pause (z) -> q (back to categories) -> q (leave help)
	f := &fakeSession{keys: []rune("c\r\rzqq")}
	w := newWorld()
	helpBrowse(f, w)
	out := f.out.String()
	if !strings.Contains(out, "Controls") {
		t.Error("help browser should list the Controls category")
	}
	// A BODY phrase, not a topic title: titles are printed by the topic
	// lightbar before RenderANSI runs, so a title match passes with the render
	// call deleted.
	if !strings.Contains(out, "You do not press Enter to pick a menu item") {
		t.Error("help browser should render the topic body, not just list titles")
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
	// The real assertion is err == nil: with the DefaultOnEnter hook removed,
	// Enter is swallowed, the script runs dry, and Run returns io.EOF. (A bare
	// Contains("Quit") could never fail — the menu always draws that label.)
	_, _, err := run(t, "\r", menus.Bank)
	if err != nil {
		t.Fatalf("Enter should trigger the default Quit and end Run cleanly, got %v", err)
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
	// With the pref off, Enter is not the default, so it's ignored (not treated as
	// Quit) and the next real key is what selects: feed Enter then '0'.
	if it, err := menus.Spending.readChoice(&fakeSession{keys: []rune{'\r', '0'}}, w2); err != nil {
		t.Fatal(err)
	} else if it == nil || it.Key != '0' {
		t.Errorf("pref off: Enter should be ignored (not select Quit), then '0' selects; got %v", it)
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

// TestPromptClampAndConfirm verifies #9: a numeric entry over the max is clamped
// to the max ON SCREEN and requires a second Enter to commit, instead of silently
// clamping and committing in one keystroke.
func TestPromptClampAndConfirm(t *testing.T) {
	// Within max: a single Enter commits the typed value.
	if got := promptSuggested(&fakeSession{keys: []rune("5000\r")}, "Deposit", 0, 12899); got != 5000 {
		t.Errorf("within max: got %d, want 5000", got)
	}

	// Over max: first Enter clamps to the max on screen, a second Enter commits it.
	f := &fakeSession{keys: []rune("35000\r\r")}
	if got := promptSuggested(f, "Deposit", 0, 12899); got != 12899 {
		t.Errorf("over max, two Enters: got %d, want 12899 (clamped)", got)
	}
	if !strings.Contains(f.out.String(), "12899") {
		t.Errorf("over-max entry should show the clamped value on screen:\n%q", f.out.String())
	}

	// Over max with a single Enter must NOT commit: after the clamp the stream ends,
	// so it falls back to the suggested value (0), never the clamped 12899.
	if got := promptSuggested(&fakeSession{keys: []rune("35000\r")}, "Deposit", 0, 12899); got == 12899 {
		t.Errorf("one Enter must not commit an over-max entry, but it returned %d", got)
	}
}

// TestTitleBarPadsToRuleWidth guards #8: the blue title bar must be padded to the
// rule's DISPLAY width (rune count), not its byte length — the box-drawing rune is
// 3 bytes, so the byte-count bug stretched the bar to ~180 cols and wrapped.
func TestTitleBarPadsToRuleWidth(t *testing.T) {
	f := &fakeSession{}
	titleBar(f, "About")
	out := f.out.String()
	for _, esc := range []string{ansi.BgBlue, ansi.FgBrightWhite, ansi.Reset, "\n"} {
		out = strings.ReplaceAll(out, esc, "")
	}
	if got, want := len([]rune(out)), len([]rune(rule)); got != want {
		t.Errorf("title bar width = %d runes, want %d (rule display width); the byte-vs-rune bug gives ~180", got, want)
	}
}
