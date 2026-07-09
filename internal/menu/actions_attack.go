package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// targetRow snapshots the identity plus displayed fields of one attackable
// empire, taken under the world lock so the picker list can be rendered and
// prompted over safely (w.Targets ranges the shared w.Empires slice).
type targetRow struct {
	e          *game.Empire
	name       string
	land, army int
}

// snapshotTargets takes w.Targets(w.Player()) under the lock, copying the
// display fields the picker needs. The *game.Empire pointers stay valid
// afterward (empires are never reallocated), but re-validate with
// stillTarget inside the resolving w.With before acting on a choice — the
// target may have died or gone under protection in the meantime.
func snapshotTargets(w *ctx) []targetRow {
	var rows []targetRow
	w.With(func() {
		for _, e := range w.Targets(w.Player()) {
			rows = append(rows, targetRow{e, e.Name, e.Land, e.Army()})
		}
	})
	return rows
}

// stillTarget reports whether target is still among w.Player()'s valid
// targets. Call only from inside a w.With block.
func stillTarget(w *ctx, target *game.Empire) bool {
	for _, t := range w.Targets(w.Player()) {
		if t == target {
			return true
		}
	}
	return false
}

func regularAttack(s session.Session, w *ctx) Result {
	if w.Player().Protection > 0 {
		ok(s, "You are under New Realm Protection and cannot attack yet.")
		return Stay
	}
	rows := snapshotTargets(w)
	if len(rows) == 0 {
		ok(s, "There are no rival empires left to attack.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Choose a target:"), ansi.Reset)
	printTargetRows(s, rows)
	i := promptInt(s, "Attack which empire (0 to cancel)?")
	if i < 1 || i > len(rows) {
		return Stay
	}
	target := rows[i-1].e
	var report string
	var err error
	w.With(func() {
		if !stillTarget(w, target) {
			err = fmt.Errorf("that empire is no longer a valid target")
			return
		}
		report = w.World.Attack(w.Player(), target)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", report)
	pause(s)
	return Stay
}

// printTargetRows lists attackable empires with their Land and Army columns.
func printTargetRows(s session.Session, rows []targetRow) {
	for i, r := range rows {
		fmt.Fprintf(s, "  %d) %-16s %s %-5d %s %-7d\n", i+1, r.name, tr(s, "Land"), r.land, tr(s, "Army"), r.army)
	}
}

// specialAttack shares the target-selection loop used by the nuclear,
// chemical, and biological attacks.
func specialAttack(s session.Session, w *ctx, label string, cost int, strike func(a, d *game.Empire) (string, error)) Result {
	if w.Player().Protection > 0 {
		ok(s, "You are under New Realm Protection and cannot attack yet.")
		return Stay
	}
	rows := snapshotTargets(w)
	if len(rows) == 0 {
		ok(s, "There are no rival empires left to attack.")
		return Stay
	}
	if cost > 0 {
		fmt.Fprintf(s, "\n%s"+tr(s, "%s — %d gold. Choose a target:")+"%s\n", ansi.FgBrightCyan, label, cost, ansi.Reset)
	} else {
		fmt.Fprintf(s, "\n%s"+tr(s, "%s — choose a target:")+"%s\n", ansi.FgBrightCyan, label, ansi.Reset)
	}
	printTargetRows(s, rows)
	i := promptInt(s, "Attack which empire (0 to cancel)?")
	if i < 1 || i > len(rows) {
		return Stay
	}
	target := rows[i-1].e
	var report string
	var err error
	w.With(func() {
		if !stillTarget(w, target) {
			err = fmt.Errorf("that empire is no longer a valid target")
			return
		}
		report, err = strike(w.Player(), target)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", report)
	pause(s)
	return Stay
}

func nuclearAttack(s session.Session, w *ctx) Result {
	return specialAttack(s, w, "Nuclear Attack", game.NukeCost, func(a, d *game.Empire) (string, error) { return w.NuclearStrike(a, d) })
}

func chemicalAttack(s session.Session, w *ctx) Result {
	return specialAttack(s, w, "Chemical Attack", game.ChemCost, func(a, d *game.Empire) (string, error) { return w.ChemicalStrike(a, d) })
}

func biologicalAttack(s session.Session, w *ctx) Result {
	return specialAttack(s, w, "Biological Attack", game.BioCost, func(a, d *game.Empire) (string, error) { return w.BiologicalStrike(a, d) })
}

func attackPirates(s session.Session, w *ctx) Result {
	type pirateRow struct {
		name                              string
		forces, land, gold                int
		lootT, lootJ, lootU, lootK, lootA int
	}
	var rows []pirateRow
	w.With(func() {
		for _, p := range w.Pirates {
			rows = append(rows, pirateRow{
				p.Name, p.Forces, p.Land, p.Gold,
				p.LootTroopers, p.LootJets, p.LootTurrets, p.LootTanks, p.LootAgents,
			})
		}
	})
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Pirate factions (strength is random; fat ones just raided someone):"), ansi.Reset)
	fmt.Fprintf(s, "  %-3s %-11s %-7s %-4s %-8s %s\n", "#", tr(s, "Faction"), tr(s, "Forces"), tr(s, "Rgn"), tr(s, "Gold"), tr(s, "Loot T/J/U/K/A"))
	for i, r := range rows {
		fmt.Fprintf(s, "  %d) %-11s %-7d %-4d %-8d %d/%d/%d/%d/%d\n",
			i+1, r.name, r.forces, r.land, r.gold,
			r.lootT, r.lootJ, r.lootU, r.lootK, r.lootA)
	}
	f := promptInt(s, "Raid which faction (0 to cancel)?")
	if f < 1 || f > len(rows) {
		return Stay
	}
	p := w.Player()
	troopers := promptSuggested(s, "Commit how many Troopers?", 0, p.Troopers)
	jets := promptSuggested(s, "Commit how many Jets?", 0, p.Jets)
	tanks := promptSuggested(s, "Commit how many Tanks?", 0, p.Tanks)

	// RaidFaction bounds-checks the faction index itself, so a faction that
	// vanished between the snapshot above and here just reports "no such
	// faction" instead of a stale read.
	var report string
	w.With(func() { report = w.World.RaidFaction(p, f-1, troopers, jets, tanks) })
	fmt.Fprintf(s, "\n%s\n", report)
	pause(s)
	return Stay
}

func sdiProgram(s session.Session, w *ctx) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%s"+tr(s, "SDI Program — current defense: %d%%")+"%s\n", ansi.FgBrightCyan, p.SDI, ansi.Reset)
	gold := promptInt(s, "Fund SDI — gold to spend (10000 per +1%%, max 75%%)?")
	if gold <= 0 {
		return Stay
	}
	var level int
	var err error
	w.With(func() { level, err = w.World.FundSDI(p, gold) })
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "SDI is now %d%%.", level)
	return Stay
}

func doomerKaboomer(s session.Session, w *ctx) Result {
	if w.Player().Protection > 0 {
		ok(s, "You are under New Realm Protection and cannot attack yet.")
		return Stay
	}
	answer := promptInt(s, fmt.Sprintf("A Doomer Kaboomer costs %d gold. Launch? (1 = yes)", game.DoomerCost))
	if answer != 1 {
		return Stay
	}
	var report string
	var err error
	w.With(func() { report, err = w.World.DoomerKaboomer(w.Player()) })
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", report)
	pause(s)
	return Stay
}
