package game

import "fmt"

// Random per-empire "while you were away" events, in the spirit of BRE's
// events.dat file (see docs/mechanics-reference.md). Wording here is
// original, not copied from the original's data file.

// RandomEventChancePct is the odds (per empire, per maintenance day) that a
// random event fires. Tunable.
const RandomEventChancePct = 25

// eventResource names one of the seven resources a random event can move.
type eventResource int

const (
	eventTroopers eventResource = iota
	eventJets
	eventTurrets
	eventTanks
	eventAgents
	eventFood
	eventPeople
	numEventResources
)

// magRange is the inclusive [Min, Max] a random event's magnitude is drawn
// from for one resource.
type magRange struct{ Min, Max int }

// eventMagnitude gives the roll range per resource. Food and People move in
// bigger chunks than the military/agent counts.
var eventMagnitude = [numEventResources]magRange{
	eventTroopers: {1, 20},
	eventJets:     {1, 5},
	eventTurrets:  {1, 5},
	eventTanks:    {1, 3},
	eventAgents:   {1, 3},
	eventFood:     {50, 500},
	eventPeople:   {10, 200},
}

// eventGainLines and eventLoseLines hold 2-4 original one-line variants per
// resource, each with a single %d for the magnitude.
var eventGainLines = [numEventResources][]string{
	eventTroopers: {
		"%d wandering mercenaries pledge themselves to your banner.",
		"%d refugees take up arms and swell your ranks.",
		"%d deserters from a rival warband defect to you.",
	},
	eventJets: {
		"Scavengers restore %d derelict jets found in the wastes.",
		"A salvage crew hands over %d flyable jets.",
	},
	eventTurrets: {
		"Your engineers finish %d salvaged turret emplacements.",
		"%d turrets are recovered from an old battlefield and repaired.",
	},
	eventTanks: {
		"A wandering mechanic crew restores %d tanks to running order.",
		"%d abandoned tanks are towed in and returned to service.",
	},
	eventAgents: {
		"%d disaffected spies from a rival barony offer their services.",
		"%d agents-in-training complete their work early.",
		"%d informants come forward and join your network.",
	},
	eventFood: {
		"A traveling caravan trades you %d units of food at a fair price.",
		"Foragers return with %d units of food from the wastes.",
		"A bumper harvest yields %d extra units of food.",
	},
	eventPeople: {
		"%d settlers, drawn by tales of your barony, arrive seeking work.",
		"%d refugees from a fallen realm ask for your protection.",
		"A wandering tribe of %d joins your people.",
	},
}

var eventLoseLines = [numEventResources]([]string){
	eventTroopers: {
		"A fever sweeps the barracks — %d troopers are lost.",
		"%d troopers desert in the night.",
		"An ambush in the wastes costs you %d troopers.",
	},
	eventJets: {
		"A fuel-line fire grounds and destroys %d jets.",
		"%d jets are lost to mechanical failure over the wastes.",
	},
	eventTurrets: {
		"Corrosion and neglect take %d turrets out of service.",
		"%d turret emplacements collapse and are scrapped.",
	},
	eventTanks: {
		"%d tanks break down beyond repair.",
		"A minefield left from an old war claims %d tanks.",
	},
	eventAgents: {
		"%d agents vanish without a trace.",
		"A rival's counter-intelligence quietly turns %d of your agents.",
		"%d agents are lost crossing hostile territory.",
	},
	eventFood: {
		"Rot and vermin claim %d units of stored food.",
		"A spoiled shipment costs you %d units of food.",
		"Scavengers raid your stores, making off with %d units of food.",
	},
	eventPeople: {
		"A sickness moves through the barony, claiming %d lives.",
		"%d people wander off into the wastes and are never seen again.",
		"An accident at the mines kills %d people.",
	},
}

// resourcePtr returns a pointer to e's field for the given resource, so
// callers can read/adjust it in place.
func resourcePtr(e *Empire, r eventResource) *int {
	switch r {
	case eventTroopers:
		return &e.Troopers
	case eventJets:
		return &e.Jets
	case eventTurrets:
		return &e.Turrets
	case eventTanks:
		return &e.Tanks
	case eventAgents:
		return &e.Agents
	case eventFood:
		return &e.Food
	case eventPeople:
		return &e.People
	default:
		panic("resourcePtr: unknown eventResource")
	}
}

// maybeRandomEvent fires at most one "while you were away" random event for
// e, with probability RandomEventChancePct. A "lose" category is skipped
// when the resource is already 0 (a player is never told they lost units
// they don't have), and the delta is clamped so the resource never goes
// below 0. All randomness draws from w.rng for determinism.
func maybeRandomEvent(w *World, e *Empire) {
	if w.rng.Intn(100) >= RandomEventChancePct {
		return
	}

	r := eventResource(w.rng.Intn(int(numEventResources)))
	gain := w.rng.Intn(2) == 0
	ptr := resourcePtr(e, r)
	if !gain && *ptr <= 0 {
		// Nothing to lose here; skip this roll rather than pick another
		// category (keeps the fire chance == RandomEventChancePct exactly).
		return
	}

	rng := eventMagnitude[r]
	amount := rng.Min + w.rng.Intn(rng.Max-rng.Min+1)

	var lines []string
	if gain {
		lines = eventGainLines[r]
	} else {
		lines = eventLoseLines[r]
		if amount > *ptr {
			amount = *ptr
		}
	}

	if gain {
		*ptr += amount
	} else {
		*ptr -= amount
	}

	line := lines[w.rng.Intn(len(lines))]
	e.Events = append(e.Events, fmt.Sprintf(line, amount))
}
