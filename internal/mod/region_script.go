package mod

import (
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/andy5995/immortal-barons/internal/game"
)

// RegionsDir is the folder in a mod that holds its region scripts, one
// optional file per region type named for the type: regions/desert.star. Each
// is written in Starlark like PiratesFile and must define region(realm, row);
// the game asks it what that type's regions yield and cost for each realm.
const RegionsDir = "regions"

// regionScriptFile is the key a type's script has in a mod's files: always a
// forward slash, so a world saved on one system reads on another.
func regionScriptFile(g *game.Region) string { return RegionsDir + "/" + g.ID + ".star" }

// regionScriptType is the region type a file key names, or nil.
func regionScriptType(name string) *game.Region {
	id, ok := strings.CutPrefix(name, RegionsDir+"/")
	if !ok {
		return nil
	}
	id, ok = strings.CutSuffix(id, ".star")
	if !ok {
		return nil
	}
	return game.RegionByID(id)
}

// regionCacheMax bounds how many answers one region script remembers. The game
// asks for a realm's rows many times a turn and on every screen that shows a
// projection; a script's answer depends only on its arguments, so a repeat is
// answered from memory. When the memory fills it starts over.
const regionCacheMax = 4096

// regionPredeclared is what a region script sees besides Starlark's own
// built-ins: nothing. Unlike a pirate script it has no rand() or day(): its
// answer must be the same each time it is asked, or a screen would show one
// figure and the turn credit another; the day is a field of realm instead.
// option() is added at compile time and is fixed for the game being loaded,
// so it does not make an answer change from one screen to the next.
var regionPredeclared = starlark.StringDict{}

// regionScript is one compiled regions/<type>.star: the game.RegionBrain a mod
// adds to its rules for that type.
type regionScript struct {
	mod, hash string
	file      string // for messages: the path in the mod folder
	region    *game.Region
	prefs     *prefValues
	ref       *scriptRef

	mu    sync.Mutex
	cache map[regionKey]game.RegionRules
}

// regionKey is one question asked of a script: the realm and the row it was
// given, both comparable.
type regionKey struct {
	view game.RegionView
	row  game.RegionRules
}

// compileRegion runs src's top level and checks that it defines region with
// two parameters, under the same rules as compilePirates.
func compileRegion(file, src string, options *optionValues) (*starlark.Function, error) {
	pre := starlark.StringDict{}
	maps.Copy(pre, regionPredeclared)
	if options != nil {
		pre["option"] = options.builtin()
	}
	globals, err := execScript(file, src, pre)
	if err != nil {
		return nil, scriptFailure(file, err)
	}
	fn, ok := globals["region"].(*starlark.Function)
	if !ok {
		return nil, fmt.Errorf("%s: it must define region(realm, row)", file)
	}
	if fn.NumParams() != 2 || fn.HasVarargs() || fn.HasKwargs() || fn.NumKwonlyParams() != 0 {
		return nil, fmt.Errorf("%s: region must take exactly two parameters, region(realm, row)", file)
	}
	return fn, nil
}

// newRegionScript checks a mod's script for type g compiles with its chosen
// options, once per process for a given file, source and option values (see
// validated).
func newRegionScript(path, name, hash string, g *game.Region, src string, options *optionValues) (*regionScript, error) {
	file := filepath.Join(path, RegionsDir, g.ID+".star")
	ref := newScriptRef(kindRegion, file, src, options).named(name, hash, regionScriptFile(g))
	if err := ref.validate(); err != nil {
		return nil, err
	}
	return &regionScript{mod: name, hash: hash, file: file, region: g, ref: ref, cache: map[regionKey]game.RegionRules{}}, nil
}

// Region asks the script's region() about v and row, in a worker, and reads its
// answer: a dict of the figures it changes, such as {"food_base": 100}. A figure
// it leaves out keeps its value from row.
func (s *regionScript) Region(v game.RegionView, row game.RegionRules) (game.RegionRules, error) {
	key := regionKey{view: v, row: row}
	s.mu.Lock()
	if out, ok := s.cache[key]; ok {
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()

	call := wireMsg{Op: "call", ID: s.ref.id, Region: &wireRegion{View: v, Row: row}}
	call.PrefMod, call.Prefs = prefsToWire(s.prefs)
	res, err := callWorker(s.ref, call, nil)
	if err != nil {
		return row, err
	}
	out := *res.Row
	s.mu.Lock()
	if len(s.cache) >= regionCacheMax {
		clear(s.cache)
	}
	s.cache[key] = out
	s.mu.Unlock()
	return out, nil
}

// Failed reports why a realm played the type's row unchanged (warnScript).
func (s *regionScript) Failed(err error) {
	file := regionScriptFile(s.region)
	warnScript(s.mod+"\x00"+s.hash+"\x00"+file, s.mod, file, err, s.region.Name+" regions use the mod's values where it fails")
}

// regionRealmValue is region()'s first argument: a read-only struct, with the
// realm's land as a struct of counts by type ID.
func regionRealmValue(v game.RegionView, prefs *prefValues) starlark.Value {
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
		"pref":         prefs.builtin(v.ModPrefs),
	})
}

// regionRowValue is region()'s second argument: a read-only struct of the
// type's figures, named as values.toml names them.
func regionRowValue(row game.RegionRules) starlark.Value {
	fields := starlark.StringDict{}
	for _, f := range game.RegionFields {
		fields[f.Key] = starlark.MakeInt(f.Get(row))
	}
	return starlarkstruct.FromStringDict(starlarkstruct.Default, fields)
}

// readRegionRow reads region()'s answer onto a copy of row. Whether the
// figures are inside their limits is game.askRegionBrain's check.
func readRegionRow(res starlark.Value, row game.RegionRules) (game.RegionRules, error) {
	if res == starlark.None {
		return row, nil
	}
	d, ok := res.(*starlark.Dict)
	if !ok {
		return row, fmt.Errorf("region returned a %s; it must return a dict", res.Type())
	}
	out := row
	for _, kv := range d.Items() {
		key, _ := starlark.AsString(kv[0])
		f := game.RegionFieldByKey(key)
		if f == nil {
			return row, fmt.Errorf("unknown key %s", show(kv[0]))
		}
		n, err := starlark.AsInt32(kv[1])
		if err != nil {
			return row, fmt.Errorf("%s: %s", key, clean(truncate(err.Error(), 100)))
		}
		f.Set(&out, n)
	}
	return out, nil
}
