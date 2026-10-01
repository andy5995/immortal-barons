package game

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// The origin board's half of an interplanetary strike: what happens when the
// answer comes home. Until this existed a baron who sent a force abroad learned
// nothing about it — the survivors reappeared in their army and one line went to
// the planet's news, which every realm reads (#107).
//
// BRE files a private per-baron report instead, and its shape is read off the
// original's returning-attack routine (BRE.OVR 0x04136c, unit ovr_03f4a0
// +0x1ecc): a header naming the attack type and the target realm on its planet,
// a verdict of SUCCESS / FAILURE / NOT FOUND / PROTECTED, one sentence saying
// what happened, then per-unit-type lines for what was lost, what came back, and
// what it destroyed. The wording below is IB's own; only the structure is the
// original's.

// applyAttackResult takes one returning result — an attack's or a terror op's —
// and gives the baron who sent it their forces, their report, and their spoils.
// spy is the report that came home beside a terror op, or nil; a Send Spy
// shows its figures to the sender.
func (w *World) applyAttackResult(res AttackResult, spy *SpyReport) {
	sent, waiting := w.takeInFlight(res.ID)
	if !waiting {
		// Nothing was waiting on this ID, so the force it belongs to has already
		// been given back by the lost-forces timer (or this is a second copy of a
		// packet). Crediting the survivors now would hand the owner a second army,
		// and BRE refuses the same case in the same place — its returning-attack
		// routine prints "Duplicate or Late Attack Return Recieved - Packet
		// Deleted" (BRE.OVR string 0x04115f), and reset.hlp's "Days for Lost
		// Attacks" entry says a late return is processed as though the assault was
		// never made.
		w.postNews("A report reached us from a force we had already given up for lost. It was set aside.")
		return
	}
	if res.Kind == "terror" {
		w.applyTerrorResult(sent, res, spy)
		return
	}
	if sent.Kind == "special" {
		w.applySpecialOpResult(sent, res)
		return
	}
	// Return each contributor's surviving forces to their army, and tell them
	// what the strike cost and did.
	for _, c := range sent.Contributors {
		e := w.FindByOwner(c.Owner)
		if e == nil {
			continue
		}
		back := survivorFor(res.Survivors, c.Owner)
		e.Troopers += back.Troopers
		e.Jets += back.Jets
		e.Tanks += back.Tanks
		e.Bombers += back.Bombers
		e.addEvent(strikeReport(sent, res, c.AttackForce, back))
	}
	// Captured land is parked rather than granted: the winner chooses the region
	// types at home, exactly as they do after a Regular Attack (#58), and the
	// result arrives while they are not in a session. The menu asks at the start
	// of their next turn (menu.spoilsStage).
	if res.LandTaken > 0 {
		for _, share := range splitSpoils(sent.Contributors, res.LandTaken) {
			if e := w.FindByOwner(share.Owner); e != nil {
				e.PendingRegions += share.Land
			}
		}
	}
	if line := w.returnNews(sent, res); line != "" {
		w.postNews(line)
	}
}

// applyTerrorResult is the returning half of a Terrorist Op: the agents are
// spent either way, so only the report comes home. It is the sender's alone —
// the original's returning-report routine (process_terrorist_report, BRE.OVR
// 0x04b38a) files recap entries and never calls the news writer, and IB posted
// "Our terror op on …" to the planet until #285.
func (w *World) applyTerrorResult(sent InFlightStrike, res AttackResult, spy *SpyReport) {
	e := w.FindByOwner(sent.Owner)
	if e == nil {
		return
	}
	report := terrorReturnReport(sent, res)
	// A spy that got in reads its figures out on the recap, not only into the
	// Spy Database, so a spy sent after a strike shows what the strike did.
	if spy != nil {
		report += fmt.Sprintf("\nLand %s  Off %s  Def %s  Gold %s",
			numfmt.Comma(spy.Land), numfmt.Comma(spy.Offense),
			numfmt.Comma(spy.Defense), numfmt.Comma(spy.Gold))
	}
	e.addEvent(report)
}

// terrorReturnReport is what the sender reads when a terrorist op comes home.
// It names the operation and says WHY nothing happened, where the three silent
// outcomes — repelled, sheltered, no such realm — used to arrive as one
// sentence the sender could not tell apart (#165).
func terrorReturnReport(sent InFlightStrike, res AttackResult) string {
	op := sent.TerrorOp.String()
	switch res.outcome() {
	case OutcomeNotFound:
		return fmt.Sprintf("Your agents reached %s and found no realm called %s, so they came home.",
			res.TargetBoard, res.TargetEmpire)
	case OutcomeProtected:
		return fmt.Sprintf("Your %s bounced off %s's New Realm Protection on %s.",
			op, res.TargetEmpire, res.TargetBoard)
	}
	if res.Report != "" {
		// The target board settled what the operation did and wrote the whole
		// sentence, naming the target and its own board; only it knows what was
		// there to damage (#166).
		return res.Report
	}
	if res.Won {
		return fmt.Sprintf("Your %s against %s of %s destroyed %d of its forces.",
			op, res.TargetEmpire, res.TargetBoard, res.LandTaken)
	}
	return fmt.Sprintf("Your %s against %s of %s was turned away.", op, res.TargetEmpire, res.TargetBoard)
}

