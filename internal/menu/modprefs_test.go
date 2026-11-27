package menu

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/mod"
)

const prefsModManifest = `name = "pirates"
version = "1.0"
mod_api = 1
license = "CC0-1.0"

[prefs.fierce]
label = "Fiercer pirate raids"
description = "Pirates raid you every turn, whatever the odds, and take your jets."
default = false

[prefs.quiet]
label = "Quiet reports"
default = true
`

const prefsModScript = `def raid(realm, factions):
    if realm.pref("fierce"):
        return [{"faction": 1, "take": "jets"}]
    return []
`

// prefsModWorld is a menu world playing a mod that declares two preferences.
func prefsModWorld(t *testing.T) *ctx {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "mods", "pirates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{mod.ManifestFile: prefsModManifest, mod.PiratesFile: prefsModScript} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, rules, err := mod.Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := newWorld()
	w.World.Rules, w.World.Mods = rules, []game.ModRef{m.Ref()}
	p := w.Player()
	p.Protection = 0
	p.Jets, p.Gold = 1_000_000, 9_000_000
	return w
}

// A player toggles a mod's preference on the Preferences menu, and the mod's
// pirates script then raids differently for that realm alone.
func TestModPreferenceToggleChangesWhatTheScriptOrders(t *testing.T) {
	menus := BuildMenus()
	w := prefsModWorld(t)
	p := w.Player()
	w.World.PlayTurn(p, "2026-11-23")
	if len(p.PirateHits) != 0 {
		t.Fatalf("raided before any choice: %+v", p.PirateHits)
	}

	f := &fakeSession{keys: []rune("P8")} // P = Preferences, 8 = Fiercer pirate raids
	if err := Run(f, w, menus.System); err != io.EOF {
		t.Fatalf("expected EOF after script, got %v", err)
	}
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "(8) Fiercer pirate raids") || !strings.Contains(out, "(9) Quiet reports") {
		t.Fatalf("the Preferences screen was not reached or lists no mod rows:\n%s", out)
	}
	if got, ok := p.ModPrefs["pirates/fierce"]; !ok || !got {
		t.Fatalf("ModPrefs %v, want pirates/fierce set", p.ModPrefs)
	}
	if _, ok := p.ModPrefs["pirates/quiet"]; ok {
		t.Error("a preference the player never touched was stored")
	}
	w.World.PlayTurn(p, "2026-11-23")
	if len(p.PirateHits) == 0 {
		t.Error("the script did not see the toggled preference")
	}
	if !p.ModPref("pirates", "quiet", true) {
		t.Error("an untouched preference lost its default")
	}
}

// Mod rows follow the built-in ones and share their Yes/No column; the
// description is wrapped inside the box.
func TestModPreferenceRowsShareTheColumnAndWrapTheirDescription(t *testing.T) {
	menus := BuildMenus()
	w := prefsModWorld(t)
	f := &fakeSession{}
	draw(f, w, menus.Prefs)
	out := stripANSI(f.out.String())
	t.Log("\n" + out)
	ends := map[int]bool{}
	for _, key := range []string{"(7) Auto-Feed Empire", "(8) Fiercer pirate raids", "(9) Quiet reports"} {
		found := false
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, key) {
				found = true
				ends[len([]rune(strings.TrimRight(line, " ")))] = true
			}
		}
		if !found {
			t.Errorf("missing row %q", key)
		}
	}
	if len(ends) != 1 {
		t.Errorf("built-in and mod rows end at %d different columns, want one", len(ends))
	}
	if strings.Index(out, "(8)") > strings.Index(out, "(L)") {
		t.Error("mod rows come after the Language row, want them after the built-in toggles")
	}
	for _, line := range strings.Split(out, "\n") {
		if n := len([]rune(strings.TrimRight(line, " "))); n > 79 {
			t.Errorf("line of %d columns: %q", n, line)
		}
	}
	if !strings.Contains(out, "(8) Pirates raid you every turn") {
		t.Error("the description is not shown")
	}
}

// A mod label is the mod's own text: it is not looked up in the catalogs, and
// only the Yes/No words translate.
func TestModPreferenceLabelIsNotTranslatedButYesNoIs(t *testing.T) {
	menus := BuildMenus()
	w := prefsModWorld(t)
	w.Player().Language = "de"
	f := &fakeSession{}
	draw(f, w, menus.Prefs)
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Fiercer pirate raids") {
		t.Error("the mod's label was altered")
	}
	if !strings.Contains(out, i18n.T("de", "No")) {
		t.Error("the No word did not translate")
	}
}

// Without a mod the Preferences menu is exactly what it was.
func TestPreferencesWithoutModsHasNoModRows(t *testing.T) {
	menus := BuildMenus()
	w := newWorld()
	f := &fakeSession{}
	draw(f, w, menus.Prefs)
	if strings.Contains(stripANSI(f.out.String()), "(8)") {
		t.Error("a row 8 appeared with no mod in play")
	}
}

func TestModPreferenceDescriptionWithAnUnbrokenWordFitsTheScreen(t *testing.T) {
	menus := BuildMenus()
	w := prefsModWorld(t)
	w.World.Mods[0].Files[mod.ManifestFile] = strings.Replace(w.World.Mods[0].Files[mod.ManifestFile],
		"Pirates raid you every turn", strings.Repeat("x", 140), 1)
	f := &fakeSession{}
	draw(f, w, menus.Prefs)
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "(8) xxx") {
		t.Fatalf("description not shown:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if n := len([]rune(strings.TrimRight(line, " "))); n > 79 {
			t.Errorf("line of %d columns: %q", n, line)
		}
	}
}
