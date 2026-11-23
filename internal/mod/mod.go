// Package mod reads game mods: folders under <DataDir>/mods that change a
// world's Rules (#202).
//
// A mod is a folder holding a manifest, mod.toml, and its value overrides,
// values.toml. The base game is the base mod: its values are the balance*.go
// constants, built into game.BaseRules, so a mod states only what it changes.
// Mods apply in the order a sysop lists them, and a later one wins.
//
// A mod is read strictly. A key this version does not know is an error rather
// than something skipped, because a misspelled key that was silently ignored
// would leave a sysop believing a value changed when it had not. The same goes
// for a file in the folder this version does not use: a mod carrying one would
// otherwise load here and play without it.
//
// A mod may also carry scripts: pirates.star, which decides pirate raids
// (script.go), and regions.star, which decides what each realm's regions yield
// and cost (region_script.go).
package mod

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"

	"github.com/andy5995/immortal-barons/internal/game"
)

// Dir is the folder under the data directory that holds the mods.
const Dir = "mods"

// The files a mod folder may hold, besides PiratesFile and RegionsFile. values.toml is
// optional: a mod that changes nothing yet is still a mod.
const (
	ManifestFile = "mod.toml"
	ValuesFile   = "values.toml"
)

// validName keeps a mod's name to one plain path element, so a name read from
// config.json can never reach outside the mods folder.
var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Manifest is mod.toml.
type Manifest struct {
	// Name must match the folder's name: it is what config.json lists.
	Name    string `toml:"name"`
	Version string `toml:"version"`
	// Label is what the Game Setup screen shows; blank shows Name.
	Label       string `toml:"label"`
	Description string `toml:"description"`
	// ScriptAPI is the version of the script interface the mod's scripts were
	// written for. It is required of a mod that carries a script: see
	// ScriptAPI.
	ScriptAPI int `toml:"script_api"`
}

// ScriptAPI is the version of the script interface this game runs: what a
// script is given and what it may return. A change that could break a script
// written for it raises the number, so the game can tell a sysop which mods
// need updating instead of failing in play; adding a field a script may read,
// or a key it may return, does not. A mod declares the version its scripts
// were written for as script_api in mod.toml.
const ScriptAPI = 1

// Mod is one mod folder, read and hashed.
type Mod struct {
	Manifest
	Path string
	// Hash covers every file in the folder, so a world can tell that a mod it
	// was reset with has since been edited.
	Hash string
	// Files are the folder's contents, kept so a world can go on playing the
	// mod exactly as it was reset with.
	Files   map[string]string
	values  values
	pirates *pirateScript
	regions *regionScript
}

// Ref is what a world records about m.
func (m *Mod) Ref() game.ModRef {
	return game.ModRef{Name: m.Name, Version: m.Version, Label: m.Label, Hash: m.Hash, Files: m.Files}
}

// values is values.toml. Each unit's section is read into a plain map and
// checked against unitFields, so the keys a file may use are named only there.
type values struct {
	Units map[string]map[string]any `toml:"units"`
	HQ    map[string]any            `toml:"hq"`
	// Regions is one section per region type (regions.go).
	Regions map[string]map[string]any `toml:"regions"`
}

// number takes a TOML integer or float, so `upkeep = 1` and `upkeep = 0.4`
// both read.
func number(v any) (float64, error) {
	switch n := v.(type) {
	case int64:
		return float64(n), nil
	case float64:
		return n, nil
	}
	return 0, fmt.Errorf("want a number, got %v", v)
}

// fixed reads v as a count of 1/scale units, refusing a value with more
// decimal places than scale holds rather than rounding it away unseen.
func fixed(v any, scale int64) (int64, error) {
	f, err := number(v)
	if err != nil {
		return 0, err
	}
	x := f * float64(scale)
	if math.IsNaN(x) || math.Abs(x) > math.MaxInt32 {
		return 0, fmt.Errorf("%v is out of range", f)
	}
	r := math.Round(x)
	if math.Abs(x-r) > 1e-6 {
		return 0, fmt.Errorf("%v has more decimal places than %s allows", f, game.FixedPoint(1, scale))
	}
	return int64(r), nil
}

// whole reads v as an int, which is 32 bits on the door builds. The real
// limits are game.Rules.Validate's; this only keeps the conversion from
// wrapping before they are checked.
func whole(v any) (int, error) {
	n, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("want a whole number, got %v", v)
	}
	if n < math.MinInt32 || n > math.MaxInt32 {
		return 0, fmt.Errorf("%d is out of range", n)
	}
	return int(n), nil
}

// Entry is one folder in the mods directory. Mod is nil when the folder does
// not read as a mod, and Err says why.
type Entry struct {
	Name string
	Mod  *Mod
	Err  error
}

// Available lists the folders in dataDir's mods directory, by name, for the
// settings editor. A missing mods directory is an empty list, not an error.
func Available(dataDir string) ([]Entry, error) {
	dir := filepath.Join(dataDir, Dir)
	des, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, de := range des {
		if !de.IsDir() {
			continue
		}
		m, err := Read(filepath.Join(dir, de.Name()))
		out = append(out, Entry{Name: de.Name(), Mod: m, Err: err})
	}
	return out, nil
}

