package game

import (
	"fmt"
	"strings"
)

// PirateFactions are the nine raidable pirate factions. Their strength is
// randomized per game (NOT an easiest-to-hardest ladder), so any faction can
// turn out the strongest.
// The three generic words (Humans, Barbarians, Sharks) are kept; the rest are
// IB-original names, mostly adapted from Devonian sea life — fitting, since the
// kept "Sharks" are themselves Devonian survivors — plus Nightjackals. BRE's
// distinctive faction names are its own creative expression, so we don't reuse
// them (clean-room — see the bre-gather skill's license notes).
var PirateFactions = []string{
	"Humans", "Barbarians", "Nightjackals", "Sharks", "Dunkleoids",
	"Trilobarians", "Raptorians", "Gorgonoids", "Ammonians",
}

// PirateFaction is a living pirate band. It raids empires — carrying off a
// share of their units and gold and being granted new regions by the game (it
// does not steal the victim's regions) — and grows the longer it is ignored.
// Beating it reclaims a portion of its holdings per hit, so draining a fat
// faction takes several attacks. Pirates never take bombers or carriers.
type PirateFaction struct {
	Name         string
	Forces       int // combat strength (its defense when attacked)
	Land         int // regions it holds (game-granted on raids; capturable)
	Gold         int64
	LootTroopers int
	LootJets     int
	LootTurrets  int
	LootTanks    int
	LootAgents   int
}

// Pirate tuning. Steal rates/chances are reconstructed; the take cap and
// several holding caps are cross-checked against BRE.EXE's constants table
// (marked "binary" below); the rest are reconstructed from play data (see
// docs/mechanics-reference.md).
const (
	pirateForcesMin         = 150 // seed-strength range, randomized (no faction ladder)
	pirateForcesMax         = 600
	pirateSeedLandMax       = 20
	PirateRaidChancePerTurn = 20    // % chance an empire suffers a pirate raid on a given turn (BRE ~1-in-5, random)
	PirateSecondRaidChance  = 5     // % chance of a SECOND raid by a DIFFERENT faction, once a first has landed
	PirateRaidUnitPct       = 5     // % of a victim's holdings a raid carries off
	PirateRaidMaxTake       = 24999 // binary: constant is 25000; observed max take is 24999
	PirateRaidLandMax       = 15    // regions the game grants a raiding faction (grows toward the cap)
	PirateReclaimPct        = 20    // % of on-hand reclaimed per winning hit (~a fifth; several hits to drain)
	pirateRaidLossPct       = 15    // % of committed force lost on a failed raid

	// Hard caps on faction holdings. No bombers/carriers (absent from the caps).
	// Verified against the BRE.EXE caps table at 0x14ede
	// (5000,25000,50000,80000,80000,100000,100000,200000,600000): the clone's
	// earlier 300000 gold cap is absent from the binary; 600000 is present.
	PirateCapGold     = 600_000 // binary (caps table); was a guessed 300000
	PirateCapRegions  = 100     // play-data estimate (not a distinctive binary constant)
	PirateCapAgents   = 65_000  // binary (BRE.EXE ×3)
	PirateCapTroopers = 100_000 // binary
	PirateCapJets     = 100_000 // binary
	PirateCapTurrets  = 100_000 // binary
	PirateCapTanks    = 50_000  // binary
)

// seedPirates creates the nine factions with randomized strength.
func (w *World) seedPirates() {
	w.Pirates = make([]PirateFaction, len(PirateFactions))
	for i, name := range PirateFactions {
		w.Pirates[i] = PirateFaction{
			Name:   name,
			Forces: pirateForcesMin + w.rng.Intn(pirateForcesMax-pirateForcesMin),
			Land:   w.rng.Intn(pirateSeedLandMax),
		}
	}
}

// EnsurePirates seeds the factions after loading a save that predates them.
func (w *World) EnsurePirates() {
	if len(w.Pirates) == 0 {
		w.seedPirates()
	}
}

// pirateTake is how much of a holding of size `have` a single raid carries
// off: a small percentage, but never more than the per-raid take cap.
func pirateTake[T number](have T) T {
	t := pctOf(have, PirateRaidUnitPct)
	if t > PirateRaidMaxTake {
		t = PirateRaidMaxTake
	}
	return t
}

// capAdd adds delta to cur, clamped to cap.
func capAdd[T number](cur, delta, cap T) T {
	cur += delta
	if cur > cap {
		cur = cap
	}
	return cur
}

// maybePirateRaid gives one empire a per-turn chance of suffering a pirate raid
// (#21 — BRE raids hit ~1-in-5 turns, randomly, not once per day): a
// PirateRaidChancePerTurn% roll for a raid by a random faction, and — only if
// that lands — a further PirateSecondRaidChance% roll for a SECOND raid by a
// DIFFERENT faction the same turn. New-realm protection wards it off. Called
// from PlayTurn so it applies uniformly to human and AI empires; a human
// victim's notice surfaces at the next turn's income (BRE's "since your last
// play" timing).
func (w *World) maybePirateRaid(e *Empire) {
	if !e.Alive || e.Protection > 0 || len(w.Pirates) == 0 {
		return
	}
	if w.rng.Intn(100) >= PirateRaidChancePerTurn {
		return
	}
	first := w.rng.Intn(len(w.Pirates))
	w.pirateRaidVictim(&w.Pirates[first], e)
	if len(w.Pirates) > 1 && w.rng.Intn(100) < PirateSecondRaidChance {
		// Pick a DIFFERENT faction: step past `first` into the remaining ones.
		second := (first + 1 + w.rng.Intn(len(w.Pirates)-1)) % len(w.Pirates)
		w.pirateRaidVictim(&w.Pirates[second], e)
	}
}

