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
// Protection or an alliance); protected says which of the two shields it is,
// because only one of them is announced on the row.
//
// people and troopers are NOT displayed: they are carried so a missile can be
// priced off the target without a second trip under the lock (the chemical and
// biological missiles read both, see game.ChemCostForTarget).
type targetRow struct {
	name                  string
	letter                string // the realm's permanent slot letter; its selection key
	land, score, netWorth int
	people, troopers      int
	attackable            bool
	protected             bool
	presence              string
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
			protected := e.Protection > 0
			attackable := !protected && !w.AreAllied(p, e)
			rows = append(rows, targetRow{
				name: e.Name, letter: e.Letter(),
				land: e.Land, score: e.Score, netWorth: w.NetWorth(e),
				people: e.People, troopers: e.Troopers,
				attackable: attackable, protected: protected,
				presence: presenceOf(e, false, w.Today),
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
	name, chosen := pickAttackTarget(s, w.Term, rows, attackPrompts(tr(s, "Attack which realm? (letter, RETURN to abort)")))
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
// reads the player's single-key choice. Returns the chosen realm name, or
// chosen=false if the player aborts (RETURN / any non-letter), picks a realm
// that cannot be targeted, or nothing is attackable.
//
// A REALM'S LETTER IS ITS OWN — its permanent slot letter, not a count of the
// rows above it. A fallen realm therefore leaves a gap, exactly as it does in
// the message picker. Numbering the rows instead re-letters everyone the moment
// a neighbour dies, so the key that attacked one realm yesterday attacks a
// different one today.
//
// A realm under New Realm Protection KEEPS ITS LETTER and wears it in brackets
// (#214). It withheld the letter until 2026-08-26, which said only that
// something about the row was different and left the player to guess what; the
// bracket says it outright, and pressing the letter now answers with the reason
// rather than behaving like a mistyped key. An ALLIED realm still shows no
// letter — that standing is the diplomacy screens' to report, and the alliance
// is the player's own doing.
// targetPrompt is the wording one target list uses: what it asks, what it says
// when a shielded realm is picked, and what it says when nothing on the list can
// be chosen at all. A parameter because the same table serves a strike, a covert
// op and an interplanetary trade deal (#195), and only the words differ.
type targetPrompt struct {
	ask     string
	refuse  string // takes the realm name
	nothing string
}

// attackPrompts is the wording for the war lists, and the default everywhere the
// caller has nothing more specific to say.
func attackPrompts(ask string) targetPrompt {
	return targetPrompt{
		ask:     ask,
		refuse:  "%s is under New Realm Protection and cannot be targeted yet.",
		nothing: "None of these realms can be attacked — they are protected or allied with you.",
	}
}

func pickAttackTarget(s session.Session, t Term, rows []targetRow, p targetPrompt) (name string, chosen bool) {
	scoreTableHead(s, t)
	byLetter := make(map[string]targetRow, len(rows))
	attackable := 0
	for _, r := range rows {
		id := "" // no selection letter for a realm allied with the player
		if r.attackable || r.protected {
			id = scoreID(r.letter, r.protected)
			byLetter[r.letter] = r
		}
		if r.attackable {
			attackable++
		}
		scoreTableRow(s, t, id, r.name, ansi.FgBrightWhite, r.presence, r.land, r.score, r.netWorth)
	}
	scoreTableRule(s)
	if attackable == 0 {
		ok(s, p.nothing)
		return "", false
	}
	fmt.Fprintf(s, "\n%s%s%s ", ansi.FgBrightWhite, p.ask, ansi.Reset)
	r, err := readKey(s)
	if err != nil {
		return "", false
	}
	// A letter with no realm behind it — a gap, or past the end — aborts rather
	// than selecting a neighbour.
	row, found := byLetter[string(unicode.ToUpper(r))]
	if !found {
		fmt.Fprint(s, "\n")
		return "", false
	}
	fmt.Fprintf(s, "%c\n", unicode.ToUpper(r))
	if !row.attackable {
		ok(s, p.refuse, row.name)
		return "", false
	}
	return row.name, true
}

// costOf prices a gold-fee op against the target it is aimed at. The three
// missiles read different fields of it — the nuclear one only the land, the
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
// a missile priced off the target has somewhere to read that target from.
func localAttack(s session.Session, w *ctx, label string, price costOf, endsTurn bool, strike func(a, d *game.Empire) (string, error)) Result {
	if blockedByProtection(s, w) {
		return Stay
	}
	return pickAndStrike(s, w, label, price, endsTurn, strike)
}

// pickAndStrike is localAttack without the New Realm Protection gate: the
// target list, the arms-broker quote, and the strike itself. The covert menu
// calls it directly for the two operations the original lets a sheltered realm
// still run (see covertInfoOp).
func pickAndStrike(s session.Session, w *ctx, label string, price costOf, endsTurn bool, strike func(a, d *game.Empire) (string, error)) Result {
	rows := snapshotTargets(w)
	if len(rows) == 0 {
		ok(s, "There are no rival empires left to attack.")
		return Stay
	}
	name, chosen := pickAttackTarget(s, w.Term, rows, attackPrompts(tr(s, "Choose a target (letter, RETURN to abort)")))
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
		fmt.Fprintf(s, "\n%s"+tr(s, "%s — an arms broker wants %s gold for the missile.")+"%s\n",
			ansi.FgBrightCyan, label, comma(price(target)), ansi.Reset)
		// Default NO: a missile can cost tens of millions, and Enter is the
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

// The three missile prices, each reading only the target fields its own routine
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
// after the faction-name array): 0a 0e 0c 04 05 0d 09 03 0b.
//
// The lookup is by SLOT, not by name, so a saved world whose factions carry
// other names still gets coloured (see pirateColor). That leaves this list
// paired with a list in another package by position alone, so each row carries
// the faction it paints and TestPirateColorsMatchTheFactions checks the pairing
// — a reordering of game.PirateFactions would otherwise repaint every faction
// silently.
var pirateColors = []struct {
	Faction string
	Color   string
}{
	{"Humans", ansi.FgBrightGreen},         // 10 light green
	{"Barbarians", ansi.FgBrightYellow},    // 14 yellow
	{"Nightjackals", ansi.FgBrightRed},     // 12 light red
	{"Sharks", ansi.FgRed},                 // 4 red
	{"Dunkleoids", ansi.FgMagenta},         // 5 magenta
	{"Trilobarians", ansi.FgBrightMagenta}, // 13 light magenta
	{"Raptorians", ansi.FgBrightBlue},      // 9 light blue
	{"Gorgonoids", ansi.FgCyan},            // 3 cyan
	{"Ammonians", ansi.FgBrightCyan},       // 11 light cyan
}

// pirateRaiderMark FOLLOWS the name of whichever faction raided the player since
// their last play (Empire.RaidersThisTurn) — IB's own addition, not BRE's;
// see the online-baron mark it borrows its treatment from (actions_info.go).
// Translatable like scoreOnlineMark, though an arrow is unlikely to need it.
// CP437 174, pointing back at the name it marks — an arrow drawn `->` to the
// RIGHT of a name points away from it at nothing. It is the head alone: the
// guillemet is a text glyph and sits off the horizontal centerline the rule
// characters are drawn on, so `«═` reads as two separate marks rather than one
// arrow. An ASCII session gets pirateRaiderMarkASCII instead: session/ascii.go
// would render the guillemet as a bare `<`, which reads as a stray character
// rather than a mark.
//
// It sat to the LEFT of the name until 2026-08-25, with every unmarked row
// holding the column blank so the names stayed in one column. That reserved
// indent read as a formatting fault on a screen where nothing had raided (#197),
// which is the common case; the mark trails the name now and an unmarked row
// carries nothing at all.
const pirateRaiderMark = "«"

// pirateRaiderMarkASCII is the mark for a session held to 7-bit ASCII. Not
// translated: such a session is held to English (see session/ascii.go). The
// shaft is welcome here — `<` and `=` are both drawn on the baseline, which is
// the alignment the CP437 pair lacks.
const pirateRaiderMarkASCII = "<="

// raiderMark renders the mark after a single space, and nothing whatsoever for
// a faction that did not raid.
//
// The mark carries its meaning alone, so it takes the same brighter gray
// (FgWhite) the online mark's letter does — measured 9.04:1 on VGA/CP437 and
// 11.54:1 on xterm against black, where dark gray manages 2.82:1 / 5.32:1. A
// translation that reinstates a shaft (`<-`) still gets the online mark's split:
// rule characters are decoration and take the dark gray.
func raiderMark(s session.Session, raided bool) string {
	if !raided {
		return ""
	}
	mark := tr(s, pirateRaiderMark)
	if session.IsASCII(s) {
		mark = pirateRaiderMarkASCII
	}
	var b strings.Builder
	b.WriteString(" ")
	for _, r := range mark {
		if r == '═' || r == '-' || r == '=' {
			b.WriteString(ansi.FgBrightBlack)
		} else {
			b.WriteString(ansi.FgWhite)
		}
		b.WriteRune(r)
	}
	if b.Len() == 1 {
		return ""
	}
	b.WriteString(ansi.Reset)
	return b.String()
}

// raidedSlot reports whether slot is in raiders — the factions that hit the
// player since their last play (Empire.RaidersThisTurn).
func raidedSlot(raiders []int, slot int) bool {
	for _, r := range raiders {
		if r == slot {
			return true
		}
	}
	return false
}

func attackPirates(s session.Session, w *ctx) Result {
	if blockedByProtection(s, w) {
		return Stay
	}
	// BRE lists only the colored faction names — a faction's strength and hoard
	// are hidden, so raiding blind (not knowing which of them is fat or lean) is
	// part of the game.
	var names []string
	var raiders []int
	w.With(func() {
		for _, p := range w.Pirates {
			names = append(names, p.Name)
		}
		if p := w.Player(); p != nil {
			raiders = p.RaidersThisTurn
		}
	})
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Attack Pirates"), ansi.Reset)
	for i, name := range names {
		color := ""
		if i < len(pirateColors) {
			color = pirateColors[i].Color
		}
		fmt.Fprintf(s, "  %d) %s%s%s%s\n", i+1, color, name, ansi.Reset, raiderMark(s, raidedSlot(raiders, i)))
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

// gooieKablooie is the planet's doomsday-weapon desk, BRE's "Gooie Kablooie
// Ops": start one, put money in, launch it, or scrap it. A planet builds one at
// a time and the barons fund it between them (#16).
func gooieKablooie(s session.Session, w *ctx) Result {
	if blockedByProtection(s, w) {
		return Stay
	}
	var d *game.Annihilator
	var enabled bool
	w.Read(func() {
		enabled = w.Config.GooieKablooie
		if w.Annihilator != nil {
			c := *w.Annihilator
			d = &c
		}
	})
	if !enabled {
		ok(s, "Gooie Kablooie operations are switched off in this game.")
		return Stay
	}
	if d == nil {
		return startAnnihilator(s, w)
	}
	showAnnihilator(s, w, d)
	switch {
	case d.Launched:
		ok(s, "The Gooie Kablooie is on its way. Nothing more can be done with it.")
	case d.Funded:
		// Nobody presses a button here: the weapon launches itself once
		// construction has run, and only the Coordinator can stand it down (#114).
		var hours int
		w.Read(func() { hours = (d.LaunchDay - w.GameDay) * 24 })
		if hours < 0 {
			hours = 0
		}
		ok(s, "The Gooie Kablooie is complete. It launches at %s in %d hours.", d.TargetBoard, hours)
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
	ok(s, "This planet has no Gooie Kablooie.")
	if !AskYesNo(s, "Would you like to begin construction?", false) {
		return Stay
	}
	var boards []string
	w.Read(func() {
		for _, b := range w.RemoteBoards {
			boards = append(boards, b.BoardID)
		}
	})
	if len(boards) == 0 {
		ok(s, "No other planets are known yet. Wait for inter-BBS scores to arrive.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightCyan, tr(s, "Target which planet?"), ansi.Reset)
	board := pickAddressee(s, w, boards)
	if board == "" {
		return Stay
	}
	var quote int
	w.Read(func() { quote = w.AnnihilatorQuote(board) })
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

// annihilatorDefense is the planet's answer to a Gooie Kablooie squatting on
// it: throw jets at the thing until it dies. The original asks every baron this
// at the top of their turn rather than hiding it behind a menu item
// (run_player_turn calls the routine directly), because the weapon needs the
// whole planet's air forces and a baron who never opens the InterPlanetary menu
// would never see it (#112).
//
// IB tracks one incoming weapon at a time, so the original's numbered picker is
// a single row here and there is no "Enter Gooie Number" prompt.
func annihilatorDefense(s session.Session, w *ctx) {
	var d *game.Annihilator
	var jets, needed int
	w.With(func() {
		if w.Incoming == nil || w.Incoming.DaysLeft <= 0 {
			return
		}
		c := *w.Incoming
		d = &c
		needed = int(w.AnnihilatorJetsNeeded())
	})
	if d == nil {
		return
	}
	withPlayer(w, func(p *game.Empire) { jets = p.Jets })

	fmt.Fprintf(s, "\n%s%-24s %-14s %s%s\n", ansi.FgWhite,
		tr(s, "From"), tr(s, "Strength"), tr(s, "Days Until Self-Destruct"), ansi.Reset)
	fmt.Fprintf(s, "%s%s %-14s %d%s\n", ansi.FgBrightWhite,
		padColumn(w.Term, d.Creator, 24), fmt.Sprintf("%d%%", d.Intact), d.DaysLeft, ansi.Reset)
	fmt.Fprintf(s, "%s\n", hiNums(fmt.Sprintf(
		tr(s, "It would take %s jets to destroy it outright."), comma(needed))))

	if !AskYesNo(s, "Do you wish to attack the Gooie Kablooie?", false) {
		return
	}
	if jets < 1 {
		ok(s, "Only jets can attack a Gooie Kablooie, and you have none.")
		return
	}
	send := promptSuggested(s, "Send how many jets?", 0, jets)
	if send < 1 {
		return
	}
	var knocked, lost int
	err := w.mutatePlayer(func(p *game.Empire) error {
		var e error
		knocked, lost, e = w.World.InterceptAnnihilator(p, send)
		return e
	})
	if err != nil {
		fail(s, err)
		return
	}
	var gone bool
	w.Read(func() { gone = w.Incoming == nil })
	fmt.Fprintf(s, "%s\n", hiNums(fmt.Sprintf(tr(s, "%s jets were destroyed in the attack!"), comma(lost))))
	if gone {
		ok(s, "The Gooie Kablooie was DESTROYED!")
		return
	}
	ok(s, "%d%% of the Gooie Kablooie destroyed!", knocked)
}
