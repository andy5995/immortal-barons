package game

// ip_attack_ref_test.go — the interplanetary attack's resolver and its return as
// they were before the attack table, kept verbatim as the reference the base
// rules must still reproduce.

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/numfmt"
)

func (w *World) refResolveRemoteAttack(atk RemoteAttack) AttackResult {
	target := w.remoteTarget(atk.TargetEmpire)
	res := AttackResult{ID: atk.ID, TargetBoard: w.Config.BoardID, TargetEmpire: atk.TargetEmpire}
	kind := atk.Kind
	// A group attack has no type of its own, so it fights on the normal
	// attack's terms — and it is the baseline the individual strike's doubled
	// returns are measured against.
	returnsPct := 100
	if atk.Group {
		kind = NormalAttack
		res.Kind = msgid("Group Attack")
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
			// Realms that are all protected are not "no such realm": the strike
			// found the planet and was turned away, which is what the attackers
			// should read and the planet should hear.
			res.Outcome = OutcomeNotFound
			if w.anyLivingRealm() {
				res.Outcome = OutcomeProtected
				w.postNews(fmt.Sprintf("A strike by %s broke on the planet's New Realm Protection.", raider(atk)))
			}
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
		target.addEvent(say("An interplanetary strike from {board} was stopped by your New Realm Protection.", "board", atk.FromBoard))
		w.postNews(fmt.Sprintf("A strike by %s broke on %s's New Realm Protection.", raider(atk), target.Name))
		return res
	}
	// Measure the defense BEFORE the battle costs it — the fight is decided by
	// what was standing when the force arrived, not by what is left afterward.
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
	// What the attackers lost, for every defender's report: the same reckoning
	// their own returning report makes (forceLosses).
	theirs := forceLosses(forceOf(atk.Contributors), forceOf(res.Survivors))
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
		// and itemises afterward (BRE.OVR resolve_received_invasion). Everyone
		// who stood is told: a planet-wide strike bleeds every realm, and only
		// the strongest of them was hearing about it.
		for i, e := range defenders {
			e.addEvent(invasionReport(atk, false, losses[i], 0, theirs))
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
		// The same block knocks a fifth off the realm's HeadQuarters
		// (InvasionHQLossDivisor), whatever the regions came to.
		e.HQ -= e.HQ / InvasionHQLossDivisor
		e.addEvent(invasionReport(atk, true, losses[i], take, theirs))
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

func (w *World) refApplyAttackResult(res AttackResult, spy *SpyReport) {
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
	// what their own detachment cost, took and destroyed. Captured land is
	// parked rather than granted: the winner chooses the region types at home,
	// exactly as they do after a Regular Attack (#58), and the result arrives
	// while they are not in a session. The menu asks at the start of their next
	// turn (menu.spoilsStage).
	shares := splitSpoils(sent.Contributors, res.LandTaken)
	for i, c := range sent.Contributors {
		e := w.FindByOwner(c.Owner)
		if e == nil {
			continue
		}
		back := survivorFor(res.Survivors, c.Owner)
		e.Troopers += back.Troopers
		e.Jets += back.Jets
		e.Tanks += back.Tanks
		e.Bombers += back.Bombers
		land := 0
		if shares != nil {
			land = shares[i].Land
		}
		e.PendingRegions += land
		kills := shareOfLoss(res.Enemy, killShare(sent.Contributors, c))
		e.addEvent(strikeReport(sent, res, c.AttackForce, back, land, kills))
	}
	if line := w.returnNews(sent, res); line != "" {
		w.postNews(line)
	}
}
