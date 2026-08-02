package game

import (
	"errors"
	"fmt"
)

// DoomerKaboomerWeapon is the planet's doomsday weapon — one at a time, funded
// by the barons who live here rather than bought by any one of them. It is IB's
// rename of BRE's Gooie Kablooie, and it goes through the original's whole life:
// a planet starts one against a named target, everyone chips in until it is
// paid for, it flies for a couple of days in plain sight of the planet it is
// aimed at, and it arrives with whatever is left of it after that planet's jets
// have had their turn.
type DoomerKaboomerWeapon struct {
	TargetBoard string // the planet it is aimed at
	Creator     string // owner handle of the baron who started it
	CostMillion int    // total funding needed, in millions of gold
	PaidMillion int    // raised so far
	StartedDay  int
	// Funded and Launched are explicit rather than inferred from their day
	// fields: day 0 is a real game day, so a zero FundedDay cannot mean "not
	// funded yet".
	Funded      bool
	FundedDay   int
	Launched    bool
	LaunchedDay int
	ArrivesDay  int
	Intact      int // percent of the weapon still flying; jets whittle this down
}

var (
	ErrDoomerExists   = errors.New("This planet is already building a Doomer Kaboomer.")
	ErrNoDoomer       = errors.New("This planet has no Doomer Kaboomer.")
	ErrDoomerFunded   = errors.New("The Doomer Kaboomer is already paid for.")
	ErrDoomerUnfunded = errors.New("The Doomer Kaboomer is not paid for yet.")
	ErrDoomerFlying   = errors.New("The Doomer Kaboomer has already launched.")
	ErrDoomerDisabled = errors.New("Doomer Kaboomer operations are switched off in this game.")
	ErrNotYours       = errors.New("Only the baron who started it may do that.")
)

// doomerCost prices a Doomer Kaboomer aimed at targetLand, from a planet holding
// ourLand, in millions of gold. The shape is BRE's (see balance.go).
func doomerCost(targetLand, ourLand int) int {
	if targetLand < DoomerMinTargetLand {
		targetLand = DoomerMinTargetLand
	}
	cost := (targetLand*DoomerCostPerLand + DoomerCostPerLandDenom/2) / DoomerCostPerLandDenom
	cost += DoomerCostBase
	if cost > DoomerCostCap {
		cost = DoomerCostCap
	}
	switch {
	case ourLand <= 0:
		// Nothing to compare against; treat the target as overwhelming.
		cost = cost * DoomerSurchargeHugePct / 100
	case targetLand > ourLand*4:
		cost = cost * DoomerSurchargeHugePct / 100
	case targetLand > ourLand*2:
		cost = cost * DoomerSurchargeBigPct / 100
	case targetLand > ourLand:
		cost = cost * DoomerSurchargeAheadPct / 100
	}
	return cost
}

// planetLand totals every living realm's land on this board.
func (w *World) planetLand() int {
	total := 0
	for _, e := range w.Empires {
		if e.Alive {
			total += e.Land
		}
	}
	return total
}

// remoteLand totals what this board knows of another planet's land, from the
// scores it has imported. Zero when the planet is unknown, which prices the
// weapon off the floor rather than refusing to build it.
func (w *World) remoteLand(board string) int {
	total := 0
	for _, b := range w.RemoteBoards {
		if b.BoardID != board {
			continue
		}
		for _, sc := range b.Scores {
			total += sc.Land
		}
	}
	return total
}

// DoomerQuote is what a Doomer Kaboomer aimed at board would cost this planet,
// in millions of gold. Shown before anyone commits to starting one.
func (w *World) DoomerQuote(board string) int {
	return doomerCost(w.remoteLand(board), w.planetLand())
}

// StartDoomer begins construction of the planet's one Doomer Kaboomer, aimed at
// board. It raises no money by itself — the barons fund it afterwards.
func (w *World) StartDoomer(e *Empire, board string) error {
	if !w.Config.DoomerKaboomer {
		return ErrDoomerDisabled
	}
	if w.Doomer != nil {
		return ErrDoomerExists
	}
	if board == "" {
		return ErrNoTarget
	}
	w.Doomer = &DoomerKaboomerWeapon{
		TargetBoard: board,
		Creator:     e.Owner,
		CostMillion: w.DoomerQuote(board),
		StartedDay:  w.GameDay,
		Intact:      100,
	}
	w.postNews(fmt.Sprintf("Construction of a Doomer Kaboomer aimed at %s has begun.", board))
	return nil
}