// survivorFor picks one owner's returning detachment out of the result. An owner
// the target board did not answer for gets nothing back, which is the safe
// reading: the alternative is inventing units.
func survivorFor(cs []Contribution, owner string) AttackForce {
	for _, c := range cs {
		if c.Owner == owner {
			return c.AttackForce
		}
	}
	return AttackForce{}
}

// LandShare is one contributor's cut of a strike's captured regions.
type LandShare struct {
	Owner string
	Land  int
}

// splitSpoils divides captured land between the barons who paid for it, in
// proportion to the offense each committed. The remainder goes to the largest
// contributor rather than being dropped — a group attack that takes 3 regions
// between five barons must still hand over three.
func splitSpoils(cs []Contribution, land int) []LandShare {
	if land <= 0 || len(cs) == 0 {
		return nil
	}
	total := 0
	for _, c := range cs {
		total += c.offense()
	}
	if total <= 0 { // a force of nothing captured something: split it evenly
		total = len(cs)
	}
	shares := make([]LandShare, len(cs))
	left, biggest := land, 0
	for i, c := range cs {
		weight := c.offense()
		if weight <= 0 && total == len(cs) {
			weight = 1
		}
		shares[i] = LandShare{Owner: c.Owner, Land: land * weight / total}
		left -= shares[i].Land
		if weight > cs[biggest].offense() {
			biggest = i
		}
	}
	shares[biggest].Land += left
	return shares
}

// strikeReport is the private report one baron reads when their force comes
// home: what it was, where it went, how it went, and what it cost them.
func strikeReport(sent InFlightStrike, res AttackResult, committed, back AttackForce) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s results — %s.\n", res.Kind, strikeTarget(sent, res))
	switch res.outcome() {
	case OutcomeNotFound:
		b.WriteString("Your forces crossed the void and found no such realm waiting.\n")
	case OutcomeProtected:
		b.WriteString("Your forces found their target was in protection.\n")
	case OutcomeWon:
		fmt.Fprintf(&b, "Your forces broke the enemy and captured %d regions.\n", res.LandTaken)
	default:
		b.WriteString("Your forces were beaten off the field.\n")
	}
	// One line each from here, naming only the types that took a loss, which is
	// the shape of the original's returning report (resolve_returning_attack,
	// BRE.OVR 0x04136c) in two captures. See writeUnitLines.
	writeUnitLines(&b, "You lost %s!", attackUnits(forceLosses(committed, back)))
	// What the strike destroyed is only known where a battle was fought; a force
	// that found no realm, or found it shielded, destroyed nothing and says so by
	// omitting the line.
	switch res.outcome() {
	case OutcomeWon, OutcomeRepelled:
		writeUnitLines(&b, "You destroyed %s!", defenseUnits(res.Enemy))
	}
	writeUnitLines(&b, "%s returned.", attackUnits(back))
	return strings.TrimRight(b.String(), "\n")
}

// strikeTarget names what the strike was aimed at AND the board it went to,
// which the answer alone cannot: a whole-planet strike has no named realm on
// the way out, and one that found nothing has none on the way back either.
//
// strikeAim (ibbs_attack.go) is the same decision for a notice that has already
// named the board and so wants the target alone — hence "the whole planet"
// there against "the whole of <board>" here. The wordings differ because the
// sentences do; the whole-planet test is the part that must not.
func strikeTarget(sent InFlightStrike, res AttackResult) string {
	name := res.TargetEmpire
	if name == "" {
		name = sent.TargetEmpire
	}
	if sent.Whole || name == "" {
		return fmt.Sprintf("the whole of %s", res.TargetBoard)
	}
	return fmt.Sprintf("%s of %s", name, res.TargetBoard)
}

// forceLosses is what a detachment lost, by unit type.
func forceLosses(committed, back AttackForce) AttackForce {
	return AttackForce{
		Troopers: max(committed.Troopers-back.Troopers, 0),
		Jets:     max(committed.Jets-back.Jets, 0),
		Tanks:    max(committed.Tanks-back.Tanks, 0),
		Bombers:  max(committed.Bombers-back.Bombers, 0),
	}
}

