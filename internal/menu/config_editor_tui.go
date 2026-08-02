package menu

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// The tview Configuration Editor: the polished, tabbed form used when -reset
// runs on a real terminal. Fields are grouped into four tabs so the whole
// ruleset never overflows a screen (issue #7); a ★ marks the league-ruleset
// fields (the ones a Coordinator broadcasts with -league-config). When stdin is
// not a terminal (piped -reset, CI) the caller falls back to the line-based
// runConfigEditor, which shares the same clamping/validation intent.

var configTabTitles = []string{"Game & Timing", "Economy", "Military & Combat", "Caps & Node"}

// star marks a label as a league-ruleset field; noStar pads the rest so labels
// line up whether or not they carry the mark.
const (
	star   = "★ "
	noStar = "  "
)

func mark(leagueRuleset bool) string {
	if leagueRuleset {
		return star
	}
	return noStar
}

// fieldBgColor darkens the input background; tview's default (ColorBlue) leaves
// white field text washed out (Andy's contrast note).
var fieldBgColor = tcell.NewHexColor(0x0d2b50)

// Per-field help shown in the bottom pane when a field is focused. The facts are
// BRE's (rates, caps, behaviour); the wording is our own — BRE's help text is
// copyrighted, so it is paraphrased, not copied.
const (
	defaultHelp         = "Move with Tab / Shift-Tab; each field's help appears here."
	helpTurnsPerDay     = "How many turns each player may take per day."
	helpProtection      = "New-realm protection: turns a fresh empire cannot attack and cannot be attacked, giving newcomers time to get established."
	helpGameLength      = "Days before the league ends and resets. 0 keeps it running with no scheduled end."
	helpIdleRemove      = "Days a realm may go unplayed before it is removed from the game. Also removes a realm whose owner created it and never played a turn. 0 never removes anything."
	helpStartDate       = "The game begins on this date; until then, daily maintenance is paused. Blank starts it right away."
	helpJoinDate        = "New players may not join after this date. Blank leaves joining open."
	helpAICount         = "Number of computer-run empires seeded when the world is reset. A league board never gets any, whatever this says."
	helpInitialLand     = "Land for sale on the market when the game resets."
	helpLandPerDay      = "Land added to the market each day."
	helpInterest        = "Bank interest paid over 10 days, credited at the end of each turn. The maximum (200) works out to 20% per day. High rates let banking dominate the game, so a low value (under about 40) keeps play balanced."
	helpStdInvest       = "The baseline investment return over 10 days. Unless Steady Investment Rate is on, the real rate drifts each day around this value based on how much everyone is investing. The maximum (100) is 10% per day; a low value (under about 50) keeps the economy balanced."
	helpSteadyInvest    = "When on, the investment return stays fixed at the Standard Investment Rate instead of drifting each day with total investment."
	helpMaxTax          = "The highest tax rate a player is allowed to set."
	helpPlanetaryTax    = "The crown tax the Queen Royale takes from each turn's gold income, as a whole percent (default 5, maximum 20). The gold leaves the economy entirely — nobody receives it."
	helpFoodUnlimited   = "When on, the food market never runs short — its daily supply is unlimited."
	helpBuyMilitary     = "Whether players may buy military units: freely (Yes), not at all (No), or a limited amount each day (Limited)."
	helpMaintCosts      = "Upkeep cost for regions and forces: High, Medium, Low, or None."
	helpTradeCosts      = "Cost of trade deals: High, Medium, Low, or None."
	helpRegionCosts     = "Price of buying regions: High, Medium, Low, or None."
	helpAttackDamage    = "How much a conventional attack destroys on both sides: High, Medium, or Low."
	helpAttackRewards   = "How much land and goods the winner of an attack gains: High, Medium, or Low."
	helpSlappenheimer   = "How R5-Slappenheimer missiles behave when fired."
	helpMaxAttacks      = "The most conventional attacks a player may launch in one day. 0 means no limit."
	helpMaxGroupAttacks = "The most group (interplanetary) attacks a player may lead or join in one day. 0 means no limit."
	helpMaxTerrorOps    = "The most terrorist operations a player may launch in one day. 0 means no limit."
	helpMaxBombingOps   = "The most bombing operations a player may launch in one day. 0 means no limit."
	helpLostForcesDays  = "How long forces sent to another board wait for a result before they are given back. Packets go missing; this stops an army being lost for good. 0 means they never come back."
	helpAttackCosts     = "How much gold an attack costs to launch."
	helpTerrorCosts     = "How much gold a terrorist operation costs to launch."
	helpBombingOps      = "Whether Bomb Enemy Targets is offered at all."
	helpMissileOps      = "Whether nuclear, chemical and biological strikes are offered at all."
	helpDoomerKaboomer  = "Whether the Doomer Kaboomer doomsday weapon may be built."
	helpMaxRegions      = "The most regions a single player may own."
	helpMaxPlayers      = "The most human empires allowed on this board. 0 means no limit."
	helpIBBS            = "Take part in inter-BBS play across multiple boards. This turns on the interplanetary menus."
	helpBoardID         = "The name this board uses in inter-BBS packets."
	helpIdleTimeout     = "End a session after this many seconds with no keypress, freeing the shared world lock. 0 never times out."
	helpIdleWarnings    = "How many idle warnings a session receives before it is disconnected."
)

