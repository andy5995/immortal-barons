package game

import (
	"errors"
	"fmt"
)

// Annihilator is the planet's doomsday weapon — one at a time, funded
// by the barons who live here rather than bought by any one of them. It is IB's
// rename of BRE's Gooie Kablooie, and it goes through the original's whole life:
// a planet starts one against a named target, everyone chips in until it is
// paid for, it flies for a couple of days in plain sight of the planet it is
// aimed at, and it arrives with whatever is left of it after that planet's jets
// have had their turn.
type Annihilator struct {
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
	LaunchDay   int // funding day + AnnihilatorBuildDays; it goes by itself (#114)
	Launched    bool
	LaunchedDay int
	ArrivesDay  int
	Intact      int // percent of the weapon still standing; jets whittle this down
	// DaysLeft is the siege countdown once it has landed on the target planet:
	// AnnihilatorSiegeDays on arrival, one less after every day's damage, gone at
	// zero. Zero also means "not landed", which is why the flying weapon and the
	// besieging one are never the same record — the target keeps its own.
	DaysLeft int
}

var (
	ErrAnnihilatorExists   = errors.New("This planet is already building a Clingy Annihilator.")
	ErrNoAnnihilator       = errors.New("This planet has no Clingy Annihilator.")
	ErrAnnihilatorFunded   = errors.New("The Clingy Annihilator is already paid for.")
	ErrAnnihilatorFlying   = errors.New("The Clingy Annihilator has already launched.")
	ErrAnnihilatorAloft    = errors.New("The Clingy Annihilator has not landed yet. Nothing can reach it up there.")
	ErrNoJets              = errors.New("Only jets can attack a Clingy Annihilator, and you have none.")
	ErrAnnihilatorDisabled = errors.New("Clingy Annihilator operations are switched off in this game.")
	ErrNotPlanetCO         = errors.New("Only the elected BBS Coordinator may do that.")
)

