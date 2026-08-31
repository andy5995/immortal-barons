package game

import (
	"fmt"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// ibbs_attack.go — interplanetary attacks: the force committed to one, the
// group and individual strikes that carry it, and the battle that resolves
// when it lands.

// AttackForce is the detachment a baron commits to a group attack (BRE lets you
// send troopers, jets, tanks, and bombers — real units, deducted from your
// army, not gold).
type AttackForce struct {
	Troopers int
	Jets     int
	Tanks    int
	Bombers  int
}

// Empty reports whether no units were committed.
func (f AttackForce) Empty() bool { return f.units() == 0 }

// units counts the whole detachment, whatever the type.
func (f AttackForce) units() int { return f.Troopers + f.Jets + f.Tanks + f.Bombers }

// offense values the detachment by the combat table (trooper 1, jet 2, tank 4).
//
// BOMBERS ADD NOTHING. BRE's interplanetary offence builder (ovr_03f4a0 +0x0000)
// reads the force record's troopers, jets and tanks (+0x4, +0x8, +0xc) and never
// its bombers (+0x10), because bombers fight the air battle against the
// defender's jets instead (remoteBattleAttrition). IB used to value one at a
// tank's 4, a placeholder from before this was read. Note the LOCAL resolver is
// different and does consult bombers — the two are separate subsystems.
func (f AttackForce) offense() int {
	return f.Troopers + f.Jets*2 + f.Tanks*4
}

// offenseAgainstSDI values the detachment as a defender's shield leaves it: the
// jets are blunted in proportion to the percentage and nothing else in the force
// is touched. Basis points, so the ceiling lands exactly where the original's
// reals do. The shield's effect on the BOMBERS is not here — they carry no
// offence at all, so it lands on the air battle instead (bombersAgainstSDI).
func (f AttackForce) offenseAgainstSDI(sdi int) int {
	jets := f.Jets * 2 * (10_000 - SDIJetReductionPct*sdi) / 10_000
	return f.Troopers + jets + f.Tanks*4
}

// bombersAgainstSDI is the bomber count that reaches the airfields, which is the
// only thing the air battle is decided by. BRE hands the SDI percentage to the
// bomber-count routine (ovr_03f4a0 +0x015a) exactly as it hands it to the
// offence builder, and that routine returns trunc((1 - SDI*0.2/100) * bombers).
func (f AttackForce) bombersAgainstSDI(sdi int) int {
	return f.Bombers * (10_000 - SDIBomberReductionPct*sdi) / 10_000
}

// Contribution records one baron's committed detachment, so the strike's
// strength and the returning survivors split per baron.
type Contribution struct {
	Owner string
	AttackForce
	// Tech is the contributor's Technology military factor at the moment the
	// detachment was committed, in TechFactorUnit fixed point (10000 = x1.0), and
	// it rides the packet so the target board can weigh this slot by it.
	//
	// BINARY-VERIFIED, and the piece #200 recorded as unread: the original's
	// force slot carries a Real48 at +0x1c that the offence builder multiplies
	// into that slot's troopers/jets/tanks total. Its writer is
	// configure_attack_forces (BRE.OVR ovr_02b783 +0x7f2..+0x825): it loads 1.4,
	// pushes slot 5, calls technology_factor, and stores the result into the
	// CALLER's own slot — so each contributor to a group attack brings their own
	// research, fixed when they joined, and the target board never recomputes
	// it. Zero in a packet written before this field existed, which reads as x1:
	// the strength that packet was sent with.
	Tech int `json:",omitempty"`
}

// techFactor is the slot's factor, x1 for a packet that carries none.
func (c Contribution) techFactor() int {
	if c.Tech <= 0 {
		return TechFactorUnit
	}
	return c.Tech
}

// offense and offenseAgainstSDI are the detachment's values with the
// contributor's own technology applied, which is what the original's builder
// computes per slot. They shadow AttackForce's so that every site summing
// contributors — the group's total, the SDI ratio, the spoils split — weighs
// each baron by what their forces are really worth.
func (c Contribution) offense() int {
	return techRaise(c.AttackForce.offense(), c.techFactor())
}

func (c Contribution) offenseAgainstSDI(sdi int) int {
	return techRaise(c.AttackForce.offenseAgainstSDI(sdi), c.techFactor())
}

// GroupAttack is a strike being assembled on this board. Until it leaves, other
// barons here may join it.
type GroupAttack struct {
	ID           int
	TargetBoard  string
	TargetEmpire string // "" = the whole planet (its strongest baron)
	// DepartAt is the instant the force leaves. BRE asks for a delay in HOURS
	// (12-120) and stores the answer as a wall-clock departure, which is what
	// lets a strike be timed to land before an opponent's next turn; a day
	// number cannot express that (#124).
	DepartAt time.Time
	// DepartDay is the pre-hours field, kept so a world saved before the change
	// still knows when its pending attacks leave. Only read when DepartAt is
	// zero; never written for a new attack.
	DepartDay    int `json:",omitempty"`
	Contributors []Contribution
}

// Due reports whether this attack's force has left by now. A pre-hours attack
// has no DepartAt, so it falls back to the game day it was filed against.
func (g GroupAttack) Due(now time.Time, gameDay int) bool {
	if g.DepartAt.IsZero() {
		return gameDay >= g.DepartDay
	}
	return !now.Before(g.DepartAt)
}

// DepartureAfter is the instant a group attack filed now leaves, given a delay
// in hours. It clamps to BRE's own window rather than trusting the caller: a
// packet-driven or scripted launch must not slip past the bounds the prompt
// enforces.
func DepartureAfter(now time.Time, hours int) time.Time {
	if hours < GroupAttackHoursMin {
		hours = GroupAttackHoursMin
	}
	if hours > GroupAttackHoursMax {
		hours = GroupAttackHoursMax
	}
	return now.Add(time.Duration(hours) * time.Hour)
}

// Offense is the strike's offensive strength: every contributor's detachment
// valued by the combat table.
func (g GroupAttack) Offense() int {
	total := 0
	for _, c := range g.Contributors {
		total += c.offense()
	}
	return total
}

// AttackKind is how an individual interplanetary strike is pressed. The values
// are BRE's own: its IBBS attack record stores Quick=0, Normal=1, Extended=2,
// so a packet stays readable to the original. The zero value is QuickStrike
// rather than the common case, which is BRE's encoding and not a default —
// every send names its kind.
type AttackKind int

const (
	QuickStrike AttackKind = iota
	NormalAttack
	ExtendedBattle
)

// attackKindRates holds each variant's published figures in one row, so adding
// a variant is one entry rather than an arm in four parallel switches. The
// figures themselves live in balance.go; this only names them.
var attackKindRates = map[AttackKind]struct {
	name                    string
	strength, capture, loss int
}{
	QuickStrike:    {"Quick Strike", QuickStrikeStrengthPct, QuickStrikeCapturePct, QuickStrikeLossPct},
	NormalAttack:   {"Normal Attack", NormalAttackStrengthPct, NormalAttackCapturePct, NormalAttackLossPct},
	ExtendedBattle: {"Extended Battle", ExtendedBattleStrengthPct, ExtendedBattleCapturePct, ExtendedBattleLossPct},
}

// rates returns k's row, falling back to the normal attack for a value that is
// not one of the three — an unreadable kind in an arriving packet resolves as
// an ordinary attack rather than crashing the far board's maintenance run.
func (k AttackKind) rates() (name string, strength, capture, loss int) {
	r, ok := attackKindRates[k]
	if !ok {
		r = attackKindRates[NormalAttack]
	}
	return r.name, r.strength, r.capture, r.loss
}

// String names the kind as BRE's screens and result headers do.
func (k AttackKind) String() string { name, _, _, _ := k.rates(); return name }

// strengthPct is how hard the attacker fights, as a percentage of its offense.
func (k AttackKind) strengthPct() int { _, s, _, _ := k.rates(); return s }

// capturePct is how much land the strike takes, relative to a normal attack.
func (k AttackKind) capturePct() int { _, _, c, _ := k.rates(); return c }

// lossPct is the share of each side's committed force spent before it retreats.
func (k AttackKind) lossPct() int { _, _, _, l := k.rates(); return l }

// RemoteAttack is a departed strike aimed at an empire on another board.
type RemoteAttack struct {
	ID           int
	FromBoard    string
	TargetEmpire string
	Offense      int
	Contributors []Contribution
	// Kind is how an individual strike is pressed. A group attack leaves it at
	// the zero value and is resolved as a normal attack (see Group below), which
	// is why resolution reads Group first.
	Kind AttackKind
	// Group marks a strike assembled by CreateGroupAttack. BRE gives group
	// attacks no type choice — only individual strikes pick one — so this
	// distinguishes "a group attack" from "an individual quick strike", which
	// share Kind's zero value.
	Group bool
	// FromEmpire is the realm that sent an INDIVIDUAL strike, so the defending
	// planet's news can name it (#108). A group attack leaves it empty: it is the
	// whole planet's doing, and naming one of several contributors would be a
	// lie. Empty in a packet written before this existed, which reads as "we know
	// only the planet" — what those boards could say anyway.
	FromEmpire string `json:",omitempty"`
}

// AttackResult returns to the origin board after a remote strike resolves. For
// a terror op (Kind == "terror") LandTaken carries the troopers destroyed
// instead of regions.
type AttackResult struct {
	ID           int
	TargetBoard  string
	TargetEmpire string
	LandTaken    int
	Won          bool
	Survivors    []Contribution // forces returning to their contributors (per owner)
	// Kind is what came home: "terror" for a terror op, otherwise how the strike
	// was pressed — "Quick Strike", "Normal Attack", "Extended Battle" or
	// "Group Attack", which is what BRE heads each returning report with.
	Kind string
	// Outcome says WHY, which Won alone cannot: BRE's returning report prints
	// SUCCESS, FAILURE, NOT FOUND or PROTECTED, and a baron whose force never
	// found its target is owed a different sentence from one that was beaten.
	// Empty in a packet written before this existed, which reads as Won deciding
	// between success and failure — the behaviour that packet was written under.
	Outcome AttackOutcome `json:",omitempty"`
	// Enemy is what the strike destroyed on the defender, by unit type. BRE's
	// returning report itemises it beside the attacker's own casualties; without
	// it the origin board can only say whether the strike won.
	Enemy UnitLoss `json:",omitempty"`
	// Report is the sentence the target board wrote for what a Special Operation
	// did (#49), and Score what the strike earned its sender. Both are settled
	// where the target lives, so neither can be worked out back home.
	Report string `json:",omitempty"`
	Score  int    `json:",omitempty"`
	// Backfired says an S3-Sabre turned on the realm that fired it. The
	// damage cannot be applied where it was rolled — that realm is on the board
	// that sent the missile — so it is applied when this answer gets home.
	Backfired bool `json:",omitempty"`
}

// AttackOutcome is a returning strike's verdict, matching the four BRE prints
// on its result header.
type AttackOutcome string

const (
	OutcomeWon       AttackOutcome = "success"
	OutcomeRepelled  AttackOutcome = "failure"
	OutcomeNotFound  AttackOutcome = "notfound"  // no such realm on the target board
	OutcomeProtected AttackOutcome = "protected" // shielded by New Realm Protection
)

// outcome reads a result's verdict, falling back to Won for a packet written
// before Outcome existed.
func (r AttackResult) outcome() AttackOutcome {
	if r.Outcome != "" {
		return r.Outcome
	}
	if r.Won {
		return OutcomeWon
	}
	return OutcomeRepelled
}

// InFlightStrike is a strike that has left this board and has not been answered.
// An inter-BBS attack is away for a whole packet round trip — out on this board's
// schedule, resolved on the target's, home again on theirs — and packets go
// missing. Holding the committed forces here lets them be given back when no
// result arrives (#96), rather than being lost with the packet.
type InFlightStrike struct {
	ID           int
	Kind         string // "attack" or "terror"
	TargetBoard  string
	TargetEmpire string
	LaunchedDay  int
	Contributors []Contribution // an attack's detachments, by owner
	Owner        string         // a terror op's sender
	Agents       int            // a terror op's committed agents
	// TerrorOp is which terrorist operation went out, so the returning report can
	// name it even when the far board answers with none (#165). SpecialOp's own
	// Op field below is a different menu's.
	TerrorOp TerrorOpType `json:",omitempty"`
	// Group marks a strike assembled by CreateGroupAttack, and Whole a strike
	// aimed at the planet rather than a named baron. Both are needed to word the
	// returning report and its news line: BRE keeps separate copy for an
	// individual strike, a group strike on one realm, and a group strike on a
	// whole planet, and the target board's answer carries none of that.
	Group bool `json:",omitempty"`
	Whole bool `json:",omitempty"`
	// An interplanetary trade bid's escrow (Kind "trade"): the gold held while
	// the bid is away, and what it was bidding for, so the lost-packet timer can
	// hand the money back and word the notice.
	Gold  int64  `json:",omitempty"`
	Good  string `json:",omitempty"`
	Qty   int    `json:",omitempty"`
	Price int    `json:",omitempty"`
	// Op is which Special Operation is away (Kind "special"), so a lost-packet
	// notice can name it.
	Op SpecialOp `json:",omitempty"`
}

// commitForce deducts the detachment f from e's army; ErrCantAfford if e lacks
// any unit type.
func (e *Empire) commitForce(f AttackForce) error {
	if e.Troopers < f.Troopers || e.Jets < f.Jets || e.Tanks < f.Tanks || e.Bombers < f.Bombers {
		return ErrCantAfford
	}
	e.Troopers -= f.Troopers
	e.Jets -= f.Jets
	e.Tanks -= f.Tanks
	e.Bombers -= f.Bombers
	return nil
}

// CreateGroupAttack starts a new group strike led by e, aimed at targetEmpire
// on targetBoard, leaving after hours hours (BRE's 12-120 window). e commits the
// detachment f (deducted from its army); ErrCantAfford if it lacks the units.
func (w *World) CreateGroupAttack(e *Empire, targetBoard, targetEmpire string, hours int, f AttackForce) (*GroupAttack, error) {
	if !w.CanGroupAttack(e) {
		return nil, ErrGroupAttacksExhausted
	}
	if err := e.commitForce(f); err != nil {
		return nil, err
	}
	e.GroupAttacksToday++
	w.NextAttackID++
	w.GroupAttacks = append(w.GroupAttacks, GroupAttack{
		ID:           w.NextAttackID,
		TargetBoard:  targetBoard,
		TargetEmpire: targetEmpire,
		DepartAt:     DepartureAfter(time.Now(), hours),
		Contributors: []Contribution{{Owner: e.Owner, AttackForce: f, Tech: e.TechMilitaryFactor()}},
	})
	g := &w.GroupAttacks[len(w.GroupAttacks)-1]
	// A watcher from the target planet sees the force assembling and sends the
	// hours home — the whole reason his planet paid for him.
	w.reportToSpy(targetBoard, groupAttackSpyLine(w.Config.BoardID, *g))
	return g, nil
}

// CreateIndividualAttack sends one baron's detachment against one named remote
// baron, BRE's "Indiv. Attack Force". Unlike a group attack it assembles nothing
// and waits for nobody: the force leaves on this run and the strike is in flight
// straight away. It spends one of the day's individual attacks, the same
// allowance a conventional attack at home draws on, and it must name a target —
// there is no whole-planet form.
//
// kind picks how the strike is pressed; it scales the offense that leaves here
// and travels with the packet so the target board can apply the matching
// capture and casualty rates.
func (w *World) CreateIndividualAttack(e *Empire, targetBoard, targetEmpire string, kind AttackKind, f AttackForce) (int, error) {
	if targetEmpire == "" {
		return 0, ErrNoTarget
	}
	if !w.CanAttack(e) {
		return 0, ErrAttacksExhausted
	}
	cost := w.AttackGoldCost(f)
	if e.Gold < cost {
		return 0, ErrCantAfford
	}
	if err := e.commitForce(f); err != nil {
		return 0, err
	}
	e.Gold -= cost
	e.AttacksToday++
	w.NextAttackID++
	id := w.NextAttackID
	contributors := []Contribution{{Owner: e.Owner, AttackForce: f, Tech: e.TechMilitaryFactor()}}
	w.enqueue(targetBoard, RemoteAttack{
		ID:           id,
		FromBoard:    w.Config.BoardID,
		TargetEmpire: targetEmpire,
		Offense:      contributors[0].offense() * kind.strengthPct() / 100,
		Contributors: contributors,
		Kind:         kind,
		FromEmpire:   e.Name,
	})
	w.InFlight = append(w.InFlight, InFlightStrike{
		ID:           id,
		Kind:         "attack",
		TargetBoard:  targetBoard,
		TargetEmpire: targetEmpire,
		LaunchedDay:  w.GameDay,
		Contributors: contributors,
	})
	return id, nil
}

// JoinGroupAttack commits e's detachment f to a pending group attack (before it
// leaves). ErrCantAfford if e lacks the units.
func (w *World) JoinGroupAttack(e *Empire, id int, f AttackForce) error {
	for i := range w.GroupAttacks {
		ga := &w.GroupAttacks[i]
		if ga.ID != id {
			continue
		}
		if ga.Due(time.Now(), w.GameDay) {
			return ErrDeparted
		}
		if !w.CanGroupAttack(e) {
			return ErrGroupAttacksExhausted
		}
		if err := e.commitForce(f); err != nil {
			return err
		}
		e.GroupAttacksToday++
		ga.Contributors = append(ga.Contributors, Contribution{Owner: e.Owner, AttackForce: f, Tech: e.TechMilitaryFactor()})
		return nil
	}
	return ErrNoAttack
}

// LaunchDueGroupAttacks turns every group attack whose departure has arrived
// into an outbound RemoteAttack and removes it from the pending list. Run
// during the PLANETARY maintenance step, which is why the window is worth
// having in hours: the step runs several times a day, so a 12-hour delay really
// does leave before a day-long one.
func (w *World) LaunchDueGroupAttacks() { w.LaunchDueGroupAttacksAt(time.Now()) }

// LaunchDueGroupAttacksAt is LaunchDueGroupAttacks against a given instant, so a
// test can watch a strike sit and then leave without waiting for the clock.
func (w *World) LaunchDueGroupAttacksAt(now time.Time) {
	var remaining []GroupAttack
	for _, ga := range w.GroupAttacks {
		if !ga.Due(now, w.GameDay) {
			remaining = append(remaining, ga)
			continue
		}
		w.enqueue(ga.TargetBoard, RemoteAttack{
			ID:           ga.ID,
			FromBoard:    w.Config.BoardID,
			TargetEmpire: ga.TargetEmpire,
			Offense:      ga.Offense(),
			Contributors: ga.Contributors,
			Group:        true,
		})
		w.InFlight = append(w.InFlight, InFlightStrike{
			ID:           ga.ID,
			Kind:         "attack",
			TargetBoard:  ga.TargetBoard,
			TargetEmpire: ga.TargetEmpire,
			LaunchedDay:  w.GameDay,
			Contributors: ga.Contributors,
			Group:        true,
			Whole:        ga.TargetEmpire == "",
		})
	}
	w.GroupAttacks = remaining
}

// offenseAgainstSDI is the strike's offense once the defender's shield has taken
// its share of the jets and bombers. Offense arrives already scaled by the
// strike's kind, so the shield is applied as the ratio between the blunted and
// the whole force rather than recomputed from scratch.
//
// A GROUP attack passes through untouched, which is the original's behaviour and
// not an oversight: it feeds the planet's land-weighted average shield through
// an expression that divides by 100 a second time (BRE.OVR ovr_03f4a0 +0xf18),
// and since no average can reach 100 the truncated result is always zero. Only a
// named baron's own shield ever defends anything.
func (atk RemoteAttack) offenseAgainstSDI(d *Empire) int {
	if atk.Group || d.SDI <= 0 || len(atk.Contributors) == 0 {
		return atk.Offense
	}
	whole, blunted := 0, 0
	for _, c := range atk.Contributors {
		whole += c.offense()
		blunted += c.offenseAgainstSDI(d.SDI)
	}
	if whole <= 0 {
		return atk.Offense
	}
	return int(int64(atk.Offense) * int64(blunted) / int64(whole))
}

// resolveRemoteAttack resolves a remote strike against its target empire (or,
// for a whole-planet attack, this board's strongest baron).
//
// Both armies press until they have spent the attack type's share of what they
// brought and then break off — BRE states that for the interplanetary variants
// outright ("both sides will only fight until they suffer 8% losses",
// game/attack.hlp), which is why this does not run the local Regular Attack's
// round-by-round attrition. That resolver exists to make the WINNER's casualties
// an outcome rather than a rate; here the rate is the published mechanic. The
// sysop's Attack Damage rescales it (Level.InterplanetaryLossPct).
func (w *World) resolveRemoteAttack(atk RemoteAttack) AttackResult {
	target := w.remoteTarget(atk.TargetEmpire)
	res := AttackResult{ID: atk.ID, TargetBoard: w.Config.BoardID, TargetEmpire: atk.TargetEmpire}
	kind := atk.Kind
	// A group attack has no type of its own, so it fights on the normal
	// attack's terms — and it is the baseline the individual strike's doubled
	// returns are measured against.
	returnsPct := 100
	if atk.Group {
		kind = NormalAttack
		res.Kind = "Group Attack"
	} else {
		res.Kind = kind.String()
		returnsPct = IndividualAttackReturnsPct
	}
	lossPct := w.Config.AttackDamage.InterplanetaryLossPct(kind.lossPct())
	// Survivors return to their contributors whatever the outcome — the force
	// bleeds on the way in, not only when it wins.
	res.Survivors = survivorsOf(atk.Contributors, lossPct)
	if target == nil {
		res.Outcome = OutcomeNotFound
		return res
	}
	// Logged where it RESOLVES, which is the board that can see the outcome.
	// The attacker's own board learns it from the result packet and would
	// otherwise record the same battle a second time.
	defer func() {
		attacker := atk.FromEmpire
		if attacker == "" {
			// A group attack is the whole planet's, not one realm's: BRE names
			// the board rather than inventing a leader for it.
			attacker = atk.FromBoard
		}
		w.logBattle(BattleLogEntry{
			Attacker: attacker, Defender: atk.TargetEmpire,
			Won: res.Won, Land: res.LandTaken, Remote: true,
		})
	}()
	// Who stands. A strike that names no baron is aimed at the PLANET and fights
	// every living realm at once; the strongest of them only names the report.
	planetWide := atk.TargetEmpire == ""
	defenders := []*Empire{target}
	if planetWide {
		defenders = w.planetDefenders()
		target = nil
		for _, e := range defenders {
			if target == nil || w.NetWorth(e) > w.NetWorth(target) {
				target = e
			}
		}
		if target == nil {
			res.Outcome = OutcomeNotFound
			return res
		}
	}
	res.TargetEmpire = target.Name
	// A target still under New Realm Protection is shielded, same as an incoming
	// terror op (resolveRemoteTerror). Protection counts down in transit, so this
	// arrival-time check — not the sender's view days earlier — is authoritative.
	// planetDefenders has already left protected realms out of the pool.
	if !planetWide && target.Protection > 0 {
		res.Outcome = OutcomeProtected
		target.addEvent(fmt.Sprintf("An interplanetary strike from %s was stopped by your New Realm Protection.", atk.FromBoard))
		w.postNews(fmt.Sprintf("A strike by %s broke on %s's New Realm Protection.", raider(atk), target.Name))
		return res
	}
	// Measure the defence BEFORE the battle costs it — the fight is decided by
	// what was standing when the force arrived, not by what is left afterwards.
	// The shield takes its share of the arriving jets and bombers first.
	offense := atk.offenseAgainstSDI(target)
	def, pooledJets := 0, 0
	for _, e := range defenders {
		def += e.remoteDefense()
		pooledJets += e.Jets
	}
	// Fight it out. What each side loses is the OUTCOME of the attrition, not
	// lossPct: that only says where the loop stops. Handing the defender lossPct
	// outright made a strike's damage independent of its size, so a token force
	// cost a large realm the full share every time (#199).
	won, atkLost, defLost, jetLost := w.remoteBattleAttrition(offense, def, pooledJets, atk.bombers(target), lossPct)
	res.Survivors = survivorsOfFrac(atk.Contributors, atkLost)
	// One battle, then the same two fractions applied to everyone who stood in it.
	// Each realm's OWN losses are kept apart from the planet's total: the report
	// a realm reads names what it lost, and on a planet-wide strike every realm
	// that stood used to be handed the planet's figure as if it were its own.
	losses := make([]UnitLoss, len(defenders))
	for i, e := range defenders {
		losses[i] = loseForcesSplit(e, defLost, jetLost)
		res.Enemy.Troopers += losses[i].Troopers
		res.Enemy.Jets += losses[i].Jets
		res.Enemy.Turrets += losses[i].Turrets
		res.Enemy.Tanks += losses[i].Tanks
	}
	if !won {
		res.Outcome = OutcomeRepelled
		// Say who held the field BEFORE the casualties, and say whose the
		// casualties are. A defender who beat the strike off still loses units,
		// and pairing a bare "repelled" with a count of the reader's own dead
		// reads as a contradiction (#201) — the original settles the battle first
		// and itemises afterwards (BRE.OVR resolve_received_invasion). Everyone
		// who stood is told: a planet-wide strike bleeds every realm, and only
		// the strongest of them was hearing about it.
		for i, e := range defenders {
			e.addEvent(invasionReport(atk, false, losses[i], 0))
		}
		// The whole planet hears it: an interplanetary exchange is planet against
		// planet, and until this the defending board printed nothing at all while
		// its scores moved (#108).
		if planetWide {
			w.postNews(fmt.Sprintf("The whole planet held the field against a strike by %s, losing %s of our forces.",
				raider(atk), numfmt.Comma(int64(res.Enemy.Total()))))
		} else {
			w.postNews(fmt.Sprintf("%s held the field against a strike by %s, losing %s of its own forces.",
				target.Name, raider(atk), numfmt.Comma(int64(res.Enemy.Total()))))
		}
		return res
	}
	// Overwhelmed: take a bite of land proportional to the margin, scaled by what
	// this kind of strike can carry off (capped). The margin is widened to int64
	// first: a league-sized strike can pass 21 million offense, and multiplying
	// that by 100 wraps a 32-bit int on the door builds this project supports.
	// A FLAT SHARE of what the defender holds, not a share of the margin. BRE
	// (resolve_received_invasion +0x1862) calls total_regions on the target,
	// multiplies by the capture percentage it built, divides by 100, truncates,
	// and takes max_i32 against a floor of 10.
	//
	// The percentage is the Attack Rewards ladder, doubled for a strike a baron
	// sent alone, then scaled by the type: Quick halves it, Extended multiplies
	// by 5/4 (+0x1338 onwards). None of it looks at how lopsided the battle was
	// — a squeaker and a rout carry off the same share, which is what makes an
	// interplanetary campaign a grind rather than one decisive blow.
	//
	// IB used to take a share of the MARGIN and quarter it, which is its own and
	// had no support.
	pct := float64(w.Config.AttackRewards.InterplanetaryCapturePct()) *
		float64(returnsPct) / 100 * float64(kind.capturePct()) / 100
	// The share is charged against each realm that stood, because the capture
	// reads total_regions on the realm it is taking from (+0x1862) and a
	// planet-wide strike ran that inside the same per-realm loop.
	land := 0
	for i, e := range defenders {
		take := int(float64(e.Land) * pct / 100)
		if take < InterplanetaryCaptureFloor {
			take = InterplanetaryCaptureFloor
		}
		if take > e.Land {
			take = e.Land
		}
		if take > 0 {
			e.Regions.remove(take)
			e.syncLand()
		}
		land += take
		e.addEvent(invasionReport(atk, true, losses[i], take))
	}
	if planetWide {
		w.postNews(fmt.Sprintf("%s struck the whole planet, carrying off %s regions and %s of our forces!",
			raider(atk), numfmt.Comma(int64(land)), numfmt.Comma(int64(res.Enemy.Total()))))
	} else {
		w.postNews(fmt.Sprintf("%s overran %s, carrying off %s of its regions and %s of its forces!",
			raider(atk), target.Name, numfmt.Comma(int64(land)), numfmt.Comma(int64(res.Enemy.Total()))))
	}
	res.LandTaken = land
	res.Won = true
	res.Outcome = OutcomeWon
	return res
}

// invasionReport is the private report a realm reads after an interplanetary
// strike landed on it. The SHAPE is the original's (resolve_received_invasion,
// BRE.OVR 0x040012, its strings read 2026-08-26): the verdict first, then the
// attacking force by unit type, then what THIS realm lost by unit type, each on
// a line of its own — and a zero is printed rather than skipped, because the
// original's lines are unrolled one per unit with no test on the count. The
// words are IB's. IB used to print a single total ("lost N units"), which
// answered nothing a player wants to know after a battle.
func invasionReport(atk RemoteAttack, won bool, lost UnitLoss, regions int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Invasion from %s.\n", raider(atk))
	if won {
		b.WriteString("Your forces lost the field.\n")
	} else {
		b.WriteString("Your forces held the field.\n")
	}
	var sent AttackForce
	for _, c := range atk.Contributors {
		sent.Troopers += c.Troopers
		sent.Jets += c.Jets
		sent.Tanks += c.Tanks
		sent.Bombers += c.Bombers
	}
	writeUnitLines(&b, "%s %s attacked.", attackUnits(sent))
	if won {
		fmt.Fprintf(&b, "You lost %d regions.\n", regions)
	}
	writeUnitLines(&b, "You lost %s %s.", defenceUnits(lost))
	return strings.TrimRight(b.String(), "\n")
}

// unitCount is one line of a battle report: how many of one unit type.
type unitCount struct {
	n    int
	name string
}

// attackUnits and defenceUnits are the two sides' unit lists in the order the
// original's reports print them — an attacker fields troopers, jets, tanks and
// bombers; a defender loses troopers, jets, tanks and turrets.
func attackUnits(f AttackForce) []unitCount {
	return []unitCount{{f.Troopers, "troopers"}, {f.Jets, "jets"}, {f.Tanks, "tanks"}, {f.Bombers, "bombers"}}
}

func defenceUnits(u UnitLoss) []unitCount {
	return []unitCount{{u.Troopers, "troopers"}, {u.Jets, "jets"}, {u.Tanks, "tanks"}, {u.Turrets, "turrets"}}
}

// writeUnitLines writes one line per unit type, format taking the count and
// the name, a zero printed rather than skipped: the original's report lines are
// unrolled one per unit with no test on the count, so "You lost 0 Bombers"
// appears on its screen and on this one.
// The count is SHORTENED (numfmt.Short), so the format's first verb is %s.
// BRE's interplanetary reports run it through the same helper its score table
// uses — a capture has "1000k Tanks returned." and "115k Turrets"
// (cap/20240527-134Pho_Lazarus_Public.cap). Its LOCAL resolver does not: a
// staged local battle printed "10469 Troopers" whole, and
// resolve_regular_attack is absent from the helper's caller list. That split is
// deliberate on both sides — do not make the two agree.
func writeUnitLines(b *strings.Builder, format string, units []unitCount) {
	for _, u := range units {
		fmt.Fprintf(b, format+"\n", numfmt.Short(u.n), u.name)
	}
}

// survivorsOfFrac returns each contributor's detachment reduced by the fraction
// the battle actually cost the attacker, for a strike that was fought. A force
// far stronger than the defence is barely touched; an evenly matched one pays
// close to the full retreat share.
func survivorsOfFrac(cs []Contribution, lost float64) []Contribution {
	keep := 1 - lost
	if keep < 0 {
		keep = 0
	}
	out := make([]Contribution, 0, len(cs))
	for _, c := range cs {
		out = append(out, Contribution{Owner: c.Owner, Tech: c.Tech, AttackForce: AttackForce{
			Troopers: shareOf(c.Troopers, keep),
			Jets:     shareOf(c.Jets, keep),
			Tanks:    shareOf(c.Tanks, keep),
			Bombers:  shareOf(c.Bombers, keep),
		}})
	}
	return out
}

// bombers is the whole arriving force's bomber count once the defender's shield
// has taken its share — the only thing that decides the air battle against the
// defender's jets. A group attack faces no shield at all, for the reason
// offenseAgainstSDI gives.
func (a RemoteAttack) bombers(d *Empire) int {
	sdi := d.SDI
	if a.Group || sdi < 0 {
		sdi = 0
	}
	n := 0
	for _, c := range a.Contributors {
		n += c.bombersAgainstSDI(sdi)
	}
	return n
}

// survivorsOf returns each contributor's detachment reduced by lossPct — the
// forces that come home after the strike. Used where no battle was fought (the
// target had vanished, or was under New Realm Protection).
func survivorsOf(cs []Contribution, lossPct int) []Contribution {
	keep := 100 - lossPct
	out := make([]Contribution, 0, len(cs))
	for _, c := range cs {
		out = append(out, Contribution{Owner: c.Owner, Tech: c.Tech, AttackForce: AttackForce{
			Troopers: c.Troopers * keep / 100,
			Jets:     c.Jets * keep / 100,
			Tanks:    c.Tanks * keep / 100,
			Bombers:  c.Bombers * keep / 100,
		}})
	}
	return out
}

// remoteTarget finds the empire a remote attack should hit: the named baron, or
// (for a whole-planet strike) this board's strongest living human empire.
func (w *World) remoteTarget(name string) *Empire {
	if name != "" {
		// Through the former name too, so a strike, message or op sent before the
		// realm renamed still finds it (see RenameEmpire).
		if e := w.FindByNameOrFormer(name); e != nil && e.Alive {
			return e
		}
		return nil
	}
	var best *Empire
	for _, e := range w.Empires {
		if e.Alive && e.Owner != "" && (best == nil || w.NetWorth(e) > w.NetWorth(best)) {
			best = e
		}
	}
	return best
}

// planetDefenders is everyone who stands when a strike is aimed at the PLANET
// rather than at a named baron.
//
// BRE marks that target with the letter Z — "Z=All" in its own picker — and on
// seeing it loops A..Y, summing each realm's defence and pooling their jets
// before ONE battle (resolve_received_invasion +0x0d3d, the loop at +0x0d47).
// Each realm is measured exactly as a lone defender would be, technology factor
// and all; the planet is simply the sum. IB used to send the whole strike
// against the single strongest baron, which made a planet-wide attack far
// cheaper than the original's and left everyone else untouched.
//
// Realms under New Realm Protection are left out of the pool: the loop skips a
// realm on a per-letter flag whose meaning is not read, and protection is what
// excludes a realm from every other arriving strike.
func (w *World) planetDefenders() []*Empire {
	var out []*Empire
	for _, e := range w.Empires {
		if e.Alive && e.Owner != "" && e.Protection <= 0 {
			out = append(out, e)
		}
	}
	return out
}

// takeInFlight removes the strike with this ID from the waiting list and returns
// it, because its result has come home. ok is false when nothing was waiting —
// see applyAttackResult for why that is not the same as "harmless".
func (w *World) takeInFlight(id int) (InFlightStrike, bool) {
	for i, f := range w.InFlight {
		if f.ID == id {
			w.InFlight = append(w.InFlight[:i], w.InFlight[i+1:]...)
			return f, true
		}
	}
	return InFlightStrike{}, false
}

// ReturnLostForces gives back the forces of any strike still unanswered after
// Config.LostForcesDays, and reports how many strikes it recovered. This is what
// stops a lost packet — a board that stops running transfers, a sysop who drops
// out — from costing a player their army for good (#96). A LostForcesDays of 0
// or less turns the recovery off, and the forces wait indefinitely.
//
// Run from the planetary step, after inbound packets have been applied, so a
// result that did arrive this run is never overtaken by the timer.
func (w *World) ReturnLostForces() int {
	days := w.Config.LostForcesDays
	if days <= 0 {
		return 0
	}
	var waiting []InFlightStrike
	recovered := 0
	for _, f := range w.InFlight {
		if w.GameDay-f.LaunchedDay < days {
			waiting = append(waiting, f)
			continue
		}
		recovered++
		if f.Kind == "trade" {
			if e := w.FindByOwner(f.Owner); e != nil {
				w.creditGold(e, f.Gold, "a bid that never came home")
				e.addEvent(fmt.Sprintf("No word came back from %s. Your bid for %d %s was abandoned and the gold returned.",
					f.TargetBoard, f.Qty, f.Good))
			}
			continue
		}
		if f.Kind == "terror" {
			if e := w.FindByOwner(f.Owner); e != nil {
				e.Agents += f.Agents
				e.addEvent(fmt.Sprintf("No word came back from %s. Your %d agents have returned home.", f.TargetBoard, f.Agents))
			}
			continue
		}
		// A Special Operation commits no forces and no agents — only the gold,
		// which was spent on launching it. There is nothing to hand back, so the
		// baron is told the strike was never heard of again rather than left
		// waiting on a report that is not coming.
		if f.Kind == "special" {
			if e := w.FindByOwner(f.Owner); e != nil {
				e.addEvent(fmt.Sprintf("No word came back from %s. Your %s against %s is presumed lost.",
					f.TargetBoard, SpecialOpLabel(f.Op), f.TargetEmpire))
			}
			continue
		}
		for _, c := range f.Contributors {
			e := w.FindByOwner(c.Owner)
			if e == nil {
				continue
			}
			e.Troopers += c.Troopers
			e.Jets += c.Jets
			e.Tanks += c.Tanks
			e.Bombers += c.Bombers
			e.addEvent(fmt.Sprintf("No word came back from %s. Your force sent against %s has returned home.", f.TargetBoard, f.TargetEmpire))
		}
	}
	w.InFlight = waiting
	if recovered > 0 {
		w.postNews(fmt.Sprintf("%d interplanetary force(s) gave up waiting and came home.", recovered))
	}
	return recovered
}

// raider names the far realm behind an incoming strike, for the defending
// planet's news: "Ironhold of Alpha" when the packet says which realm sent it,
// and the bare planet name otherwise (#108). A group attack is deliberately
// anonymous — it is the whole planet's doing — and so is a packet from a board
// old enough not to carry the field.
func raider(atk RemoteAttack) string {
	if atk.FromEmpire == "" {
		return atk.FromBoard
	}
	return atk.FromEmpire + " of " + atk.FromBoard
}
