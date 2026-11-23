package mod

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"

	"github.com/andy5995/immortal-barons/internal/game"
)

// RegionsFile is a mod's optional region script, written in Starlark like
// PiratesFile. It must define regions(realm, rows); the game asks it what each
// realm's regions yield and cost, in place of the rows every realm plays by.
const RegionsFile = "regions.star"

// regionCacheMax bounds how many answers one region script remembers. The game
// asks for a realm's rows many times a turn and on every screen that shows a
// projection; a script's answer depends only on its arguments, so a repeat is
// answered from memory. When the memory fills it starts over.
const regionCacheMax = 4096

// regionPredeclared is what a region script sees besides Starlark's own
// built-ins: nothing. Unlike a pirate script it has no rand() or day(): its
// answer must be the same each time it is asked, or a screen would show one
// figure and the turn credit another; the day is a field of realm instead.
var regionPredeclared = starlark.StringDict{}

// regionScript is a compiled regions.star: the game.RegionBrain a mod adds to
// its rules.
type regionScript struct {
	mod, hash string
	file      string
	fn        *starlark.Function

	mu    sync.Mutex
	cache map[regionKey]map[*game.Region]game.RegionRules
}

// regionKey is one question asked of the script: the realm and the rows it
// was given, both comparable.
type regionKey struct {
	view game.RegionView
	rows [regionTypeCount]game.RegionRules
}

// regionTypeCount is how many region types there are, for regionKey's array.
const regionTypeCount = 9

func init() {
	if len(game.RegionTypes) != regionTypeCount {
		panic("mod: regionTypeCount does not match game.RegionTypes")
	}
}

// compileRegions runs src's top level and checks that it defines regions with
// two parameters, under the same rules as compilePirates.
func compileRegions(file, src string) (*starlark.Function, error) {
	globals, err := starlark.ExecFileOptions(&syntax.FileOptions{}, newThread(file), file, src, regionPredeclared)
	var ee *starlark.EvalError
	if errors.As(err, &ee) {
		return nil, fmt.Errorf("%s: %w", file, scriptError(err))
	}
	if err != nil {
		return nil, err
	}
	fn, ok := globals["regions"].(*starlark.Function)
	if !ok {
		return nil, fmt.Errorf("%s: it must define regions(realm, rows)", file)
	}
	if fn.NumParams() != 2 || fn.HasVarargs() || fn.HasKwargs() || fn.NumKwonlyParams() != 0 {
		return nil, fmt.Errorf("%s: regions must take exactly two parameters, regions(realm, rows)", file)
	}
	return fn, nil
}

// newRegionScript compiles a mod's regions.star, once per process for a given
// file and source (see compiled).
func newRegionScript(path, name, hash, src string) (*regionScript, error) {
	file := filepath.Join(path, RegionsFile)
	key := "regions\x00" + file + "\x00" + src
	s := &regionScript{mod: name, hash: hash, file: file, cache: map[regionKey]map[*game.Region]game.RegionRules{}}
	if fn, ok := compiled.Load(key); ok {
		s.fn = fn.(*starlark.Function)
		return s, nil
	}
	fn, err := compileRegions(file, src)
	if err != nil {
		return nil, err
	}
	compiled.Store(key, fn)
	s.fn = fn
	return s, nil
}

