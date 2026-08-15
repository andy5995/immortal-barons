package menu

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

// cycleCost steps a cost knob through High -> Medium -> Low -> None (BRE
// "[H,M,L,N]").
func cycleCost(l game.Level) game.Level {
	switch l {
	case game.High:
		return game.Medium
	case game.Medium:
		return game.Low
	case game.Low:
		return game.None
	default:
		return game.High
	}
}

// cycleHML steps a damage/reward knob through High -> Medium -> Low (no None).
func cycleHML(l game.Level) game.Level {
	switch l {
	case game.High:
		return game.Medium
	case game.Medium:
		return game.Low
	default:
		return game.High
	}
}

// cycleBuy steps Buy Military through Yes -> No -> Limited.
func cycleBuy(b game.BuyMode) game.BuyMode {
	switch b {
	case game.BuyYes:
		return game.BuyNo
	case game.BuyNo:
		return game.BuyLimited
	default:
		return game.BuyYes
	}
}

// cycleSlappenheimer steps Slappenheimer Handling through its four settings.
func cycleSlappenheimer(m game.SlappenheimerMode) game.SlappenheimerMode {
	switch m {
	case game.SlappenheimerUserSelect:
		return game.SlappenheimerNone
	case game.SlappenheimerNone:
		return game.SlappenheimerRandom
	case game.SlappenheimerRandom:
		return game.SlappenheimerConstant
	default:
		return game.SlappenheimerUserSelect
	}
}

// ConfigEditor runs the Configuration Editor standalone (used by the door's
// -reset command, BRE's "reset" being the settings screen). It edits w.Config
// in place, saves config.json when the sysop exits with S, and reports whether
// they saved (true) or cancelled with Q/0 (false) — so -reset can abort.
func ConfigEditor(s session.Session, w *game.World) (saved bool) {
	// -reset runs this on a bare console with no GameLoop above it, so recover
	// an End panic (a closed stdin during editing) here and report "not saved"
	// instead of crashing. The in-game path lets End propagate to GameLoop.
	defer func() {
		if r := recover(); r != nil {
			if _, ok := session.AsEnd(r); ok {
				saved = false
				return
			}
			panic(r)
		}
	}()
	return runConfigEditor(s, w)
}

// runConfigEditor is BRE's Configuration Editor: the LC (or a single-BBS sysop)
// sets the league ruleset. It edits the world's Config in place and writes it
// back to config.json. Defaults and the turns/day cap come from BRE's compiled
// code; other caps are generous where BRE's exact max isn't confirmed. The
// inter-BBS fields (marked *, as the original marks them) are the ones that do
// nothing on a stand-alone board. Returns true if saved, false if cancelled.
//
// Changes take effect going forward (a new TurnsPerDay applies at the next
// daily maintenance; AICount does not retroactively add or remove AI empires).
func runConfigEditor(s session.Session, w *game.World) bool {
	c := &w.Config
	pages := configPages(c.IBBS)
	page := 0
	for {
		g := pages[page]
		fmt.Fprintf(s, "%s\n", titleRule(ansi.FgBrightBlue, "Configuration Editor", len([]rune(rule))))
		fmt.Fprintf(s, "%s  %s  (page %d of %d)%s\n", ansi.FgBrightCyan, g.title, page+1, len(pages), ansi.Reset)
		for _, f := range g.fields {
			label := f.label
			if ibbsOnlyFields[f.n] {
				label = "* " + label
			}
			fmt.Fprintf(s, "  %2d) %-30s %s\n", f.n, label, f.value(c))
		}
		// A stand-alone board never sees a starred field (withoutIBBSFields drops
		// them), so the legend would explain a mark that is not on the screen.
		fmt.Fprintf(s, "%s\n", closingRule(ansi.FgBrightBlue, len([]rune(rule))))
		if c.IBBS {
			fmt.Fprintf(s, "%s* = inter-BBS option%s\n", ansi.FgWhite, ansi.Reset)
		}

		choice := strings.ToLower(strings.TrimSpace(prompt(s, "Edit which (N = next page, P = previous, S = save & exit, Q = cancel)?")))
		switch choice {
		case "n", "next":
			page = (page + 1) % len(pages)
			continue
		case "p", "prev", "previous":
			page = (page - 1 + len(pages)) % len(pages)
			continue
		case "s", "save":
			if err := store.SaveConfig(*c); err != nil {
				fail(s, err)
			} else {
				okNoPause(s, "Configuration saved.")
			}
			return true
		case "q", "quit", "cancel", "0":
			ok(s, "Cancelled — no changes were saved.")
			return false
		}
		n, err := strconv.Atoi(choice)
		if err != nil {
			continue
		}
		// A field number reaches its field from any page: the numbers are stable
		// identifiers, not positions on the current screen.
		if f := findConfigField(pages, n); f != nil {
			f.edit(s, c)
		}
	}
}

