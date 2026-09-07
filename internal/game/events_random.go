package game

import "fmt"

// Random per-empire "while you were away" events. The MECHANIC is BRE's,
// binary-verified from resolve_random_game_event (BRE.OVR ovr_00dde0 +0x05d3);
// the WORDING is IB's own, because the original keeps its lines in a data file
// (GAME\\EVENTS.DAT) and they are its expression, not its rules.
//
// See docs/mechanics-reference.md, "Random events".

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

// eventMagnitudePct is the share of what a realm ALREADY HOLDS that one event
// moves, per resource. BINARY-VERIFIED: the original stores these as one byte
// per resource in a ten-byte record beside the resource's name, at DGROUP
// offset 0x476 (`BRE.EXE`, DS base 0x148a0), and reads the percent at +9.
//
// A share rather than a flat count is the whole character of the mechanic. A
// fixed band cannot serve a realm holding four hundred troopers and one holding
// four hundred thousand: IB drew troopers from 1-20 until 2026-09-08, so a
// mature realm's "event" moved a rounding error and the feature was invisible
// past the first day.
var eventMagnitudePct = [numEventResources]int{
	eventTroopers: 3,
	eventJets:     5,
	eventTurrets:  5,
	eventTanks:    7,
	eventAgents:   5,
	eventFood:     15,
	eventPeople:   4,
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

// maybeRandomEvent fires at most one random event for e at the END OF A TURN.
// BINARY-VERIFIED shape (resolve_random_game_event): a realm under New Realm
// Protection is skipped outright, the event fires on RandomEventChancePct of
// turns, then one of the seven resources and one of gain/lose are drawn flat.
// The amount is a share of what is held, and an amount that truncates to zero
// means nothing happens at all — which is the original's own way of leaving a
// realm with little of something alone.
func maybeRandomEvent(w *World, e *Empire) {
	// A protected realm is left out: the original tests is_under_protection
	// before it even rolls (+0x05e6).
	if e.Protection > 0 {
		return
	}
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

	// trunc(held x pct / 100); zero means the event does not happen.
	amount := *ptr * eventMagnitudePct[r] / 100
	if amount <= 0 {
		return
	}

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
	e.addEvent(fmt.Sprintf(line, amount))
}
