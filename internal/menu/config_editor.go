package menu

import (
	"fmt"
	"strings"

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

// cycleSabre steps Sabre Handling through its four settings.
func cycleSabre(m game.SabreMode) game.SabreMode {
	switch m {
	case game.SabreUserSelect:
		return game.SabreNone
	case game.SabreNone:
		return game.SabreRandom
	case game.SabreRandom:
		return game.SabreConstant
	default:
		return game.SabreUserSelect
	}
}

// configEditor is BRE's Configuration Editor: the LC (or a single-BBS sysop)
// sets the league ruleset. It edits the world's Config in place and writes it
// back to config.json. Defaults and the turns/day cap come from BRE's compiled
// code; other caps are generous where BRE's exact max isn't confirmed. The
// league ruleset fields (marked *) are the ones a Coordinator broadcasts to the
// league with -league-config.
//
// Changes take effect going forward (a new TurnsPerDay applies at the next
// daily maintenance; AICount does not retroactively add or remove AI empires).
func configEditor(s session.Session, w *game.World) Result {
	c := &w.Config
	for {
		fmt.Fprint(s, ansi.Clear)
		fmt.Fprintf(s, "%s\n", titleRule(ansi.FgBrightBlue, "Configuration Editor"))
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
		p(10, "* Max Purchasable Regions", fmt.Sprintf("%d", c.MaxRegions))
		p(11, "* Max Players Per BBS", fmt.Sprintf("%d", c.MaxPlayers))
		p(12, "* Buy Military", c.BuyMilitary.String())
		p(13, "* Maintenance Costs", c.MaintCosts.String())
		p(14, "* Trade Deal Costs", c.TradeCosts.String())
		p(15, "* Region Costs", c.RegionCosts.String())
		p(16, "* Attack Damage", c.AttackDamage.String())
		p(17, "* Attack Rewards", c.AttackRewards.String())
		p(18, "* Sabre Handling", c.SabreHandling.String())
		p(19, "AI empires", fmt.Sprintf("%d", c.AICount))
		p(20, "Inter-BBS play", onOffStr(c.IBBS))
		p(21, "Board ID", c.BoardID)
		fmt.Fprintf(s, "%s\n%s* = league ruleset (Coordinator broadcasts with -league-config)%s\n",
			rule, ansi.FgWhite, ansi.Reset)

		switch promptInt(s, "Edit which (0 to save and exit)?") {
		case 0:
			if err := store.SaveConfig(*c); err != nil {
				fail(s, err)
			} else {
				ok(s, "Configuration saved.")
			}
			return Stay
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
			c.MaxTaxRate = promptSuggested(s, "Max Tax Rate", c.MaxTaxRate, game.MaxPlanetaryTaxRate)
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
			c.SabreHandling = cycleSabre(c.SabreHandling)
		case 19:
			c.AICount = promptSuggested(s, "AI empires", c.AICount, 5)
		case 20:
			c.IBBS = !c.IBBS
		case 21:
			if v := strings.TrimSpace(prompt(s, "Board ID:")); v != "" {
				c.BoardID = v
			}
		}
	}
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
