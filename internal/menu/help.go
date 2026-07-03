package menu

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// helpTopic is one entry: a display name and our own description.
var helpTopics = []struct{ Name, Text string }{
	// Units
	{"Troopers", "Your cheapest unit: they count toward both Offense and Defense directly, and every other unit's contribution scales alongside them."},
	{"Jets", "Offense-only fliers. You can only commit up to 100 Jets per Carrier to an attack, so idle Jets beyond that cap add nothing until you build more Carriers."},
	{"Turrets", "Pure defense: each Turret doubles its own weight against incoming attacks, but contributes nothing to your Offense."},
	{"Tanks", "Your strongest dual-purpose unit, counting fully toward both Offense and Defense; a completed HeadQuarters multiplies their contribution further."},
	{"Bombers", "A high-value unit for your Net Worth score. Industrial regions can be set to build them, but they are not yet consumed by any attack."},
	{"Carriers", "Carry your Jets into battle: each Carrier lets you commit up to 100 Jets to an attack, so Jets beyond Carriers*100 sit idle."},
	{"Agents", "Fuel your covert operations (Send Spy, Special Operations). The more Agents you hold relative to your target's, the better your odds of success."},

	// Regions
	{"Coastal", "Earns gold from tourism, but that income scales with your Popular Support — let Support collapse and Coastal income collapses with it."},
	{"Mountain", "Earns a steady, modest amount of gold per region (ore mining) that does not depend on Support."},
	{"Desert", "Earns gold per region from solar power."},
	{"River", "Earns the highest flat gold-per-region of any region type."},
	{"Agricultural", "Grows most of your food; every region type also contributes a small base amount of food regardless of type."},
	{"Urban", "Produces no gold or food directly, but draws new settlers, growing your population each turn."},
	{"Industrial", "Yields gold every turn and converts into production points that get built into units per your Set Industries split (or your Specialization)."},
	{"Technology", "Yields gold and raises your tech factor (a capped percentage based on the share of your land that is Technology), which boosts income, tax revenue, Offense and Defense, and cuts maintenance cost and food spoilage."},

	// Attacks
	{"Regular Attack", "Commits your full Offense against the target's Defense plus a land-based home bonus, both randomized. Win and you capture some of their regions and gold; lose and you both take casualties, yours worse."},
	{"Nuclear", "A costly strike that reduces some of the target's regions to waste, scaled down by their SDI defense."},
	{"Chemical", "A costly strike that kills people and Troopers and destroys a smaller share of regions, scaled down by the target's SDI."},
	{"Biological", "A costly strike that kills people and Troopers but leaves land untouched, scaled down by the target's SDI."},
	{"Attack Pirates", "Raid one of nine pirate factions by committing Troopers, Jets, and Tanks. Their strength is random, not a fixed order. Pirates raid players and hoard the units they carry off, growing over time; a winning raid reclaims a portion of a faction's loot and regions, so it takes several hits to drain a fat one. Losing costs you a share of the committed force."},
	{"Gooie Kablooie", "An extremely expensive superweapon that strikes every living rival empire at once, stripping a share of each one's land, reduced by their SDI."},
	{"SDI Program", "Spend gold in fixed increments to raise your SDI level, which reduces the damage you take from Nuclear, Chemical, Biological, and Gooie Kablooie strikes, up to a cap."},

	// Covert
	{"Send Spy", "Requires at least one Agent. Success depends on your Agents versus the target's; on success you get a full intel report, on failure you lose the agent and the target is alerted."},
	{"Special Operations", "Sabotage: requires at least one Agent and uses the same odds as Send Spy. Success destroys a share of the target's Troopers; failure costs you the agent."},

	// Economy
	{"Bank / Savings", "Deposit gold to keep it safe from being spent, or plundered in a lost attack; banked gold slowly earns interest each game day."},
	{"Investments", "Lock gold away for a chosen number of days at the current floating investment rate for a guaranteed payout at maturity. The rate drifts daily with total demand across all empires."},
	{"Loans", "Borrow gold against your land now; the debt keeps growing each turn until you repay it, so treat it as expensive short-term credit."},
	{"Land Market", "Buying regions gets more expensive the more land you already own, so expanding late in the game costs much more per region than expanding early."},
	{"Food Market", "The market sells you food for more than it will buy it back, so it is not a place to store surplus food for profit."},
	{"Food Spoilage", "Food you hold beyond roughly two turns' worth of consumption spoils each turn; a high tech factor slows the spoilage rate."},

	// Empire
	{"HeadQuarters", "A one-time construction project that advances a little each turn until complete; once building (and more so once finished) it boosts what your Tanks contribute to Offense and Defense."},
	{"Popular Support", "Drifts each turn toward a target set by your tax rate — a stable, moderate tax rate holds Support near its maximum, while high taxes push it down. Low Support cripples Coastal income."},
	{"Tax Rate & Riots", "Higher tax rates raise more revenue but also raise the odds of riots each turn once tax climbs past a threshold; a riot crashes your Support and drives off part of your population."},
	{"Protection", "New Realm Protection is a countdown of turns during which you cannot launch or receive attacks; it ticks down only on turns you actually play."},
	{"Industrial Production", "Each Industrial region generates gold and production points every turn; the points are spent on units according to your Set Industries percentages, unless you have specialized."},
	{"Specialize", "Permanently commits all of your Industrial production to a single unit type instead of splitting it by percentage. This choice cannot be undone."},

	// Other
	{"Diplomacy / Alliances", "Propose or accept alliances from the Diplomacy menu; allied empires cannot be chosen as targets for attacks until the alliance is broken."},
	{"Trade Deals", "Send gold directly to another empire from the Trading menu, no strings attached."},
	{"Leagues & Planetary Master", "See Scores ranks every empire by Net Worth; the current leader is tracked as the Planetary Master."},
	{"Net Worth", "Your overall strategic score: mostly land and weighted military strength, minus debt. Gold, food, and population are not counted directly, so a cash-rich empire with little land or army still scores low."},
	{"Inter-BBS Scores", "View IPScores in the Trading menu shows empires and rankings imported from other boards' exported score packets."},
}

