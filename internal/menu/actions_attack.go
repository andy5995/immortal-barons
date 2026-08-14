package menu

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
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

// hiTokens paints whole-word occurrences of any of words in color on an
// otherwise-plain line, then back to ansi.Reset — the same shape as hiNums,
// but for known literal words instead of digit runs. Longest word first, so a
// short token that is a substring of a longer one (an empire named "Trade"
// against "Free Trade Agreement") can't steal the longer token's match; \b
// keeps a short token from painting itself inside an unrelated word.
func hiTokens(s string, words []string, color string) string {
	if len(words) == 0 {
		return s
	}
	sorted := append([]string(nil), words...)
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })
	quoted := make([]string, len(sorted))
	for i, w := range sorted {
		quoted[i] = regexp.QuoteMeta(w)
	}
	re := regexp.MustCompile(`\b(?:` + strings.Join(quoted, "|") + `)\b`)
	return re.ReplaceAllStringFunc(s, func(m string) string {
		return color + m + ansi.Reset
	})
}

// targetRow snapshots the identity plus displayed fields of one living rival,
// taken under the world lock so the picker list can be rendered and prompted
// over safely (the snapshot ranges the shared w.Empires slice). The realm name
// is the identity the resolving w.With re-finds by (see findTarget); a
// pre-gathered pointer is not carried across the reload. attackable is false for
// a realm the player can see in the list but cannot hit (shielded by New Realm
// Protection or an alliance) — it is shown without a selection letter.
//
// people and troopers are NOT displayed: they are carried so a warhead can be
// priced off the target without a second trip under the lock (the chemical and
// biological missiles read both, see game.ChemCostForTarget).
type targetRow struct {
	name                  string
	land, score, netWorth int
	people, troopers      int
	attackable, online    bool
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
			rows = append(rows, targetRow{
				name: e.Name, land: e.Land, score: e.Score, netWorth: w.NetWorth(e),
				people: e.People, troopers: e.Troopers,
				attackable: attackable, online: e.Online(),
			})
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
	var trimmed bool
	err := w.mutatePlayer(func(p *game.Empire) error {
		d := findTarget(w, p, name)
		if d == nil {
			return errTargetGone
		}
		if !w.CanAttack(p) {
			return errAttacksExhausted
		}
		// The committed force was typed against the pre-prompt holdings; if a
		// concurrent node's strike thinned them meanwhile, clampTo in Attack sends
		// only what remains — note it so the player learns the numbers moved.
		trimmed = force.Troopers > p.Troopers || force.Tanks > p.Tanks ||
			force.Bombers > p.Bombers || force.Jets > min(p.Jets, p.Carriers*game.JetsPerCarrier)
		// Deferred capture (autoCapture=false): the defender bleeds its regions but
		// the attacker gains none yet, so the human can pick the types below (#58).
		report, captured = w.World.Attack(p, d, force, false)
		return nil
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	warnTrimmedForce(s, trimmed)
	fmt.Fprintf(s, "\n%s\n", hiNums(wrapReport(report)))
	if captured > 0 {
		allocateCaptured(s, w, captured)
	}
	pause(s)
	// One attack per turn: leave the War menu so the turn moves forward (BRE-
	// style — the player can't keep attacking until their next turn).
	return Back
}

// warnTrimmedForce tells the player their committed force was clamped because
// a concurrent node's strike thinned their holdings between the prompt and the
// launch (regular attacks and pirate raids both clamp silently at game level).
func warnTrimmedForce(s session.Session, trimmed bool) {
	if !trimmed {
		return
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightRed,
		tr(s, "Your forces changed while you prepared the attack — only units still under your command were sent."),
		ansi.Reset)
}

// pickAttackTarget renders the living rivals in IB's familiar scores-table
// layout (Id / Empire Name / Territory / Score / Net Worth, lettered ids) and
// reads the player's single-key choice. Only attackable realms get a selection
// letter; shielded ones (New Realm Protection or an alliance) are listed with a
// blank id so they can be seen but not picked — their diplomacy status shows in
// the diplomacy menus. Returns the chosen realm name, or chosen=false if the
// player aborts (RETURN / any non-letter) or nothing is attackable.
func pickAttackTarget(s session.Session, rows []targetRow, prompt string) (name string, chosen bool) {
	scoreTableHead(s)
	var names []string
	for _, r := range rows {
		id := "" // no selection letter for a realm that can't be attacked
		if r.attackable {
			id = scoreID(len(names))
			names = append(names, r.name)
		}
		scoreTableRow(s, id, r.name, ansi.FgBrightWhite, r.online, r.land, r.score, r.netWorth)
	}
	scoreTableRule(s)
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

// costOf prices a gold-fee op against the target it is aimed at. The three
// warheads read different fields of it — the nuclear missile only the land, the
// chemical one the people too, the biological one the troopers as well — so
// they are all handed the whole snapshotted row.
type costOf func(t targetRow) int64

// localAttack shares the target-selection loop used by the nuclear, chemical,
// and biological attacks and by the covert/bomb ops — every attack aimed at a
// realm on the local planet (as opposed to an interplanetary/group attack).
// endsTurn is true only for the War-menu WMD (one attack per turn); covert ops
// stay in their own menu.
//
// price is nil for an op with no fee. The original quotes the figure only once
// a target is named — the three missile screens all reach the same arms-dealer
// routine, which prints it and asks for a yes before anything is deducted — so
// a warhead priced off the target has somewhere to read that target from.
func localAttack(s session.Session, w *ctx, label string, price costOf, endsTurn bool, strike func(a, d *game.Empire) (string, error)) Result {
	if blockedByProtection(s, w) {
		return Stay
	}
	rows := snapshotTargets(w)
	if len(rows) == 0 {
		ok(s, "There are no rival empires left to attack.")
		return Stay
	}
	name, chosen := pickAttackTarget(s, rows, tr(s, "Choose a target (letter, RETURN to abort)"))
	if !chosen {
		return Stay
	}
	if price != nil {
		var target targetRow
		for _, r := range rows {
			if r.name == name {
				target = r
			}
		}
		fmt.Fprintf(s, "\n%s"+tr(s, "%s — an arms broker wants %s gold for the warhead.")+"%s\n",
			ansi.FgBrightCyan, label, comma(price(target)), ansi.Reset)
		// Default NO: a warhead can cost tens of millions, and Enter is the
		// default-accept key everywhere else in the turn.
		if !AskYesNo(s, "Buy it?", false) {
			return Stay
		}
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
	fmt.Fprintf(s, "\n%s\n", hiNums(wrapReport(report)))
	pause(s)
	if endsTurn {
		return Back // one War-menu attack per turn (see regularAttack)
	}
	return Stay
}

// The three warhead prices, each reading only the target fields its own routine
// reads in the original.
func nukePrice(t targetRow) int64 { return game.NukeCostForLand(t.land) }
func chemPrice(t targetRow) int64 { return game.ChemCostForTarget(t.people, t.land) }
func bioPrice(t targetRow) int64 {
	return game.BioCostForTarget(t.troopers, t.people, t.land)
}

func nuclearAttack(s session.Session, w *ctx) Result {
	return localAttack(s, w, "Nuclear Attack", nukePrice, true, func(a, d *game.Empire) (string, error) { return w.NuclearStrike(a, d) })
}

func chemicalAttack(s session.Session, w *ctx) Result {
	return localAttack(s, w, "Chemical Attack", chemPrice, true, func(a, d *game.Empire) (string, error) { return w.ChemicalStrike(a, d) })
}

func biologicalAttack(s session.Session, w *ctx) Result {
	return localAttack(s, w, "Biological Attack", bioPrice, true, func(a, d *game.Empire) (string, error) { return w.BiologicalStrike(a, d) })
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
	f := ChoiceQuit(s, len(names))
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
	var trimmed bool
	err := w.mutatePlayer(func(fp *game.Empire) error {
		// Like regularAttack: the committed force was typed against pre-prompt
		// holdings; RaidFaction clamps to the fresh stock, so say when it did.
		trimmed = troopers > fp.Troopers || jets > fp.Jets || tanks > fp.Tanks
		report, captured = w.World.RaidFaction(fp, f-1, troopers, jets, tanks)
		return nil
	})
	if err != nil {
		fail(s, err)
		return Stay
	}
	warnTrimmedForce(s, trimmed)
	fmt.Fprintf(s, "\n%s\n", hiNums(wrapReport(report)))
	// A pirate win with land opens the same type picker a Regular Attack uses; a
	// landless faction wins gold/military only, so no picker appears (#21, BRE).
	if captured > 0 {
		allocateCaptured(s, w, captured)
	}
	pause(s)
	return Back // one attack per turn (see regularAttack)
}

func sdiProgram(s session.Session, w *ctx) Result {
	// BRE's SDI screen: four figures, then the turn's allowance and the
	// increments note, then the funding prompt. Labels white, figures
	// bright-yellow, the note gray (docs/dev/bre-screens.md).
	var funding, maint, perRegion, allowance, gold int64
	var strength int
	w.With(func() {
		p := w.Player()
		funding, strength = p.SDIFunding, p.SDI
		maint = w.SDIMaintenance(p)
		perRegion = w.SDIFundingPerRegion(p)
		allowance = min(w.SDISpendAllowance(p), p.Gold)
	})
	fmt.Fprintln(s)
	for _, row := range []struct {
		label string
		value string
	}{
		{"Total Funding: %s Gold", comma(funding)},
		{"Yearly Maintenance: %s Gold", comma(maint)},
		{"Funding / Region: %s Gold", comma(perRegion)},
		{"Current SDI Strength: %s%%", strconv.Itoa(strength)},
	} {
		fmt.Fprintf(s, "%s%s%s\n", ansi.FgWhite, hiNumsReset(fmt.Sprintf(tr(s, row.label), row.value), ansi.FgBrightYellow, ansi.FgWhite), ansi.Reset)
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgWhite,
		hiNumsReset(fmt.Sprintf(tr(s, "Maximum productive spending this year is: %s Gold."), comma(allowance)), ansi.FgBrightYellow, ansi.FgWhite), ansi.Reset)
	// The original prints this figure ungrouped ("1000 Gold"), unlike every other
	// number on the screen.
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightBlack,
		fmt.Sprintf(tr(s, "Note: You should only fund the SDI in increments of %d Gold."), game.SDIIncrement), ansi.Reset)
	gold = promptSuggested(s, "Add how much gold for funding?", 0, allowance)
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
	fmt.Fprintf(s, "%s\n", hiNums(fmt.Sprintf(tr(s, "%s Gold added."), comma(gold))))
	fmt.Fprintf(s, "%s\n", hiNums(fmt.Sprintf(tr(s, "Current SDI Strength: %s%%"), strconv.Itoa(level))))
	return Stay
}

// clingyAnnihilator is the planet's doomsday-weapon desk, BRE's "Gooie Kablooie
// Ops": start one, put money in, launch it, or scrap it. A planet builds one at
// a time and the barons fund it between them (#16).
func clingyAnnihilator(s session.Session, w *ctx) Result {
	if blockedByProtection(s, w) {
		return Stay
	}
	var d *game.Annihilator
	var enabled bool
	w.With(func() {
		enabled = w.Config.ClingyAnnihilator
		if w.Annihilator != nil {
			c := *w.Annihilator
			d = &c
		}
	})
	if !enabled {
		ok(s, "Clingy Annihilator operations are switched off in this game.")
		return Stay
	}
	if d == nil {
		return startAnnihilator(s, w)
	}
	showAnnihilator(s, w, d)
	switch {
	case d.Launched:
		ok(s, "The Clingy Annihilator is on its way. Nothing more can be done with it.")
	case d.Funded:
		if AskYesNo(s, "Launch it now?", false) {
			runAnnihilator(s, w, func(p *game.Empire) error { return w.World.LaunchAnnihilator(p) }, "Launched.")
		} else if AskYesNo(s, "Dismantle it?", false) {
			runAnnihilator(s, w, func(p *game.Empire) error { return w.World.DismantleAnnihilator(p) }, "Dismantled.")
		}
	default:
		millions := promptSuggested(s, "How many million gold do you wish to put in?", 0, d.CostMillion-d.PaidMillion)
		if millions > 0 {
			var put int
			err := w.mutatePlayer(func(p *game.Empire) error {
				var e error
				put, e = w.World.FundAnnihilator(p, millions)
				return e
			})
			if err != nil {
				fail(s, err)
			} else {
				ok(s, "You put in %d million gold.", put)
			}
		}
	}
	return Stay
}

// startAnnihilator offers to begin construction, quoting what the planet will have to
// raise for the target it picks.
func startAnnihilator(s session.Session, w *ctx) Result {
	ok(s, "This planet has no Clingy Annihilator.")
	if !AskYesNo(s, "Would you like to begin construction?", false) {
		return Stay
	}
	var boards []string
	w.With(func() {
		for _, b := range w.RemoteBoards {
			boards = append(boards, b.BoardID)
		}
	})
	if len(boards) == 0 {
		ok(s, "No other planets are known yet. Wait for inter-BBS scores to arrive.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Target which planet?"), ansi.Reset)
	board := pickPlanetNamed(s, w, boards)
	if board == "" {
		return Stay
	}
	var quote int
	w.With(func() { quote = w.AnnihilatorQuote(board) })
	if !AskYesNo(s, fmt.Sprintf(tr(s, "It will cost your planet %s million gold to fund. Accept?"), comma(quote)), false) {
		return Stay
	}
	runAnnihilator(s, w, func(p *game.Empire) error { return w.World.StartAnnihilator(p, board) }, "Construction started.")
	return Stay
}

// showAnnihilator prints the weapon's status board, matching the original's
// Target / Total Cost / Cost Left / Creator lines.
func showAnnihilator(s session.Session, w *ctx, d *game.Annihilator) {
	fmt.Fprintf(s, "\n%sTarget:     %s%s\n", ansi.FgWhite, ansi.FgBrightWhite, d.TargetBoard)
	fmt.Fprintf(s, "%sTotal Cost: %s%s mil gold%s   Cost Left: %s%s mil gold%s\n",
		ansi.FgWhite, ansi.FgBrightWhite, comma(d.CostMillion), ansi.FgWhite,
		ansi.FgBrightWhite, comma(d.CostMillion-d.PaidMillion), ansi.Reset)
	fmt.Fprintf(s, "%sCreator:    %s%s%s\n", ansi.FgWhite, ansi.FgBrightWhite, d.Creator, ansi.Reset)
}

// runAnnihilator applies one weapon action under the player lock and reports it.
func runAnnihilator(s session.Session, w *ctx, act func(*game.Empire) error, done string) {
	if err := w.mutatePlayer(act); err != nil {
		fail(s, err)
		return
	}
	ok(s, done)
}
