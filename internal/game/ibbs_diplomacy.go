package game

// Planetary diplomacy — the chart of this planet's standing with each of the
// others, shown on InterPlanetary Ops as "Diplomacy List" and beside every
// planet the player names.
//
// It is deliberately NOT a treaty system. BRE's own note on the editing screen
// says so (BRE.OVR 0x23425): "Planetary Diplomacy is *not* official. This is
// used for you to allow the gamers to learn of your status with other Planets.
// None of the info in this chart is official. (ie, none of this is forced nor
// reported to the other planets.)" So the value is set by hand, binds nothing,
// and never rides a packet — two boards can each call the other an Enemy, or
// disagree entirely, and the game does not care.
//
// One thing does read it: Allied Planets on the IP Messages menu, which
// addresses everyone the chart calls Allied.

// PlanetRelation is one entry in that chart. The four values are BRE's own
// display words (BRE.EXE 0x158b5, a four-slot table: Enemy, None, Peace,
// Allied); the editing screen offers them as War, None, Peace and Ally.
type PlanetRelation string

const (
	PlanetEnemy  PlanetRelation = "Enemy"
	PlanetNone   PlanetRelation = "None"
	PlanetPeace  PlanetRelation = "Peace"
	PlanetAllied PlanetRelation = "Allied"
)

// PlanetRelations lists the settable values in the order the editing prompt
// offers them.
func PlanetRelations() []PlanetRelation {
	return []PlanetRelation{PlanetEnemy, PlanetNone, PlanetPeace, PlanetAllied}
}

// PlanetRelationWith reports the recorded standing with board. A planet nobody
// has filed anything about reads None, which is what an empty chart shows.
func (w *World) PlanetRelationWith(board string) PlanetRelation {
	if r, ok := w.PlanetDiplomacy[board]; ok && r != "" {
		return r
	}
	return PlanetNone
}

// SetPlanetRelationWith files a standing with board. None is stored rather than
// deleted so the chart keeps a record of having been set back.
func (w *World) SetPlanetRelationWith(board string, r PlanetRelation) {
	if w.PlanetDiplomacy == nil {
		w.PlanetDiplomacy = map[string]PlanetRelation{}
	}
	w.PlanetDiplomacy[board] = r
}

// AlliedPlanetNames lists the known planets the chart calls Allied, in the
// order the planet list shows them, for the Allied Planets message target.
func (w *World) AlliedPlanetNames() []string {
	var allied []string
	for _, p := range w.KnownBoards() {
		if w.PlanetRelationWith(p) == PlanetAllied {
			allied = append(allied, p)
		}
	}
	return allied
}