// configTUI holds the tview app and the widget read-back closures. Each field's
// widget appends a binder that reads its current value (clamped/validated) back
// into a Config on Save, so collect() needs no per-field wiring at the call site
// and is testable without a live screen.
type configTUI struct {
	app       *tview.Application
	pages     *tview.Pages
	tabs      *tview.TextView
	help      *tview.TextView // right-hand pane; shows the focused field's help
	forms     []*tview.Form
	root      tview.Primitive // the main layout, restored after a modal
	world     *game.World
	base      game.Config
	binders   []func(*game.Config)
	resetters []func() // restore each widget to its opening (default) value
	aligns    []rangeField
	cur       int
	saved     bool
	modalUp   bool // a confirm/error modal owns the screen; global keys stand down
}

// rangeField remembers an int field so alignRanges can right-align its [lo-hi]
// hint against the form's (fixed) input column once every field is known.
type rangeField struct {
	form  *tview.Form
	field *tview.InputField
	base  string // label without the range
	rng   string // "[lo-hi]"
}

// ConfigEditorTUI runs the tabbed Configuration Editor on the process's real
// terminal, returning whether the sysop saved. A screen-init failure (no usable
// terminal) is returned so the caller can fall back to the line editor.
func ConfigEditorTUI(w *game.World) (saved bool, err error) {
	t := newConfigTUI(w)
	screen, err := tcell.NewScreen()
	if err != nil {
		return false, err
	}
	t.app.SetScreen(screen)
	if err := t.app.SetRoot(rootLayout(t), true).EnableMouse(true).Run(); err != nil {
		return false, err
	}
	return t.saved, nil
}

