package game

import "fmt"

// PirateFactions are the nine raidable pirate factions. Their strength is
// randomized per game (NOT an easiest-to-hardest ladder), so any faction can
// turn out the strongest.
var PirateFactions = []string{
	"Humans", "Barbarians", "Solarians", "Sharks", "Mechanoids",
	"Rexxogans", "Xandorians", "Monitorians", "Spacians",
}

// PirateFaction is a living pirate band. It raids empires — carrying off a
// small share of their units and being granted new regions by the game (it
// does not steal the victim's regions) — and grows the longer it is ignored.
// Beating it reclaims a portion of its holdings per hit, so draining a fat
// faction takes several attacks.
type PirateFaction struct {
	Name         string
	Forces       int // combat strength (its defense when attacked)
	Land         int // regions it holds (game-granted on raids; capturable)
	LootTroopers int // units taken from victims, reclaimable by attackers
	LootJets     int
	LootTanks    int
}

// Pirate tuning (reconstructed; exact rates are compiled into BRE, not in its
// strings — see docs/mechanics-reference.md).
const (
	pirateForcesMin   = 150 // seed-strength range, randomized (no faction ladder)
	pirateForcesMax   = 600
	pirateSeedLandMax = 40
	PirateRaidChance  = 35 // % chance a faction raids on a given maintenance day
	PirateRaidUnitPct = 5  // % of a victim's units a raid carries off
	PirateRaidLandMax = 60 // up to this many regions the game grants a raiding faction
	PirateReclaimPct  = 25 // % of a faction's holdings reclaimed per winning hit (~4-5 hits to drain)
	pirateRaidLossPct = 15 // % of committed force lost on a failed raid
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

// piratesRaid runs one day of pirate activity: each faction may raid a random
// living empire.
func (w *World) piratesRaid() {
	var victims []*Empire
	for _, e := range w.Empires {
		if e.Alive {
			victims = append(victims, e)
		}
	}
	if len(victims) == 0 {
		return
	}
	for i := range w.Pirates {
		if w.rng.Intn(100) >= PirateRaidChance {
			continue
		}
		w.pirateRaidVictim(&w.Pirates[i], victims[w.rng.Intn(len(victims))])
	}
}

// pirateRaidVictim carries off a small share of v's units into p's loot and
// grows p (the game grants it new regions; it does not take v's regions).
func (w *World) pirateRaidVictim(p *PirateFaction, v *Empire) {
	tookT := v.Troopers * PirateRaidUnitPct / 100
	tookJ := v.Jets * PirateRaidUnitPct / 100
	tookK := v.Tanks * PirateRaidUnitPct / 100
	v.Troopers -= tookT
	v.Jets -= tookJ
	v.Tanks -= tookK
	p.LootTroopers += tookT
	p.LootJets += tookJ
	p.LootTanks += tookK
	p.Forces += tookT + tookJ*2 + tookK*4
	p.Land += w.rng.Intn(PirateRaidLandMax) + 1 // game-granted, not stolen from v
	// Only humans read the notice; the raid itself still hits AI victims.
	if tookT+tookJ+tookK > 0 && v.Owner != "" {
		v.PirateRaids = append(v.PirateRaids, fmt.Sprintf(
			"The %s pirates raided you, carrying off %d troopers, %d jets, and %d tanks!",
			p.Name, tookT, tookJ, tookK))
	}
}

// RaidFaction resolves a player's attack on a pirate faction. The attacker
// commits troopers/jets/tanks (each clamped to what it owns) against the
// faction's current (random, possibly grown) Forces. On a win the attacker
// reclaims a portion of the faction's loot and regions and weakens it — so
// draining a large faction takes several hits. On a loss the attacker loses a
// fraction of the committed force.
func (w *World) RaidFaction(a *Empire, faction, troopers, jets, tanks int) string {
	if faction < 0 || faction >= len(w.Pirates) {
		return "There is no such pirate faction."
	}
	p := &w.Pirates[faction]

	troopers = clamp(a.Troopers, troopers)
	jets = clamp(a.Jets, jets)
	tanks = clamp(a.Tanks, tanks)

	offense := w.jitter(troopers + jets*2 + tanks*4)
	defense := w.jitter(p.Forces)

	if offense > defense {
		gotT := p.LootTroopers * PirateReclaimPct / 100
		gotJ := p.LootJets * PirateReclaimPct / 100
		gotK := p.LootTanks * PirateReclaimPct / 100
		gotLand := p.Land * PirateReclaimPct / 100
		p.LootTroopers -= gotT
		p.LootJets -= gotJ
		p.LootTanks -= gotK
		p.Land -= gotLand
		p.Forces -= p.Forces * PirateReclaimPct / 100

		a.Troopers += gotT
		a.Jets += gotJ
		a.Tanks += gotK
		if gotLand > 0 {
			a.Regions.Coastal += gotLand
			a.syncLand()
		}
		return fmt.Sprintf("You broke the %s and recovered %d troopers, %d jets, %d tanks, and %d regions.",
			p.Name, gotT, gotJ, gotK, gotLand)
	}

	tLost := troopers * pirateRaidLossPct / 100
	jLost := jets * pirateRaidLossPct / 100
	kLost := tanks * pirateRaidLossPct / 100
	a.Troopers -= tLost
	a.Jets -= jLost
	a.Tanks -= kLost
	return fmt.Sprintf("You could not break the %s. You lost %d Troopers, %d Jets, and %d Tanks.",
		p.Name, tLost, jLost, kLost)
}
