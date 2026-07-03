package game

import (
	"fmt"
	"strings"
)

const (
	NukeCost = 50000
	ChemCost = 40000
	BioCost  = 40000

	GooieCost = 500000
	SDIStep   = 10000 // gold per +1% SDI
	SDIMax    = 75
)

// FundSDI raises e's SDI defense level. Only whole SDIStep chunks of gold
// are spent (leftover gold under SDIStep is not charged), and the level is
// capped at SDIMax. Returns the new SDI level.
func (w *World) FundSDI(e *Empire, gold int) (int, error) {
	if e.Gold < gold {
		return 0, ErrCantAfford
	}
	steps := gold / SDIStep
	if e.SDI+steps > SDIMax {
		steps = SDIMax - e.SDI
	}
	e.Gold -= steps * SDIStep
	e.SDI += steps
	return e.SDI, nil
}

// NuclearStrike destroys some of the defender's regions, turning them to
// waste. It is a v1 gold-cost strike (no missile inventory yet).
func (w *World) NuclearStrike(a, d *Empire) (string, error) {
	if a.Gold < NukeCost {
		return "", ErrCantAfford
	}
	a.Gold -= NukeCost

	regions := w.jitter(d.Land/10) + 1
	regions = regions * (100 - d.SDI) / 100
	if regions > d.Land {
		regions = d.Land
	}
	d.Regions.remove(regions)
	d.syncLand()
	if d.Land <= 0 || d.People <= 0 {
		d.Alive = false
	}

	d.Events = append(d.Events, fmt.Sprintf("%s hit you with a nuclear strike: %d regions reduced to waste.", a.Name, regions))

	report := fmt.Sprintf("Nuclear strike! %d regions of %s reduced to waste.", regions, d.Name)
	if !d.Alive {
		report += fmt.Sprintf("\n%s has been utterly conquered!", d.Name)
	}
	return report, nil
}

// ChemicalStrike kills people and troopers and damages some land.
func (w *World) ChemicalStrike(a, d *Empire) (string, error) {
	if a.Gold < ChemCost {
		return "", ErrCantAfford
	}
	a.Gold -= ChemCost

	people := w.jitter(d.People*15/100) * (100 - d.SDI) / 100
	troops := w.jitter(d.Troopers*20/100) * (100 - d.SDI) / 100
	regions := d.Land / 20 * (100 - d.SDI) / 100

	people, troops, regions = clamp(d.People, people), clamp(d.Troopers, troops), clamp(d.Land, regions)
	d.People -= people
	d.Troopers -= troops
	d.Regions.remove(regions)
	d.syncLand()

	if d.Land <= 0 || d.People <= 0 {
		d.Alive = false
	}

	d.Events = append(d.Events, fmt.Sprintf("%s hit you with a chemical strike: %d people, %d troopers, and %d regions lost.", a.Name, people, troops, regions))

	report := fmt.Sprintf("Chemical strike! %s lost %d people, %d troopers, and %d regions.", d.Name, people, troops, regions)
	if !d.Alive {
		report += fmt.Sprintf("\n%s has been utterly conquered!", d.Name)
	}
	return report, nil
}

// BiologicalStrike kills people and troopers but leaves land untouched.
func (w *World) BiologicalStrike(a, d *Empire) (string, error) {
	if a.Gold < BioCost {
		return "", ErrCantAfford
	}
	a.Gold -= BioCost

	people := w.jitter(d.People*15/100) * (100 - d.SDI) / 100
	troops := w.jitter(d.Troopers*20/100) * (100 - d.SDI) / 100

	people, troops = clamp(d.People, people), clamp(d.Troopers, troops)
	d.People -= people
	d.Troopers -= troops

	if d.People <= 0 {
		d.Alive = false
	}

	d.Events = append(d.Events, fmt.Sprintf("%s hit you with a biological strike: %d people and %d troopers lost.", a.Name, people, troops))

	report := fmt.Sprintf("Biological strike! %s lost %d people and %d troopers.", d.Name, people, troops)
	if !d.Alive {
		report += fmt.Sprintf("\n%s has been utterly conquered!", d.Name)
	}
	return report, nil
}