func newConfigTUI(w *game.World) *configTUI {
	t := &configTUI{app: tview.NewApplication(), world: w, base: w.Config}
	c := w.Config

	costOpts := []string{"High", "Medium", "Low", "None"}
	costVals := []game.Level{game.High, game.Medium, game.Low, game.None}
	dmgOpts := []string{"High", "Medium", "Low"}
	dmgVals := []game.Level{game.High, game.Medium, game.Low}
	buyOpts := []string{"Yes", "No", "Limited"}
	buyVals := []game.BuyMode{game.BuyYes, game.BuyNo, game.BuyLimited}
	slapOpts := []string{"User Select/Original", "None/Disabled", "Random", "Constant"}
	slapVals := []game.SlappenheimerMode{game.SlappenheimerUserSelect, game.SlappenheimerNone, game.SlappenheimerRandom, game.SlappenheimerConstant}

	timing := tview.NewForm()
	t.addInt(timing, true, "Turns per day", helpTurnsPerDay, c.TurnsPerDay, 1, game.MaxTurnsPerDay, func(c *game.Config, n int) { c.TurnsPerDay = n })
	t.addInt(timing, true, "Turns of Protection", helpProtection, c.ProtectionTurns, 0, game.MaxProtectionTurns, func(c *game.Config, n int) { c.ProtectionTurns = n })
	t.addInt(timing, true, "Game length (days, 0=endless)", helpGameLength, c.GameLength, 0, 100000, func(c *game.Config, n int) { c.GameLength = n })
	t.addInt(timing, true, "Remove idle realms (days, 0=never)", helpIdleRemove, c.IdleDaysRemove, 0, game.MaxIdleDaysRemove, func(c *game.Config, n int) { c.IdleDaysRemove = n })
	t.addDate(timing, true, "Game Start Date (YYYY-MM-DD)", helpStartDate, c.GameStartDate, func(c *game.Config, v string) { c.GameStartDate = v })
	t.addDate(timing, true, "Join Cutoff Date (YYYY-MM-DD)", helpJoinDate, c.JoinDate, func(c *game.Config, v string) { c.JoinDate = v })
	t.addInt(timing, false, "AI empires", helpAICount, c.AICount, 0, 5, func(c *game.Config, n int) { c.AICount = n })

	econ := tview.NewForm()
	t.addInt(econ, true, "Initial Market Land", helpInitialLand, c.InitialMarketLand, 0, game.MaxInitialMarketLand, func(c *game.Config, n int) { c.InitialMarketLand = n })
	t.addInt(econ, true, "Land Created / Day", helpLandPerDay, c.LandPerDay, 0, game.MaxLandPerDay, func(c *game.Config, n int) { c.LandPerDay = n })
	t.addInt(econ, true, "Interest Rate", helpInterest, c.InterestRate, 0, game.MaxBankInterest, func(c *game.Config, n int) { c.InterestRate = n })
	t.addInt(econ, true, "Standard Investment Rate", helpStdInvest, c.StdInvestRate, 0, game.MaxStdInvestRate, func(c *game.Config, n int) { c.StdInvestRate = n })
	t.addBool(econ, true, "Steady Investment Rate", helpSteadyInvest, c.SteadyInvest, func(c *game.Config, b bool) { c.SteadyInvest = b })
	t.addInt(econ, true, "Max Tax Rate", helpMaxTax, c.MaxTaxRate, 0, game.MaxPlayerTaxRate, func(c *game.Config, n int) { c.MaxTaxRate = n })
	t.addInt(econ, true, "Planetary Tax Rate (%)", helpPlanetaryTax, c.PlanetaryTaxRate, 0, game.MaxPlanetaryTaxRate, func(c *game.Config, n int) { c.PlanetaryTaxRate = n })
	t.addBool(econ, true, "Food Unlimited", helpFoodUnlimited, c.FoodUnlimited, func(c *game.Config, b bool) { c.FoodUnlimited = b })

	mil := tview.NewForm()
	addChoice(t, mil, true, "Buy Military", helpBuyMilitary, buyOpts, buyVals, c.BuyMilitary, func(c *game.Config, v game.BuyMode) { c.BuyMilitary = v })
	addChoice(t, mil, true, "Maintenance Costs", helpMaintCosts, costOpts, costVals, c.MaintCosts, func(c *game.Config, v game.Level) { c.MaintCosts = v })
	addChoice(t, mil, true, "Trade Deal Costs", helpTradeCosts, costOpts, costVals, c.TradeCosts, func(c *game.Config, v game.Level) { c.TradeCosts = v })
	addChoice(t, mil, true, "Region Costs", helpRegionCosts, costOpts, costVals, c.RegionCosts, func(c *game.Config, v game.Level) { c.RegionCosts = v })
	addChoice(t, mil, true, "Attack Damage", helpAttackDamage, dmgOpts, dmgVals, c.AttackDamage, func(c *game.Config, v game.Level) { c.AttackDamage = v })
	addChoice(t, mil, true, "Attack Rewards", helpAttackRewards, dmgOpts, dmgVals, c.AttackRewards, func(c *game.Config, v game.Level) { c.AttackRewards = v })
	addChoice(t, mil, true, "R5-Slappenheimer Handling", helpSlappenheimer, slapOpts, slapVals, c.SlappenheimerHandling, func(c *game.Config, v game.SlappenheimerMode) { c.SlappenheimerHandling = v })
	t.addInt(mil, true, "Max Individual Attacks/Day (0=unlimited)", helpMaxAttacks, c.MaxIndividualAttacks, 0, 100, func(c *game.Config, n int) { c.MaxIndividualAttacks = n })
	t.addInt(mil, true, "Max Group Attacks/Day (0=unlimited)", helpMaxGroupAttacks, c.MaxGroupAttacks, 0, 100, func(c *game.Config, n int) { c.MaxGroupAttacks = n })
	t.addInt(mil, true, "Max Terrorist Ops/Day (0=unlimited)", helpMaxTerrorOps, c.MaxTerrorOps, 0, 100, func(c *game.Config, n int) { c.MaxTerrorOps = n })
	t.addInt(mil, true, "Max Bombing Ops/Day (0=unlimited)", helpMaxBombingOps, c.MaxBombingOps, 0, 100, func(c *game.Config, n int) { c.MaxBombingOps = n })
	t.addInt(mil, true, "Days before lost forces return (0=never)", helpLostForcesDays, c.LostForcesDays, 0, game.MaxLostForcesDays, func(c *game.Config, n int) { c.LostForcesDays = n })
	addChoice(t, mil, true, "Attack Costs", helpAttackCosts, costOpts, costVals, c.AttackCosts, func(c *game.Config, v game.Level) { c.AttackCosts = v })
	addChoice(t, mil, true, "Terrorism Costs", helpTerrorCosts, costOpts, costVals, c.TerrorCosts, func(c *game.Config, v game.Level) { c.TerrorCosts = v })
	t.addBool(mil, true, "Bombing Ops", helpBombingOps, c.BombingOps, func(c *game.Config, b bool) { c.BombingOps = b })
	t.addBool(mil, true, "Missile Ops", helpMissileOps, c.MissileOps, func(c *game.Config, b bool) { c.MissileOps = b })
	t.addBool(mil, true, "Doomer Kaboomer", helpDoomerKaboomer, c.DoomerKaboomer, func(c *game.Config, b bool) { c.DoomerKaboomer = b })

	caps := tview.NewForm()
	t.addInt(caps, true, "Max Purchasable Regions", helpMaxRegions, c.MaxRegions, 0, game.MaxPurchasableRegions, func(c *game.Config, n int) { c.MaxRegions = n })
	t.addInt(caps, true, "Max Players Per BBS (0=unlimited)", helpMaxPlayers, c.MaxPlayers, 0, 100000, func(c *game.Config, n int) { c.MaxPlayers = n })
	t.addBool(caps, false, "Inter-BBS play", helpIBBS, c.IBBS, func(c *game.Config, b bool) { c.IBBS = b })
	t.addText(caps, false, "Board ID", helpBoardID, c.BoardID, func(c *game.Config, v string) { c.BoardID = v })
	t.addInt(caps, false, "Idle timeout (sec, 0=never)", helpIdleTimeout, c.IdleTimeoutSecs, 0, 86400, func(c *game.Config, n int) { c.IdleTimeoutSecs = n })
	t.addInt(caps, false, "Idle warnings before boot", helpIdleWarnings, c.MaxIdleWarnings, 1, 100, func(c *game.Config, n int) { c.MaxIdleWarnings = n })

	t.forms = []*tview.Form{timing, econ, mil, caps}
	t.pages = tview.NewPages()
	for i, f := range t.forms {
		f.AddButton("Save", t.save).AddButton("Cancel", t.cancel).AddButton("Load Defaults", t.loadDefaults)
		// No blank line between fields, so all fields + the buttons fit above the
		// help pane on an 80x25 screen.
		f.SetItemPadding(0)
		f.SetBorder(true).SetTitle(" " + configTabTitles[i] + " ")
		f.SetFieldBackgroundColor(fieldBgColor).
			SetFieldTextColor(tcell.ColorWhite).
			SetLabelColor(tcell.ColorSkyblue)
		// Orange normally, bright yellow when focused, so the active button is
		// unmistakable.
		f.SetButtonBackgroundColor(tcell.ColorOrange).
			SetButtonTextColor(tcell.ColorBlack).
			SetButtonActivatedStyle(tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack).Bold(true))
		t.pages.AddPage(strconv.Itoa(i), f, true, i == 0)
	}
	t.alignRanges()
	return t
}