// returnNews is the planet-wide line about a strike of ours coming home. BRE
// keeps six of these (game/ipnews.dat's IP-RET-INDIV / GROUPSINGLE / GROUPBBS,
// each with a win and a loss form), because who to congratulate differs: a solo
// baron is named, a group strike is the planet's doing, and a whole-planet raid
// names no enemy realm at all. Each line is a whole translatable sentence.
func (w *World) returnNews(sent InFlightStrike, res AttackResult) string {
	realm := strikeTarget(sent, res)
	// Won alone cannot pick the line: it is false for a strike that found no such
	// realm and for one turned away by New Realm Protection, and both were being
	// announced as a defeat in battle (#201). The original has no news category
	// for either — game/ipnews.dat carries a win and a loss form and nothing else
	// — so neither is planet news here.
	switch res.outcome() {
	case OutcomeNotFound, OutcomeProtected:
		return ""
	}
	won := res.outcome() == OutcomeWon
	if !sent.Group {
		// Name the baron who sent it. The planet's own news used to report the
		// strike anonymously, so nobody reading it knew which of their realms had
		// gone abroad (#108); a group attack stays the planet's doing.
		sender := strikeSender(w, sent)
		if won {
			return fmt.Sprintf("%s has returned in triumph from %s, carrying off %d regions!", sender, realm, res.LandTaken)
		}
		return fmt.Sprintf("%s has returned from %s with news of failure.", sender, realm)
	}
	if won {
		return fmt.Sprintf("Our planet's forces have returned in triumph from %s and captured %d regions!", realm, res.LandTaken)
	}
	return fmt.Sprintf("Our planet's forces have returned in disarray from a loss against %s.", realm)
}

// strikeSender is the realm behind an individual strike, for our own planet's
// news. A strike has one contributor; a realm that has since fallen leaves only
// the handle, so the line still names somebody rather than reading as nobody.
func strikeSender(w *World, sent InFlightStrike) string {
	owner := sent.Owner
	if owner == "" && len(sent.Contributors) > 0 {
		owner = sent.Contributors[0].Owner
	}
	if e := w.FindByOwner(owner); e != nil {
		return e.Name
	}
	if owner == "" {
		return "Our forces"
	}
	return owner
}

// applySpecialOpResult files the answer to an interplanetary Special Operation
// with the baron who sent it: what it did and what it earned.
//
// The target's board wrote the report as one sentence naming the target and its
// own board, and it is printed as it stands.
func (w *World) applySpecialOpResult(sent InFlightStrike, res AttackResult) {
	e := w.FindByOwner(sent.Owner)
	if e == nil {
		return
	}
	label := SpecialOpLabel(sent.Op)
	missile := missileNoun(sent.Op)
	board := res.TargetBoard
	if board == "" {
		board = sent.TargetBoard
	}
	target := res.TargetEmpire
	if target == "" {
		target = sent.TargetEmpire
	}
	news := func() { w.postNews(missileReturnNews(e.Name, missile, target, board, res)) }
	if res.Backfired {
		// A backfire costs the firer NOTHING. The original's return path composes a
		// report and writes no field of the firer's record at all
		// (process_sabre_return, BRE.OVR +0x0F9C through its retf at +0x1173) — the
		// harm is that the missile developed land for the realm it was aimed at,
		// which the target's own board applied when it resolved the strike
		// (sabreDevelop). IB damaged the firer here until 2026-09-14; that was
		// invented before the return path was read (#266).
		report := fmt.Sprintf("Your %s backfired on %s of %s.", missile, target, board)
		if res.Report != "" {
			report = res.Report
		}
		e.addEvent(report)
		news()
		return
	}
	// A bombing run's firer learns of it through the planet news alone.
	// BINARY-VERIFIED: process_bombing_results (BRE.OVR 0x04a4a6) calls the
	// news writer at +0x0622 and never the per-realm event writer. IB filed an
	// event with the firer as well until 2026-10-01. A landed run's report is the
	// share destroyed, which the news line names as the original's does.
	if !isMissileOp(sent.Op) {
		w.postNews(bombingReturnNews(e.Name, sent.Op, board, res))
		return
	}
	// A failed S3-Sabre is news alone too. process_sabre_return (BRE.OVR
	// 0x046045) posts the failure line at +0x1051 and then, for the Sabre
	// (+0x1056), skips the event its nuclear and chemical siblings get at
	// +0x10b1; only a Sabre that landed tells its firer in person (+0x116b).
	// Protection fails a strike the same way on the target's board, so it is
	// covered too. IB told the firer in person until 2026-10-01.
	if sent.Op == OpSabre && sabreReturnFailed(res) {
		news()
		return
	}
	switch res.outcome() {
	case OutcomeNotFound:
		e.addEvent(fmt.Sprintf("Your %s reached %s and found no realm called %s.", missile, sent.TargetBoard, sent.TargetEmpire))
		return
	case OutcomeProtected:
		e.addEvent(fmt.Sprintf("Your %s bounced off %s's New Realm Protection on %s.", missile, target, board))
		news()
		return
	}
	if res.Score > 0 {
		addScore(e, res.Score)
	}
	report := fmt.Sprintf("Your %s against %s is over.", label, strikeTarget(sent, res))
	if res.Report != "" {
		report = res.Report
	}
	e.addEvent(report)
	news()
}

