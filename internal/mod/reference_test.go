package mod

import (
	"flag"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

var updateReference = flag.Bool("update", false, "rewrite docs/mod-reference.md from the code")

const referencePath = "../../docs/mod-reference.md"

func TestModReferenceIsCurrent(t *testing.T) {
	want := Reference()
	if *updateReference {
		if err := os.WriteFile(referencePath, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(referencePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatal("docs/mod-reference.md is out of date; rewrite it with: go test ./internal/mod -run TestModReferenceIsCurrent -update")
	}
}

// Every key a mod may set must say what it means: the reference prints it.
func TestEveryModKeyHasADescription(t *testing.T) {
	for _, f := range unitFields {
		if f.desc == "" || f.limits == "" {
			t.Errorf("units.%s: description or limits empty", f.key)
		}
	}
	for _, f := range game.HQFields {
		if f.Desc == "" {
			t.Errorf("hq.%s: description empty", f.Key)
		}
	}
	for _, f := range game.RegionFields {
		if f.Desc == "" {
			t.Errorf("regions.<id>.%s: description empty", f.Key)
		}
	}
	for _, f := range game.AttackFields {
		if f.Desc == "" {
			t.Errorf("attacks.<kind>.<side>.%s: description empty", f.Key)
		}
	}
	for _, r := range slices.Concat(manifestKeys, optionKeys, valuesSections) {
		if r.key == "" || r.desc == "" {
			t.Errorf("reference row %q: description empty", r.key)
		}
	}
}

// tomlKeys are the toml tag names of struct type t, in field order.
func tomlKeys(t reflect.Type) []string {
	var out []string
	for i := range t.NumField() {
		if k := t.Field(i).Tag.Get("toml"); k != "" {
			out = append(out, k)
		}
	}
	return out
}

func docKeyNames(rows []docKey) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.key)
	}
	return out
}

func TestReferenceKeysMatchTheStructsTheReaderDecodes(t *testing.T) {
	for _, c := range []struct {
		name string
		typ  reflect.Type
		rows []docKey
	}{
		{"mod.toml", reflect.TypeFor[Manifest](), manifestKeys},
		{"options", reflect.TypeFor[Option](), optionKeys},
		{"values.toml", reflect.TypeFor[values](), valuesSections},
	} {
		got, want := docKeyNames(c.rows), tomlKeys(c.typ)
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s: reference lists %v, the reader decodes %v", c.name, got, want)
		}
	}
}