// rootLayout assembles the header/tab-bar/pages/footer and installs the global
// key handler (onKey).
func rootLayout(t *configTUI) tview.Primitive {
	header := tview.NewTextView().SetTextAlign(tview.AlignCenter).SetText("Immortal Barons — Configuration Editor")
	t.tabs = tview.NewTextView().SetDynamicColors(true).SetRegions(true)
	// Clicking a tab highlights its region; use that as a click handler, then clear
	// the highlight so renderTabs' colour stays the source of the active-tab look.
	t.tabs.SetHighlightedFunc(func(added, _, _ []string) {
		if len(added) == 0 {
			return
		}
		if i, err := strconv.Atoi(added[0]); err == nil {
			t.tabs.Highlight()
			t.gotoTab(i)
		}
	})
	t.renderTabs()
	// Full-width help along the bottom (BRE puts config help full-width too); a
	// right-hand pane would starve the forms at 80 columns.
	t.help = tview.NewTextView().SetWordWrap(true).SetDynamicColors(true)
	t.help.SetBorder(true).SetTitle(" Help ")
	t.showHelp("")

	footer := tview.NewTextView().SetDynamicColors(true).SetWordWrap(true).
		SetText(" F1–F4 / Ctrl-N,P: tabs · Tab/Shift-Tab: move · ↑↓ or click: change a choice · Ctrl-S: save · Esc: cancel · ★ = league ruleset")

	t.app.SetInputCapture(t.onKey)

	t.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(t.tabs, 1, 0, false).
		AddItem(t.pages, 0, 1, true).
		AddItem(t.help, 5, 0, false).
		AddItem(footer, 2, 0, false)
	return t.root
}