// sabreReturnFailed reports whether an S3-Sabre's answer is one of the
// original's failures, which come home as news only: a misfire, an SDI
// interception, the garrison, New Realm Protection, or a plain failure from an
// older board. A Sabre that landed, even for negligible damage, is not one.
func sabreReturnFailed(res AttackResult) bool {
	switch res.outcome() {
	case OutcomeMisfire, OutcomeIntercepted, OutcomeGuarded, OutcomeProtected, OutcomeRepelled:
		return true
	}
	return false
}

// bombingReturnNews is the line the FIRER's planet reads when a bombing run's
// answer comes home, one per outcome the target's planet can read about it
// (planetOpNews) and agreeing with it. BINARY-VERIFIED that there is one:
// process_bombing_results (BRE.OVR 0x04a4a6) has a single branch, at +0x501,
// which only picks the failure or success line, and reaches its news call at
// +0x0622 either way. The wording is IB's own. A run driven off reads one plain
// line here: the pool entry picked on the target's board does not travel.
//
// A result from a board that predates the narrower verdicts says only
// "failure", which covers both ways of coming to nothing.
func bombingReturnNews(firer string, op SpecialOp, board string, res AttackResult) string {
	switch res.outcome() {
	case OutcomeWon:
		line := fmt.Sprintf("%s's %s against %s landed.", firer, SpecialOpLabel(op), board)
		switch op {
		case OpBombFood:
			line = fmt.Sprintf("%s's bombers burned %s's food market.", firer, board)
		case OpBombMarket:
			line = fmt.Sprintf("%s's bombers wrecked %s's trading market.", firer, board)
		case OpBombRoutes:
			line = fmt.Sprintf("%s's bombers hit trade routes across %s.", firer, board)
		case OpUndermine:
			line = fmt.Sprintf("%s's bombers undermined %s's bank.", firer, board)
		}
		// The share destroyed, as the target board reported it.
		if res.Report != "" {
			line += " " + res.Report
		}
		return line
	case OutcomeDrivenOff:
		return fmt.Sprintf("%s's bombers were turned back before they reached %s.", firer, board)
	case OutcomeNothing:
		return fmt.Sprintf("%s's bombers found nothing to destroy on %s.", firer, board)
	}
	return fmt.Sprintf("%s's %s against %s came to nothing.", firer, SpecialOpLabel(op), board)
}

// missileReturnNews is the line the FIRER's planet reads when a missile's answer
// comes home, one for every outcome the target's planet can read about it
// (missileNews) and agreeing with it. BINARY-VERIFIED that there is one:
// process_sabre_return (BRE.OVR 0x046045) calls the news writer on both of its
// paths — at +0x1051 for a nuclear or chemical strike, and for any failed
// strike, and at +0x1107 for an S3-Sabre that landed — so the original's firing
// planet reads a line whatever happened. The wording is IB's own. A misfire
// reads one plain line here: the pool entry picked on the target's board does
// not travel.
//
// A result from a board that predates the narrower verdicts says only
// "failure", and gets a line that claims no more than that. A backfire's line
// gives no count: the land it opened is settled on the target's board and does
// not travel.
func missileReturnNews(firer, missile, target, board string, res AttackResult) string {
	if res.Backfired {
		return fmt.Sprintf("%s's %s backfired on %s of %s.", firer, missile, target, board)
	}
	switch res.outcome() {
	case OutcomeWon:
		return fmt.Sprintf("%s's %s hit %s of %s.", firer, missile, target, board)
	case OutcomeMisfire:
		return fmt.Sprintf("%s's %s vanished on the way to %s.", firer, missile, board)
	case OutcomeIntercepted:
		return fmt.Sprintf("%s's SDI shot down %s's %s.", target, firer, missile)
	case OutcomeGuarded:
		return fmt.Sprintf("%s's defenses brought down %s's %s.", target, firer, missile)
	case OutcomeNegligible:
		return fmt.Sprintf("%s's %s barely scratched %s of %s.", firer, missile, target, board)
	case OutcomeProtected:
		return fmt.Sprintf("New Realm Protection turned aside %s's %s.", firer, missile)
	}
	return fmt.Sprintf("%s's %s against %s of %s failed.", firer, missile, target, board)
}