// pirateRaidVictim carries off a share of v's units, agents, and gold into p's
// hoard (never bombers or carriers) and grows p; the game grants it new
// regions rather than taking v's. All holdings are clamped to their caps.
func (w *World) pirateRaidVictim(p *PirateFaction, v *Empire) {
	tookT := pirateTake(v.Troopers)
	tookJ := pirateTake(v.Jets)
	tookU := pirateTake(v.Turrets)
	tookK := pirateTake(v.Tanks)
	tookA := pirateTake(v.Agents)
	tookG := pirateTake(v.Gold)

	v.Troopers -= tookT
	v.Jets -= tookJ
	v.Turrets -= tookU
	v.Tanks -= tookK
	v.Agents -= tookA
	v.Gold -= tookG

	p.LootTroopers = capAdd(p.LootTroopers, tookT, PirateCapTroopers)
	p.LootJets = capAdd(p.LootJets, tookJ, PirateCapJets)
	p.LootTurrets = capAdd(p.LootTurrets, tookU, PirateCapTurrets)
	p.LootTanks = capAdd(p.LootTanks, tookK, PirateCapTanks)
	p.LootAgents = capAdd(p.LootAgents, tookA, PirateCapAgents)
	p.Gold = capAdd(p.Gold, tookG, PirateCapGold)
	p.Forces += tookT + tookJ*2 + tookU*2 + tookK*4
	p.Land = capAdd(p.Land, w.rng.Intn(PirateRaidLandMax)+1, PirateCapRegions)

	// Only humans read the notice; the raid itself still hits AI victims.
	if v.Owner != "" {
		if loot := lootList(tookT, tookJ, tookU, tookK, tookA, tookG); loot != "" {
			v.PirateRaids = append(v.PirateRaids, fmt.Sprintf(
				"The %s raided you, carrying off %s!", p.Name, loot))
		}
	}
}

// lootList joins the non-zero amounts into a readable phrase. Used for the raid
// a player SUFFERS, which BRE reports nowhere — the wording is IB's own.
func lootList(troopers, jets, turrets, tanks, agents int, gold int64) string {
	var parts []string
	add := func(n int64, label string) {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, label))
		}
	}
	add(int64(troopers), "troopers")
	add(int64(jets), "jets")
	add(int64(turrets), "turrets")
	add(int64(tanks), "tanks")
	add(int64(agents), "agents")
	add(gold, "gold")
	return strings.Join(parts, ", ")
}