// showHelp puts text (or a default when empty) in the bottom help pane.
func (t *configTUI) showHelp(text string) {
	if t.help == nil {
		return
	}
	if text == "" {
		text = defaultHelp
	}
	t.help.SetText(text)
}

// onKey is the global key handler. Tabs switch via F1–F4, Ctrl-N/P, or a mouse
// click; Tab/Shift-Tab move within the current form (tview's own wrap). Up/Down
// cycle a focused choice field but are otherwise inert (they don't navigate
// between fields, and inertness dodges tview's InputField eating ↓ for
// autocomplete).
func (t *configTUI) onKey(ev *tcell.EventKey) *tcell.EventKey {
	if t.modalUp {
		return ev // a modal owns the screen; let its buttons handle keys
	}

	n := len(t.forms)
	switch ev.Key() {
	case tcell.KeyF1:
		t.gotoTab(0)
	case tcell.KeyF2:
		t.gotoTab(1)
	case tcell.KeyF3:
		t.gotoTab(2)
	case tcell.KeyF4:
		t.gotoTab(3)
	case tcell.KeyCtrlN:
		t.gotoTab((t.cur + 1) % n)
	case tcell.KeyCtrlP:
		t.gotoTab((t.cur - 1 + n) % n)
	case tcell.KeyCtrlS:
		t.save()
	case tcell.KeyEscape:
		t.confirmCancel()
	case tcell.KeyUp, tcell.KeyDown:
		if _, isCycle := t.app.GetFocus().(*cycleField); isCycle {
			return ev // let the choice field cycle its options
		}
		// otherwise arrows don't navigate between fields
	default:
		return ev // Tab/Shift-Tab (and cycle-field keys) go to the focused field
	}
	return nil
}

func (t *configTUI) renderTabs() {
	var sb strings.Builder
	for i, name := range configTabTitles {
		if i == t.cur {
			fmt.Fprintf(&sb, ` ["%d"][black:white] F%d %s [-:-][""]`, i, i+1, name)
		} else {
			fmt.Fprintf(&sb, ` ["%d"][white] F%d %s [-][""]`, i, i+1, name)
		}
	}
	t.tabs.SetText(sb.String())
}

// gotoTab shows tab i and focuses its first field.
func (t *configTUI) gotoTab(i int) {
	t.cur = i
	t.pages.SwitchToPage(strconv.Itoa(i))
	t.renderTabs()
	t.forms[i].SetFocus(0)
	t.app.SetFocus(t.forms[i])
}

