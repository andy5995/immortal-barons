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

	w.postStrikeNews(a, d, "nuclear")
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

	w.postStrikeNews(a, d, "chemical")
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

	w.postStrikeNews(a, d, "biological")
	report := fmt.Sprintf("Biological strike! %s lost %d people and %d troopers.", d.Name, people, troops)
	if !d.Alive {
		report += fmt.Sprintf("\n%s has been utterly conquered!", d.Name)
	}
	return report, nil
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
