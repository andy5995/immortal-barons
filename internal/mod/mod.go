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
// (script.go), and one regions/<type>.star per region type it changes, which
// decides what that type yields and costs for each realm (region_script.go).
package mod

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
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
	"strconv"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"

	"github.com/andy5995/immortal-barons/internal/game"
)

// Dir is the folder under the data directory that holds the mods.
const Dir = "mods"

// The files a mod folder may hold, besides PiratesFile and the RegionsDir folder. values.toml is
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
	// License is the mod's license, as an SPDX identifier (GPL-3.0-or-later,
	// CC-BY-4.0). It is required, so a sysop can tell whether they may share a
	// mod before passing it on.
	License string `toml:"license"`
	// ModAPI is the version of the mod format the mod was written for. Every
	// mod must give it: see ModAPI.
	ModAPI int `toml:"mod_api"`
	// Options are the [options.<id>] tables a sysop may set at a reset. A mod
	// without options leaves this empty.
	Options map[string]*Option `toml:"options"`
}

// ModAPI is the version of the mod format this game reads: the folder's
// layout, the keys values.toml may set and what each one means, and what a
// script is given and may return. A change that could make a mod that worked
// before fail to load, or load and play differently, raises the number, so
// the game refuses an out-of-date mod by name before the game starts. Adding
// a key or a field does not raise it. A mod gives the version it was written
// for as mod_api in mod.toml.
const ModAPI = 1

// Mod is one mod folder, read and hashed.
type Mod struct {
	Manifest
	Path string
	// Hash covers every file in the folder, so a world can tell that a mod it
	// was reset with has since been edited.
	Hash string
	// Files are the folder's contents, kept so a world can go on playing the
	// mod exactly as it was reset with.
	Files map[string]string
	// chosen is every option's value for the game being loaded: the default
	// for one the sysop did not set. Scripts compile against these values, so
	// they stay fixed for the whole game.
	chosen  map[string]int64
	values  values
	pirates *pirateScript
	regions map[*game.Region]*regionScript
}

// Ref is what a world records about m.
func (m *Mod) Ref() game.ModRef {
	return game.ModRef{
		Name: m.Name, Version: m.Version, Label: m.Label,
		Hash: m.Hash, Files: m.Files, Options: maps.Clone(m.chosen),
	}
}

// ManifestFromRef reads just the manifest from a mod saved in a world. A
// screen showing what a game plays needs the option labels but not its scripts.
func ManifestFromRef(ref game.ModRef) (*Manifest, error) {
	data, ok := ref.Files[ManifestFile]
	if !ok {
		return nil, fmt.Errorf("%s: no %s", ref.Name, ManifestFile)
	}
	var man Manifest
	if err := decodeStrict([]byte(data), &man); err != nil {
		return nil, fmt.Errorf("%s/%s: %w", ref.Name, ManifestFile, err)
	}
	return &man, nil
}

// values is values.toml. Each unit's section is read into a plain map and
// checked against unitFields, so the keys a file may use are named only there.
type values struct {
	Units map[string]map[string]any `toml:"units"`
	HQ    map[string]any            `toml:"hq"`
	// Combat is the one [combat] table: what the Attack Rewards and Attack
	// Damage levels mean (combat.go).
	Combat map[string]any `toml:"combat"`
	// Regions is one section per region type (regions.go).
	Regions map[string]map[string]any `toml:"regions"`
	// Attacks holds the [attacks.<kind>.<side>] tables, as nested tables
	// whose depth depends on the kind (attacks.go).
	Attacks map[string]any `toml:"attacks"`
	// When holds sections selected by a bool option. Its shape is this same
	// struct, so [when.fast_start.units.tank] is written just like its
	// top-level counterpart.
	When map[string]values `toml:"when"`
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

// Read reads the mod folder at path, and its RegionsDir folder if it has one.
// A file in RegionsDir is keyed "regions/<name>", with a forward slash on
// every system.
func Read(path string) (*Mod, error) {
	return ReadWithOptions(path, nil)
}

// ReadWithOptions reads a mod with one game's chosen option values, for the
// settings editors and reports that show what that game would play.
func ReadWithOptions(path string, options map[string]int64) (*Mod, error) {
	files := map[string]string{}
	if err := readFiles(path, "", files); err != nil {
		return nil, err
	}
	return parseWithOptions(path, files, options, filepath.Join(path, ValuesFile))
}

// readFiles reads dir's files into files, each keyed prefix + its name. Only
// the mod folder itself may hold a folder, and only RegionsDir: parse names
// anything else as a mistake.
func readFiles(dir, prefix string, files map[string]string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if prefix != "" || e.Name() != RegionsDir {
				return fmt.Errorf("%s: a folder in a mod may only be %s, holding a script for each region type it changes", full, RegionsDir)
			}
			if err := readFiles(full, RegionsDir+"/", files); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return err
		}
		files[prefix+e.Name()] = string(data)
	}
	return nil
}

