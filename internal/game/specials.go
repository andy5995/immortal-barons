package game

import "fmt"

const (
	NukeCost = 50000
	ChemCost = 40000
	BioCost  = 40000
)

// NuclearStrike destroys some of the defender's regions, turning them to
// waste. It is a v1 gold-cost strike (no missile inventory yet).
func (w *World) NuclearStrike(a, d *Empire) (string, error) {
	if a.Gold < NukeCost {
		return "", ErrCantAfford
	}
	a.Gold -= NukeCost

	regions := w.jitter(d.Land/10) + 1
	if regions > d.Land {
		regions = d.Land
	}
	d.Land -= regions
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

	people := w.jitter(d.People * 15 / 100)
	troops := w.jitter(d.Troopers * 20 / 100)
	regions := d.Land / 20

	people, troops, regions = clamp(d.People, people), clamp(d.Troopers, troops), clamp(d.Land, regions)
	d.People -= people
	d.Troopers -= troops
	d.Land -= regions

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

	people := w.jitter(d.People * 15 / 100)
	troops := w.jitter(d.Troopers * 20 / 100)

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

// RaidPirates raids NPC pirates: no gold cost and no target empire. The
// attacker gains land and troopers but loses some troopers in the raid.
func (w *World) RaidPirates(a *Empire) string {
	land := w.rng.Intn(8) + 1
	troops := w.rng.Intn(200)
	lost := a.Troopers / 20

	a.Land += land
	a.Troopers += troops
	a.Troopers -= lost

	return fmt.Sprintf("You raided the pirates! Gained %d regions and %d troopers, but lost %d troopers in the fighting.", land, troops, lost)
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