// PirateFactions lists the raidable pirate factions, weakest first.
var PirateFactions = []string{
	"Humans", "Barbarians", "Solarians", "Sharks", "Mechanoids",
	"Rexxogans", "Xandorians", "Monitorians", "Spacians",
}

// pirateFactionBaseDefense is the v1 defense model: base 200, scaling up
// 50% per faction index so Humans (0) are easiest and Spacians (8) are
// hardest (tunable — see docs/mechanics-reference.md).
const pirateFactionBaseDefense = 200

// pirateRaidLossPct is the fraction of committed force lost on a failed
// raid (v1 tunable).
const pirateRaidLossPct = 15

// RaidFaction resolves a pirate raid against the given faction index
// (0-based, into PirateFactions). The attacker commits troopers/jets/tanks
// (each clamped to what it owns); harder factions have stronger defense.
// On success the attacker gains land (and a little plundered military); on
// failure it loses a fraction of the committed force. Returns a report
// string.
func (w *World) RaidFaction(a *Empire, faction, troopers, jets, tanks int) string {
	name := "Unknown"
	if faction >= 0 && faction < len(PirateFactions) {
		name = PirateFactions[faction]
	}

	troopers = clamp(a.Troopers, troopers)
	jets = clamp(a.Jets, jets)
	tanks = clamp(a.Tanks, tanks)

	offense := w.jitter(troopers + jets*2 + tanks*4)
	defense := w.jitter(pirateFactionBaseDefense * (2 + faction) / 2)

	if offense > defense {
		land := w.rng.Intn(8) + 3
		troopsGained := w.rng.Intn(100)

		a.Regions.Coastal += land
		a.syncLand()
		a.Troopers += troopsGained

		return fmt.Sprintf("You raided the %s and won! Gained %d regions and %d troopers.", name, land, troopsGained)
	}

	tLost := troopers * pirateRaidLossPct / 100
	jLost := jets * pirateRaidLossPct / 100
	kLost := tanks * pirateRaidLossPct / 100
	a.Troopers -= tLost
	a.Jets -= jLost
	a.Tanks -= kLost

	return fmt.Sprintf("You could not successfully raid the %s. You lost %d Troopers, %d Jets, and %d Tanks.", name, tLost, jLost, kLost)
}

// GooieKablooie is a planet-wide superweapon: for a very high cost it
// destroys ~10% of EVERY other living empire's land at once (reduced by each
// target's SDI). v1 simplification of the original's multi-day build/decay.
func (w *World) GooieKablooie(a *Empire) (string, error) {
	if a.Gold < GooieCost {
		return "", ErrCantAfford
	}
	a.Gold -= GooieCost
	var b strings.Builder
	b.WriteString("Your Gooie Kablooie detonates across the planet!\n")
	for _, d := range w.Empires {
		if d == a || !d.Alive {
			continue
		}
		regions := d.Land / 10 * (100 - d.SDI) / 100
		if regions < 1 {
			regions = 1
		}
		if regions > d.Land {
			regions = d.Land
		}
		d.Regions.remove(regions)
		d.syncLand()
		if d.Land <= 0 || d.People <= 0 {
			d.Alive = false
		}
		d.Events = append(d.Events, fmt.Sprintf("A Gooie Kablooie struck the planet — you lost %d regions.", regions))
		fmt.Fprintf(&b, "  %s lost %d regions.\n", d.Name, regions)
	}
	return b.String(), nil
}

// clamp caps n so subtracting it from total never goes negative.
func clamp(total, n int) int {
	if n > total {
		return total
	}
	if n < 0 {
		return 0
	}
	return n
}