// parse reads a mod from its files with its options' defaults. It is what
// -check-mod and the editors use when they only look at a mod; the game calls
// parseWithOptions with the values the sysop chose at the reset.
func parse(path string, files map[string]string) (*Mod, error) {
	return parseWithOptions(path, files, nil, filepath.Join(path, ValuesFile))
}

// parseWithOptions reads a mod from its files, keyed by name. path is the
// folder they came from, or would have, for a copy saved in a world: its last
// element is the name the manifest must carry. given holds the values chosen
// for this game; source names where they came from in an error (config.json,
// world.json, or the league's reset order).
func parseWithOptions(path string, files map[string]string, given map[string]int64, source string) (*Mod, error) {
	m := &Mod{Path: path, Files: files}
	names := slices.Sorted(maps.Keys(files))
	var haveManifest bool
	for _, name := range names {
		data := []byte(files[name])
		if name != ManifestFile && name != ValuesFile && name != PiratesFile && regionScriptType(name) == nil && !slices.Contains(LicenseFiles, name) {
			if strings.HasPrefix(name, RegionsDir+"/") {
				return nil, fmt.Errorf("%s: %s is not a region script; %s holds a <type>.star for each region type the mod changes (%s)", path, name, RegionsDir, strings.Join(regionIDs(), ", "))
			}
			return nil, fmt.Errorf("%s: %s is not a file this version of the game uses in a mod; a mod holds %s and, optionally, %s, %s, a %s folder and a license file (%s)", path, name, ManifestFile, ValuesFile, PiratesFile, RegionsDir, strings.Join(LicenseFiles, ", "))
		}
		file := filepath.Join(path, name)
		switch name {
		case ManifestFile:
			haveManifest = true
			if err := decodeStrict(data, &m.Manifest); err != nil {
				return nil, fmt.Errorf("%s: %w", file, err)
			}
		case ValuesFile:
			if err := decodeValues(data, &m.values); err != nil {
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
	if strings.TrimSpace(m.License) == "" {
		return nil, fmt.Errorf("%s: license is required, as an SPDX identifier such as GPL-3.0-or-later or CC-BY-4.0", filepath.Join(path, ManifestFile))
	}
	// The label and version are printed on every player's Game Setup screen,
	// where a control character would reach the terminal as a command.
	for key, v := range map[string]string{"label": m.Label, "version": m.Version, "description": m.Description, "license": m.License} {
		if strings.ContainsFunc(v, unicode.IsControl) {
			return nil, fmt.Errorf("%s: %s may not hold control characters", filepath.Join(path, ManifestFile), key)
		}
	}
	m.Hash = HashFiles(files)
	if err := checkModAPI(path, m.ModAPI); err != nil {
		return nil, err
	}
	if err := checkOptions(m.Manifest.Options, filepath.Join(path, ManifestFile)); err != nil {
		return nil, err
	}
	var err error
	if m.chosen, err = chosenValues(m.Manifest.Options, given, source); err != nil {
		return nil, err
	}
	if src, ok := files[PiratesFile]; ok {
		s, err := newPirateScript(path, m.Name, m.Hash, src, m.scriptOptions())
		if err != nil {
			return nil, err
		}
		m.pirates = s
	}
	for _, g := range game.RegionTypes {
		src, ok := files[regionScriptFile(g)]
		if !ok {
			continue
		}
		s, err := newRegionScript(path, m.Name, m.Hash, g, src, m.scriptOptions())
		if err != nil {
			return nil, err
		}
		if m.regions == nil {
			m.regions = map[*game.Region]*regionScript{}
		}
		m.regions[g] = s
	}
	return m, nil
}

// HashFiles is a mod's hash: its files' names and contents, in name order so
// it is stable.
func HashFiles(files map[string]string) string {
	hash := sha256.New()
	for _, name := range slices.Sorted(maps.Keys(files)) {
		fmt.Fprintf(hash, "%s\x00%d\x00", name, len(files[name]))
		hash.Write([]byte(files[name]))
	}
	return hex.EncodeToString(hash.Sum(nil)[:8])
}

// LicenseFiles are the names a mod's license text may have. A license file is
// hashed and carried with the rest of the mod, so it goes wherever the mod
// does, a league's boards included.
var LicenseFiles = []string{"LICENSE", "LICENSE.txt", "LICENSE.md", "COPYING"}

// The starting files -init-mod copies. {{MOD_NAME}} and {{MOD_API}} in
// template/mod.toml are filled when a mod is created.
var (
	//go:embed template/mod.toml
	templateManifest []byte
	//go:embed template/values.toml
	templateValues []byte
)

// DefaultLicense is the license Init writes into a new mod's mod.toml, for its
// author to change.
const DefaultLicense = "MIT"

// Init creates a new, empty mod in dir: a mod.toml with the folder's name, a
// first version, this game's ModAPI and DefaultLicense, and a values.toml that
// changes nothing yet. It refuses a dir that already exists, or whose name
// config.json could not list.
func Init(dir string) error {
	name := filepath.Base(dir)
	if !validName.MatchString(name) {
		return fmt.Errorf("%q: a mod's name may hold only lowercase letters, digits, - and _", name)
	}
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s already exists", dir)
	}
	manifest := strings.NewReplacer("{{MOD_NAME}}", name, "{{MOD_API}}", strconv.Itoa(ModAPI)).
		Replace(string(templateManifest))
	if strings.Contains(manifest, "{{") {
		return fmt.Errorf("the mod template has a placeholder Init does not fill")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(manifest), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ValuesFile), templateValues, 0o644)
}

// checkModAPI holds a mod to the mod format it says it was written for,
// before any of its values are applied or its scripts compiled.
func checkModAPI(path string, api int) error {
	file := filepath.Join(path, ManifestFile)
	switch {
	case api == 0:
		return fmt.Errorf("%s: mod_api is required; this version of the game reads mod_api = %d", file, ModAPI)
	case api < 0:
		return fmt.Errorf("%s: mod_api %d is not a version", file, api)
	case api > ModAPI:
		return fmt.Errorf("%s: written for mod_api %d, but this version of the game reads mod_api %d; it needs a newer version of the game", file, api, ModAPI)
	case api < ModAPI:
		return fmt.Errorf("%s: written for mod_api %d, but this version of the game reads mod_api %d; the mod needs updating (the ChangeLog says what changed)", file, api, ModAPI)
	}
	return nil
}

// decodeStrict decodes TOML into v and fails on any key v has no field for.
func decodeStrict(data []byte, v any) error { return decodeTOML(data, v, false) }

// decodeValues reads values.toml. BurntSushi reports the nested keys of an
// inline table decoded into any, so option = "id" would look like an unknown
// key; optionValue checks that table itself and refuses anything else in it.
func decodeValues(data []byte, v any) error { return decodeTOML(data, v, true) }

func decodeTOML(data []byte, v any, allowOption bool) error {
	md, err := toml.NewDecoder(bytes.NewReader(data)).Decode(v)
	if err != nil {
		return err
	}
	var keys []string
	for _, k := range md.Undecoded() {
		name := k.String()
		// The decoder reports every key under an interface-typed table as
		// undecoded. attacks.go reads these tables in a values file and names
		// its own errors; anywhere else they are unknown keys.
		if allowOption && isAttackKey(k) {
			continue
		}
		if allowOption && isOptionReference(k) {
			continue
		}
		keys = append(keys, name)
	}
	if len(keys) > 0 {
		return fmt.Errorf("unknown key %s", strings.Join(keys, ", "))
	}
	return nil
}

// isAttackKey reports whether k lies under [attacks] or [when.<id>.attacks].
func isAttackKey(k toml.Key) bool {
	return len(k) > 1 && (k[0] == "attacks" || (k[0] == "when" && len(k) > 2 && k[2] == "attacks"))
}

// isOptionReference reports whether an undecoded key is the option of a
// { option = "id" } inline table standing in for a figure: units.<id>.<key>,
// hq.<key>, combat.<key> or regions.<id>.<key>, each also under when.<bool>. Any other key
// named option is a stray, and stays unknown.
func isOptionReference(k toml.Key) bool {
	if len(k) < 3 || k[len(k)-1] != "option" {
		return false
	}
	if k[0] == "when" {
		k = k[2:]
	}
	switch {
	case len(k) == 4 && (k[0] == "units" || k[0] == "regions"):
		return true
	case len(k) == 3 && (k[0] == "hq" || k[0] == "combat"):
		return true
	}
	return false
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
// mod has been applied. Option references are replaced before the figures go
// through the same conversion and limit checks as a figure written directly.
func (m *Mod) Apply(r *game.Rules) error {
	plain, err := m.resolveValues(m.values)
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Join(m.Path, ValuesFile), err)
	}
	errs := plain.apply(r, m)
	errs = append(errs, m.applyWhen(r)...)
	if m.pirates != nil {
		r.Pirates = m.pirates
	}
	// A later mod's script for a type replaces an earlier one's, as a later
	// mod's figure in values.toml does.
	for g, s := range m.regions {
		if r.RegionBrains == nil {
			r.RegionBrains = map[*game.Region]game.RegionBrain{}
		}
		r.RegionBrains[g] = s
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
	return CheckWithOptions(path, nil)
}

// CheckWithOptions reads the mod at path with one game's chosen options, which
// lets -check-mod show what config.json would play rather than only defaults.
func CheckWithOptions(path string, options map[string]int64) (*Mod, *game.Rules, error) {
	m, err := ReadWithOptions(path, options)
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

// modLoader is one way of loading a list of mods. Every caller checks the names,
// applies each mod in order and validates the combined result; its own checks
// and wording live in source and the three error helpers.
type modLoader struct {
	// source reads one mod. A caller's extra checks belong here: the league
	// reader refuses scripts before parse compiles one, and Restore compares a
	// saved copy with the folder it came from.
	source func(name string, options map[string]int64) (*Mod, error)
	// options supplies the values chosen for one mod. A loader reading a
	// saved game takes them from the world; one applying config.json takes
	// them from there. A nil map means "use the defaults".
	options     func(name string) map[string]int64
	listErr     func(error) error
	applyErr    func(name string, err error) error
	validateErr func(names []string, err error) error
}

// load applies the mods in names, in order, to a fresh copy of the base rules.
func (l modLoader) load(names []string) (*game.Rules, error) {
	r := game.BaseRules()
	var seen []string
	for _, name := range names {
		if err := checkListed(name, seen); err != nil {
			return nil, l.listErr(err)
		}
		seen = append(seen, name)
		var chosen map[string]int64
		if l.options != nil {
			chosen = l.options(name)
		}
		m, err := l.source(name, chosen)
		if err != nil {
			return nil, err
		}
		if err := m.Apply(r); err != nil {
			return nil, l.applyErr(name, err)
		}
	}
	if err := r.Validate(); err != nil {
		return nil, l.validateErr(seen, err)
	}
	return r, nil
}

// modRefs indexes refs by the name a list names them by. A list with a name
// twice is refused by load before its source can look one up.
func modRefs(refs []game.ModRef) map[string]game.ModRef {
	byName := make(map[string]game.ModRef, len(refs))
	for _, ref := range refs {
		byName[ref.Name] = ref
	}
	return byName
}

// Resolve reads the named mods from dataDir and applies them, in order, to the
// base rules. options is config.json's ModOptions: mod name, option id, value.
// It returns the rules and what a world records about each mod, options filled
// in with defaults where the sysop made no choice.
func Resolve(dataDir string, names []string, options map[string]map[string]int64) (*game.Rules, []game.ModRef, error) {
	if len(names) == 0 {
		return nil, nil, nil
	}
	refs := make([]game.ModRef, 0, len(names))
	l := modLoader{
		options: func(name string) map[string]int64 { return options[name] },
		source: func(name string, given map[string]int64) (*Mod, error) {
			files := map[string]string{}
			if err := readFiles(filepath.Join(dataDir, Dir, name), "", files); err != nil {
				return nil, err
			}
			m, err := parseWithOptions(filepath.Join(dataDir, Dir, name), files, given, "config.json")
			if err != nil {
				return nil, err
			}
			refs = append(refs, m.Ref())
			return m, nil
		},
		listErr:  func(err error) error { return fmt.Errorf("mod %w", err) },
		applyErr: func(_ string, err error) error { return err },
		validateErr: func(names []string, err error) error {
			return fmt.Errorf("mods %s: %w", strings.Join(names, ", "), err)
		},
	}
	r, err := l.load(names)
	if err != nil {
		return nil, nil, err
	}
	return r, refs, nil
}

// Replacement is a script in a mod list that never runs: mod By carries a
// script File that replaces Over's.
type Replacement struct{ File, By, Over string }

// Replaced is each script in the mods names lists that a later mod's script of
// the same name replaces. It is for the editors that build the list, which
// word it in the reader's language; a list Resolve refuses gives nothing here.
func Replaced(dataDir string, names []string) []Replacement {
	owner := map[string]string{}
	var out []Replacement
	for i, name := range names {
		if checkListed(name, names[:i]) != nil {
			return nil
		}
		m, err := Read(filepath.Join(dataDir, Dir, name))
		if err != nil {
			return nil
		}
		for _, file := range slices.Sorted(maps.Keys(m.Files)) {
			if file != PiratesFile && regionScriptType(file) == nil {
				continue
			}
			if prev, ok := owner[file]; ok {
				out = append(out, Replacement{File: file, By: name, Over: prev})
			}
			owner[file] = name
		}
	}
	return out
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

// LoadForLeague rebuilds the rules a league reset's mods play by, from the
// files the Coordinator's order carries (game.LoadLeagueMods). Each copy is
// checked as a stand-alone reset checks a mod folder, in the order listed, and
// the result against the limits. A mod with a script is refused unless scripts
// is set: that is this board's AcceptLeagueScripts, its sysop's choice.
func LoadForLeague(refs []game.ModRef, scripts bool) (*game.Rules, error) {
	byName := modRefs(refs)
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, ref.Name)
	}
	l := modLoader{
		options: func(name string) map[string]int64 { return byName[name].Options },
		source: func(name string, given map[string]int64) (*Mod, error) {
			ref := byName[name]
			// Before parse, which compiles each script, and compiling runs its top
			// level: a board that does not accept scripts must run none of it.
			if !scripts {
				for file := range ref.Files {
					if file == PiratesFile || strings.HasPrefix(file, RegionsDir+"/") {
						return nil, fmt.Errorf("the league's mod %q carries scripts, and this board does not accept them; set AcceptLeagueScripts Yes in bbs.cfg to play it", ref.Name)
					}
				}
			}
			m, err := parseWithOptions(ref.Name, ref.Files, given, "the league's reset order")
			if err != nil {
				return nil, fmt.Errorf("the league's mod %q: %w", ref.Name, err)
			}
			if m.Hash != ref.Hash {
				return nil, fmt.Errorf("the league's mod %q does not match the hash the Coordinator sent", ref.Name)
			}
			return m, nil
		},
		listErr:  func(err error) error { return fmt.Errorf("the league's mod %w", err) },
		applyErr: func(name string, err error) error { return fmt.Errorf("the league's mod %q: %w", name, err) },
		validateErr: func(names []string, err error) error {
			return fmt.Errorf("the league's mods %s: %w", strings.Join(names, ", "), err)
		},
	}
	return l.load(names)
}

// Restore rebuilds the rules a world was reset with from the copies of its
// mods saved in the world. A game's rules change only at a reset, so a mod
// folder edited or removed since then does not reach the game in progress: each
// such mod is named in a warning, and the saved copy is played until the next
// reset (#202).
//
// folders says whether to compare each saved copy with its folder in dataDir.
// A league board has no folders: its mods came from the Coordinator.
func Restore(dataDir string, recorded []game.ModRef, folders bool) (rules *game.Rules, warnings []string, err error) {
	if len(recorded) == 0 {
		return nil, nil, nil
	}
	byName := modRefs(recorded)
	names := make([]string, 0, len(recorded))
	for _, ref := range recorded {
		names = append(names, ref.Name)
	}
	l := modLoader{
		options: func(name string) map[string]int64 { return byName[name].Options },
		source: func(name string, given map[string]int64) (*Mod, error) {
			ref := byName[name]
			// The name comes from world.json, which a reset wrote but anyone with
			// the file can edit; load holds it to a plain name so it cannot leave
			// the mods folder, and checks it as a reset checks the list.
			path := filepath.Join(dataDir, Dir, ref.Name)
			m, err := parseWithOptions(path, ref.Files, given, "world.json")
			if err != nil {
				return nil, fmt.Errorf("the copy of mod %q saved with this game: %w", ref.Name, err)
			}
			// The copy is in world.json, which anyone with the file can edit. It is
			// played as it stands, but not silently.
			if m.Hash != ref.Hash {
				warnings = append(warnings, fmt.Sprintf("the copy of mod %q saved with this game has been edited since the reset; this game plays it as it now stands", ref.Name))
			}
			// Only the hash is wanted from the folder, so its files are read and not
			// parsed: a script there is compiled at the next reset, not here.
			if folders {
				disk := map[string]string{}
				switch err := readFiles(path, "", disk); {
				case err != nil:
					warnings = append(warnings, fmt.Sprintf("mod %q could not be read (%v); this game keeps the values it was reset with until the next reset", ref.Name, err))
				case HashFiles(disk) != ref.Hash:
					warnings = append(warnings, fmt.Sprintf("mod %q has changed since this game was reset; this game keeps the mod's old values until the next reset", ref.Name))
				}
			}
			return m, nil
		},
		listErr: func(err error) error { return fmt.Errorf("this game's saved mod %w", err) },
		applyErr: func(name string, err error) error {
			return fmt.Errorf("the copy of mod %q saved with this game: %w", name, err)
		},
		// A newer version of the game may have tightened a limit the saved
		// values met at the reset. That is an error, not a warning: the limits
		// are what keep the arithmetic safe.
		validateErr: func(names []string, err error) error {
			return fmt.Errorf("this game's mods %s: %w", strings.Join(names, ", "), err)
		},
	}
	rules, err = l.load(names)
	if err != nil {
		return nil, nil, err
	}
	return rules, warnings, nil
}