// collect applies every widget's read-back closure to a copy of the config the
// editor opened with, so a field left invalid (a bad date) keeps its original.
func (t *configTUI) collect() game.Config {
	c := t.base
	for _, b := range t.binders {
		b(&c)
	}
	return c
}

func (t *configTUI) save() {
	c := t.collect()
	if err := store.SaveConfig(c); err != nil {
		t.showError(err)
		return
	}
	t.world.Config = c
	t.saved = true
	t.app.Stop()
}

func (t *configTUI) cancel() {
	t.saved = false
	t.app.Stop()
}

// loadDefaults restores every field to the value the editor opened with. Under
// -reset that opening value is game.DefaultConfig(), so this is "load defaults".
func (t *configTUI) loadDefaults() {
	for _, r := range t.resetters {
		r()
	}
}

func (t *configTUI) showError(err error) {
	t.showModal(tview.NewModal().
		SetText("Could not save config.json:\n" + err.Error()).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(int, string) { t.dismissModal() }))
}

// confirmCancel guards Esc: discarding edits takes a deliberate second press, so
// a stray Esc doesn't throw away a reset the sysop was in the middle of. "Keep
// editing" is the default (Enter), so the safe choice is the easy one.
func (t *configTUI) confirmCancel() {
	t.showModal(tview.NewModal().
		SetText("Discard changes and exit the editor?").
		AddButtons([]string{"Keep editing", "Discard"}).
		SetDoneFunc(func(_ int, label string) {
			if label == "Discard" {
				t.cancel()
				return
			}
			t.dismissModal()
		}))
}

func (t *configTUI) showModal(m tview.Primitive) {
	t.modalUp = true
	t.app.SetRoot(m, true)
}

func (t *configTUI) dismissModal() {
	t.modalUp = false
	t.app.SetRoot(t.root, true)
}

// addInt adds a digits-only field that clamps to [lo, hi] on read-back.
func (t *configTUI) addInt(form *tview.Form, leagueRuleset bool, label, help string, val, lo, hi int, set func(*game.Config, int)) {
	base := mark(leagueRuleset) + label
	f := tview.NewInputField().
		SetLabel(base).
		SetText(strconv.Itoa(val)).
		SetFieldWidth(12).
		SetAcceptanceFunc(tview.InputFieldInteger)
	f.SetFocusFunc(func() { t.showHelp(help) })
	// Clamp to [lo, hi] when the field loses focus, so an out-of-range entry
	// visibly snaps to the bound (belt-and-suspenders with the Save-time clamp).
	f.SetDoneFunc(func(tcell.Key) {
		n, _ := strconv.Atoi(strings.TrimSpace(f.GetText()))
		f.SetText(strconv.Itoa(clampInt(n, lo, hi)))
	})
	form.AddFormItem(zebra(form, f))
	openText := strconv.Itoa(val)
	t.resetters = append(t.resetters, func() { f.SetText(openText) })
	// The [lo-hi] hint is right-aligned to the input column by alignRanges.
	t.aligns = append(t.aligns, rangeField{form: form, field: f, base: base, rng: fmt.Sprintf("[%d-%d]", lo, hi)})
	t.binders = append(t.binders, func(c *game.Config) {
		n, _ := strconv.Atoi(strings.TrimSpace(f.GetText()))
		set(c, clampInt(n, lo, hi))
	})
}

// alignRanges right-aligns every int field's [lo-hi] hint so it ends at the
// form's input column, giving a consistent one-space gap to the field. tview's
// vertical Form pins all inputs to maxLabelWidth+1, so the hint can hug either
// the label or the input but not both; the input side is the useful one.
func (t *configTUI) alignRanges() {
	for _, form := range t.forms {
		w := 0
		for i := 0; i < form.GetFormItemCount(); i++ {
			if lw := tview.TaggedStringWidth(form.GetFormItem(i).GetLabel()); lw > w {
				w = lw
			}
		}
		for _, a := range t.aligns {
			if a.form == form {
				if need := tview.TaggedStringWidth(a.base) + 1 + tview.TaggedStringWidth(a.rng); need > w {
					w = need
				}
			}
		}
		for _, a := range t.aligns {
			if a.form != form {
				continue
			}
			pad := w - tview.TaggedStringWidth(a.base) - tview.TaggedStringWidth(a.rng)
			if pad < 1 {
				pad = 1
			}
			a.field.SetLabel(a.base + strings.Repeat(" ", pad) + a.rng)
		}
	}
}