// Read reads the mod folder at path.
func Read(path string) (*Mod, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(path, e.Name()))
		if err != nil {
			return nil, err
		}
		files[e.Name()] = string(data)
	}
	return parse(path, files)
}

// parse reads a mod from its files, keyed by name. path is the folder they
// came from, or would have, for a copy saved in a world: its last element is
// the name the manifest must carry.
func parse(path string, files map[string]string) (*Mod, error) {
	m := &Mod{Path: path, Files: files}
	names := slices.Sorted(maps.Keys(files)) // so the hash is stable
	hash := sha256.New()
	var haveManifest bool
	for _, name := range names {
		data := []byte(files[name])
		if name != ManifestFile && name != ValuesFile && name != PiratesFile && name != RegionsFile {
			return nil, fmt.Errorf("%s: %s is not a file this version of the game uses in a mod; a mod holds %s and, optionally, %s, %s and %s", path, name, ManifestFile, ValuesFile, PiratesFile, RegionsFile)
		}
		fmt.Fprintf(hash, "%s\x00%d\x00", name, len(data))
		hash.Write(data)
		file := filepath.Join(path, name)
		switch name {
		case ManifestFile:
			haveManifest = true
			if err := decodeStrict(data, &m.Manifest); err != nil {
				return nil, fmt.Errorf("%s: %w", file, err)
			}
		case ValuesFile:
			if err := decodeStrict(data, &m.values); err != nil {
				return nil, fmt.Errorf("%s: %w", file, err)
			}
		}
	}
	if !haveManifest {
		return nil, fmt.Errorf("%s: no %s", path, ManifestFile)
	}
	if m.Name != filepath.Base(path) {
		return nil, fmt.Errorf("%s: name %q must match the folder's name, %q", filepath.Join(path, ManifestFile), m.Name, filepath.Base(path))
	}
	if !validName.MatchString(m.Name) {
		return nil, fmt.Errorf("%s: name %q may hold only lowercase letters, digits, - and _", path, m.Name)
	}
	if m.Version == "" {
		return nil, fmt.Errorf("%s: version is required", filepath.Join(path, ManifestFile))
	}
	// The label and version are printed on every player's Game Setup screen,
	// where a control character would reach the terminal as a command.
	for key, v := range map[string]string{"label": m.Label, "version": m.Version, "description": m.Description} {
		if strings.ContainsFunc(v, unicode.IsControl) {
			return nil, fmt.Errorf("%s: %s may not hold control characters", filepath.Join(path, ManifestFile), key)
		}
	}
	m.Hash = hex.EncodeToString(hash.Sum(nil)[:8])
	if err := checkScriptAPI(path, m.ScriptAPI, files); err != nil {
		return nil, err
	}
	if src, ok := files[PiratesFile]; ok {
		s, err := newPirateScript(path, m.Name, m.Hash, src)
		if err != nil {
			return nil, err
		}
		m.pirates = s
	}
	if src, ok := files[RegionsFile]; ok {
		s, err := newRegionScript(path, m.Name, m.Hash, src)
		if err != nil {
			return nil, err
		}
		m.regions = s
	}
	return m, nil
}

// checkScriptAPI holds a mod with a script to the script interface it
// declares, before any script is compiled.
func checkScriptAPI(path string, api int, files map[string]string) error {
	_, pirates := files[PiratesFile]
	_, regions := files[RegionsFile]
	file := filepath.Join(path, ManifestFile)
	switch {
	case api < 0:
		return fmt.Errorf("%s: script_api %d is not a version", file, api)
	case api == 0 && (pirates || regions):
		return fmt.Errorf("%s: a mod with a script must say which script interface it was written for; this version of the game runs script_api = %d", file, ScriptAPI)
	case api > ScriptAPI:
		return fmt.Errorf("%s: written for script_api %d, but this version of the game runs script_api %d; it needs a newer version of the game", file, api, ScriptAPI)
	}
	return nil
}

// decodeStrict decodes TOML into v and fails on any key v has no field for.
func decodeStrict(data []byte, v any) error {
	md, err := toml.NewDecoder(bytes.NewReader(data)).Decode(v)
	if err != nil {
		return err
	}
	if extra := md.Undecoded(); len(extra) > 0 {
		keys := make([]string, len(extra))
		for i, k := range extra {
			keys[i] = k.String()
		}
		return fmt.Errorf("unknown key %s", strings.Join(keys, ", "))
	}
	return nil
}

// moddableUnits are the goods a values file may name, by ID.
func moddableUnit(id string) *game.Good {
	for _, g := range game.MilitaryGoods {
		if g.ID == id {
			return g
		}
	}
	return nil
}

