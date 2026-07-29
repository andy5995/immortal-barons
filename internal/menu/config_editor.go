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
// league ruleset fields (marked *) are the ones a Coordinator broadcasts to the
// league with -league-config. Returns true if saved, false if cancelled.
//
// Changes take effect going forward (a new TurnsPerDay applies at the next
// daily maintenance; AICount does not retroactively add or remove AI empires).
func runConfigEditor(s session.Session, w *game.World) bool {
	c := &w.Config
	for {
		fmt.Fprintf(s, "%s\n", titleRule(ansi.FgBrightBlue, "Configuration Editor", len([]rune(rule))))
		p := func(n int, label, val string) { fmt.Fprintf(s, "  %2d) %-26s %s\n", n, label, val) }
		p(1, "* Turns per day", fmt.Sprintf("%d", c.TurnsPerDay))
		p(2, "* Turns of Protection", fmt.Sprintf("%d", c.ProtectionTurns))
		p(3, "* Game length (days)", fmt.Sprintf("%d (0 = endless)", c.GameLength))
		p(4, "* Initial Market Land", fmt.Sprintf("%d", c.InitialMarketLand))
		p(5, "* Land Created / Day", fmt.Sprintf("%d", c.LandPerDay))
		p(6, "* Interest Rate", fmt.Sprintf("%d", c.InterestRate))
		p(7, "* Standard Investment Rate", fmt.Sprintf("%d", c.StdInvestRate))
		p(8, "* Steady Investment Rate", enabledStr(c.SteadyInvest))
		p(9, "* Max Tax Rate", fmt.Sprintf("%d", c.MaxTaxRate))
		p(28, "* Planetary Tax Rate", fmt.Sprintf("%d%%", c.PlanetaryTaxRate))
		p(10, "* Max Purchasable Regions", fmt.Sprintf("%d", c.MaxRegions))
		p(11, "* Max Players Per BBS", fmt.Sprintf("%d", c.MaxPlayers))
		p(12, "* Buy Military", c.BuyMilitary.String())
		p(13, "* Maintenance Costs", c.MaintCosts.String())
		p(14, "* Trade Deal Costs", c.TradeCosts.String())
		p(15, "* Region Costs", c.RegionCosts.String())
		p(16, "* Attack Damage", c.AttackDamage.String())
		p(17, "* Attack Rewards", c.AttackRewards.String())
		p(18, "* R5-Slappenheimer Handling", c.SlappenheimerHandling.String())
		p(19, "* Game Start Date", dateOr(c.GameStartDate, "starts immediately"))
		p(20, "* Join Cutoff Date", dateOr(c.JoinDate, "always open"))
		p(21, "AI empires", fmt.Sprintf("%d", c.AICount))
		p(22, "Inter-BBS play", onOffStr(c.IBBS))
		p(23, "Board ID", c.BoardID)
		p(24, "Idle timeout (sec)", fmt.Sprintf("%d (0 = never)", c.IdleTimeoutSecs))
		p(25, "Idle warnings before boot", fmt.Sprintf("%d", c.MaxIdleWarnings))
		p(26, "* Food Unlimited", onOffStr(c.FoodUnlimited))
		p(27, "* Max Individual Attacks/Day", fmt.Sprintf("%d (0 = unlimited)", c.MaxIndividualAttacks))
		fmt.Fprintf(s, "%s\n%s* = league ruleset (Coordinator broadcasts with -league-config)%s\n",
			rule, ansi.FgWhite, ansi.Reset)

		choice := strings.ToLower(strings.TrimSpace(prompt(s, "Edit which (S = save & exit, Q = cancel)?")))
		if choice == "s" || choice == "save" {
			if err := store.SaveConfig(*c); err != nil {
				fail(s, err)
			} else {
				okNoPause(s, "Configuration saved.")
			}
			return true
		}
		if choice == "q" || choice == "quit" || choice == "cancel" || choice == "0" {
			ok(s, "Cancelled — no changes were saved.")
			return false
		}
		n, err := strconv.Atoi(choice)
		if err != nil {
			continue
		}
		switch n {
		case 1:
			c.TurnsPerDay = max(1, promptSuggested(s, "Turns per day", c.TurnsPerDay, game.MaxTurnsPerDay))
		case 2:
			c.ProtectionTurns = promptSuggested(s, "Turns of Protection", c.ProtectionTurns, game.MaxProtectionTurns)
		case 3:
			c.GameLength = promptSuggested(s, "Game length in days (0 = endless)", c.GameLength, 100000)
		case 4:
			c.InitialMarketLand = promptSuggested(s, "Initial Market Land", c.InitialMarketLand, game.MaxInitialMarketLand)
		case 5:
			c.LandPerDay = promptSuggested(s, "Land Created / Day", c.LandPerDay, game.MaxLandPerDay)
		case 6:
			c.InterestRate = promptSuggested(s, "Interest Rate", c.InterestRate, game.MaxBankInterest)
		case 7:
			c.StdInvestRate = promptSuggested(s, "Standard Investment Rate", c.StdInvestRate, game.MaxStdInvestRate)
		case 8:
			c.SteadyInvest = !c.SteadyInvest
		case 9:
			c.MaxTaxRate = promptSuggested(s, "Max Tax Rate", c.MaxTaxRate, game.MaxPlayerTaxRate)
		case 28:
			c.PlanetaryTaxRate = promptSuggested(s, "Planetary Tax Rate (%)", c.PlanetaryTaxRate, game.MaxPlanetaryTaxRate)
		case 10:
			c.MaxRegions = promptSuggested(s, "Max Purchasable Regions", c.MaxRegions, game.MaxPurchasableRegions)
		case 11:
			c.MaxPlayers = promptSuggested(s, "Max Players Per BBS (0 = unlimited)", c.MaxPlayers, 100000)
		case 12:
			c.BuyMilitary = cycleBuy(c.BuyMilitary)
		case 13:
			c.MaintCosts = cycleCost(c.MaintCosts)
		case 14:
			c.TradeCosts = cycleCost(c.TradeCosts)
		case 15:
			c.RegionCosts = cycleCost(c.RegionCosts)
		case 16:
			c.AttackDamage = cycleHML(c.AttackDamage)
		case 17:
			c.AttackRewards = cycleHML(c.AttackRewards)
		case 18:
			c.SlappenheimerHandling = cycleSlappenheimer(c.SlappenheimerHandling)
		case 19:
			c.GameStartDate = promptDate(s, "Game Start Date", c.GameStartDate)
		case 20:
			c.JoinDate = promptDate(s, "Join Cutoff Date", c.JoinDate)
		case 21:
			c.AICount = promptSuggested(s, "AI empires", c.AICount, 5)
		case 22:
			c.IBBS = !c.IBBS
		case 23:
			if v := strings.TrimSpace(prompt(s, "Board ID:")); v != "" {
				c.BoardID = v
			}
		case 24:
			c.IdleTimeoutSecs = promptSuggested(s, "Idle timeout in seconds (0 = never)", c.IdleTimeoutSecs, 86400)
		case 25:
			c.MaxIdleWarnings = max(1, promptSuggested(s, "Idle warnings before boot", c.MaxIdleWarnings, 100))
		case 26:
			c.FoodUnlimited = !c.FoodUnlimited
		case 27:
			c.MaxIndividualAttacks = promptSuggested(s, "Max Individual Attacks/Day (0 = unlimited)", c.MaxIndividualAttacks, 100)
		}
	}
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
