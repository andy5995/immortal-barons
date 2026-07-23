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
	Gold         int
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
func pirateTake(have int) int {
	t := have * PirateRaidUnitPct / 100
	if t > PirateRaidMaxTake {
		t = PirateRaidMaxTake
	}
	return t
}

// capAdd adds delta to cur, clamped to cap.
func capAdd(cur, delta, cap int) int {
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

// lootList joins the non-zero amounts into a readable phrase.
func lootList(troopers, jets, turrets, tanks, agents, gold int) string {
	var parts []string
	add := func(n int, label string) {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, label))
		}
	}
	add(troopers, "troopers")
	add(jets, "jets")
	add(turrets, "turrets")
	add(tanks, "tanks")
	add(agents, "agents")
	add(gold, "gold")
	return strings.Join(parts, ", ")
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
		gotG := p.Gold * PirateReclaimPct / 100
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
		a.Gold += gotG
		// The captured land is DEFERRED, not auto-added: the caller opens the
		// region-type picker so the player chooses the composition (#21). Reclaimed
		// gold/military above land immediately; only the regions wait.
		capturedLand = gotLand
		loot := lootList(gotT, gotJ, gotU, gotK, gotA, gotG)
		if gotLand > 0 {
			if loot != "" {
				loot += ", "
			}
			loot += fmt.Sprintf("%d regions", gotLand)
		}
		if loot == "" {
			loot = "nothing of value"
		}
		addScore(a, startForces/PirateScoreDivisor)
		w.postPirateNews(a, p.Name, true)
		return fmt.Sprintf("You broke the %s and recovered %s.", p.Name, loot), capturedLand
	}

	tLost := troopers * pirateRaidLossPct / 100
	jLost := jets * pirateRaidLossPct / 100
	kLost := tanks * pirateRaidLossPct / 100
	a.Troopers -= tLost
	a.Jets -= jLost
	a.Tanks -= kLost
	addScore(a, -startForces/PirateScoreDivisor)
	w.postPirateNews(a, p.Name, false)
	return fmt.Sprintf("You could not break the %s. You lost %d Troopers, %d Jets, and %d Tanks.",
		p.Name, tLost, jLost, kLost), 0
}
