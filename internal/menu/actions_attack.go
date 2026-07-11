package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// targetRow snapshots the identity plus displayed fields of one attackable
// empire, taken under the world lock so the picker list can be rendered and
// prompted over safely (w.Targets ranges the shared w.Empires slice). The realm
// name is the identity the resolving w.With re-finds by (see findTarget); a
// pre-gathered pointer is not carried across the reload.
type targetRow struct {
	name       string
	land, army int
}

// snapshotTargets takes w.Targets(w.Player()) under the lock, copying the
// display fields the picker needs. It captures only the realm name as identity;
// the acting w.With later re-finds the chosen target by that name against fresh
// state (findTarget), since a pointer cached here would go stale after a reload.
func snapshotTargets(w *ctx) []targetRow {
	var rows []targetRow
	w.With(func() {
		p := w.Player()
		if p == nil {
			return
		}
		for _, e := range w.Targets(p) {
			rows = append(rows, targetRow{e.Name, e.Land, e.Army()})
		}
	})
	return rows
}

// findTarget re-resolves a target empire by realm name among attacker's
// CURRENT valid targets. Call only from inside a w.With block, after the world
// has reloaded. Matching by name (not by a pre-gathered pointer) is what makes
// this safe across a reload: json.Unmarshal reuses *Empire pointers by slice
// INDEX, so when the empire set changes shape — a rival is eliminated or
// abdicates and the slots shift — a cached pointer silently rebinds to a
// DIFFERENT realm. Names are unique (RealmNameTaken guards onboarding), so this
// returns the intended realm or nil (gone/dead/protected/allied → not a target).
func findTarget(w *ctx, attacker *game.Empire, name string) *game.Empire {
	for _, t := range w.Targets(attacker) {
		if t.Name == name {
			return t
		}
	}
	return nil
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
	name := rows[i-1].name
	var report string
	var err error
	w.With(func() {
		p := w.Player()
		if p == nil {
			err = errRealmChanged
			return
		}
		d := findTarget(w, p, name)
		if d == nil {
			err = errTargetGone
			return
		}
		report = w.World.Attack(p, d)
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
	name := rows[i-1].name
	var report string
	var err error
	w.With(func() {
		p := w.Player()
		if p == nil {
			err = errRealmChanged
			return
		}
		d := findTarget(w, p, name)
		if d == nil {
			err = errTargetGone
			return
		}
		report, err = strike(p, d)
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

// pirateColors are BRE's per-faction name colors, in game.PirateFactions order.
// Verified from BRE.EXE's color table (the 9 bytes right after the faction-name
// array): 0a 0e 0c 04 05 0d 09 03 0b.
var pirateColors = []string{
	ansi.FgBrightGreen,   // Humans      (10 light green)
	ansi.FgBrightYellow,  // Barbarians  (14 yellow)
	ansi.FgBrightRed,     // Solarians   (12 light red)
	ansi.FgRed,           // Sharks      (4 red)
	ansi.FgMagenta,       // Mechanoids  (5 magenta)
	ansi.FgBrightMagenta, // Rexxogans   (13 light magenta)
	ansi.FgBrightBlue,    // Xandorians  (9 light blue)
	ansi.FgCyan,          // Monitorians (3 cyan)
	ansi.FgBrightCyan,    // Spacians    (11 light cyan)
}

func attackPirates(s session.Session, w *ctx) Result {
	// BRE lists only the colored faction names — a faction's strength and hoard
	// are hidden, so raiding blind (not knowing which band is fat or lean) is
	// part of the game.
	var names []string
	w.With(func() {
		for _, p := range w.Pirates {
			names = append(names, p.Name)
		}
	})
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Attack Pirates"), ansi.Reset)
	for i, name := range names {
		color := ""
		if i < len(pirateColors) {
			color = pirateColors[i]
		}
		fmt.Fprintf(s, "  %d) %s%s%s\n", i+1, color, name, ansi.Reset)
	}
	fmt.Fprintf(s, "  0) %s\n", tr(s, "Quit"))
	f := choiceQuit(s, len(names))
	if f < 1 {
		return Stay
	}
	p := w.Player()
	troopers := promptSuggested(s, "Commit how many Troopers?", 0, p.Troopers)
	jets := promptSuggested(s, "Commit how many Jets?", 0, p.Jets)
	tanks := promptSuggested(s, "Commit how many Tanks?", 0, p.Tanks)

	// RaidFaction bounds-checks the faction index itself, so a faction that
	// vanished between the snapshot above and here just reports "no such
	// faction" instead of a stale read. Re-resolve the raider inside the
	// transaction: the p gathered above (before the commit prompts) is stale
	// after a concurrent node's reload, and clamps against the fresh stock.
	var report string
	var err error
	w.With(func() {
		fp := w.Player()
		if fp == nil {
			err = errRealmChanged
			return
		}
		report = w.World.RaidFaction(fp, f-1, troopers, jets, tanks)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
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
	w.With(func() {
		// Re-resolve inside the transaction: FundSDI re-checks gold and the
		// SDIMax cap against fresh state, so a concurrent node can't let two
		// sessions spend the same gold or push past the cap.
		fp := w.Player()
		if fp == nil {
			err = errRealmChanged
			return
		}
		level, err = w.World.FundSDI(fp, gold)
	})
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
	w.With(func() {
		p := w.Player()
		if p == nil {
			err = errRealmChanged
			return
		}
		report, err = w.World.DoomerKaboomer(p)
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", report)
	pause(s)
	return Stay
}
