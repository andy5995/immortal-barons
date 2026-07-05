package game

import "fmt"

// Planet-wide news, in the spirit of BRE's news.dat feed (see
// docs/mechanics-reference.md, "News files"). Wording here is original — the
// original's news lines are not copied verbatim — but the events broadcast
// match: regular-attack wins/losses and total conquests go to every player's
// news, not just the victim.

// postNews appends a system news line to the planetary bulletin, keeping only
// the most recent entries (same cap as player bulletins).
func (w *World) postNews(line string) {
	w.Bulletin = append(w.Bulletin, line)
	if len(w.Bulletin) > 20 {
		w.Bulletin = w.Bulletin[len(w.Bulletin)-20:]
	}
}

// postCombatNews broadcasts the outcome of a regular attack to the planet.
func (w *World) postCombatNews(a, d *Empire, won, conquered bool) {
	var lines []string
	switch {
	case conquered:
		lines = []string{
			fmt.Sprintf("NEWS! The empire of %s has been wiped from the map by %s!", d.Name, a.Name),
			fmt.Sprintf("%s conquered %s outright — the realm is no more.", a.Name, d.Name),
			fmt.Sprintf("Historians will remember %s's destruction of %s.", a.Name, d.Name),
		}
	case won:
		lines = []string{
			fmt.Sprintf("The wars grind on: %s overran %s in battle.", a.Name, d.Name),
			fmt.Sprintf("%s broke through the defenses of %s today.", a.Name, d.Name),
			fmt.Sprintf("%s claimed a hard-won victory over %s.", a.Name, d.Name),
		}
	default:
		lines = []string{
			fmt.Sprintf("%s threw itself at %s and was thrown back.", a.Name, d.Name),
			fmt.Sprintf("%s repelled an assault from %s.", d.Name, a.Name),
			fmt.Sprintf("%s retreated in disarray from the walls of %s.", a.Name, d.Name),
		}
	}
	w.postNews(lines[w.rng.Intn(len(lines))])
}

// postStrikeNews broadcasts a WMD strike (weapon = "nuclear"/"chemical"/
// "biological"), matching BRE's NUKE/CHEM/BIO news categories.
func (w *World) postStrikeNews(a, d *Empire, weapon string) {
	lines := []string{
		fmt.Sprintf("ALERT: %s struck %s with %s weapons!", a.Name, d.Name, weapon),
		fmt.Sprintf("%s unleashed %s fire upon %s.", a.Name, weapon, d.Name),
		fmt.Sprintf("The planet recoils as %s hits %s with %s missiles.", a.Name, d.Name, weapon),
	}
	w.postNews(lines[w.rng.Intn(len(lines))])
}

// postPirateNews broadcasts a pirate-raid outcome (BRE PIRATEWIN/PIRATELOSS).
func (w *World) postPirateNews(a *Empire, faction string, won bool) {
	var lines []string
	if won {
		lines = []string{
			fmt.Sprintf("%s drove off the %s in a daring raid.", a.Name, faction),
			fmt.Sprintf("The %s were bloodied by %s today.", faction, a.Name),
		}
	} else {
		lines = []string{
			fmt.Sprintf("%s's raid on the %s ended in humiliation.", a.Name, faction),
			fmt.Sprintf("The %s repelled %s with ease.", faction, a.Name),
		}
	}
	w.postNews(lines[w.rng.Intn(len(lines))])
}

// postRiotNews broadcasts tax riots in an empire (BRE RIOTS).
func (w *World) postRiotNews(e *Empire) {
	lines := []string{
		fmt.Sprintf("Riots erupt in %s over crushing taxes!", e.Name),
		fmt.Sprintf("The people of %s take to the streets against high taxes.", e.Name),
	}
	w.postNews(lines[w.rng.Intn(len(lines))])
}
