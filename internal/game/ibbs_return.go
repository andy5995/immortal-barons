package game

import (
	"fmt"
	"strings"
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
func (w *World) applyAttackResult(res AttackResult) {
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
		w.applyTerrorResult(sent, res)
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
func (w *World) applyTerrorResult(sent InFlightStrike, res AttackResult) {
	if e := w.FindByOwner(sent.Owner); e != nil {
		e.addEvent(terrorReturnReport(sent, res))
	}
}

// terrorReturnReport is what the sender reads when a terrorist op comes home.
// It names the operation and says WHY nothing happened, where the three silent
// outcomes — repelled, sheltered, no such realm — used to arrive as one
// sentence the sender could not tell apart (#165). The Special Ops path already
// worked this way; this is the terror path catching up.
func terrorReturnReport(sent InFlightStrike, res AttackResult) string {
	op := sent.TerrorOp.String()
	switch res.outcome() {
	case OutcomeNotFound:
		return fmt.Sprintf("Your agents reached %s and found no realm called %s; the %s was abandoned.",
			res.TargetBoard, res.TargetEmpire, op)
	case OutcomeProtected:
		return fmt.Sprintf("Your %s against %s of %s broke on their New Realm Protection.",
			op, res.TargetEmpire, res.TargetBoard)
	}
	if res.Report != "" {
		// The target board settled what the operation did and wrote the lines;
		// only it knows what was there to damage (#166). The heading names the
		// operation and the target, which is what the original's report opens
		// with (process_terrorist_report's `Target:` header).
		return fmt.Sprintf("%s against %s of %s:\n%s", op, res.TargetEmpire, res.TargetBoard, res.Report)
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
// with the baron who sent it: what it did, what it earned, and — for an
// S3-Sabre that turned on its owner — the damage that could only be
// rolled where the target lives but has to land here.
func (w *World) applySpecialOpResult(sent InFlightStrike, res AttackResult) {
	e := w.FindByOwner(sent.Owner)
	if e == nil {
		return
	}
	label := SpecialOpLabel(sent.Op)
	if res.Backfired {
		// A backfire costs the firer NOTHING. The original's return path composes a
		// report and writes no field of the firer's record at all
		// (process_sabre_return, BRE.OVR +0x0F9C through its retf at +0x1173) — the
		// harm is that the missile developed land for the realm it was aimed at,
		// which the target's own board applied when it resolved the strike
		// (sabreDevelop). IB damaged the firer here until 2026-09-14; that was
		// invented before the return path was read (#266).
		// The target's board composed the report, land opened and all, so use it
		// rather than restating the outcome here: IB said "broke up over their
		// land instead of yours" until this was fixed, which named a thing that
		// cannot happen and read as a lucky escape rather than a loss.
		report := res.Report
		if report == "" {
			report = fmt.Sprintf("Your %s backfired and broke up over the realm it was aimed at.", label)
		}
		e.addEvent(fmt.Sprintf("%s (%s): %s", label, strikeTarget(sent, res), report))
		w.postNews(missileReturnNews(e.Name, label, strikeTarget(sent, res), res))
		return
	}
	switch res.outcome() {
	case OutcomeNotFound:
		e.addEvent(fmt.Sprintf("Your %s found no realm named %s on %s.", label, sent.TargetEmpire, sent.TargetBoard))
		return
	case OutcomeProtected:
		e.addEvent(fmt.Sprintf("Your %s against %s broke on their New Realm Protection.",
			label, strikeTarget(sent, res)))
		w.postNews(missileReturnNews(e.Name, label, strikeTarget(sent, res), res))
		return
	}
	if res.Score > 0 {
		addScore(e, res.Score)
	}
	report := res.Report
	if report == "" {
		report = fmt.Sprintf("Your %s against %s is over.", label, strikeTarget(sent, res))
	}
	e.addEvent(fmt.Sprintf("%s (%s): %s", label, strikeTarget(sent, res), report))
	if isMissileOp(sent.Op) {
		w.postNews(missileReturnNews(e.Name, label, strikeTarget(sent, res), res))
	}
}

// missileReturnNews is the line the FIRER's planet reads when a missile's answer
// comes home, one for every outcome the target's planet can read about it
// (missileNews) and agreeing with it. BINARY-VERIFIED that there is one:
// process_sabre_return (BRE.OVR 0x046045) calls the news writer on both of its
// paths — at +0x1051 for a nuclear or chemical strike, and for any failed
// strike, and at +0x1107 for an S3-Sabre that landed — so the original's firing
// planet reads a line whatever happened. The wording is IB's own.
//
// A result from a board that predates the narrower verdicts says only
// "failure", and gets a line that claims no more than that.
func missileReturnNews(firer, label, target string, res AttackResult) string {
	if res.Backfired {
		return fmt.Sprintf("%s's %s broke up over %s.", firer, label, target)
	}
	switch res.outcome() {
	case OutcomeWon:
		return fmt.Sprintf("%s's %s hit %s.", firer, label, target)
	case OutcomeMisfire:
		return fmt.Sprintf("%s's %s misfired on its way to %s.", firer, label, target)
	case OutcomeIntercepted:
		return fmt.Sprintf("The SDI of %s shot down %s's %s.", target, firer, label)
	case OutcomeNegligible:
		return fmt.Sprintf("%s's %s reached %s and did little harm.", firer, label, target)
	case OutcomeProtected:
		return fmt.Sprintf("New Realm Protection turned aside %s's %s against %s.", firer, label, target)
	}
	return fmt.Sprintf("%s's %s against %s failed.", firer, label, target)
}