func (t *configTUI) addBool(form *tview.Form, leagueRuleset bool, label, help string, val bool, set func(*game.Config, bool)) {
	f := tview.NewCheckbox().SetLabel(mark(leagueRuleset) + label).SetChecked(val)
	f.SetFocusFunc(func() { t.showHelp(help) })
	form.AddFormItem(zebra(form, f))
	t.resetters = append(t.resetters, func() { f.SetChecked(val) })
	t.binders = append(t.binders, func(c *game.Config) { set(c, f.IsChecked()) })
}

func (t *configTUI) addText(form *tview.Form, leagueRuleset bool, label, help, val string, set func(*game.Config, string)) {
	f := tview.NewInputField().SetLabel(mark(leagueRuleset) + label).SetText(val).SetFieldWidth(16)
	f.SetFocusFunc(func() { t.showHelp(help) })
	form.AddFormItem(zebra(form, f))
	t.resetters = append(t.resetters, func() { f.SetText(val) })
	t.binders = append(t.binders, func(c *game.Config) { set(c, strings.TrimSpace(f.GetText())) })
}

// addDate adds an ISO-date field; an empty value clears it, a malformed one is
// ignored (keeps the opening value), matching the line editor's promptDate.
func (t *configTUI) addDate(form *tview.Form, leagueRuleset bool, label, help, val string, set func(*game.Config, string)) {
	f := tview.NewInputField().SetLabel(mark(leagueRuleset) + label).SetText(val).SetFieldWidth(14)
	f.SetFocusFunc(func() { t.showHelp(help) })
	form.AddFormItem(zebra(form, f))
	t.resetters = append(t.resetters, func() { f.SetText(val) })
	t.binders = append(t.binders, func(c *game.Config) {
		v := strings.TrimSpace(f.GetText())
		if v == "" || validISODate(v) {
			set(c, v)
		}
	})
}

// addChoice adds a cycle-through-options field (no dropdown popup — for a few
// options, cycling in place reads better). A free function because Go methods
// cannot carry type parameters.
func addChoice[T comparable](t *configTUI, form *tview.Form, leagueRuleset bool, label, help string, opts []string, vals []T, cur T, set func(*game.Config, T)) {
	idx := 0
	for i, v := range vals {
		if v == cur {
			idx = i
		}
	}
	cf := newCycleField(mark(leagueRuleset)+label, opts, idx, stripeBg(form))
	cf.Box.SetFocusFunc(func() { t.showHelp(help) })
	form.AddFormItem(cf)
	t.resetters = append(t.resetters, func() { cf.index = idx })
	t.binders = append(t.binders, func(c *game.Config) {
		if cf.index >= 0 && cf.index < len(vals) {
			set(c, vals[cf.index])
		}
	})
}

// cycleField is a choose-one-of-N form field: it shows the current option and
// cycles with ↑/↓ (also ←/→ or Space), or a click while already focused. No
// popup — for a handful of options that beats a dropdown. It carries its own
// zebra background (SetFormAttributes ignores the Form's uniform field colours)
// and a brighter background while focused.
type cycleField struct {
	*tview.Box
	label      string
	labelWidth int
	labelColor tcell.Color
	options    []string
	index      int
	fieldBg    tcell.Color
	finished   func(tcell.Key)
	wasFocused bool // recorded on mouse-down so the first click only selects
}

func newCycleField(label string, options []string, index int, fieldBg tcell.Color) *cycleField {
	return &cycleField{
		Box:        tview.NewBox(),
		label:      label,
		options:    options,
		index:      index,
		fieldBg:    fieldBg,
		labelColor: tcell.ColorSkyblue,
	}
}

func (c *cycleField) GetLabel() string { return c.label }

