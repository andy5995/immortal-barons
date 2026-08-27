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
	w.NewsToday = append(w.NewsToday, line)
	if len(w.NewsToday) > 20 {
		w.NewsToday = w.NewsToday[len(w.NewsToday)-20:]
	}
}

// noteSysop records a transport fault for whoever runs the game. See
// World.SysopNotices for why these do not go to the planet's news.
func (w *World) noteSysop(format string, a ...any) {
	w.SysopNotices = append(w.SysopNotices, fmt.Sprintf(format, a...))
}

// NoteProtocolHold records that a board's packets are being set aside because
// this build cannot read their format, once per board per run rather than once
// per packet — a mismatch affects every packet that board sends, and repeating
// it per file buries the one line that matters.
func (w *World) NoteProtocolHold(board string, protocol int) {
	if w.heldNoted == nil {
		w.heldNoted = map[string]bool{}
	}
	if w.heldNoted[board] {
		return
	}
	w.heldNoted[board] = true
	// Which way the mismatch runs decides whether waiting fixes it, and the
	// sysop's next move differs completely: upgrading this board releases a
	// newer board's packets, while an older board's are held by a format this
	// build has already moved past and no upgrade of theirs brings back. Saying
	// "when both boards run the same release" for both was true only of the
	// first (#228 review).
	if protocol > Protocol {
		w.noteSysop("Packets from %s are being held: they speak protocol %d and this board speaks %d. Upgrading this board applies them.",
			board, protocol, Protocol)
		return
	}
	w.noteSysop("Packets from %s are being held: they speak protocol %d, which this board (protocol %d) has moved past. They will NOT be applied on their own, even once that board upgrades — ask them to resend anything that mattered.",
		board, protocol, Protocol)
}

// postCombatNews broadcasts the outcome of a regular attack to the planet.
func (w *World) postCombatNews(a, d *Empire, won, conquered bool) {
	// Every conventional battle funnels through here, so this is the honest place
	// to tally them for the -spectate balance probe. Counting them by matching the
	// news prose below would break the moment the wording changes.
	w.BattlesTotal++
	if conquered {
		w.ConquestsTotal++
	}
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
			fmt.Sprintf("%s got thrashed by %s.", d.Name, a.Name),
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

// postCivilWarNews broadcasts a realm collapsing into civil war after a famine.
func (w *World) postCivilWarNews(e *Empire) {
	lines := []string{
		fmt.Sprintf("Civil war breaks out in %s as the hungry turn on the crown.", e.Name),
		fmt.Sprintf("The realm of %s tears itself apart over empty food stores.", e.Name),
	}
	w.postNews(lines[w.rng.Intn(len(lines))])
}

// postStarvationNews broadcasts an empire's food shortfall.
func (w *World) postStarvationNews(e *Empire) {
	lines := []string{
		fmt.Sprintf("Famine grips %s as food stocks run out.", e.Name),
		fmt.Sprintf("Reports of starvation reach the planet from %s.", e.Name),
	}
	w.postNews(lines[w.rng.Intn(len(lines))])
}

// postInvestRateNews broadcasts a change in the planetary investment rate
// (BRE's daily bank-rate float). No line posts when the rate did not move.
func (w *World) postInvestRateNews(before int) {
	switch {
	case w.InvestRate > before:
		w.postNews(fmt.Sprintf("The planetary investment rate rose to %s%%.", PctTenths(w.InvestRate)))
	case w.InvestRate < before:
		w.postNews(fmt.Sprintf("The planetary investment rate fell to %s%%.", PctTenths(w.InvestRate)))
	}
}

// postMasterNews broadcasts the planet's political standing: the empire
// with the highest net worth among the living either keeps or claims the
// title of Planetary Master, and CurrentMaster is kept in sync with it. This
// runs every maintenance day (matching BRE, which shows the title daily),
// separate from endGame's one-time crowning of LastMaster at a league's end.
func (w *World) postMasterNews() {
	best := ""
	bestNW := 0
	found := false
	for _, e := range w.Empires {
		if e.Alive {
			if nw := w.NetWorth(e); !found || nw > bestNW {
				bestNW = nw
				best = e.Name
				found = true
			}
		}
	}
	if !found {
		return
	}
	switch {
	case best == w.CurrentMaster:
		w.postNews(fmt.Sprintf("%s retains the title of Planetary Master.", best))
	case w.CurrentMaster == "":
		w.postNews(fmt.Sprintf("%s claims the title of Planetary Master!", best))
	default:
		w.postNews(fmt.Sprintf("%s has seized the title of Planetary Master from %s!", best, w.CurrentMaster))
	}
	w.CurrentMaster = best
}