// cfgField is one editable setting. n is a stable identifier — it does not
// track the field's position, so regrouping the pages renumbers nothing (28 and
// 29 already sat outside the printed sequence before there were pages).
type cfgField struct {
	n     int
	label string
	value func(*game.Config) string
	edit  func(session.Session, *game.Config)
}

// cfgPage is one screenful of fields.
type cfgPage struct {
	title  string
	fields []cfgField
}

// configPages groups the settings into the same four topics the tview editor
// uses as tabs, so a sysop sees one layout whichever editor they land in — and,
// unlike the single 38-field list this replaces, each page fits an 80x24 screen
// (issue #100).
func configPages(ibbs bool) []cfgPage {
	// Shorthands: most fields either prompt for a number or cycle a choice.
	// num prompts for a value in [minV, maxV]. The prompt itself carries the
	// current value and the ceiling; a field with a floor above zero names its
	// range in the prompt text, since the "(current; max)" hint cannot show one.
	// The tview editor holds the same bounds through addInt.
	num := func(n int, label string, prompt string, get func(*game.Config) int, set func(*game.Config, int), minV, maxV int) cfgField {
		return cfgField{
			n: n, label: label,
			value: func(c *game.Config) string { return fmt.Sprintf("%d", get(c)) },
			edit: func(s session.Session, c *game.Config) {
				set(c, max(minV, promptSuggested(s, prompt, get(c), maxV)))
			},
		}
	}
	toggle := func(n int, label string, get func(*game.Config) bool, set func(*game.Config, bool)) cfgField {
		return cfgField{
			n: n, label: label,
			value: func(c *game.Config) string { return onOffStr(get(c)) },
			edit:  func(_ session.Session, c *game.Config) { set(c, !get(c)) },
		}
	}
	cycle := func(n int, label string, get func(*game.Config) game.Level, set func(*game.Config, game.Level), step func(game.Level) game.Level) cfgField {
		return cfgField{
			n: n, label: label,
			value: func(c *game.Config) string { return get(c).String() },
			edit:  func(_ session.Session, c *game.Config) { set(c, step(get(c))) },
		}
	}

	timing := []cfgField{
		{n: 1, label: "Turns per day",
			value: func(c *game.Config) string { return fmt.Sprintf("%d", c.TurnsPerDay) },
			edit: func(s session.Session, c *game.Config) {
				c.TurnsPerDay = max(1, promptSuggested(s, "Turns per day", c.TurnsPerDay, game.MaxTurnsPerDay))
			}},
		num(2, "Turns of Protection", "Turns of Protection",
			func(c *game.Config) int { return c.ProtectionTurns },
			func(c *game.Config, v int) { c.ProtectionTurns = v }, 0, game.MaxProtectionTurns),
		{n: 3, label: "Game length (days)",
			value: func(c *game.Config) string { return fmt.Sprintf("%d (0 = endless)", c.GameLength) },
			edit: func(s session.Session, c *game.Config) {
				c.GameLength = promptSuggested(s, "Game length in days (0 = endless)", c.GameLength, 100000)
			}},
		{n: 29, label: "Remove idle realms (days)",
			value: func(c *game.Config) string { return fmt.Sprintf("%d (0 = never)", c.IdleDaysRemove) },
			edit: func(s session.Session, c *game.Config) {
				c.IdleDaysRemove = promptSuggested(s, "Remove a realm unplayed for how many days (0 = never)", c.IdleDaysRemove, game.MaxIdleDaysRemove)
			}},
		{n: 19, label: "Game Start Date",
			value: func(c *game.Config) string { return dateOr(c.GameStartDate, "starts immediately") },
			edit: func(s session.Session, c *game.Config) {
				c.GameStartDate = promptDate(s, "Game Start Date", c.GameStartDate)
			}},
		{n: 20, label: "Join Cutoff Date",
			value: func(c *game.Config) string { return dateOr(c.JoinDate, "always open") },
			edit:  func(s session.Session, c *game.Config) { c.JoinDate = promptDate(s, "Join Cutoff Date", c.JoinDate) }},
		{n: 21, label: "AI empires", value: aiCountStr,
			edit: func(s session.Session, c *game.Config) {
				if c.IBBS {
					ok(s, "A league board has no computer barons.")
					return
				}
				c.AICount = promptSuggested(s, "AI empires", c.AICount, 5)
			}},
		// 45 is past the original's own field numbers, which end at 44: pirates are
		// IB's switch, so it takes the next free number rather than one of BRE's.
		toggle(45, "Pirates", func(c *game.Config) bool { return c.Pirates },
			func(c *game.Config, v bool) { c.Pirates = v }),
	}

	econ := []cfgField{
		num(4, "Initial Market Land", "Initial Market Land",
			func(c *game.Config) int { return c.InitialMarketLand },
			func(c *game.Config, v int) { c.InitialMarketLand = v }, 0, game.MaxInitialMarketLand),
		num(5, "Land Created / Day", "Land Created / Day",
			func(c *game.Config) int { return c.LandPerDay },
			func(c *game.Config, v int) { c.LandPerDay = v }, 0, game.MaxLandPerDay),
		num(6, "Interest Rate", "Interest Rate (50-200)",
			func(c *game.Config) int { return c.InterestRate },
			func(c *game.Config, v int) { c.InterestRate = v }, game.MinBankInterest, game.MaxBankInterest),
		num(7, "Standard Investment Rate", "Standard Investment Rate (35-100)",
			func(c *game.Config) int { return c.StdInvestRate },
			func(c *game.Config, v int) { c.StdInvestRate = v }, game.MinStdInvestRate, game.MaxStdInvestRate),
		{n: 8, label: "Steady Investment Rate",
			value: func(c *game.Config) string { return enabledStr(c.SteadyInvest) },
			edit:  func(_ session.Session, c *game.Config) { c.SteadyInvest = !c.SteadyInvest }},
		num(9, "Max Tax Rate", "Max Tax Rate",
			func(c *game.Config) int { return c.MaxTaxRate },
			func(c *game.Config, v int) { c.MaxTaxRate = v }, 0, game.MaxPlayerTaxRate),
		{n: 28, label: "Planetary Tax Rate",
			value: func(c *game.Config) string { return fmt.Sprintf("%d%%", c.PlanetaryTaxRate) },
			edit: func(s session.Session, c *game.Config) {
				c.PlanetaryTaxRate = promptSuggested(s, "Planetary Tax Rate (%)", c.PlanetaryTaxRate, game.MaxPlanetaryTaxRate)
			}},
		toggle(26, "Food Unlimited",
			func(c *game.Config) bool { return c.FoodUnlimited },
			func(c *game.Config, v bool) { c.FoodUnlimited = v }),
	}

	mil := []cfgField{
		{n: 12, label: "Buy Military",
			value: func(c *game.Config) string { return c.BuyMilitary.String() },
			edit:  func(_ session.Session, c *game.Config) { c.BuyMilitary = cycleBuy(c.BuyMilitary) }},
		cycle(13, "Maintenance Costs", func(c *game.Config) game.Level { return c.MaintCosts },
			func(c *game.Config, v game.Level) { c.MaintCosts = v }, cycleCost),
		cycle(14, "Trade Deal Costs", func(c *game.Config) game.Level { return c.TradeCosts },
			func(c *game.Config, v game.Level) { c.TradeCosts = v }, cycleCost),
		cycle(15, "Region Costs", func(c *game.Config) game.Level { return c.RegionCosts },
			func(c *game.Config, v game.Level) { c.RegionCosts = v }, cycleCost),
		cycle(16, "Attack Damage", func(c *game.Config) game.Level { return c.AttackDamage },
			func(c *game.Config, v game.Level) { c.AttackDamage = v }, cycleHML),
		cycle(17, "Attack Rewards", func(c *game.Config) game.Level { return c.AttackRewards },
			func(c *game.Config, v game.Level) { c.AttackRewards = v }, cycleHML),
		{n: 18, label: "R5-Slappenheimer Handling",
			value: func(c *game.Config) string { return c.SlappenheimerHandling.String() },
			edit: func(_ session.Session, c *game.Config) {
				c.SlappenheimerHandling = cycleSlappenheimer(c.SlappenheimerHandling)
			}},
		{n: 27, label: "Max Individual Attacks/Day",
			value: func(c *game.Config) string { return fmt.Sprintf("%d (0 = unlimited)", c.MaxIndividualAttacks) },
			edit: func(s session.Session, c *game.Config) {
				c.MaxIndividualAttacks = promptSuggested(s, "Max Individual Attacks/Day (0 = unlimited)", c.MaxIndividualAttacks, 100)
			}},
		{n: 30, label: "Max Group Attacks/Day",
			value: func(c *game.Config) string { return fmt.Sprintf("%d (0 = unlimited)", c.MaxGroupAttacks) },
			edit: func(s session.Session, c *game.Config) {
				c.MaxGroupAttacks = promptSuggested(s, "Max Group Attacks/Day (0 = unlimited)", c.MaxGroupAttacks, 100)
			}},
		{n: 31, label: "Max Terrorist Ops/Day",
			value: func(c *game.Config) string { return fmt.Sprintf("%d (0 = unlimited)", c.MaxTerrorOps) },
			edit: func(s session.Session, c *game.Config) {
				c.MaxTerrorOps = promptSuggested(s, "Max Terrorist Ops/Day (0 = unlimited)", c.MaxTerrorOps, 100)
			}},
		{n: 32, label: "Max Bombing Ops/Day",
			value: func(c *game.Config) string { return fmt.Sprintf("%d (0 = unlimited)", c.MaxBombingOps) },
			edit: func(s session.Session, c *game.Config) {
				c.MaxBombingOps = promptSuggested(s, "Max Bombing Ops/Day (0 = unlimited)", c.MaxBombingOps, 100)
			}},
		{n: 33, label: "Days before lost forces return",
			value: func(c *game.Config) string { return fmt.Sprintf("%d (0 = never)", c.LostForcesDays) },
			edit: func(s session.Session, c *game.Config) {
				c.LostForcesDays = promptSuggested(s, "Days before lost forces return (0 = never)", c.LostForcesDays, game.MaxLostForcesDays)
			}},
		cycle(34, "Attack Costs", func(c *game.Config) game.Level { return c.AttackCosts },
			func(c *game.Config, v game.Level) { c.AttackCosts = v }, cycleCost),
		cycle(35, "Terrorism Costs", func(c *game.Config) game.Level { return c.TerrorCosts },
			func(c *game.Config, v game.Level) { c.TerrorCosts = v }, cycleCost),
		toggle(36, "Bombing Ops", func(c *game.Config) bool { return c.BombingOps },
			func(c *game.Config, v bool) { c.BombingOps = v }),
		toggle(37, "Missile Ops", func(c *game.Config) bool { return c.MissileOps },
			func(c *game.Config, v bool) { c.MissileOps = v }),
		toggle(38, "Clingy Annihilator", func(c *game.Config) bool { return c.ClingyAnnihilator },
			func(c *game.Config, v bool) { c.ClingyAnnihilator = v }),
		toggle(42, "Local Attacks", func(c *game.Config) bool { return c.LocalAttacks },
			func(c *game.Config, v bool) { c.LocalAttacks = v }),
		toggle(43, "Local Attack Scoring", func(c *game.Config) bool { return c.LocalAttackScoring },
			func(c *game.Config, v bool) { c.LocalAttackScoring = v }),
	}

	caps := []cfgField{
		num(10, "Max Purchasable Regions", "Max Purchasable Regions",
			func(c *game.Config) int { return c.MaxRegions },
			func(c *game.Config, v int) { c.MaxRegions = v }, 0, game.MaxPurchasableRegions),
		num(11, "Max Players Per BBS", "Max Players Per BBS (1-25; 0 = unlimited)",
			func(c *game.Config) int { return c.MaxPlayers },
			func(c *game.Config, v int) { c.MaxPlayers = v }, 0, game.MaxPlayersPerBoard),
		{n: 23, label: "Board ID",
			value: func(c *game.Config) string { return c.BoardID },
			edit: func(s session.Session, c *game.Config) {
				if v := strings.TrimSpace(prompt(s, "Board ID:")); v != "" {
					c.BoardID = v
				}
			}},
		{n: 41, label: "League Number",
			value: func(c *game.Config) string { return fmt.Sprintf("%d (0 = unset)", c.LeagueNumber) },
			edit: func(s session.Session, c *game.Config) {
				c.LeagueNumber = promptSuggested(s, "League Number (1-999, 0 = unset)", c.LeagueNumber, game.MaxLeagueNumber)
			}},
		{n: 39, label: "Inbound Dir",
			value: func(c *game.Config) string { return c.InboundDir },
			edit: func(s session.Session, c *game.Config) {
				if v := strings.TrimSpace(prompt(s, "Inbound Dir (relative to the data directory):")); v != "" {
					c.InboundDir = v
				}
			}},
		{n: 40, label: "Outbound Dir",
			value: func(c *game.Config) string { return c.OutboundDir },
			edit: func(s session.Session, c *game.Config) {
				if v := strings.TrimSpace(prompt(s, "Outbound Dir (relative to the data directory):")); v != "" {
					c.OutboundDir = v
				}
			}},
		toggle(44, "Dupe Checking", func(c *game.Config) bool { return c.DupeChecking },
			func(c *game.Config, v bool) { c.DupeChecking = v }),
		{n: 24, label: "Idle timeout (sec)",
			value: func(c *game.Config) string { return fmt.Sprintf("%d (0 = never)", c.IdleTimeoutSecs) },
			edit: func(s session.Session, c *game.Config) {
				c.IdleTimeoutSecs = promptSuggested(s, "Idle timeout in seconds (0 = never)", c.IdleTimeoutSecs, 86400)
			}},
		{n: 25, label: "Idle warnings before boot",
			value: func(c *game.Config) string { return fmt.Sprintf("%d", c.MaxIdleWarnings) },
			edit: func(s session.Session, c *game.Config) {
				c.MaxIdleWarnings = max(1, promptSuggested(s, "Idle warnings before boot", c.MaxIdleWarnings, 100))
			}},
	}

	pages := []cfgPage{
		{configTabTitles[0], timing},
		{configTabTitles[1], econ},
		{configTabTitles[2], mil},
		{configTabTitles[3], caps},
	}
	if !ibbs {
		for i := range pages {
			pages[i].fields = withoutIBBSFields(pages[i].fields)
		}
	}
	return pages
}