// annihilatorCost prices a Clingy Annihilator aimed at targetLand, from a planet holding
// ourLand, in millions of gold. The shape is BRE's (see balance.go).
func annihilatorCost(targetLand, ourLand int) int {
	if targetLand < AnnihilatorMinTargetLand {
		targetLand = AnnihilatorMinTargetLand
	}
	cost := (targetLand*AnnihilatorCostPerLand + AnnihilatorCostPerLandDenom/2) / AnnihilatorCostPerLandDenom
	cost += AnnihilatorCostBase
	if cost > AnnihilatorCostCap {
		cost = AnnihilatorCostCap
	}
	switch {
	case ourLand <= 0:
		// Nothing to compare against; treat the target as overwhelming.
		cost = cost * AnnihilatorSurchargeHugePct / 100
	case targetLand > ourLand*4:
		cost = cost * AnnihilatorSurchargeHugePct / 100
	case targetLand > ourLand*2:
		cost = cost * AnnihilatorSurchargeBigPct / 100
	case targetLand > ourLand:
		cost = cost * AnnihilatorSurchargeAheadPct / 100
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

// AnnihilatorQuote is what a Clingy Annihilator aimed at board would cost this planet,
// in millions of gold. Shown before anyone commits to starting one.
func (w *World) AnnihilatorQuote(board string) int {
	return annihilatorCost(w.remoteLand(board), w.planetLand())
}

// StartAnnihilator begins construction of the planet's one Clingy Annihilator, aimed at
// board. It raises no money by itself — the barons fund it afterwards.
func (w *World) StartAnnihilator(e *Empire, board string) error {
	if !w.Config.ClingyAnnihilator {
		return ErrAnnihilatorDisabled
	}
	if w.Annihilator != nil {
		return ErrAnnihilatorExists
	}
	if board == "" {
		return ErrNoTarget
	}
	w.Annihilator = &Annihilator{
		TargetBoard: board,
		Creator:     e.Owner,
		CostMillion: w.AnnihilatorQuote(board),
		StartedDay:  w.GameDay,
		Intact:      100,
	}
	w.postNews(fmt.Sprintf("Construction of a Clingy Annihilator aimed at %s has begun.", board))
	w.reportToSpy(board, annihilatorSpyLine(w.Config.BoardID, w.Annihilator))
	return nil
}

// FundAnnihilator puts millions of the baron's gold into the weapon, up to whatever
// it still needs, and reports how much was actually taken. Funding it fully
// completes construction; from then on it waits to be launched.
func (w *World) FundAnnihilator(e *Empire, millions int) (int, error) {
	if w.Annihilator == nil {
		return 0, ErrNoAnnihilator
	}
	d := w.Annihilator
	if d.Funded {
		return 0, ErrAnnihilatorFunded
	}
	if millions < 1 {
		return 0, nil
	}
	if left := d.CostMillion - d.PaidMillion; millions > left {
		millions = left
	}
	gold := goldCost(millions, AnnihilatorMillion)
	if e.Gold < gold {
		return 0, ErrCantAfford
	}
	e.Gold -= gold
	d.PaidMillion += millions
	w.postNews(fmt.Sprintf("%s agrees to put in %d million gold to the Clingy Annihilator.", e.Name, millions))
	if d.PaidMillion >= d.CostMillion {
		d.Funded = true
		d.FundedDay = w.GameDay
		d.LaunchDay = w.GameDay + AnnihilatorBuildDays
		w.postNews(fmt.Sprintf("The Clingy Annihilator is complete. It launches at %s in %d hours.",
			d.TargetBoard, AnnihilatorBuildDays*24))
		w.reportToSpy(d.TargetBoard, annihilatorSpyLine(w.Config.BoardID, d))
	}
	return millions, nil
}

// LaunchDueAnnihilator sends the finished weapon on its way by itself, once
// construction has run its AnnihilatorBuildDays. No baron decides this: the
// original has no launch prompt anywhere, and funding reaching its target is the
// whole trigger (#114). Run once a day from maintenance.
func (w *World) LaunchDueAnnihilator() {
	d := w.Annihilator
	if d == nil || !d.Funded || d.Launched || w.GameDay < d.LaunchDay {
		return
	}
	d.Launched = true
	d.LaunchedDay = w.GameDay
	d.ArrivesDay = w.GameDay + AnnihilatorFlightDays
	w.postNews(fmt.Sprintf("The Clingy Annihilator has launched at %s.", d.TargetBoard))
}

// DismantleAnnihilatorByCoordinator stands the weapon down on the elected BBS
// Coordinator's order instead of its builder's. In BRE this is Dismantle Gooie,
// the first item of the Coordinator Ops menu, and it is a diplomatic lever
// rather than a change of mind: a planet that has just made peace calls off the
// weapon still aimed at its new ally, so the office holds the switch (#45).
func (w *World) DismantleAnnihilatorByCoordinator(e *Empire) error {
	if w.BBSCoordinator() != e {
		return ErrNotPlanetCO
	}
	return w.scrapAnnihilator()
}

// scrapAnnihilator is the dismantling itself, without asking who ordered it.
func (w *World) scrapAnnihilator() error {
	if w.Annihilator == nil {
		return ErrNoAnnihilator
	}
	if w.Annihilator.Launched {
		return ErrAnnihilatorFlying
	}
	board := w.Annihilator.TargetBoard
	w.Annihilator = nil
	w.ExportAnnihilatorGone(board) // let the target stop watching for it (#63)
	w.postNews("The Clingy Annihilator has been dismantled.")
	w.reportToSpy(board, fmt.Sprintf("Our agent on %s reports their Clingy Annihilator has been dismantled.", w.Config.BoardID))
	return nil
}

// AnnihilatorJetsNeeded is how many jets it takes to destroy an arrived weapon
// outright. It scales with the whole planet's land, which is what makes killing
// one need the cooperation the original's instructions describe: one baron's air
// force is never enough on a developed planet.
func (w *World) AnnihilatorJetsNeeded() int64 {
	land := int64(w.planetLand())
	need := land * (land/AnnihilatorJetsLandDivisor + AnnihilatorJetsLandBase)
	if need > AnnihilatorJetsRequiredCap {
		need = AnnihilatorJetsRequiredCap
	}
	if need < 1 {
		need = 1
	}
	return need
}

// InterceptAnnihilator sends jets at the weapon squatting on this planet.
// Nothing but jets can reach it, and they are spent whether or not they knock
// anything off. Returns the percentage destroyed by this sortie and the jets
// lost doing it.
func (w *World) InterceptAnnihilator(e *Empire, jets int) (int, int, error) {
	if w.Incoming == nil {
		return 0, 0, ErrNoAnnihilator
	}
	if w.Incoming.DaysLeft <= 0 {
		return 0, 0, ErrAnnihilatorAloft
	}
	if e.Jets < 1 {
		return 0, 0, ErrNoJets
	}
	if jets < 1 {
		return 0, 0, nil
	}
	if jets > e.Jets {
		jets = e.Jets
	}
	knocked := int(int64(jets) * 100 / w.AnnihilatorJetsNeeded())
	if knocked > AnnihilatorMaxSortiePct {
		knocked = AnnihilatorMaxSortiePct
	}
	if knocked > w.Incoming.Intact {
		knocked = w.Incoming.Intact
	}
	// 33% of the wing does not come home, give or take five points either way.
	lossPct := AnnihilatorJetLossPct + w.rng.Intn(AnnihilatorJetLossSpread) - w.rng.Intn(AnnihilatorJetLossSpread)
	lost := jets * lossPct / 100
	if lost > e.Jets {
		lost = e.Jets
	}
	e.Jets -= lost

	w.Incoming.Intact -= knocked
	if w.Incoming.Intact <= 0 {
		w.Incoming = nil
		w.postNews(fmt.Sprintf("%s destroyed the Clingy Annihilator!", e.Name))
		return knocked, lost, nil
	}
	w.postNews(fmt.Sprintf("%s knocked %d%% off the Clingy Annihilator; it is still at %d%% strength.",
		e.Name, knocked, w.Incoming.Intact))
	return knocked, lost, nil
}

// RetireSpentAnnihilator frees the builder's planet to start another once its
// weapon has reached its target. Without this the planet keeps announcing a
// weapon that has already landed, and the target would keep landing it again.
func (w *World) RetireSpentAnnihilator() {
	d := w.Annihilator
	if d == nil || !d.Launched || w.GameDay <= d.ArrivesDay {
		return
	}
	w.Annihilator = nil
}

// TickAnnihilator is one day of the siege: the weapon takes its share of every
// living realm's regions, its countdown drops by a day, and at zero it
// self-destructs. AnnihilatorFirstDayPct goes the day it lands and
// AnnihilatorLaterDayPct on each of the days after — a fixed share of what the
// realm still holds, so the bite shrinks as the planet does. Neither the SDI nor
// the weapon's own battered strength changes the figure (#111); a realm under
// new-realm protection is passed over.
func (w *World) TickAnnihilator() {
	d := w.Incoming
	if d == nil || d.DaysLeft <= 0 {
		return
	}
	pct := AnnihilatorLaterDayPct
	first := d.DaysLeft == AnnihilatorSiegeDays
	if first {
		pct = AnnihilatorFirstDayPct
		w.postNews(fmt.Sprintf("A Clingy Annihilator from %s has landed on our planet!", d.Creator))
	}
	total := 0
	for _, e := range w.Empires {
		if !e.Alive || e.Protection > 0 {
			continue
		}
		lost := e.Land * pct / 100
		if lost < 1 {
			continue
		}
		if lost > e.Land {
			lost = e.Land
		}
		e.Regions.remove(lost)
		e.syncLand()
		if e.Land <= 0 || e.People <= 0 {
			w.Kill(e)
		}
		total += lost
		if first {
			e.addEvent(fmt.Sprintf("A Clingy Annihilator landed on the planet — you lost %d regions.", lost))
		} else {
			e.addEvent(fmt.Sprintf("The Clingy Annihilator destroyed %d more regions.", lost))
		}
	}
	if first {
		w.postNews(fmt.Sprintf("The Clingy Annihilator destroyed %d regions!", total))
	} else {
		w.postNews(fmt.Sprintf("The Clingy Annihilator continues its attack and destroyed %d more regions!", total))
	}
	d.DaysLeft--
	if d.DaysLeft <= 0 {
		w.Incoming = nil
		w.postNews("The Clingy Annihilator has burned itself out.")
	}
}
