package game

import "errors"

// RegionBrain changes what one region type does for each realm (#202). A mod
// supplies one per type through Rules.RegionBrains; internal/mod runs it as a
// script, so this package never sees the scripting language. A type has at most
// one brain: a later mod's replaces an earlier one's, as a later mod's
// values.toml figure does.
//
// Region is given the realm and the type's row as every realm plays it
// (Rules.Regions, values.toml applied), and returns the row this realm's land
// of that type plays by instead. The engine asks wherever it needs a region
// figure — the income report, a projection on a screen, the turn itself — so
// the answer must depend on nothing but its arguments: a brain that drew random
// numbers would show a player one figure and credit another for the same realm.
// RegionView holds no gold for the same reason, since gold moves within a turn.
// The rest of it changes between turns (TurnsPlayed, People, Support), so a
// projection holds for the realm as it stands: a brain keyed on those can pay
// next turn a figure this turn's screen did not show.
//
// When Region fails, or returns a row outside the limits (RegionRules.Validate),
// the game calls Failed with the reason and that realm plays the type's row
// unchanged.
type RegionBrain interface {
	Region(v RegionView, row RegionRules) (RegionRules, error)
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

// regionsOf is the region rules e's land plays by: the world's, with each type
// that has a brain changed by it for e.
func (w *World) regionsOf(e *Empire) map[*Region]RegionRules {
	r := w.rules()
	if len(r.RegionBrains) == 0 {
		return r.Regions
	}
	rows := make(map[*Region]RegionRules, len(r.Regions))
	for g, rr := range r.Regions {
		rows[g] = rr
	}
	v := w.regionView(e)
	for _, g := range RegionTypes {
		b := r.RegionBrains[g]
		if b == nil {
			continue
		}
		row, err := askRegionBrain(b, g, v, rows[g])
		if err != nil {
			b.Failed(err)
			continue
		}
		rows[g] = row
	}
	return rows
}

func (w *World) regionView(e *Empire) RegionView {
	return RegionView{
		Name: e.Name, Day: w.GameDay, Land: e.Land, Regions: e.Regions,
		People: e.People, Support: e.Support, Tax: e.Tax, TurnsPlayed: e.TurnsPlayed,
	}
}

// askRegionBrain asks b about type g for v and checks the answer before it is
// used, so a refused answer changes nothing.
func askRegionBrain(b RegionBrain, g *Region, v RegionView, row RegionRules) (RegionRules, error) {
	out, err := b.Region(v, row)
	if err != nil {
		return row, err
	}
	if errs := out.Validate(g); len(errs) > 0 {
		return row, errors.Join(errs...)
	}
	return out, nil
}
