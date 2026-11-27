package mod

import (
	"slices"
	"strings"
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/andy5995/immortal-barons/internal/game"
)

// fieldNames are the sorted names a reference table lists.
func fieldNames(fs []scriptField) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.names...)
	}
	slices.Sort(out)
	return out
}

func attrNames(t *testing.T, v starlark.Value) []string {
	t.Helper()
	out := slices.Clone(v.(starlark.HasAttrs).AttrNames())
	slices.Sort(out)
	return out
}

// Every field a script is handed must be in the reference's tables, and the
// tables must not list a field the game does not hand over.
func TestScriptFieldTablesMatchWhatScriptsAreGiven(t *testing.T) {
	units := game.PirateHoldings{}
	for _, s := range game.PirateSpoils() {
		if s.Good() != nil {
			units[s] = 1
		}
	}
	view := game.PirateView{
		Realm:    game.PirateRealmView{Name: "r", Units: units, Market: units},
		Factions: []game.PirateFactionView{{Name: "f", Loot: units}},
	}
	check := func(what string, got []string, table []scriptField) {
		t.Helper()
		if want := fieldNames(table); !slices.Equal(got, want) {
			t.Errorf("%s: the script is given %v, the reference lists %v", what, got, want)
		}
	}

	check("raid realm", attrNames(t, realmValue(view, nil)), pirateRealmFields())
	market := realmValue(view, nil).(*starlarkstruct.Struct)
	m, _ := market.Attr("market")
	if got, want := attrNames(t, m), spoilNames(); !slices.Equal(got, sortedCopy(want)) {
		t.Errorf("raid realm.market: the script is given %v, the reference says %v", got, want)
	}
	f := factionsValue(view).(*starlark.List).Index(0)
	check("raid faction", attrNames(t, f), pirateFactionFields())

	check("region realm", attrNames(t, regionRealmValue(game.RegionView{}, nil)), regionRealmFields())
	rv := regionRealmValue(game.RegionView{}, nil).(*starlarkstruct.Struct)
	regs, _ := rv.Attr("regions")
	if got, want := attrNames(t, regs), sortedCopy(regionIDs()); !slices.Equal(got, want) {
		t.Errorf("realm.regions: the script is given %v, the reference says %v", got, want)
	}
	check("region row", attrNames(t, regionRowValue(game.RegionRules{})), regionRowFields())
}

func sortedCopy(s []string) []string {
	s = slices.Clone(s)
	slices.Sort(s)
	return s
}

// The built-in functions table must list exactly the names a script sees.
func TestBuiltinTableMatchesWhatScriptsSee(t *testing.T) {
	names := func(kind string) []string {
		var out []string
		for _, b := range builtins {
			if strings.Contains(b.kinds, kind) {
				out = append(out, b.sig[:strings.IndexByte(b.sig, '(')])
			}
		}
		slices.Sort(out)
		return out
	}
	var pirates []string
	for k := range predeclaredFor(&optionValues{}) {
		pirates = append(pirates, k)
	}
	slices.Sort(pirates)
	if got := names("pirates"); !slices.Equal(got, pirates) {
		t.Errorf("pirate scripts see %v, the reference lists %v", pirates, got)
	}
	// compileRegion adds option() to regionPredeclared, which has nothing else.
	var regions []string
	for k := range regionPredeclared {
		regions = append(regions, k)
	}
	regions = append(regions, "option")
	slices.Sort(regions)
	if got := names("regions"); !slices.Equal(got, regions) {
		t.Errorf("region scripts see %v, the reference lists %v", regions, got)
	}
}

// The raid dict's keys the reference lists are the ones the reader accepts.
func TestRaidKeysInTheReferenceAreTheOnesReadOrdersAccepts(t *testing.T) {
	text := Reference()
	start := strings.Index(text, "Each raid is a dict:")
	table := text[start : start+strings.Index(text[start:], "### Region script")]
	var keys []string
	for _, line := range strings.Split(table, "\n") {
		if strings.HasPrefix(line, "| `") {
			keys = append(keys, strings.Split(line, "`")[1])
		}
	}
	if want := []string{"faction", "take", "market", "share", "land"}; !slices.Equal(keys, want) {
		t.Fatalf("the reference lists %v, want %v", keys, want)
	}
	values := map[string]starlark.Value{
		"faction": starlark.MakeInt(0), "take": starlark.String("jets"),
		"market": starlark.False, "share": starlark.MakeInt(1), "land": starlark.MakeInt(1),
	}
	d := starlark.NewDict(len(keys))
	for _, k := range keys {
		d.SetKey(starlark.String(k), values[k])
	}
	if _, err := readOrders(starlark.NewList([]starlark.Value{d})); err != nil {
		t.Errorf("readOrders refused a raid with every documented key: %v", err)
	}
}
