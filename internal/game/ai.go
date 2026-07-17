package game

import (
	"hash/fnv"
	"io"
)

// AI personality archetypes (#36). An AI's profile shapes its diplomacy — which
// treaty offers it accepts — and is the seam for its future military posture and
// willingness to go to war. Assigned at seed (addAIEmpire); an empty profile
// (human empire, or an AI saved before profiles existed) derives a stable
// fallback from the empire name so old saves still get a personality.
const (
	AIProfileDiplomat  = "diplomat"  // accepts most cooperative pacts
	AIProfileBalanced  = "balanced"  // accepts trade and defense; guards intel/tech
	AIProfileAggressor = "aggressor" // accepts only self-serving economic pacts
)

// aiProfiles is the assignment pool, cycled at seed for an even spread.
var aiProfiles = []string{AIProfileDiplomat, AIProfileBalanced, AIProfileAggressor}

// aiProfile returns e's personality, deriving a stable one from its name when
// unset so AI empires saved before profiles existed still behave consistently.
func (e *Empire) aiProfile() string {
	if e.AIProfile != "" {
		return e.AIProfile
	}
	h := fnv.New32a()
	io.WriteString(h, e.Name)
	return aiProfiles[h.Sum32()%uint32(len(aiProfiles))]
}

// AI economic skill: sharp barons expand at full tilt; dull barons throttle their
// land-buying (AIDullLandBuyPct) and so grow slower, giving a game a mix of strong
// and weak rivals. Assigned randomly at seed (addAIEmpire).
const (
	AISkillSharp = "sharp"
	AISkillDull  = "dull"
)

// aiSkill returns e's economic skill, deriving a stable one from its name when
// unset so AI empires saved before skill existed still behave consistently.
func (e *Empire) aiSkill() string {
	if e.AISkill != "" {
		return e.AISkill
	}
	h := fnv.New32a()
	io.WriteString(h, e.Name+"skill") // distinct salt from aiProfile's name hash
	if h.Sum32()%2 == 0 {
		return AISkillDull
	}
	return AISkillSharp
}

// aiAcceptsTreaty reports whether an AI of the given profile accepts a proposed
// treaty type. A diplomat takes any cooperative pact; an aggressor takes only
// self-serving economic ones (and keeps its hands free to attack); a balanced
// realm takes trade and defense but guards its intelligence and technology.
func aiAcceptsTreaty(profile, ttype string) bool {
	switch profile {
	case AIProfileDiplomat:
		return true
	case AIProfileAggressor:
		return ttype == "Tariff Trade Agreement" || ttype == "Technology Agreement"
	default: // balanced
		return ttype != "Intelligence Alliance" && ttype != "Technology Agreement"
	}
}

// aiForceShares returns the trooper/turret/tank gold shares an AI of the given
// profile spends its military budget on (#36). Aggressors lean into offense
// (tanks + troopers, few turrets); every other profile keeps the defensive
// default (turret-heavy). Each triple sums to 100.
func aiForceShares(profile string) (trooper, turret, tank int) {
	if profile == AIProfileAggressor {
		return AIForceTrooperPctWar, AIForceTurretPctWar, AIForceTankPctWar
	}
	return AIForceTrooperPct, AIForceTurretPct, AIForceTankPct
}

// effectiveDefense is a realm's total defensive strength as the battle math
// sees it: its unit defense plus the per-region land bonus. The AI uses this to
// judge whether a target is winnable (#36).
func effectiveDefense(e *Empire) int {
	return e.Defense() + e.Land*LandDefenseBonus
}

// aiWageWar lets an aggressor-profile AI strike the weakest valid target when it
// is clearly favored (#36). It reuses World.Targets (which already excludes
// dead, protected, and allied realms) and only commits when its offense beats
// the target's effective defense by AIWarOffenseMargin, so it starts winnable
// wars rather than suicidal ones. Non-aggressors and protected AIs never start a
// war. At most one attack per call (one per turn, BRE-style).
func (w *World) aiWageWar(e *Empire) {
	if e.Protection > 0 || e.aiProfile() != AIProfileAggressor {
		return
	}
	var target *Empire
	for _, t := range w.Targets(e) {
		if target == nil || effectiveDefense(t) < effectiveDefense(target) {
			target = t
		}
	}
	if target == nil {
		return
	}
	if e.Offense() > effectiveDefense(target)*AIWarOffenseMargin/100 {
		// Soften the target with a covert strike first: demoralized forces defend
		// worse, so the aggressor's agents pave the way for the attack (#36).
		if e.Agents > 0 {
			w.DemoralizeForces(e, target)
		}
		w.Attack(e, target, FullForce(e)) // the AI commits its whole army
	}
}

// aiHandleDiplomacy responds to an AI's pending treaty offers (#36): it accepts
// the ones its personality favors and drops the rest, so a player (or another
// AI) proposing a pact to an AI gets a real answer instead of silence. Declined
// offers are discarded rather than left to re-prompt every turn.
func (w *World) aiHandleDiplomacy(e *Empire) {
	if len(e.TreatyOffers) == 0 {
		return
	}
	profile := e.aiProfile()
	for _, o := range append([]TreatyOffer(nil), e.TreatyOffers...) {
		if aiAcceptsTreaty(profile, o.Type) {
			w.AcceptTreaty(e, o.From, o.Type) // matches, consumes the offer, forms the treaty
		}
	}
	e.TreatyOffers = nil // discard any the AI declined
}