// breTally renders a raid tally the way BRE writes one: capitalised unit names
// in a fixed order, comma separated, with "and" before the last. Zero entries
// are kept, so the same fields appear every time and a missing one means the
// unit does not take part rather than that none were lost.
//
//	You took 78000 Gold, 8 Regions, 525 Troopers, 481 Jets, 606 Turrets, and 175 Tanks.
func breTally(parts []string) string {
	switch len(parts) {
	case 0:
		return "nothing"
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
}

// raidLoot is the "You took" tally. The order is BRE's, from the captured
// screen in docs/dev/bre-screens.md. Regions are omitted when the faction holds
// no land, which that capture records as BRE's own behaviour; Agents have no
// BRE counterpart and follow the units it does list.
func raidLoot(gold int64, regions, troopers, jets, turrets, tanks, agents int) string {
	parts := []string{fmt.Sprintf("%d Gold", gold)}
	if regions > 0 {
		parts = append(parts, fmt.Sprintf("%d Regions", regions))
	}
	parts = append(parts,
		fmt.Sprintf("%d Troopers", troopers),
		fmt.Sprintf("%d Jets", jets),
		fmt.Sprintf("%d Turrets", turrets),
		fmt.Sprintf("%d Tanks", tanks))
	if agents > 0 {
		parts = append(parts, fmt.Sprintf("%d Agents", agents))
	}
	return breTally(parts)
}

// raidWinLines and raidFailLines are the headlines a raid may draw, picked at
// random so a player raiding the same faction all day is not told the same
// sentence every time. The first of each is BRE's own, verbatim from BRE.OVR;
// the rest are IB's, kept to its register and near enough its length that no one
// line stands out as the "real" one — and short enough not to force a wrap.
var raidWinLines = []string{
	"Your efforts against %s have brought you success!",
	"Your forces broke the %s and came home loaded.",
	"The %s scattered before your assault.",
	"You overran the %s and took back what you could.",
	"The %s could not hold what they had taken.",
}

var raidFailLines = []string{
	"You could not successfully raid %s.",
	"Your raid on the %s came to nothing.",
	"The %s drove your forces back.",
	"Your attack on the %s was beaten off.",
	"The %s held against everything you sent.",
}

// raidWin renders a winning raid under an already-chosen headline (the caller
// owns the RNG). The losses line appears only when the raid actually cost
// something; see the note at the call site.
func raidWin(headline, loot string, troopers, jets, tanks int) string {
	report := fmt.Sprintf("%s\nYou took %s.", headline, loot)
	if troopers > 0 || jets > 0 || tanks > 0 {
		report += fmt.Sprintf("\nYou lost %s.", raidLosses(troopers, jets, tanks))
	}
	return report
}

// raidLosses is the "You lost" tally — the three types a raid may commit.
func raidLosses(troopers, jets, tanks int) string {
	return breTally([]string{
		fmt.Sprintf("%d Troopers", troopers),
		fmt.Sprintf("%d Jets", jets),
		fmt.Sprintf("%d Tanks", tanks),
	})
}

// RaidFaction resolves a player's attack on a pirate faction. The attacker
// commits troopers/jets/tanks (each clamped to what it owns) against the
// faction's current (random, possibly grown) Forces. On a win the attacker
// reclaims a portion of the faction's hoard and regions and weakens it — so
// draining a large faction takes several hits. On a loss the attacker loses a
// fraction of the committed force.
// The report names the loot; capturedLand is the number of regions won (0 on a
// loss or against a landless faction), which the caller lets the player allocate
// by type through the same picker a Regular Attack uses (#21). Reclaimed gold and
// military land in the attacker at once; only the land is deferred.
func (w *World) RaidFaction(a *Empire, faction, troopers, jets, tanks int) (report string, capturedLand int) {
	if faction < 0 || faction >= len(w.Pirates) {
		return "There is no such pirate faction.", 0
	}
	p := &w.Pirates[faction]
	startForces := p.Forces // battle scale for Score, before any reclaim shrinks it

	troopers = clamp(a.Troopers, troopers)
	jets = clamp(a.Jets, jets)
	tanks = clamp(a.Tanks, tanks)

	offense := w.jitter(troopers + jets*2 + tanks*4)
	defense := w.jitter(p.Forces)

	if offense > defense {
		gotT := p.LootTroopers * PirateReclaimPct / 100
		gotJ := p.LootJets * PirateReclaimPct / 100
		gotU := p.LootTurrets * PirateReclaimPct / 100
		gotK := p.LootTanks * PirateReclaimPct / 100
		gotA := p.LootAgents * PirateReclaimPct / 100
		gotG := pctOf(p.Gold, PirateReclaimPct)
		gotLand := p.Land * PirateReclaimPct / 100

		p.LootTroopers -= gotT
		p.LootJets -= gotJ
		p.LootTurrets -= gotU
		p.LootTanks -= gotK
		p.LootAgents -= gotA
		p.Gold -= gotG
		p.Land -= gotLand
		p.Forces -= p.Forces * PirateReclaimPct / 100

		a.Troopers += gotT
		a.Jets += gotJ
		a.Turrets += gotU
		a.Tanks += gotK
		a.Agents += gotA
		w.creditGold(a, gotG, "reclaimed pirate loot")
		// The captured land is DEFERRED, not auto-added: the caller opens the
		// region-type picker so the player chooses the composition (#21). Reclaimed
		// gold/military above land immediately; only the regions wait.
		capturedLand = gotLand
		addScore(a, startForces/PirateScoreDivisor)
		w.postPirateNews(a, p.Name, true)
		// BRE's wording, verbatim from BRE.OVR ("Your efforts against ",
		// " have brought you success!", "You took ", "You lost "). Three short
		// lines rather than IB's old single sentence — though a full tally can
		// still pass 80 columns, because BRE abbreviates gold ("78k Gold") where
		// IB prints the figure, so the display still wraps it.
		//
		// BRE's captured screen carries a third line, "You lost ...", because a
		// winning raid still costs the attacker there. IB charges a winner
		// nothing, so raidWin leaves the line out rather than print a tally of
		// zeroes — the gap is a mechanic, not wording, and setting a loss rate
		// needs evidence rather than a guess.
		headline := fmt.Sprintf(raidWinLines[w.rng.Intn(len(raidWinLines))], p.Name)
		return raidWin(headline, raidLoot(gotG, gotLand, gotT, gotJ, gotU, gotK, gotA), 0, 0, 0), capturedLand
	}

	tLost := troopers * pirateRaidLossPct / 100
	jLost := jets * pirateRaidLossPct / 100
	kLost := tanks * pirateRaidLossPct / 100
	a.Troopers -= tLost
	a.Jets -= jLost
	a.Tanks -= kLost
	addScore(a, -startForces/PirateScoreDivisor)
	w.postPirateNews(a, p.Name, false)
	return fmt.Sprintf("%s\nYou lost %s.",
		fmt.Sprintf(raidFailLines[w.rng.Intn(len(raidFailLines))], p.Name),
		raidLosses(tLost, jLost, kLost)), 0
}