// findHelpTopic matches query against a topic Name, case-insensitively.
// An exact match always wins; otherwise, if query is a prefix of exactly
// one topic Name, that topic is returned. No match (or an ambiguous
// prefix) returns nil.
func findHelpTopic(query string) *struct{ Name, Text string } {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	for i := range helpTopics {
		if strings.ToLower(helpTopics[i].Name) == q {
			return &helpTopics[i]
		}
	}
	var match *struct{ Name, Text string }
	count := 0
	for i := range helpTopics {
		if strings.HasPrefix(strings.ToLower(helpTopics[i].Name), q) {
			match = &helpTopics[i]
			count++
		}
	}
	if count == 1 {
		return match
	}
	return nil
}

// printHelpTopics lists topic Names in a compact, down-then-across 3-column
// layout that fits an 80x25 screen.
func printHelpTopics(s session.Session) {
	const cols = 3
	rows := (len(helpTopics) + cols - 1) / cols
	for r := 0; r < rows; r++ {
		line := ""
		for c := 0; c < cols; c++ {
			idx := c*rows + r
			if idx < len(helpTopics) {
				line += fmt.Sprintf("%-26s", helpTopics[idx].Name)
			}
		}
		fmt.Fprintf(s, "  %s\n", strings.TrimRight(line, " "))
	}
}

// helpDatabase presents the topic list and looks up a topic the player
// types, looping until they quit.
func helpDatabase(s session.Session, w *game.World) Result {
	for {
		fmt.Fprint(s, ansi.Clear)
		fmt.Fprintf(s, "%s[Help Database]%s\n", ansi.FgBrightCyan, ansi.Reset)
		fmt.Fprintf(s, "%s\n", rule)
		printHelpTopics(s)

		q := strings.TrimSpace(prompt(s, "Enter topic (0 to quit):"))
		if q == "" || q == "0" {
			return Stay
		}
		t := findHelpTopic(q)
		if t == nil {
			fmt.Fprintf(s, "\n  %sNo unique match for %q.%s\n", ansi.FgRed, q, ansi.Reset)
			pause(s)
			continue
		}
		fmt.Fprintf(s, "\n%s%s%s\n  %s\n", ansi.FgBrightCyan, t.Name, ansi.Reset, t.Text)
		pause(s)
	}
}
