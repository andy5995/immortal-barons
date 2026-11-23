package game

import (
	"errors"
	"fmt"
)

// RegionBrain changes what a realm's regions do (#202). A mod supplies one
// through Rules.RegionBrains; internal/mod runs it as a script, so this package
// never sees the scripting language.
//
// Brains chain, in mod order. The first is given the realm and the rules every
// realm plays by (Rules.Regions, values.toml applied); each after it is given
// the rows the one before returned. So two mods that change different region
// types both take effect, and where they change the same figure the later mod
// wins, as it does in values.toml. The engine asks wherever it needs a region figure — the income
// report, a projection on a screen, the turn itself — so the answer must
// depend on nothing but its arguments: a brain that drew random numbers would
// show a player one figure and credit another. RegionView holds no gold for
// the same reason, since gold moves within a turn.
//
// When Regions fails, or returns a row outside the limits (RegionRules.Validate),
// the game calls Failed with the reason and passes on the rows that brain was
// given, so a failing brain drops only its own changes.
type RegionBrain interface {
	Regions(v RegionView, rows map[*Region]RegionRules) (map[*Region]RegionRules, error)
	Failed(err error)
}

// RegionView is the realm a RegionBrain decides for. Every field is
// comparable, so a brain can remember its answer for a view it has seen.
type RegionView struct {
	Name    string
	Day     int
	Land    int
	Regions RegionMix
	People  int
	Support int
	Tax     int
	// TurnsPlayed is the realm's lifetime turn count.
	TurnsPlayed int
}

// regionsOf is the region rules e's land plays by: the world's, as each of
// the rules' brains in turn changes them for e.
func (w *World) regionsOf(e *Empire) map[*Region]RegionRules {
	r := w.rules()
	rows := r.Regions
	if len(r.RegionBrains) == 0 {
		return rows
	}
	v := w.regionView(e)
	for _, b := range r.RegionBrains {
		out, err := askRegionBrain(b, v, rows)
		if err != nil {
			b.Failed(err)
			continue
		}
		rows = out
	}
	return rows
}

func (w *World) regionView(e *Empire) RegionView {
	return RegionView{
		Name: e.Name, Day: w.GameDay, Land: e.Land, Regions: e.Regions,
		People: e.People, Support: e.Support, Tax: e.Tax, TurnsPlayed: e.TurnsPlayed,
	}
}

// askRegionBrain asks b about v and checks the answer before any of it is
// used, so a refused answer changes nothing.
func askRegionBrain(b RegionBrain, v RegionView, base map[*Region]RegionRules) (map[*Region]RegionRules, error) {
	rows, err := b.Regions(v, base)
	if err != nil {
		return nil, err
	}
	var errs []error
	for _, g := range RegionTypes {
		rr, ok := rows[g]
		if !ok {
			errs = append(errs, fmt.Errorf("%s: missing", g.ID))
			continue
		}
		errs = append(errs, rr.Validate(g)...)
	}
	return rows, errors.Join(errs...)
}