// Regions calls the script's regions() with v and rows and reads its answer:
// a dict of the types it changes, each a dict of the figures it changes, such
// as {"desert": {"food_base": 100}}. A type or figure it leaves out keeps its
// value from rows.
func (s *regionScript) Regions(v game.RegionView, rows map[*game.Region]game.RegionRules) (map[*game.Region]game.RegionRules, error) {
	key := regionKey{view: v}
	for i, g := range game.RegionTypes {
		key.rows[i] = rows[g]
	}
	s.mu.Lock()
	if out, ok := s.cache[key]; ok {
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()

	res, err := starlark.Call(newThread(s.file), s.fn, starlark.Tuple{regionRealmValue(v), regionRowsValue(rows)}, nil)
	if err != nil {
		return nil, scriptError(err)
	}
	out, err := readRegionRows(res, rows)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if len(s.cache) >= regionCacheMax {
		clear(s.cache)
	}
	s.cache[key] = out
	s.mu.Unlock()
	return out, nil
}

// Failed reports why a realm played the shared region rules, once per mod per
// process, as pirateScript.Failed does.
func (s *regionScript) Failed(err error) {
	warnMu.Lock()
	defer warnMu.Unlock()
	key := RegionsFile + "\x00" + s.mod + "\x00" + s.hash
	if warned[key] {
		return
	}
	warned[key] = true
	fmt.Fprintf(warnOut, "immortal-barons: mod %q: %s: %v; regions use the mod's values where it fails (reported once)\n", s.mod, RegionsFile, err)
}

// regionRealmValue is regions()'s first argument: a read-only struct, with
// the realm's land as a struct of counts by type ID.
func regionRealmValue(v game.RegionView) starlark.Value {
	counts := starlark.StringDict{}
	mix := v.Regions
	for _, g := range game.RegionTypes {
		counts[g.ID] = starlark.MakeInt(*g.Count(&mix))
	}
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"name":         starlark.String(v.Name),
		"day":          starlark.MakeInt(v.Day),
		"land":         starlark.MakeInt(v.Land),
		"regions":      starlarkstruct.FromStringDict(starlarkstruct.Default, counts),
		"people":       starlark.MakeInt(v.People),
		"support":      starlark.MakeInt(v.Support),
		"tax":          starlark.MakeInt(v.Tax),
		"turns_played": starlark.MakeInt(v.TurnsPlayed),
	})
}

// regionRowsValue is regions()'s second argument: a frozen dict from type ID
// to a struct of that type's figures, named as values.toml names them.
func regionRowsValue(rows map[*game.Region]game.RegionRules) starlark.Value {
	d := starlark.NewDict(len(game.RegionTypes))
	for _, g := range game.RegionTypes {
		fields := starlark.StringDict{}
		for _, f := range game.RegionFields {
			fields[f.Key] = starlark.MakeInt(f.Get(rows[g]))
		}
		_ = d.SetKey(starlark.String(g.ID), starlarkstruct.FromStringDict(starlarkstruct.Default, fields))
	}
	d.Freeze()
	return d
}

// readRegionRows reads regions()'s answer onto a copy of rows. Whether the
// figures are inside their limits is game.askRegionBrain's check.
func readRegionRows(res starlark.Value, rows map[*game.Region]game.RegionRules) (map[*game.Region]game.RegionRules, error) {
	out := make(map[*game.Region]game.RegionRules, len(rows))
	for g, rr := range rows {
		out[g] = rr
	}
	if res == starlark.None {
		return out, nil
	}
	d, ok := res.(*starlark.Dict)
	if !ok {
		return nil, fmt.Errorf("regions returned a %s; it must return a dict", res.Type())
	}
	for _, kv := range d.Items() {
		id, ok := starlark.AsString(kv[0])
		g := game.RegionByID(id)
		if !ok || g == nil {
			return nil, fmt.Errorf("%s is not a region type (%s)", kv[0], strings.Join(regionIDs(), ", "))
		}
		changes, ok := kv[1].(*starlark.Dict)
		if !ok {
			return nil, fmt.Errorf("%s: a %s, not a dict", id, kv[1].Type())
		}
		rr := out[g]
		for _, fv := range changes.Items() {
			key, _ := starlark.AsString(fv[0])
			f := game.RegionFieldByKey(key)
			if f == nil {
				return nil, fmt.Errorf("%s: unknown key %s", id, fv[0])
			}
			n, err := starlark.AsInt32(fv[1])
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %v", id, key, err)
			}
			f.Set(&rr, n)
		}
		out[g] = rr
	}
	return out, nil
}
