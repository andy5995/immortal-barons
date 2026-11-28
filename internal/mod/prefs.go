package mod

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"go.starlark.net/starlark"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A mod may declare up to MaxPrefs yes/no preferences in [prefs.<id>] tables.
// Each player sets them on their own Preferences menu and the mod's scripts
// read them per realm, so a preference changes what a script does for that
// player, not a world-wide rule. Their labels are drawn in one menu column, so
// they are kept short.
const (
	MaxPrefs     = 8
	MaxPrefLabel = 40
	// MaxPrefDescription keeps a description to a few lines on the menu.
	MaxPrefDescription = 200
	// MaxWorldPrefs is how many preferences all the mods in a game may declare
	// together: one for each of PrefMenuKeys. A list of mods that declares more
	// is refused at load, so none is unreachable.
	MaxWorldPrefs = len(PrefMenuKeys)
)

// PrefMenuKeys are the keys the mods' preferences take on the Preferences
// menu, after its built-in rows 1 to 7; L, T, N and 0 are that menu's own. It
// lives here rather than in the menu so the cap above is derived from it.
const PrefMenuKeys = "89ABCDEFGHIJKMOPQRSUVWXYZ"

// Pref is one [prefs.<id>] table in mod.toml. Default is a pointer to tell a
// missing default from a written false.
type Pref struct {
	Label       string `toml:"label"`
	Description string `toml:"description"`
	Default     *bool  `toml:"default"`
}

// check refuses a preference a menu or a script could not use safely.
func (p *Pref) check(id, file string) error {
	where := fmt.Sprintf("%s: prefs.%s", file, id)
	if !validName.MatchString(id) {
		return fmt.Errorf("%s: %q may hold only lowercase letters, digits, - and _", where, id)
	}
	if strings.TrimSpace(p.Label) == "" {
		return fmt.Errorf("%s: label is required", where)
	}
	if n := utf8.RuneCountInString(p.Label); n > MaxPrefLabel {
		return fmt.Errorf("%s: label is %d characters; at most %d fit the Preferences menu", where, n, MaxPrefLabel)
	}
	if n := utf8.RuneCountInString(p.Description); n > MaxPrefDescription {
		return fmt.Errorf("%s: description is %d characters; at most %d", where, n, MaxPrefDescription)
	}
	for key, v := range map[string]string{"label": p.Label, "description": p.Description} {
		if hasUnsafe(v) {
			return fmt.Errorf("%s: %s may not hold control characters", where, key)
		}
	}
	if p.Default == nil {
		return fmt.Errorf("%s: default is required, true or false", where)
	}
	return nil
}

// checkPrefs validates every declared preference.
func checkPrefs(prefs map[string]*Pref, file string) error {
	if len(prefs) > MaxPrefs {
		return fmt.Errorf("%s: %d prefs declared; a mod may have at most %d", file, len(prefs), MaxPrefs)
	}
	for _, id := range slices.Sorted(maps.Keys(prefs)) {
		if prefs[id] == nil {
			return fmt.Errorf("%s: prefs.%s is empty", file, id)
		}
		if err := prefs[id].check(id, file); err != nil {
			return err
		}
	}
	return nil
}

// checkWorldPrefs refuses a list of mods whose preferences together outnumber
// the menu's keys, naming each mod's share.
func checkWorldPrefs(mods []*Mod) error {
	total := 0
	var parts []string
	for _, m := range mods {
		if n := len(m.Manifest.Prefs); n > 0 {
			total += n
			parts = append(parts, fmt.Sprintf("%s %d", m.Name, n))
		}
	}
	if total > MaxWorldPrefs {
		return fmt.Errorf("these mods declare %d player preferences together (%s); the Preferences menu has room for %d", total, strings.Join(parts, ", "), MaxWorldPrefs)
	}
	return nil
}

// SortedPrefIDs is the preference ids in a stable order, for screens and reports.
func (m *Manifest) SortedPrefIDs() []string { return slices.Sorted(maps.Keys(m.Prefs)) }

// PrefLine describes one preference as -check-mod prints it.
func (m *Mod) PrefLine(id string) string {
	p := m.Manifest.Prefs[id]
	return fmt.Sprintf("%s: default %s", p.Label, boolWord(btoi(*p.Default)))
}

func btoi(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// PrefRow is one preference of a mod in play, as the Preferences menu shows it.
type PrefRow struct {
	Mod, ID     string
	Label       string // already cleaned for display
	Description string
	Default     bool
}

// PrefRows are the preferences the mods recorded in a world declare, in mod
// order and then id order. A mod whose manifest does not read is skipped: the
// menu has no way to tell the player why, and the sysop sees the same fault
// where the mod loads.
func PrefRows(refs []game.ModRef) []PrefRow {
	var out []PrefRow
	for _, ref := range refs {
		man := cachedManifest(ref)
		if man == nil {
			continue
		}
		for _, id := range man.SortedPrefIDs() {
			p := man.Prefs[id]
			out = append(out, PrefRow{
				Mod: ref.Name, ID: id, Label: clean(p.Label),
				Description: clean(p.Description), Default: *p.Default,
			})
		}
	}
	return out
}

// manifests remembers each manifest read for a screen. A menu asks several
// times while it draws, and the manifest text is the key, so an edited mod
// is read again.
var manifests sync.Map

func cachedManifest(ref game.ModRef) *Manifest {
	text, ok := ref.Files[ManifestFile]
	if !ok {
		return nil
	}
	key := ref.Name + "\x00" + text
	if m, ok := manifests.Load(key); ok {
		return m.(*Manifest)
	}
	man, err := ManifestFromRef(ref)
	if err != nil {
		man = nil
	}
	manifests.Store(key, man)
	return man
}

// prefValues is what a script of one mod may ask about: that mod's
// declarations only, so a script cannot read another mod's preferences.
type prefValues struct {
	mod  string
	defs map[string]*Pref
}

func (m *Mod) scriptPrefs() *prefValues {
	return &prefValues{mod: m.Name, defs: m.Manifest.Prefs}
}

// builtin is the realm's pref(id): the realm's answer when it made one, else
// the mod's default. chosen is the view's recorded preferences.
func (v *prefValues) builtin(chosen string) starlark.Value {
	var explicit map[string]bool
	return starlark.NewBuiltin("pref", func(th *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var id string
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &id); err != nil {
			return nil, err
		}
		var p *Pref
		if v != nil {
			p = v.defs[id]
		}
		if p == nil {
			return nil, fmt.Errorf("pref %q does not exist in this mod", id)
		}
		if explicit == nil {
			explicit = game.ParseModPrefs(chosen)
		}
		if got, ok := explicit[game.ModPrefKey(v.mod, id)]; ok {
			return starlark.Bool(got), nil
		}
		return starlark.Bool(*p.Default), nil
	})
}
