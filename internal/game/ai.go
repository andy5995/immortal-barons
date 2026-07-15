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
