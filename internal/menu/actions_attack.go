package menu

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// hiNumsReset returns s with each run of digits (keeping grouping commas that
// sit between digits) wrapped in numColor and then restored to resetColor, so
// figures pop against a colored line — BRE highlights numbers throughout its
// screens (docs/dev/bre-screens.md). resetColor is what the text returns to after
// a number (ansi.Reset for plain lines, or the line's own base color when the
// surrounding text is itself colored, e.g. the white Technology advisor). The
// game builds reports as plain text (it stays display-agnostic), so the coloring
// happens here at the display layer.
func hiNumsReset(s, numColor, resetColor string) string {
	var b strings.Builder
	inNum := false
	for i := 0; i < len(s); {
		// Pass an escape sequence through untouched — its digits (the "96" in
		// ESC[96m) are not figures, and wrapping them in a color code produces a
		// malformed sequence the terminal prints as literal text.
		if n := csiLen(s[i:]); n > 0 {
			if inNum {
				b.WriteString(resetColor)
				inNum = false
			}
			b.WriteString(s[i : i+n])
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		digit := r >= '0' && r <= '9'
		if inNum && r == ',' {
			b.WriteRune(r) // 1,000 — a comma between digits stays inside the run
			continue
		}
		if digit && !inNum {
			b.WriteString(numColor)
			inNum = true
		} else if !digit && inNum {
			b.WriteString(resetColor)
			inNum = false
		}
		b.WriteRune(r)
	}
	if inNum {
		b.WriteString(resetColor)
	}
	return b.String()
}

// csiLen returns the byte length of the ANSI CSI sequence at the head of s, or
// 0 if s does not start with one.
func csiLen(s string) int {
	if len(s) < 2 || s[0] != 0x1b || s[1] != '[' {
		return 0
	}
	for i := 2; i < len(s); i++ {
		if s[i] >= 0x40 && s[i] <= 0x7e {
			return i + 1
		}
	}
	return 0
}

// hiNumsIn highlights figures in the given color on an otherwise-plain line.
func hiNumsIn(s, color string) string { return hiNumsReset(s, color, ansi.Reset) }

// hiNums highlights figures in bright-yellow — BRE's default figure color for
// battle/raid/economy reports.
func hiNums(s string) string { return hiNumsIn(s, ansi.FgBrightYellow) }

// targetRow snapshots the identity plus displayed fields of one living rival,
// taken under the world lock so the picker list can be rendered and prompted
// over safely (the snapshot ranges the shared w.Empires slice). The realm name
// is the identity the resolving w.With re-finds by (see findTarget); a
// pre-gathered pointer is not carried across the reload. attackable is false for
// a realm the player can see in the list but cannot hit (shielded by New Realm
// Protection or an alliance) — it is shown without a selection letter.
type targetRow struct {
	name                  string
	land, score, netWorth int
	attackable            bool
}

// snapshotTargets copies every LIVING rival (not just the attackable ones) under
// the lock, marking which can be attacked. Listing the shielded realms — rather
// than hiding them — keeps a player from reading "all rivals protected" as "the
// world is empty". attackable mirrors game.Targets (protection, then alliance);
// an empty result means no rivals are left at all.
func snapshotTargets(w *ctx) []targetRow {
	var rows []targetRow
	w.With(func() {
		p := w.Player()
		if p == nil {
			return
		}
		for _, e := range w.Empires {
			if e == p || !e.Alive {
				continue
			}
			attackable := e.Protection == 0 && !w.AreAllied(p, e)
			rows = append(rows, targetRow{e.Name, e.Land, e.Score, w.NetWorth(e), attackable})
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

// blockedByProtection reports whether the acting empire is still under new-realm
// protection and may not attack yet, printing the standard message when it is.
// The gate belongs on the attack ATTEMPT, so players can still browse the war
// menus; only launching an attack — including a pirate raid — is refused.
func blockedByProtection(s session.Session, w *ctx) bool {
	if w.Player().Protection > 0 {
		ok(s, "You are under New Realm Protection and cannot attack yet.")
		return true
	}
	return false
}

func regularAttack(s session.Session, w *ctx) Result {
	if blockedByProtection(s, w) {
		return Stay
	}
	if !w.CanAttack(w.Player()) {
		ok(s, "You have already launched all %d of your attacks for today.", w.Config.MaxIndividualAttacks)
		return Stay
	}
	rows := snapshotTargets(w)
	if len(rows) == 0 {
		ok(s, "There are no rival empires left to attack.")
		return Stay
	}
	name, chosen := pickAttackTarget(s, rows, tr(s, "Attack which realm? (letter, RETURN to abort)"))
	if !chosen {
		return Stay
	}

	// BRE's regular-attack force selection: show the available forces and prompt
	// how many of each to commit (jets are only usable up to what the carriers can
	// transport). Held-back units stay home and out of harm's way. clampTo in
	// Attack re-checks against the reloaded empire, so a concurrent change is safe.
	p := w.Player()
	usableJets := min(p.Jets, p.Carriers*game.JetsPerCarrier)
	fmt.Fprintf(s, "\n%s"+tr(s, "You have %d Troopers, %d usable Jets, %d Tanks, and %d Bombers.")+"%s\n",
		ansi.FgBrightCyan, p.Troopers, usableJets, p.Tanks, p.Bombers, ansi.Reset)
	force := game.AttackForce{
		Troopers: promptSuggestedTight(s, "Send how many Troopers?", p.Troopers, p.Troopers),
		Jets:     promptSuggestedTight(s, "Send how many Jets?", usableJets, usableJets),
		Tanks:    promptSuggestedTight(s, "Send how many Tanks?", p.Tanks, p.Tanks),
		Bombers:  promptSuggestedTight(s, "Send how many Bombers?", p.Bombers, p.Bombers),
	}
	if force.Troopers+force.Jets+force.Tanks+force.Bombers <= 0 {
		ok(s, "You committed no forces; the attack is called off.")
		return Stay
	}

	var report string
	var captured int
	err := w.mutatePlayer(func(p *game.Empire) error {
		d := findTarget(w, p, name)
		if d == nil {
			return errTargetGone
		}
		if !w.CanAttack(p) {
			return errAttacksExhausted
		}
		// Deferred capture (autoCapture=false): the defender bleeds its regions but
		// the attacker gains none yet, so the human can pick the types below (#58).
		report, captured = w.World.Attack(p, d, force, false)
		return nil
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", hiNums(report))
	if captured > 0 {
		allocateCaptured(s, w, captured)
	}
	pause(s)
	// One attack per turn: leave the War menu so the turn moves forward (BRE-
	// style — the player can't keep attacking until their next turn).
	return Back
}

// pickAttackTarget renders the living rivals in IB's familiar scores-table
// layout (Id / Empire Name / Territory / Score / Net Worth, lettered ids) and
// reads the player's single-key choice. Only attackable realms get a selection
// letter; shielded ones (New Realm Protection or an alliance) are listed with a
// blank id so they can be seen but not picked — their diplomacy status shows in
// the diplomacy menus. Returns the chosen realm name, or chosen=false if the
// player aborts (RETURN / any non-letter) or nothing is attackable.
func pickAttackTarget(s session.Session, rows []targetRow, prompt string) (name string, chosen bool) {
	rule := strings.Repeat("─", 72)
	fmt.Fprintf(s, "%s%-4s %-26s %10s %11s %11s%s\n",
		ansi.FgBrightWhite, tr(s, "Id"), tr(s, "Empire Name"),
		tr(s, "Territory"), tr(s, "Score"), tr(s, "Net Worth"), ansi.Reset)
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgMagenta, rule, ansi.Reset)
	var names []string
	for _, r := range rows {
		id := "" // no selection letter for a realm that can't be attacked
		if r.attackable {
			id = scoreID(len(names))
			names = append(names, r.name)
		}
		fmt.Fprintf(s, "%s%-4s%s %s%-26s%s %s%10d%s %s%11d%s %s%11d%s\n",
			ansi.FgBrightMagenta, id, ansi.Reset,
			ansi.FgBrightWhite, r.name, ansi.Reset,
			ansi.FgBrightMagenta, r.land, ansi.Reset,
			ansi.FgBrightWhite, r.score, ansi.Reset,
			ansi.FgWhite, r.netWorth, ansi.Reset)
	}
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgMagenta, rule, ansi.Reset)
	if len(names) == 0 {
		ok(s, "None of these realms can be attacked — they are protected or allied with you.")
		return "", false
	}
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, prompt, ansi.Reset)
	r, err := readKey(s)
	if err != nil {
		return "", false
	}
	i := int(unicode.ToUpper(r) - 'A')
	if i < 0 || i >= len(names) {
		fmt.Fprint(s, "\n")
		return "", false
	}
	fmt.Fprintf(s, "%c\n", unicode.ToUpper(r))
	return names[i], true
}

// localAttack shares the target-selection loop used by the nuclear, chemical,
// and biological attacks and by the covert/bomb ops — every attack aimed at a
// realm on the local planet (as opposed to an interplanetary/group attack).
// endsTurn is true only for the War-menu WMD (one attack per turn); covert ops
// stay in their own menu.
func localAttack(s session.Session, w *ctx, label string, cost int, endsTurn bool, strike func(a, d *game.Empire) (string, error)) Result {
	if blockedByProtection(s, w) {
		return Stay
	}
	rows := snapshotTargets(w)
	if len(rows) == 0 {
		ok(s, "There are no rival empires left to attack.")
		return Stay
	}
	// BRE goes straight to the target prompt after the menu echoes the op; only a
	// gold-fee op (WMD) shows a cost line first, since its price isn't on a menu.
	if cost > 0 {
		fmt.Fprintf(s, "\n%s"+tr(s, "%s — %d gold.")+"%s\n", ansi.FgBrightCyan, label, cost, ansi.Reset)
	}
	name, chosen := pickAttackTarget(s, rows, tr(s, "Choose a target (letter, RETURN to abort)"))
	if !chosen {
		return Stay
	}
	var report string
	err := w.mutatePlayer(func(p *game.Empire) error {
		d := findTarget(w, p, name)
		if d == nil {
			return errTargetGone
		}
		var e error
		report, e = strike(p, d)
		return e
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", hiNums(report))
	pause(s)
	if endsTurn {
		return Back // one War-menu attack per turn (see regularAttack)
	}
	return Stay
}

func nuclearAttack(s session.Session, w *ctx) Result {
	return localAttack(s, w, "Nuclear Attack", game.NukeCost, true, func(a, d *game.Empire) (string, error) { return w.NuclearStrike(a, d) })
}

func chemicalAttack(s session.Session, w *ctx) Result {
	return localAttack(s, w, "Chemical Attack", game.ChemCost, true, func(a, d *game.Empire) (string, error) { return w.ChemicalStrike(a, d) })
}

func biologicalAttack(s session.Session, w *ctx) Result {
	return localAttack(s, w, "Biological Attack", game.BioCost, true, func(a, d *game.Empire) (string, error) { return w.BiologicalStrike(a, d) })
}

// pirateColors are the per-faction name colors, in game.PirateFactions order.
// The palette is BRE's, verified from BRE.EXE's color table (the 9 bytes right
// after the faction-name array): 0a 0e 0c 04 05 0d 09 03 0b. The names are IB's
// own; the comments track which slot each color paints.
var pirateColors = []string{
	ansi.FgBrightGreen,   // Humans       (10 light green)
	ansi.FgBrightYellow,  // Barbarians   (14 yellow)
	ansi.FgBrightRed,     // Nightjackals (12 light red)
	ansi.FgRed,           // Sharks       (4 red)
	ansi.FgMagenta,       // Dunkleoids   (5 magenta)
	ansi.FgBrightMagenta, // Trilobarians (13 light magenta)
	ansi.FgBrightBlue,    // Raptorians   (9 light blue)
	ansi.FgCyan,          // Gorgonoids   (3 cyan)
	ansi.FgBrightCyan,    // Ammonians    (11 light cyan)
}

func attackPirates(s session.Session, w *ctx) Result {
	if blockedByProtection(s, w) {
		return Stay
	}
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
	fmt.Fprint(s, "\n")
	troopers := promptSuggestedTight(s, "Commit how many Troopers?", 0, p.Troopers)
	jets := promptSuggestedTight(s, "Commit how many Jets?", 0, p.Jets)
	tanks := promptSuggestedTight(s, "Commit how many Tanks?", 0, p.Tanks)

	// RaidFaction bounds-checks the faction index itself, so a faction that
	// vanished between the snapshot above and here just reports "no such
	// faction" instead of a stale read. Re-resolve the raider inside the
	// transaction: the p gathered above (before the commit prompts) is stale
	// after a concurrent node's reload, and clamps against the fresh stock.
	var report string
	var captured int
	err := w.mutatePlayer(func(fp *game.Empire) error {
		report, captured = w.World.RaidFaction(fp, f-1, troopers, jets, tanks)
		return nil
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", hiNums(report))
	// A pirate win with land opens the same type picker a Regular Attack uses; a
	// landless faction wins gold/military only, so no picker appears (#21, BRE).
	if captured > 0 {
		allocateCaptured(s, w, captured)
	}
	pause(s)
	return Back // one attack per turn (see regularAttack)
}

func sdiProgram(s session.Session, w *ctx) Result {
	p := w.Player()
	// BRE's SDI screen shows "Current SDI Strength" and the funding step; IB
	// funds whole per-point steps (SDIStep gold each) up to SDIMax.
	fmt.Fprintf(s, "\n%s"+tr(s, "Current SDI Strength: %d%%")+"%s\n", ansi.FgBrightCyan, p.SDI, ansi.Reset)
	fmt.Fprintf(s, "%s\n", hiNums(fmt.Sprintf(tr(s, "Each +1%% costs %s gold (max %d%%); fund in whole increments of that."), comma(game.SDIStep), game.SDIMax)))
	gold := promptInt(s, "Fund SDI — gold to spend?")
	if gold <= 0 {
		return Stay
	}
	var level int
	// Re-resolve inside the transaction: FundSDI re-checks gold and the SDIMax cap
	// against fresh state, so a concurrent node can't let two sessions spend the
	// same gold or push past the cap.
	err := w.mutatePlayer(func(fp *game.Empire) error {
		var e error
		level, e = w.World.FundSDI(fp, gold)
		return e
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "SDI is now %d%%.", level)
	return Stay
}

func doomerKaboomer(s session.Session, w *ctx) Result {
	if blockedByProtection(s, w) {
		return Stay
	}
	answer := promptInt(s, fmt.Sprintf("A Doomer Kaboomer costs %d gold. Launch? (1 = yes)", game.DoomerCost))
	if answer != 1 {
		return Stay
	}
	var report string
	err := w.mutatePlayer(func(p *game.Empire) error {
		var e error
		report, e = w.World.DoomerKaboomer(p)
		return e
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	fmt.Fprintf(s, "\n%s\n", hiNums(report))
	pause(s)
	return Stay
}
