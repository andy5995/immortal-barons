package game

import (
	"slices"
	"strings"
)

// Mod preferences are yes/no choices a mod declares in its mod.toml and each
// player makes for their own realm; the mod's scripts read them. They live on
// the realm, never in a packet, and a realm that has made no choice plays the
// mod's default, so only a player's explicit answer is stored.

// ModPrefKey is the key a mod's preference is stored under on a realm.
func ModPrefKey(mod, id string) string { return mod + "/" + id }

// ModPref is the realm's answer for the preference id of mod, or def when the
// player has made none.
func (e *Empire) ModPref(mod, id string, def bool) bool {
	if v, ok := e.ModPrefs[ModPrefKey(mod, id)]; ok {
		return v
	}
	return def
}

// SetModPref records the player's choice.
func (e *Empire) SetModPref(mod, id string, v bool) {
	if e.ModPrefs == nil {
		e.ModPrefs = map[string]bool{}
	}
	e.ModPrefs[ModPrefKey(mod, id)] = v
}

// modPrefsText is the realm's recorded choices as one comparable string, the
// form the script views carry (a RegionView is a cache key, so it cannot hold
// a map). Sorted, so equal choices give equal text.
func (e *Empire) modPrefsText() string {
	if len(e.ModPrefs) == 0 {
		return ""
	}
	lines := make([]string, 0, len(e.ModPrefs))
	for k, v := range e.ModPrefs {
		if v {
			lines = append(lines, k+"=1")
		} else {
			lines = append(lines, k+"=0")
		}
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

// ParseModPrefs reads a view's ModPrefs text back into the realm's choices.
func ParseModPrefs(text string) map[string]bool {
	if text == "" {
		return nil
	}
	out := map[string]bool{}
	for line := range strings.SplitSeq(text, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			out[k] = v == "1"
		}
	}
	return out
}