func (c *cycleField) SetFormAttributes(labelWidth int, labelColor, bgColor, _, _ tcell.Color) tview.FormItem {
	c.labelWidth = labelWidth
	c.labelColor = labelColor
	c.SetBackgroundColor(bgColor)
	return c
}

func (c *cycleField) GetFieldWidth() int {
	w := 0
	for _, o := range c.options {
		if len(o) > w {
			w = len(o)
		}
	}
	return w + 2
}

func (c *cycleField) GetFieldHeight() int                              { return 1 }
func (c *cycleField) SetDisabled(bool) tview.FormItem                  { return c }
func (c *cycleField) SetFinishedFunc(h func(tcell.Key)) tview.FormItem { c.finished = h; return c }

func (c *cycleField) cycle(delta int) {
	n := len(c.options)
	c.index = (c.index + delta + n) % n
}

func (c *cycleField) Draw(screen tcell.Screen) {
	c.Box.DrawForSubclass(screen, c)
	x, y, width, height := c.GetInnerRect()
	if height < 1 || width <= 0 {
		return
	}
	labelW := c.labelWidth
	if labelW > width {
		labelW = width
	}
	tview.Print(screen, c.label, x, y, labelW, tview.AlignLeft, c.labelColor)
	fx, fw := x+labelW, width-labelW
	if fw <= 0 {
		return
	}
	bg := c.fieldBg
	if c.HasFocus() {
		bg = cycleFocusBg
	}
	st := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(bg)
	for i := 0; i < fw; i++ {
		screen.SetContent(fx+i, y, ' ', nil, st)
	}
	for i, r := range []rune(c.options[c.index]) {
		if i+1 >= fw {
			break
		}
		screen.SetContent(fx+1+i, y, r, nil, st)
	}
}

func (c *cycleField) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return c.WrapInputHandler(func(event *tcell.EventKey, _ func(tview.Primitive)) {
		switch key := event.Key(); key {
		case tcell.KeyDown, tcell.KeyRight:
			c.cycle(1)
		case tcell.KeyUp, tcell.KeyLeft:
			c.cycle(-1)
		case tcell.KeyRune:
			if event.Rune() == ' ' {
				c.cycle(1)
			}
		case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyEnter, tcell.KeyEscape:
			if c.finished != nil {
				c.finished(key)
			}
		}
	})
}

func (c *cycleField) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return c.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		x, y := event.Position()
		if !c.InRect(x, y) {
			return false, nil
		}
		switch action {
		case tview.MouseLeftDown:
			c.wasFocused = c.HasFocus() // a click that first focuses shouldn't also cycle
			setFocus(c)
			return true, nil
		case tview.MouseLeftClick:
			if c.wasFocused {
				c.cycle(1)
			}
			return true, nil
		}
		return false, nil
	})
}

// styledField wraps a form field to force its field foreground/background,
// overriding the Form's uniform per-field styling (it repaints every field the
// same colour on each Draw). Used for the yellow dropdowns and the zebra-striped
// inputs, which stack with no blank line between them.
type styledField struct {
	tview.FormItem
	fg, bg tcell.Color
}

func (s *styledField) SetFormAttributes(labelWidth int, labelColor, bgColor, _, _ tcell.Color) tview.FormItem {
	s.FormItem.SetFormAttributes(labelWidth, labelColor, bgColor, s.fg, s.bg)
	return s
}

// stripeAltColor is the darker zebra row that alternates with fieldBgColor so
// stacked fields (no blank line between them) stay legible; cycleFocusBg is the
// brighter background a choice field shows while focused.
var (
	stripeAltColor = tcell.NewHexColor(0x1f242b)
	cycleFocusBg   = tcell.NewHexColor(0x2d6cb0)
)

// stripeBg is the zebra background for the next row added to form.
func stripeBg(form *tview.Form) tcell.Color {
	if form.GetFormItemCount()%2 == 1 {
		return stripeAltColor
	}
	return fieldBgColor
}

// zebra wraps an input field with the striped background for its row.
func zebra(form *tview.Form, f tview.FormItem) *styledField {
	return &styledField{FormItem: f, fg: tcell.ColorWhite, bg: stripeBg(form)}
}

func clampInt(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

func validISODate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}