// FundDoomer puts millions of the baron's gold into the weapon, up to whatever
// it still needs, and reports how much was actually taken. Funding it fully
// completes construction; from then on it waits to be launched.
func (w *World) FundDoomer(e *Empire, millions int) (int, error) {
	if w.Doomer == nil {
		return 0, ErrNoDoomer
	}
	d := w.Doomer
	if d.Funded {
		return 0, ErrDoomerFunded
	}
	if millions < 1 {
		return 0, nil
	}
	if left := d.CostMillion - d.PaidMillion; millions > left {
		millions = left
	}
	gold := millions * DoomerMillion
	if e.Gold < gold {
		return 0, ErrCantAfford
	}
	e.Gold -= gold
	d.PaidMillion += millions
	w.postNews(fmt.Sprintf("%s agrees to put in %d million gold to the Doomer Kaboomer.", e.Name, millions))
	if d.PaidMillion >= d.CostMillion {
		d.Funded = true
		d.FundedDay = w.GameDay
		w.postNews("The Doomer Kaboomer is complete and awaiting launch.")
	}
	return millions, nil
}

// LaunchDoomer sends the finished weapon on its way. Only the baron who started
// it may launch it, and it is in the air — and visible to its target — for
// DoomerFlightDays.
func (w *World) LaunchDoomer(e *Empire) error {
	if w.Doomer == nil {
		return ErrNoDoomer
	}
	d := w.Doomer
	if !d.Funded {
		return ErrDoomerUnfunded
	}
	if d.Launched {
		return ErrDoomerFlying
	}
	if d.Creator != "" && d.Creator != e.Owner {
		return ErrNotYours
	}
	d.Launched = true
	d.LaunchedDay = w.GameDay
	d.ArrivesDay = w.GameDay + DoomerFlightDays
	w.postNews(fmt.Sprintf("The Doomer Kaboomer has launched at %s.", d.TargetBoard))
	return nil
}

// DismantleDoomer scraps the planet's weapon before it flies, refunding nothing
// — the money is spent. Only its creator may do it, and only while it is still
// on the ground.
func (w *World) DismantleDoomer(e *Empire) error {
	if w.Doomer == nil {
		return ErrNoDoomer
	}
	if w.Doomer.Launched {
		return ErrDoomerFlying
	}
	if w.Doomer.Creator != "" && w.Doomer.Creator != e.Owner {
		return ErrNotYours
	}
	board := w.Doomer.TargetBoard
	w.Doomer = nil
	w.ExportDoomerGone(board) // let the target stop watching for it (#63)
	w.postNews("The Doomer Kaboomer has been dismantled.")
	return nil
}

// InterceptDoomer sends jets at an incoming weapon. Nothing but jets can reach
// it, and the jets are spent whether or not they knock anything off. Returns the
// percentage destroyed by this sortie.
func (w *World) InterceptDoomer(e *Empire, jets int) (int, error) {
	if w.Incoming == nil {
		return 0, ErrNoDoomer
	}
	if jets < 1 {
		return 0, nil
	}
	if e.Jets < jets {
		jets = e.Jets
	}
	e.Jets -= jets
	knocked := jets / DoomerJetsPerPercent
	if knocked > w.Incoming.Intact {
		knocked = w.Incoming.Intact
	}
	w.Incoming.Intact -= knocked
	if w.Incoming.Intact <= 0 {
		w.Incoming = nil
		w.postNews("The incoming Doomer Kaboomer was destroyed in flight!")
		return knocked, nil
	}
	w.postNews(fmt.Sprintf("%d%% of the incoming Doomer Kaboomer was destroyed.", knocked))
	return knocked, nil
}

// DetonateDoomer applies an arrived weapon to this planet: every living realm
// loses DoomerDamagePct of its land, less its SDI, and scaled by how much of the
// weapon survived the flight.
func (w *World) DetonateDoomer(intact int) string {
	if intact <= 0 {
		return ""
	}
	w.postNews("A Doomer Kaboomer detonates across the planet!")
	hit := 0
	for _, d := range w.Empires {
		if !d.Alive {
			continue
		}
		pct := DoomerDamagePct * intact / 100
		pct -= pct * d.SDI / 100
		lost := d.Land * pct / 100
		if lost < 1 && pct > 0 {
			lost = 1
		}
		if lost <= 0 {
			continue
		}
		if lost > d.Land {
			lost = d.Land
		}
		d.Regions.remove(lost)
		d.syncLand()
		if d.Land <= 0 || d.People <= 0 {
			d.Alive = false
			d.DiedDay = w.GameDay
		}
		d.addEvent(fmt.Sprintf("A Doomer Kaboomer struck the planet — you lost %d regions.", lost))
		hit++
	}
	return fmt.Sprintf("The Doomer Kaboomer struck %d realms.", hit)
}
