package menu

import (
	"fmt"
	"math"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// actions_interbbs.go — planet-level inter-BBS business that is not an attack,
// an op or a score: how long packets take to reach each board, who this board
// elects as Coordinator, and standing down a Clingy Annihilator.

// Travel Times geometry, measured off a live BRE capture: a 75-column inset
// rule above and below the list, and each planet's name in a 30-column field
// with the turnaround right after it.
const travelNameWidth = 30

// travelTimes is BRE's "Average Turn Around Times to All BBSes": every other
// planet in the league with the average round trip a packet makes to it and
// back — the figure that says whether a strike aimed there lands tonight or
// next week. A board that has not yet answered a probe reads "No Data".
//
// The averaging is game.recordTravelTime's; this screen only renders it, in
// BRE's units and colors — hours in green under two days, days in cyan at or
// above it, "No Data" in red.
func travelTimes(s session.Session, w *ctx) Result {
	type planet struct {
		name string
		days float64
	}
	var planets []planet
	w.With(func() {
		for _, name := range w.KnownBoards() {
			planets = append(planets, planet{name, w.TravelTimes[name]})
		}
	})
	if len(planets) == 0 {
		ok(s, "No other planets are known yet.")
		return Stay
	}
	rule := rule75(ansi.FgBrightBlack)
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgWhite, tr(s, "Average Turn Around Times to All BBSes"), ansi.Reset)
	fmt.Fprintf(s, "%s\n", rule)
	for _, p := range planets {
		label, col := turnaroundLabel(s, p.days)
		fmt.Fprintf(s, "%s%-*s%s%s%s\n", ansi.FgWhite, travelNameWidth, game.FitColumn(p.name, travelNameWidth-1), col, label, ansi.Reset)
	}
	fmt.Fprintf(s, "%s\n", rule)
	pause(s)
	return Stay
}

// turnaroundLabel renders one average round trip and its color. BRE quantizes
// before printing — hours to a tenth, days to a hundredth — so the figures land
// on the same values the original shows. The seconds and minutes tiers are IB's
// own: BRE never needed them, but a link that answers in under a minute would
// otherwise print 0.00 hours and read as no measurement at all.
func turnaroundLabel(s session.Session, days float64) (string, string) {
	switch {
	case days <= 0:
		return tr(s, "No Data"), ansi.FgRed
	case days < game.TravelSecondsCutoff:
		// Never round a real measurement down to zero — that is the reading this
		// tier exists to avoid.
		return plural(s, math.Max(1, math.Round(days*24*60*60)), "1 second", "%.0f seconds"), ansi.FgBrightGreen
	case days < game.TravelMinutesCutoff:
		return plural(s, math.Round(days*24*60), "1 minute", "%.0f minutes"), ansi.FgBrightGreen
	case days < game.TravelHoursCutoff:
		hours := math.Round(days*24*10) / 10
		return fmt.Sprintf(tr(s, "%.2f hours"), hours), ansi.FgBrightGreen
	}
	return fmt.Sprintf(tr(s, "%.2f days"), math.Round(days*100)/100), ansi.FgCyan
}

// plural renders a whole-number count, picking the singular wording at one. The
// two forms are separate translatable strings because a PO catalogue cannot
// derive one from the other, and languages do not agree on where the plural
// starts.
func plural(s session.Session, n float64, one, many string) string {
	if n == 1 {
		return tr(s, one)
	}
	return fmt.Sprintf(tr(s, many), n)
}

// voteCoordinator lets the player cast (or change) their vote for the BBS
// Coordinator — the elected player who gets the Coordinator menu. BRE: "Who do
// you feel should be the BBS Coordinator?"; the vote can change any time.
func voteCoordinator(s session.Session, w *ctx) Result {
	p := w.Player()
	var owners, names []string
	var coordinatorName string
	w.With(func() {
		for _, e := range w.Empires {
			// Every realm holding a slot is a candidate, protected ones included,
			// and so are you (#149). BRE builds the picker's key set in
			// choose_target_empire (BRE.OVR 0x01aa99) from one test — the slot's
			// player id > 0 — and its two extra filters are both switched OFF for
			// this call: the net-worth and protection checks ride an argument the
			// vote passes as 0, and the exclude-yourself branch rides another it
			// passes as 1. IB filtered out protected realms, which in a young game
			// is every realm but the voter's own.
			//
			// The voter's OWN protection is a separate gate, and BRE does have it:
			// show_game_settings draws the Coordinator Vote item only when the
			// protection predicate says the caller is clear (see tree.go).
			if !e.Alive || e.Owner == "" {
				continue
			}
			owners = append(owners, e.Owner)
			label := e.Name
			if e.Owner == p.CoordinatorVote {
				label += " " + tr(s, "(your current vote)")
			}
			names = append(names, label)
		}
		if co := w.BBSCoordinator(); co != nil {
			coordinatorName = co.Name
		}
	})
	if len(names) == 0 {
		ok(s, "There are no barons to vote for yet.")
		return Stay
	}
	if coordinatorName != "" {
		fmt.Fprintf(s, "\n%s"+tr(s, "The current BBS Coordinator is %s.")+"%s\n", ansi.FgBrightCyan, coordinatorName, ansi.Reset)
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Who should be the BBS Coordinator?"), ansi.Reset)
	for i, n := range names {
		fmt.Fprintf(s, "    %d) %s\n", i+1, n)
	}
	i := promptInt(s, "Vote for (0 to cancel)?")
	if i < 1 || i > len(owners) {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		w.World.VoteCoordinator(p, owners[i-1])
		return nil
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Your vote is recorded. You may change it any time.")
	return Stay
}

// dismantleAnnihilator is BRE's Dismantle Gooie, item 1 of the Coordinator Ops
// menu: the elected Coordinator calls off the planet's doomsday weapon before
// it flies (#45). The builder keeps a dismantle of their own on the weapon's
// own desk for now; #114 covers taking that away, along with launching the
// weapon by hand.
func dismantleAnnihilator(s session.Session, w *ctx) Result {
	var isCoordinator, enabled bool
	var d *game.Annihilator
	w.With(func() {
		isCoordinator = w.BBSCoordinator() == w.Player()
		enabled = w.Config.ClingyAnnihilator
		if w.Annihilator != nil {
			c := *w.Annihilator
			d = &c
		}
	})
	if !isCoordinator {
		ok(s, "Only the BBS Coordinator may stand the weapon down.")
		return Stay
	}
	if !enabled {
		ok(s, "Clingy Annihilator operations are switched off in this game.")
		return Stay
	}
	if d == nil {
		ok(s, "This planet is not building a Clingy Annihilator.")
		return Stay
	}
	if d.Launched {
		ok(s, "The Clingy Annihilator has already launched. Nothing can call it back.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s"+tr(s, "The Clingy Annihilator is aimed at %s.")+"%s\n", ansi.FgWhite, d.TargetBoard, ansi.Reset)
	okNoPause(s, "Dismantling it refunds nothing — the gold is spent.")
	if !AskYesNo(s, "Stand it down?", false) {
		return Stay
	}
	err := w.mutatePlayer(func(p *game.Empire) error {
		// Re-checked inside the transaction: a vote on another node can unseat
		// the caller between the gate above and here.
		return w.World.DismantleAnnihilatorByCoordinator(p)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "The Clingy Annihilator has been dismantled.")
	return Stay
}