// Apply writes m's overrides into r. It checks only that each value converts;
// whether the result is playable is r.Validate's question, asked once every
// mod has been applied.
func (m *Mod) Apply(r *game.Rules) error {
	var errs []error
	ids := slices.Sorted(maps.Keys(m.values.Units)) // errors in a stable order
	for _, id := range ids {
		g := moddableUnit(id)
		if g == nil {
			var known []string
			for _, g := range game.MilitaryGoods {
				known = append(known, g.ID)
			}
			errs = append(errs, fmt.Errorf("units.%s: not a unit a mod can change (%s)", id, strings.Join(known, ", ")))
			continue
		}
		u := r.Units[g]
		keys := slices.Sorted(maps.Keys(m.values.Units[id]))
		for _, key := range keys {
			f := unitField(key)
			if f == nil {
				errs = append(errs, fmt.Errorf("units.%s.%s: unknown key", id, key))
				continue
			}
			if err := f.set(&u, m.values.Units[id][key]); err != nil {
				errs = append(errs, fmt.Errorf("units.%s.%s: %w", id, key, err))
			}
		}
		r.Units[g] = u
	}
	if err := m.applyHQ(r); err != nil {
		errs = append(errs, err)
	}
	if err := m.applyRegions(r); err != nil {
		errs = append(errs, err)
	}
	if m.pirates != nil {
		r.Pirates = m.pirates
	}
	if m.regions != nil {
		r.RegionBrains = append(r.RegionBrains, m.regions)
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("%s: %w", filepath.Join(m.Path, ValuesFile), err)
	}
	return nil
}

// Check reads the mod at path and applies it alone to the base rules, which is
// what -check-mod runs: the same reading and the same limits a reset uses, but
// for this mod only. It returns the rules the mod would play.
func Check(path string) (*Mod, *game.Rules, error) {
	m, err := Read(path)
	if err != nil {
		return nil, nil, err
	}
	r := game.BaseRules()
	if err := m.Apply(r); err != nil {
		return nil, nil, err
	}
	if err := r.Validate(); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, r, nil
}

// Resolve reads the named mods from dataDir and applies them, in order, to the
// base rules. It returns the rules and what a world records about each mod.
func Resolve(dataDir string, names []string) (*game.Rules, []game.ModRef, error) {
	if len(names) == 0 {
		return nil, nil, nil
	}
	r := game.BaseRules()
	refs := make([]game.ModRef, 0, len(names))
	for i, name := range names {
		if err := checkListed(name, names[:i]); err != nil {
			return nil, nil, fmt.Errorf("mod %w", err)
		}
		m, err := Read(filepath.Join(dataDir, Dir, name))
		if err != nil {
			return nil, nil, err
		}
		if err := m.Apply(r); err != nil {
			return nil, nil, err
		}
		refs = append(refs, m.Ref())
	}
	if err := r.Validate(); err != nil {
		return nil, nil, fmt.Errorf("mods %s: %w", strings.Join(names, ", "), err)
	}
	return r, refs, nil
}

// checkListed is the check a mod's name in a list passes before it becomes a
// path: a plain name, not listed earlier in the same list.
func checkListed(name string, earlier []string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("%q: a mod's name may hold only lowercase letters, digits, - and _", name)
	}
	if slices.Contains(earlier, name) {
		return fmt.Errorf("%q is listed twice", name)
	}
	return nil
}

// Restore rebuilds the rules a world was reset with from the copies of its
// mods saved in the world. A game's rules change only at a reset, so a mod
// folder edited or removed since then does not reach the game in progress: each
// such mod is named in a warning, and the saved copy is played until the next
// reset (#202).
func Restore(dataDir string, recorded []game.ModRef) (rules *game.Rules, warnings []string, err error) {
	if len(recorded) == 0 {
		return nil, nil, nil
	}
	r := game.BaseRules()
	var names []string
	for _, ref := range recorded {
		// The name comes from world.json, which a reset wrote but anyone with
		// the file can edit; held to a plain name it cannot leave the mods folder,
		// and checked as a reset checks the list.
		if err := checkListed(ref.Name, names); err != nil {
			return nil, nil, fmt.Errorf("this game's saved mod %w", err)
		}
		names = append(names, ref.Name)
		path := filepath.Join(dataDir, Dir, ref.Name)
		m, err := parse(path, ref.Files)
		if err != nil {
			return nil, nil, fmt.Errorf("the copy of mod %q saved with this game: %w", ref.Name, err)
		}
		if err := m.Apply(r); err != nil {
			return nil, nil, fmt.Errorf("the copy of mod %q saved with this game: %w", ref.Name, err)
		}
		switch disk, err := Read(path); {
		case err != nil:
			warnings = append(warnings, fmt.Sprintf("mod %q could not be read (%v); this game keeps the values it was reset with until the next reset", ref.Name, err))
		case disk.Hash != ref.Hash:
			warnings = append(warnings, fmt.Sprintf("mod %q has changed since this game was reset; this game keeps the mod's old values until the next reset", ref.Name))
		}
	}
	// A newer version of the game may have tightened a limit the saved values
	// met when the game was reset. That is an error, not a warning: the limits
	// are what keep the arithmetic safe.
	if err := r.Validate(); err != nil {
		return nil, nil, fmt.Errorf("this game's mods %s: %w", strings.Join(names, ", "), err)
	}
	return r, warnings, nil
}