// ibbsOnlyFields are the settings that mean nothing on a stand-alone board, so
// only -ibbs-reset offers them. Two other places know the same set — the tview
// editor, which branches on it, and Game Setup, which reports it — and each has
// a test holding it to this one (TestBothEditorsHideTheSameLeagueFields,
// TestGameSetupHidesLeagueRulesOffLeague). A sysop must never read a rule they
// were never asked.
var ibbsOnlyFields = map[int]bool{
	23: true, // Board ID
	41: true, // League Number
	39: true, // Inbound Dir
	40: true, // Outbound Dir
	30: true, // Max Group Attacks/Day
	31: true, // Max Terrorist Ops/Day
	32: true, // Max Bombing Ops/Day
	33: true, // Days before lost forces return
	35: true, // Terrorism Costs
	38: true, // Clingy Annihilator
	42: true, // Local Attacks
	43: true, // Local Attack Scoring
	44: true, // Dupe Checking
}

func withoutIBBSFields(fields []cfgField) []cfgField {
	out := make([]cfgField, 0, len(fields))
	for _, f := range fields {
		if !ibbsOnlyFields[f.n] {
			out = append(out, f)
		}
	}
	return out
}

// findConfigField returns the field with identifier n, or nil if there is none.
func findConfigField(pages []cfgPage, n int) *cfgField {
	for _, g := range pages {
		for i := range g.fields {
			if g.fields[i].n == n {
				return &g.fields[i]
			}
		}
	}
	return nil
}

// aiCountStr renders the AI-baron count, or says why a league board has none.
func aiCountStr(c *game.Config) string {
	if c.IBBS {
		return "0 (none in a league)"
	}
	return fmt.Sprintf("%d", c.AICount)
}

// dateOr renders an ISO date, or a placeholder when it is unset.
func dateOr(d, unset string) string {
	if d == "" {
		return "(" + unset + ")"
	}
	return d
}

// promptDate reads an ISO date (YYYY-MM-DD); blank keeps the current value and
// "-" clears it. A malformed date is rejected and the current value kept.
func promptDate(s session.Session, label, cur string) string {
	in := strings.TrimSpace(prompt(s, label+" (YYYY-MM-DD, blank = keep, - = clear)"))
	switch in {
	case "":
		return cur
	case "-":
		return ""
	}
	if _, err := time.Parse("2006-01-02", in); err != nil {
		fail(s, fmt.Errorf("not a valid date (use YYYY-MM-DD)"))
		return cur
	}
	return in
}

func onOffStr(b bool) string {
	if b {
		return "ON"
	}
	return "OFF"
}

func enabledStr(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}
